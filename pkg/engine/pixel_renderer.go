package engine

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"

	"nightmare/pkg/config"
)

// PixelRenderer implements a pixelated rendering style using OpenGL
type PixelRenderer struct {
	config    config.RendererConfig
	width     int
	height    int
	pixelSize int // Size of each rendered pixel

	// OpenGL resources
	shaderProgram uint32
	quadVAO       uint32
	quadVBO       uint32

	// Framebuffers for pixelation effect
	fbo           uint32
	rbo           uint32
	pixelTexture  uint32
	screenTexture uint32

	// Sprite sheet textures for different object types
	spriteSheets     map[string]uint32
	spriteSheetInfos map[string]*SpriteInfo

	// Post-processing effects
	effectsShader   uint32
	glitchAmount    float32
	glitchDuration  float32
	glitchStartTime time.Time
	vignetteAmount  float32
	noiseAmount     float32
	usePostProcess  bool

	// Shader uniforms for effects
	timeLocation       int32
	glitchLocation     int32
	vignetteLocation   int32
	noiseLocation      int32
	resolutionLocation int32

	// Rendering state
	lastRenderTime time.Time

	// Color palette for rendering
	paletteTexture uint32
	baseFgColor    [3]float32
	baseColorDark  [3]float32
	baseColorLight [3]float32

	// Thread safety
	mutex sync.Mutex
}

// SpriteInfo holds information about a sprite sheet
type SpriteInfo struct {
	Width      int  // Width of a single sprite
	Height     int  // Height of a single sprite
	Columns    int  // Number of sprites horizontally
	Rows       int  // Number of sprites vertically
	FrameCount int  // Total number of frames
	IsAnimated bool // Whether the sprite is animated
	FrameRate  int  // Frames per second for animation
}

// NewPixelRenderer creates a new pixelated renderer
func NewPixelRenderer(config config.RendererConfig) (*PixelRenderer, error) {
	renderer := &PixelRenderer{
		config:           config,
		width:            config.Width,
		height:           config.Height,
		pixelSize:        4, // Default size of each pixel block
		spriteSheets:     make(map[string]uint32),
		spriteSheetInfos: make(map[string]*SpriteInfo),
		glitchAmount:     0.0,
		glitchDuration:   0.0,
		vignetteAmount:   0.4,  // Default vignette intensity
		noiseAmount:      0.03, // Default noise intensity
		usePostProcess:   true, // Enable post-processing by default
		lastRenderTime:   time.Now(),
		baseFgColor:      [3]float32{0.7, 0.85, 0.7}, // Default foreground color (pale green)
		baseColorDark:    [3]float32{0.1, 0.1, 0.1},  // Dark color for palette
		baseColorLight:   [3]float32{0.9, 1.0, 0.9},  // Light color for palette
	}

	// Initialize OpenGL
	if err := renderer.initOpenGL(); err != nil {
		return nil, err
	}

	// Create sprite sheets
	if err := renderer.createSpriteSheets(); err != nil {
		return nil, err
	}

	// Create color palette
	if err := renderer.createColorPalette(); err != nil {
		return nil, err
	}

	// Initialize framebuffers for pixelation effect
	if err := renderer.setupFramebuffers(); err != nil {
		return nil, err
	}

	return renderer, nil
}

// initOpenGL initializes OpenGL resources
func (r *PixelRenderer) initOpenGL() error {
	// Initialize basic GL settings
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LEQUAL)
	gl.ClearColor(0.0, 0.0, 0.0, 1.0)

	// Create shader programs
	var err error

	// Basic shader for sprite rendering
	if r.shaderProgram, err = r.createShaderProgram(pixelVertexShaderSource, pixelFragmentShaderSource); err != nil {
		return err
	}

	// Post-processing shader
	if r.effectsShader, err = r.createShaderProgram(postProcessVertexShader, postProcessFragmentShaderSource); err != nil {
		return err
	}

	// Get uniform locations for the post-processing shader
	gl.UseProgram(r.effectsShader)
	r.timeLocation = gl.GetUniformLocation(r.effectsShader, gl.Str("time\x00"))
	r.glitchLocation = gl.GetUniformLocation(r.effectsShader, gl.Str("glitchAmount\x00"))
	r.vignetteLocation = gl.GetUniformLocation(r.effectsShader, gl.Str("vignetteAmount\x00"))
	r.noiseLocation = gl.GetUniformLocation(r.effectsShader, gl.Str("noiseAmount\x00"))
	r.resolutionLocation = gl.GetUniformLocation(r.effectsShader, gl.Str("resolution\x00"))

	// Create quad for rendering
	r.setupQuad()

	return nil
}

// setupQuad creates a quad for rendering sprites and post-processing
func (r *PixelRenderer) setupQuad() {
	vertices := []float32{
		// Positions        // Texture coords
		-1.0, -1.0, 0.0, 0.0, 1.0,
		1.0, -1.0, 0.0, 1.0, 1.0,
		1.0, 1.0, 0.0, 1.0, 0.0,
		-1.0, 1.0, 0.0, 0.0, 0.0,
	}

	// Create VAO and VBO
	gl.GenVertexArrays(1, &r.quadVAO)
	gl.GenBuffers(1, &r.quadVBO)

	// Bind and set data
	gl.BindVertexArray(r.quadVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.quadVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	// Position attribute
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 5*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)

	// Texture coordinate attribute
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 5*4, gl.PtrOffset(3*4))
	gl.EnableVertexAttribArray(1)

	// Unbind
	gl.BindVertexArray(0)
}

// setupFramebuffers initializes framebuffers for pixelation effect
func (r *PixelRenderer) setupFramebuffers() error {
	// Calculate the actual render resolution (divide window resolution by pixel size)
	renderWidth := r.width / r.pixelSize
	renderHeight := r.height / r.pixelSize

	// Generate framebuffer for low-resolution rendering
	gl.GenFramebuffers(1, &r.fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)

	// Create low-resolution texture
	gl.GenTextures(1, &r.pixelTexture)
	gl.BindTexture(gl.TEXTURE_2D, r.pixelTexture)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(renderWidth), int32(renderHeight), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, r.pixelTexture, 0)

	// Create renderbuffer for depth and stencil
	gl.GenRenderbuffers(1, &r.rbo)
	gl.BindRenderbuffer(gl.RENDERBUFFER, r.rbo)
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH24_STENCIL8, int32(renderWidth), int32(renderHeight))
	gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_STENCIL_ATTACHMENT, gl.RENDERBUFFER, r.rbo)

	// Check if framebuffer is complete
	if gl.CheckFramebufferStatus(gl.FRAMEBUFFER) != gl.FRAMEBUFFER_COMPLETE {
		return fmt.Errorf("framebuffer not complete")
	}

	// Create texture for post-processing
	gl.GenTextures(1, &r.screenTexture)
	gl.BindTexture(gl.TEXTURE_2D, r.screenTexture)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(r.width), int32(r.height), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)

	// Reset to default framebuffer
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)

	return nil
}

// createSpriteSheets generates sprite sheets for different object types
func (r *PixelRenderer) createSpriteSheets() error {
	// Create sprites for each object type
	objectTypes := []struct {
		name      string
		generator func() image.Image
		info      *SpriteInfo
	}{
		{
			"tree",
			r.generateTreeSprites,
			&SpriteInfo{Width: 32, Height: 32, Columns: 4, Rows: 4, FrameCount: 16, IsAnimated: true, FrameRate: 5},
		},
		{
			"rock",
			r.generateRockSprites,
			&SpriteInfo{Width: 16, Height: 16, Columns: 4, Rows: 1, FrameCount: 4, IsAnimated: false},
		},
		{
			"strange",
			r.generateStrangeSprites,
			&SpriteInfo{Width: 32, Height: 32, Columns: 4, Rows: 4, FrameCount: 16, IsAnimated: true, FrameRate: 3},
		},
		{
			"terrain",
			r.generateTerrainSprites,
			&SpriteInfo{Width: 16, Height: 16, Columns: 8, Rows: 8, FrameCount: 64, IsAnimated: false},
		},
	}

	// Generate and upload each sprite sheet
	for _, obj := range objectTypes {
		// Generate the sprite sheet
		spriteSheet := obj.generator()

		// Create an OpenGL texture
		var textureID uint32
		gl.GenTextures(1, &textureID)
		gl.BindTexture(gl.TEXTURE_2D, textureID)

		// Set texture parameters
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

		// Convert image to RGBA
		rgba := image.NewRGBA(spriteSheet.Bounds())
		draw.Draw(rgba, rgba.Bounds(), spriteSheet, image.Point{}, draw.Src)

		// Upload to OpenGL
		width, height := spriteSheet.Bounds().Dx(), spriteSheet.Bounds().Dy()
		gl.TexImage2D(
			gl.TEXTURE_2D,
			0,
			gl.RGBA,
			int32(width),
			int32(height),
			0,
			gl.RGBA,
			gl.UNSIGNED_BYTE,
			gl.Ptr(rgba.Pix),
		)

		// Store the texture ID and info
		r.spriteSheets[obj.name] = textureID
		r.spriteSheetInfos[obj.name] = obj.info
	}

	return nil
}

// createColorPalette creates a retro-like color palette texture
func (r *PixelRenderer) createColorPalette() error {
	// Create a 16x16 color palette (256 colors)
	paletteSize := 16
	palette := image.NewRGBA(image.Rect(0, 0, paletteSize, paletteSize))

	// Fill with a retro-like palette
	for y := 0; y < paletteSize; y++ {
		for x := 0; x < paletteSize; x++ {
			// Calculate color based on position
			// This is a simple 16x16 palette with variables in hue, saturation, and value
			r := uint8((x * 16) + (y&3)*4)
			g := uint8((y * 16) + (x&7)*2)
			b := uint8(((x ^ y) * 16) & 0xFF)

			// Add some structure to the palette for better visual results
			if x < 8 {
				// Darker colors in the first half
				r = uint8(float64(r) * 0.7)
				g = uint8(float64(g) * 0.7)
				b = uint8(float64(b) * 0.7)
			}

			// Set the color
			palette.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	// Create a texture for the palette
	gl.GenTextures(1, &r.paletteTexture)
	gl.BindTexture(gl.TEXTURE_2D, r.paletteTexture)

	// Set texture parameters
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	// Upload to OpenGL
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(paletteSize),
		int32(paletteSize),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		gl.Ptr(palette.Pix),
	)

	return nil
}

// Sprite generation functions
func (r *PixelRenderer) generateTreeSprites() image.Image {
	info := r.spriteSheetInfos["tree"]
	width, height := info.Width*info.Columns, info.Height*info.Rows
	spriteSheet := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with transparent color initially
	draw.Draw(spriteSheet, spriteSheet.Bounds(), image.Transparent, image.Point{}, draw.Src)

	// Generate different tree variants
	for i := 0; i < info.FrameCount; i++ {
		// Calculate sprite position
		col, row := i%info.Columns, i/info.Columns
		x0, y0 := col*info.Width, row*info.Height

		// Different tree types
		treeType := i % 4

		// Common colors
		trunkColor := color.RGBA{139, 69, 19, 255} // Brown
		leafColor := color.RGBA{34, 139, 34, 255}  // Forest green

		if treeType == 1 {
			// Dead tree
			trunkColor = color.RGBA{101, 67, 33, 255} // Dark brown
			leafColor = color.RGBA{85, 85, 85, 255}   // Gray
		} else if treeType == 2 {
			// Pine tree
			trunkColor = color.RGBA{160, 82, 45, 255} // Sienna
			leafColor = color.RGBA{0, 100, 0, 255}    // Dark green
		} else if treeType == 3 {
			// Autumn tree
			leafColor = color.RGBA{205, 133, 63, 255} // Peru (orange-brown)
		}

		// Draw tree trunk
		trunkWidth := info.Width / 6
		trunkHeight := info.Height / 2
		trunkRect := image.Rect(
			x0+info.Width/2-trunkWidth/2,
			y0+info.Height/2,
			x0+info.Width/2+trunkWidth/2,
			y0+info.Height/2+trunkHeight,
		)
		draw.Draw(spriteSheet, trunkRect, &image.Uniform{trunkColor}, image.Point{}, draw.Src)

		// Draw tree crown
		// Use a simple algorithm to draw a somewhat circular crown
		crownRadius := info.Width/2 - 2
		crownCenterX, crownCenterY := x0+info.Width/2, y0+info.Height/4

		for cy := -crownRadius; cy <= crownRadius; cy++ {
			for cx := -crownRadius; cx <= crownRadius; cx++ {
				// Check if point is inside the circle
				if cx*cx+cy*cy <= crownRadius*crownRadius {
					px, py := crownCenterX+cx, crownCenterY+cy
					// Add some noise to the edges for a more natural look
					if rand.Float32() > 0.95 && (float32(cx)*float32(cx)+float32(cy)*float32(cy) > float32(crownRadius)*float32(crownRadius)*float32(0.7)) {
						continue
					}
					// Only draw if inside the sprite boundaries
					if px >= x0 && px < x0+info.Width && py >= y0 && py < y0+info.Height {
						spriteSheet.Set(px, py, leafColor)
					}
				}
			}
		}

		// For animated trees, add slight variations in each frame
		if info.IsAnimated && i >= info.Columns {
			// Add some wind effect to later frames
			windOffset := (i / info.Columns) * 1
			for y := y0; y < y0+info.Height/2; y++ {
				for x := x0; x < x0+info.Width; x++ {
					// Get the color from the first frame with some offset
					srcX := x - windOffset
					if srcX >= x0 && srcX < x0+info.Width {
						c := spriteSheet.At(srcX, y)
						if _, _, _, a := c.RGBA(); a > 0 {
							spriteSheet.Set(x, y, c)
						}
					}
				}
			}
		}
	}

	return spriteSheet
}

func (r *PixelRenderer) generateRockSprites() image.Image {
	info := r.spriteSheetInfos["rock"]
	width, height := info.Width*info.Columns, info.Height*info.Rows
	spriteSheet := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with transparent color
	draw.Draw(spriteSheet, spriteSheet.Bounds(), image.Transparent, image.Point{}, draw.Src)

	// Generate different rock variants
	for i := 0; i < info.FrameCount; i++ {
		// Calculate sprite position
		col, row := i%info.Columns, i/info.Columns
		x0, y0 := col*info.Width, row*info.Height

		// Different rock colors and shapes
		var baseColor color.RGBA
		switch i {
		case 0:
			baseColor = color.RGBA{105, 105, 105, 255} // DimGray
		case 1:
			baseColor = color.RGBA{128, 128, 128, 255} // Gray
		case 2:
			baseColor = color.RGBA{169, 169, 169, 255} // DarkGray
		case 3:
			baseColor = color.RGBA{90, 90, 90, 255} // Darker gray
		}

		// Generate a rocky shape
		rockSize := info.Width * 3 / 4
		rockCenterX, rockCenterY := x0+info.Width/2, y0+info.Height/2

		// Create an irregular rock shape
		for cy := -rockSize / 2; cy <= rockSize/2; cy++ {
			for cx := -rockSize / 2; cx <= rockSize/2; cx++ {
				// Create an irregular circular shape
				distortedRadius := float64(rockSize) / 2 * (0.8 + 0.2*math.Sin(float64(cx*cy)/float64(rockSize)))
				dist := math.Sqrt(float64(cx*cx + cy*cy))

				if dist <= float64(distortedRadius) {
					px, py := rockCenterX+cx, rockCenterY+cy

					// Only draw if inside the sprite boundaries
					if px >= x0 && px < x0+info.Width && py >= y0 && py < y0+info.Height {
						// Add some shading based on position for 3D effect
						shade := uint8(200 + int(50*dist/float64(rockSize)))
						if cx < 0 {
							shade = uint8(float64(shade) * 0.8) // Darker on one side
						}

						r := uint8(float64(baseColor.R) * float64(shade) / 255.0)
						g := uint8(float64(baseColor.G) * float64(shade) / 255.0)
						b := uint8(float64(baseColor.B) * float64(shade) / 255.0)

						spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})
					}
				}
			}
		}

		// Add some texture/noise to the rock
		for y := y0; y < y0+info.Height; y++ {
			for x := x0; x < x0+info.Width; x++ {
				c := spriteSheet.At(x, y)
				r, g, b, a := c.RGBA()
				if a > 0 {
					// Add some noise
					noise := rand.Float64()*0.2 - 0.1 // -0.1 to 0.1

					newR := uint8(math.Min(255, math.Max(0, float64(r>>8)*(1.0+noise))))
					newG := uint8(math.Min(255, math.Max(0, float64(g>>8)*(1.0+noise))))
					newB := uint8(math.Min(255, math.Max(0, float64(b>>8)*(1.0+noise))))

					spriteSheet.Set(x, y, color.RGBA{newR, newG, newB, 255})
				}
			}
		}
	}

	return spriteSheet
}

func (r *PixelRenderer) generateStrangeSprites() image.Image {
	info := r.spriteSheetInfos["strange"]
	width, height := info.Width*info.Columns, info.Height*info.Rows
	spriteSheet := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with transparent color
	draw.Draw(spriteSheet, spriteSheet.Bounds(), image.Transparent, image.Point{}, draw.Src)

	// Generate strange, eerie objects
	for i := 0; i < info.FrameCount; i++ {
		// Calculate sprite position
		col, row := i%info.Columns, i/info.Columns
		x0, y0 := col*info.Width, row*info.Height

		// Use different colors for different strange objects
		baseColors := []color.RGBA{
			{139, 0, 139, 255}, // Dark magenta
			{75, 0, 130, 255},  // Indigo
			{85, 0, 0, 255},    // Dark red
			{47, 79, 79, 255},  // Dark slate gray
		}
		baseColor := baseColors[i%len(baseColors)]

		// Generate the strange object
		// Use parametric equations to create unusual shapes
		centerX, centerY := x0+info.Width/2, y0+info.Height/2

		// Different shapes for different frames
		shapeType := i / 4

		switch shapeType {
		case 0:
			// Pulsating blob
			maxRadius := info.Width * 3 / 8
			pulse := 0.8 + 0.2*math.Sin(float64(i)*0.5)

			for angle := 0.0; angle < 2*math.Pi; angle += 0.01 {
				// Distorted circle
				distortion := 0.2 * math.Sin(angle*5+float64(i)*0.1)
				radius := float64(maxRadius) * pulse * (1.0 + distortion)

				px := centerX + int(radius*math.Cos(angle))
				py := centerY + int(radius*math.Sin(angle))

				if px >= x0 && px < x0+info.Width && py >= y0 && py < y0+info.Height {
					// Color varies with angle
					r := uint8(float64(baseColor.R) * (0.8 + 0.2*math.Sin(angle)))
					g := uint8(float64(baseColor.G) * (0.8 + 0.2*math.Sin(angle+2)))
					b := uint8(float64(baseColor.B) * (0.8 + 0.2*math.Sin(angle+4)))

					spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})

					// Fill in the shape
					for r := 0; r < int(radius); r++ {
						fillX := centerX + int(float64(r)*math.Cos(angle))
						fillY := centerY + int(float64(r)*math.Sin(angle))
						if fillX >= x0 && fillX < x0+info.Width && fillY >= y0 && fillY < y0+info.Height {
							// Fade color towards center
							fade := float64(r) / radius
							fr := uint8(float64(r) * fade)
							fg := uint8(float64(g) * fade)
							fb := uint8(float64(b) * fade)
							spriteSheet.Set(fillX, fillY, color.RGBA{fr, fg, fb, 255})
						}
					}
				}
			}

		case 1:
			// Strange obelisk
			height := info.Height * 3 / 4
			width := info.Width / 6

			// Draw the main body
			for y := centerY - height/2; y <= centerY+height/2; y++ {
				for x := centerX - width/2; x <= centerX+width/2; x++ {
					if x >= x0 && x < x0+info.Width && y >= y0 && y < y0+info.Height {
						// Distance from center for shading

						// Darker at the bottom, lighter at the top
						fade := 0.5 + 0.5*float64(y-(centerY-height/2))/float64(height)

						// Add some pattern/markings
						pattern := math.Sin(float64(y-y0)*0.2+float64(i)*0.1) * 0.1

						r := uint8(float64(baseColor.R) * fade * (1.0 + pattern))
						g := uint8(float64(baseColor.G) * fade * (1.0 + pattern))
						b := uint8(float64(baseColor.B) * fade * (1.0 + pattern))

						spriteSheet.Set(x, y, color.RGBA{r, g, b, 255})
					}
				}
			}

			// Add a glowing top
			glowRadius := width
			for y := centerY - height/2 - glowRadius; y <= centerY-height/2; y++ {
				for x := centerX - glowRadius; x <= centerX+glowRadius; x++ {
					dx := float64(x - centerX)
					dy := float64(y - (centerY - height/2))
					dist := math.Sqrt(dx*dx + dy*dy)

					if dist <= float64(glowRadius) && x >= x0 && x < x0+info.Width && y >= y0 && y < y0+info.Height {
						// Glow intensity decreases with distance
						intensity := 1.0 - dist/float64(glowRadius)

						// Pulsing effect
						pulse := 0.7 + 0.3*math.Sin(float64(i)*0.3)
						intensity *= pulse

						// Glow color
						r := uint8(255 * intensity)
						g := uint8(float64(baseColor.G) * intensity)
						b := uint8(255 * intensity)

						spriteSheet.Set(x, y, color.RGBA{r, g, b, 255})
					}
				}
			}

		case 2:
			// Eldritch symbol
			size := info.Width * 3 / 8

			// Draw base circle
			for angle := 0.0; angle < 2*math.Pi; angle += 0.01 {
				radius := float64(size)
				px := centerX + int(radius*math.Cos(angle))
				py := centerY + int(radius*math.Sin(angle))

				if px >= x0 && px < x0+info.Width && py >= y0 && py < y0+info.Height {
					spriteSheet.Set(px, py, baseColor)
				}
			}

			// Draw intersecting lines
			for j := 0; j < 5; j++ {
				angle := float64(j) * math.Pi / 2.5
				startX := centerX + int(float64(size)*math.Cos(angle))
				startY := centerY + int(float64(size)*math.Sin(angle))
				endX := centerX + int(float64(size)*math.Cos(angle+math.Pi))
				endY := centerY + int(float64(size)*math.Sin(angle+math.Pi))

				// Draw line
				DrawLine(spriteSheet, startX, startY, endX, endY, baseColor)
			}

			// Add some runes/symbols
			for j := 0; j < 3; j++ {
				runeAngle := float64(j) * 2 * math.Pi / 3
				runeX := centerX + int(float64(size*2/3)*math.Cos(runeAngle))
				runeY := centerY + int(float64(size*2/3)*math.Sin(runeAngle))

				// Draw a small symbol
				for sy := -3; sy <= 3; sy++ {
					for sx := -3; sx <= 3; sx++ {
						if abs(sx)+abs(sy) <= 4 {
							px, py := runeX+sx, runeY+sy
							if px >= x0 && px < x0+info.Width && py >= y0 && py < y0+info.Height {
								// Different color for runes
								r := uint8(255)
								g := uint8(float64(baseColor.G) * 1.5)
								b := uint8(float64(baseColor.B) * 1.5)
								spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})
							}
						}
					}
				}
			}

			// Add pulsing effect for animation
			if info.IsAnimated {
				pulse := 0.7 + 0.3*math.Sin(float64(i%info.FrameRate)*0.5)
				for y := y0; y < y0+info.Height; y++ {
					for x := x0; x < x0+info.Width; x++ {
						c := spriteSheet.At(x, y)
						r, g, b, a := c.RGBA()
						if a > 0 {
							newR := uint8(float64(r>>8) * pulse)
							newG := uint8(float64(g>>8) * pulse)
							newB := uint8(float64(b>>8) * pulse)
							spriteSheet.Set(x, y, color.RGBA{newR, newG, newB, 255})
						}
					}
				}
			}

		case 3:
			// Floating energy orb
			radius := info.Width / 4

			// Draw the main orb
			for y := centerY - radius; y <= centerY+radius; y++ {
				for x := centerX - radius; x <= centerX+radius; x++ {
					dx := float64(x - centerX)
					dy := float64(y - centerY)
					dist := math.Sqrt(dx*dx + dy*dy)

					if dist <= float64(radius) && x >= x0 && x < x0+info.Width && y >= y0 && y < y0+info.Height {
						// Radial gradient from center
						intensity := 1.0 - dist/float64(radius)

						// Pulsing effect
						pulse := 0.7 + 0.3*math.Sin(float64(i)*0.3)
						intensity *= pulse

						// Orb color
						r := uint8(float64(baseColor.R) * intensity * 1.5)
						g := uint8(float64(baseColor.G) * intensity * 1.5)
						b := uint8(float64(baseColor.B) * intensity * 1.5)

						spriteSheet.Set(x, y, color.RGBA{r, g, b, 255})
					}
				}
			}

			// Add energy tendrils
			for j := 0; j < 8; j++ {
				angle := float64(j) * math.Pi / 4

				// Animate tendrils
				angle += float64(i) * 0.1

				length := float64(radius) * (1.0 + 0.3*math.Sin(float64(i)*0.2))

				// Generate a curvy tendril
				for t := 0.0; t < 1.0; t += 0.05 {
					// Add some waviness
					waveAngle := angle + 0.2*math.Sin(t*10+float64(i)*0.2)

					dist := float64(radius) + t*length
					px := centerX + int(dist*math.Cos(waveAngle))
					py := centerY + int(dist*math.Sin(waveAngle))

					if px >= x0 && px < x0+info.Width && py >= y0 && py < y0+info.Height {
						// Tendril color fades out along length
						intensity := 1.0 - t
						r := uint8(float64(baseColor.R) * intensity * 1.5)
						g := uint8(float64(baseColor.G) * intensity * 1.5)
						b := uint8(float64(baseColor.B) * intensity * 1.5)

						// Draw a small circle for the tendril point
						for sy := -1; sy <= 1; sy++ {
							for sx := -1; sx <= 1; sx++ {
								if sx*sx+sy*sy <= 1 {
									tendrilX, tendrilY := px+sx, py+sy
									if tendrilX >= x0 && tendrilX < x0+info.Width && tendrilY >= y0 && tendrilY < y0+info.Height {
										spriteSheet.Set(tendrilX, tendrilY, color.RGBA{r, g, b, 255})
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return spriteSheet
}

func (r *PixelRenderer) generateTerrainSprites() image.Image {
	info := r.spriteSheetInfos["terrain"]
	width, height := info.Width*info.Columns, info.Height*info.Rows
	spriteSheet := image.NewRGBA(image.Rect(0, 0, width, height))

	// Generate different terrain tiles
	// First 16 tiles (2 rows): regular ground with variations
	// Next 16 tiles (2 rows): rocky/mountain terrain
	// Next 16 tiles (2 rows): water/swamp
	// Last 16 tiles (2 rows): special tiles (paths, etc.)

	// Colors for different terrain types
	groundColors := []color.RGBA{
		{34, 139, 34, 255},  // Forest Green
		{85, 107, 47, 255},  // Dark Olive Green
		{107, 142, 35, 255}, // Olive Drab
		{154, 205, 50, 255}, // Yellow Green
	}

	rockColors := []color.RGBA{
		{105, 105, 105, 255}, // Dim Gray
		{119, 136, 153, 255}, // Light Slate Gray
		{112, 128, 144, 255}, // Slate Gray
		{47, 79, 79, 255},    // Dark Slate Gray
	}

	waterColors := []color.RGBA{
		{70, 130, 180, 255}, // Steel Blue
		{95, 158, 160, 255}, // Cadet Blue
		{0, 128, 128, 255},  // Teal
		{32, 178, 170, 255}, // Light Sea Green
	}

	specialColors := []color.RGBA{
		{205, 133, 63, 255},  // Peru (path)
		{210, 180, 140, 255}, // Tan (sand)
		{139, 69, 19, 255},   // Saddle Brown (dirt)
		{160, 82, 45, 255},   // Sienna (dirt path)
	}

	// Fill all tiles
	for i := 0; i < info.FrameCount; i++ {
		// Calculate sprite position
		col, row := i%info.Columns, i/info.Columns
		x0, y0 := col*info.Width, row*info.Height

		// Select color scheme based on tile type
		var baseColors []color.RGBA
		var tileType string

		if row < 2 {
			baseColors = groundColors
			tileType = "ground"
		} else if row < 4 {
			baseColors = rockColors
			tileType = "rock"
		} else if row < 6 {
			baseColors = waterColors
			tileType = "water"
		} else {
			baseColors = specialColors
			tileType = "special"
		}

		// Base color with variation
		baseColor := baseColors[col%len(baseColors)]

		// Fill the tile with base color
		tileRect := image.Rect(x0, y0, x0+info.Width, y0+info.Height)
		draw.Draw(spriteSheet, tileRect, &image.Uniform{baseColor}, image.Point{}, draw.Src)

		// Add texture/pattern based on tile type
		switch tileType {
		case "ground":
			// Add grass texture
			for py := y0; py < y0+info.Height; py++ {
				for px := x0; px < x0+info.Width; px++ {
					// Add some noise
					if rand.Float64() < 0.2 {
						// Slightly different color for noise
						r := uint8(float64(baseColor.R) * (0.9 + rand.Float64()*0.2))
						g := uint8(float64(baseColor.G) * (0.9 + rand.Float64()*0.2))
						b := uint8(float64(baseColor.B) * (0.9 + rand.Float64()*0.2))
						spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})
					}

					// Add occasional grass tufts
					if rand.Float64() < 0.01 {
						for j := 0; j < 3; j++ {
							if py-j >= y0 && px+j-1 >= x0 && px+j-1 < x0+info.Width {
								// Lighter color for grass tips
								r := uint8(float64(baseColor.R) * 1.2)
								g := uint8(float64(baseColor.G) * 1.2)
								b := uint8(float64(baseColor.B) * 1.2)
								spriteSheet.Set(px+j-1, py-j, color.RGBA{r, g, b, 255})
							}
						}
					}
				}
			}

		case "rock":
			// Add rocky texture
			for py := y0; py < y0+info.Height; py++ {
				for px := x0; px < x0+info.Width; px++ {
					// Perlin-like noise
					noiseVal := (px*17 + py*29) % 100

					if noiseVal < 30 {
						// Darker cracks/crevices
						r := uint8(float64(baseColor.R) * 0.7)
						g := uint8(float64(baseColor.G) * 0.7)
						b := uint8(float64(baseColor.B) * 0.7)
						spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})
					} else if noiseVal > 70 {
						// Lighter areas/highlights
						r := uint8(math.Min(255, float64(baseColor.R)*1.3))
						g := uint8(math.Min(255, float64(baseColor.G)*1.3))
						b := uint8(math.Min(255, float64(baseColor.B)*1.3))
						spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})
					}
				}
			}

		case "water":
			// Add water ripple effect
			for py := y0; py < y0+info.Height; py++ {
				for px := x0; px < x0+info.Width; px++ {
					// Create wave pattern
					waveVal := math.Sin(float64(px-x0)*0.5 + float64(py-y0)*0.5 + float64(col+row)*0.2)

					r := uint8(float64(baseColor.R) * (0.8 + waveVal*0.2))
					g := uint8(float64(baseColor.G) * (0.8 + waveVal*0.2))
					b := uint8(float64(baseColor.B) * (0.8 + waveVal*0.2))

					spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})
				}
			}

			// Add occasional highlights
			for j := 0; j < 5; j++ {
				highlightX := x0 + rand.Intn(info.Width)
				highlightY := y0 + rand.Intn(info.Height)

				for sy := -1; sy <= 1; sy++ {
					for sx := -1; sx <= 1; sx++ {
						if sx*sx+sy*sy <= 1 {
							hx, hy := highlightX+sx, highlightY+sy
							if hx >= x0 && hx < x0+info.Width && hy >= y0 && hy < y0+info.Height {
								// White/light blue highlight
								r := uint8(math.Min(255, float64(baseColor.R)*1.5))
								g := uint8(math.Min(255, float64(baseColor.G)*1.5))
								b := uint8(math.Min(255, float64(baseColor.B)*1.5))
								spriteSheet.Set(hx, hy, color.RGBA{r, g, b, 255})
							}
						}
					}
				}
			}

		case "special":
			// Handle different special tiles
			switch col % 4 {
			case 0: // Path
				// Add path texture with some dirt specs
				for py := y0; py < y0+info.Height; py++ {
					for px := x0; px < x0+info.Width; px++ {
						if rand.Float64() < 0.3 {
							// Darker or lighter speck
							shade := 0.8 + rand.Float64()*0.4
							r := uint8(float64(baseColor.R) * shade)
							g := uint8(float64(baseColor.G) * shade)
							b := uint8(float64(baseColor.B) * shade)
							spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})
						}
					}
				}

				// Add some path edges
				if col == 0 { // Left-right path
					for px := x0; px < x0+info.Width; px++ {
						for j := 0; j < 2; j++ {
							y1, y2 := y0+j, y0+info.Height-1-j
							r := uint8(float64(baseColor.R) * 0.7)
							g := uint8(float64(baseColor.G) * 0.7)
							b := uint8(float64(baseColor.B) * 0.7)
							spriteSheet.Set(px, y1, color.RGBA{r, g, b, 255})
							spriteSheet.Set(px, y2, color.RGBA{r, g, b, 255})
						}
					}
				}

			case 1: // Sand
				// Add sand-like texture
				for py := y0; py < y0+info.Height; py++ {
					for px := x0; px < x0+info.Width; px++ {
						// Small random variations
						if rand.Float64() < 0.4 {
							// Slight color variation
							shade := 0.9 + rand.Float64()*0.2
							r := uint8(float64(baseColor.R) * shade)
							g := uint8(float64(baseColor.G) * shade)
							b := uint8(float64(baseColor.B) * shade)
							spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})
						}
					}
				}

				// Add some ripple patterns
				for py := y0; py < y0+info.Height; py++ {
					for px := x0; px < x0+info.Width; px++ {
						// Simple wave pattern
						if (px+py)%8 == 0 {
							r := uint8(float64(baseColor.R) * 0.9)
							g := uint8(float64(baseColor.G) * 0.9)
							b := uint8(float64(baseColor.B) * 0.9)
							spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})
						}
					}
				}

			case 2, 3: // Dirt variants
				// Add dirt texture
				for py := y0; py < y0+info.Height; py++ {
					for px := x0; px < x0+info.Width; px++ {
						// Perlin-like noise
						noiseVal := (px*13 + py*7) % 100

						if noiseVal < 20 {
							// Darker dirt
							r := uint8(float64(baseColor.R) * 0.8)
							g := uint8(float64(baseColor.G) * 0.8)
							b := uint8(float64(baseColor.B) * 0.8)
							spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})
						} else if noiseVal > 80 {
							// Lighter dirt
							r := uint8(math.Min(255, float64(baseColor.R)*1.1))
							g := uint8(math.Min(255, float64(baseColor.G)*1.1))
							b := uint8(math.Min(255, float64(baseColor.B)*1.1))
							spriteSheet.Set(px, py, color.RGBA{r, g, b, 255})
						}

						// Add occasional small stones
						if rand.Float64() < 0.01 {
							stoneColor := color.RGBA{100, 100, 100, 255}
							spriteSheet.Set(px, py, stoneColor)
						}
					}
				}
			}
		}
	}

	return spriteSheet
}

// Helper function to draw a line
func DrawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx, sy := 1, 1
	if x0 >= x1 {
		sx = -1
	}
	if y0 >= y1 {
		sy = -1
	}
	err := dx - dy

	for {
		if x0 >= 0 && y0 >= 0 && x0 < img.Bounds().Dx() && y0 < img.Bounds().Dy() {
			img.Set(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// Helper function to get absolute value of int
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// createShaderProgram compiles and links a shader program from source
func (r *PixelRenderer) createShaderProgram(vertexSource, fragmentSource string) (uint32, error) {
	// Vertex shader
	vertexShader, err := compileShader(vertexSource, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}

	// Fragment shader
	fragmentShader, err := compileShader(fragmentSource, gl.FRAGMENT_SHADER)
	if err != nil {
		gl.DeleteShader(vertexShader)
		return 0, err
	}

	// Create program and attach shaders
	program := gl.CreateProgram()
	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	// Check for linking errors
	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))

		gl.DeleteProgram(program)
		gl.DeleteShader(vertexShader)
		gl.DeleteShader(fragmentShader)

		return 0, fmt.Errorf("shader program linking failed: %v", log)
	}

	// Detach and delete shaders
	gl.DetachShader(program, vertexShader)
	gl.DetachShader(program, fragmentShader)
	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil
}

// compileShader compiles a shader from source
func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source + "\x00")
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	// Check for compilation errors
	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		gl.DeleteShader(shader)

		return 0, fmt.Errorf("shader compilation failed: %v", log)
	}

	return shader, nil
}

// Render renders the scene
func (r *PixelRenderer) Render(scene *SceneData) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Get window dimensions
	winWidth, winHeight := r.getWindowDimensions()

	// If scene is nil, just clear the screen
	if scene == nil {
		gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
		gl.Viewport(0, 0, int32(winWidth), int32(winHeight))
		gl.ClearColor(0, 0, 0, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		return
	}

	// Calculate the actual render resolution (divide window resolution by pixel size)
	renderWidth := winWidth / r.pixelSize
	renderHeight := winHeight / r.pixelSize

	// Make sure framebuffer is properly sized
	r.resizeFramebufferIfNeeded(winWidth, winHeight)

	// First render pass: Render to low-resolution framebuffer
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)
	gl.Viewport(0, 0, int32(renderWidth), int32(renderHeight))
	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// Render scene objects to low-res framebuffer
	r.renderObjectsToFramebuffer(scene)

	// Second render pass: Apply pixelation and post-processing
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	gl.Viewport(0, 0, int32(winWidth), int32(winHeight))
	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// Render with pixelation and post-processing
	r.renderPostProcessed(winWidth, winHeight)

	// Update glitch effect status
	if r.glitchDuration > 0 {
		elapsed := float32(time.Since(r.glitchStartTime).Seconds())
		if elapsed >= r.glitchDuration {
			r.glitchAmount = 0
			r.glitchDuration = 0
		}
	}

	// Update last render time
	r.lastRenderTime = time.Now()
}

// renderObjectsToFramebuffer renders all objects in the scene to the low-resolution framebuffer
func (r *PixelRenderer) renderObjectsToFramebuffer(scene *SceneData) {
	// Use the basic pixel shader program
	gl.UseProgram(r.shaderProgram)

	// Set viewport for low-res rendering
	renderWidth := r.width / r.pixelSize
	renderHeight := r.height / r.pixelSize

	// Calculate camera matrix (simple view frustum for perspective)
	fov := 60.0 * (math.Pi / 180.0) // Field of view in radians
	aspectRatio := float64(renderWidth) / float64(renderHeight)

	// Sort objects by distance (back to front)
	objectsToDraw := make([]*SceneObject, len(scene.ObjectsInView))
	copy(objectsToDraw, scene.ObjectsInView)
	sortObjectsByDistance(objectsToDraw)

	// First, render terrain/ground
	r.renderTerrain(scene)

	// Then render all other objects
	for _, obj := range objectsToDraw {
		// Skip if object is too far
		if obj.Distance > 100.0 {
			continue
		}

		// Skip if object has zero visibility
		if obj.Visibility <= 0.01 {
			continue
		}

		// Calculate screen position
		screenX, screenY, screenSize := r.worldToScreen(obj.Direction, obj.Size, renderWidth, renderHeight, fov, aspectRatio)

		// Skip if offscreen
		if screenX < -screenSize || screenX > float64(renderWidth)+screenSize ||
			screenY < -screenSize || screenY > float64(renderHeight)+screenSize {
			continue
		}

		// Determine which sprite sheet to use
		var spriteSheet uint32
		var spriteInfo *SpriteInfo

		// Map object type to sprite type
		spriteType := strings.ToLower(obj.Type)
		if spriteType == "tree_trunk" || spriteType == "tree_crown" {
			spriteType = "tree"
		}

		// Get sprite sheet
		if sheet, ok := r.spriteSheets[spriteType]; ok {
			spriteSheet = sheet
			spriteInfo = r.spriteSheetInfos[spriteType]
		} else {
			// Default to a simple colored quad if no sprite sheet
			r.renderSimpleObject(obj, screenX, screenY, screenSize)
			continue
		}

		// Calculate sprite frame
		frameIndex := 0
		if spriteInfo.IsAnimated {
			// Simple frame animation based on current time
			frameRate := 10 // Default frames per second
			if spriteInfo.FrameRate > 0 {
				frameRate = spriteInfo.FrameRate
			}
			frameIndex = (int(time.Now().UnixNano() / int64(1000000000/frameRate))) % spriteInfo.FrameCount
		} else {
			// Use object ID to determine frame for non-animated sprites
			frameIndex = int(obj.ID) % spriteInfo.FrameCount
		}

		// Calculate sprite coordinates in the texture
		spriteCol := frameIndex % spriteInfo.Columns
		spriteRow := frameIndex / spriteInfo.Columns

		// Calculate texture coordinates
		texLeft := float32(spriteCol) / float32(spriteInfo.Columns)
		texRight := float32(spriteCol+1) / float32(spriteInfo.Columns)
		texTop := float32(spriteRow) / float32(spriteInfo.Rows)
		texBottom := float32(spriteRow+1) / float32(spriteInfo.Rows)

		// Determine sprite size based on object distance and size
		spriteWidth := int(screenSize)
		spriteHeight := int(screenSize)

		// Keep aspect ratio for objects
		if obj.Type == "tree" {
			spriteHeight = int(float64(spriteWidth) * float64(1.5)) // Trees are taller than wide
		}

		// Calculate position on screen (centered)
		x0 := int(screenX - float64(spriteWidth)/2)
		y0 := int(screenY - float64(spriteHeight))
		x1 := x0 + spriteWidth
		y1 := y0 + spriteHeight

		// Apply visibility/fog
		alpha := float32(obj.Visibility)

		// Render textured quad
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, spriteSheet)
		gl.Uniform1i(gl.GetUniformLocation(r.shaderProgram, gl.Str("spriteTexture\x00")), 0)

		// Pass visibility factor
		gl.Uniform1f(gl.GetUniformLocation(r.shaderProgram, gl.Str("visibility\x00")), alpha)

		// Set up vertex data for the quad
		vertices := []float32{
			// Positions           // Texture coords
			float32(x0), float32(y1), 0.0, texLeft, texTop, // Top left
			float32(x1), float32(y1), 0.0, texRight, texTop, // Top right
			float32(x1), float32(y0), 0.0, texRight, texBottom, // Bottom right
			float32(x0), float32(y0), 0.0, texLeft, texBottom, // Bottom left
		}

		// Upload vertices
		gl.BindBuffer(gl.ARRAY_BUFFER, r.quadVBO)
		gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STREAM_DRAW)

		// Draw quad
		gl.BindVertexArray(r.quadVAO)
		gl.DrawArrays(gl.TRIANGLE_FAN, 0, 4)
	}
}

// renderTerrain renders the terrain/ground
func (r *PixelRenderer) renderTerrain(scene *SceneData) {
	// Use the basic shader program
	gl.UseProgram(r.shaderProgram)

	// Bind terrain sprite sheet
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, r.spriteSheets["terrain"])
	gl.Uniform1i(gl.GetUniformLocation(r.shaderProgram, gl.Str("spriteTexture\x00")), 0)

	// Full visibility for terrain
	gl.Uniform1f(gl.GetUniformLocation(r.shaderProgram, gl.Str("visibility\x00")), 1.0)

	// Render a grid of terrain tiles
	spriteInfo := r.spriteSheetInfos["terrain"]
	renderWidth := r.width / r.pixelSize
	renderHeight := r.height / r.pixelSize

	// Size of each terrain tile
	tileSize := 32

	// Simplified ground plane rendering
	// Just render a flat grid of tiles
	for y := 0; y < renderHeight; y += tileSize {
		for x := 0; x < renderWidth; x += tileSize {
			// Determine terrain type (simplified)
			// In a full implementation, this would use scene data
			tileCol := (x / tileSize) % 4
			tileRow := (y / tileSize) % 2

			// Calculate texture coordinates
			texLeft := float32(tileCol) / float32(spriteInfo.Columns)
			texRight := float32(tileCol+1) / float32(spriteInfo.Columns)
			texTop := float32(tileRow) / float32(spriteInfo.Rows)
			texBottom := float32(tileRow+1) / float32(spriteInfo.Rows)

			// Set up vertex data for the quad
			vertices := []float32{
				// Positions              // Texture coords
				float32(x), float32(y + tileSize), 0.0, texLeft, texTop, // Top left
				float32(x + tileSize), float32(y + tileSize), 0.0, texRight, texTop, // Top right
				float32(x + tileSize), float32(y), 0.0, texRight, texBottom, // Bottom right
				float32(x), float32(y), 0.0, texLeft, texBottom, // Bottom left
			}

			// Upload vertices
			gl.BindBuffer(gl.ARRAY_BUFFER, r.quadVBO)
			gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STREAM_DRAW)

			// Draw quad
			gl.BindVertexArray(r.quadVAO)
			gl.DrawArrays(gl.TRIANGLE_FAN, 0, 4)
		}
	}
}

// renderSimpleObject renders a simple colored quad for objects without a sprite sheet
func (r *PixelRenderer) renderSimpleObject(obj *SceneObject, x, y, size float64) {
	// Determine color based on object type
	var color [3]float32

	switch strings.ToLower(obj.Type) {
	case "tree", "tree_trunk", "tree_crown":
		color = [3]float32{0.2, 0.8, 0.2} // Green
	case "rock":
		color = [3]float32{0.7, 0.7, 0.7} // Gray
	case "strange":
		color = [3]float32{0.8, 0.2, 0.8} // Purple
	default:
		color = [3]float32{1.0, 1.0, 1.0} // White
	}

	// Apply visibility/fog
	alpha := float32(obj.Visibility)

	// Set color uniform
	gl.Uniform3f(gl.GetUniformLocation(r.shaderProgram, gl.Str("objectColor\x00")), color[0], color[1], color[2])
	gl.Uniform1f(gl.GetUniformLocation(r.shaderProgram, gl.Str("visibility\x00")), alpha)

	// Draw a simple quad
	halfSize := size / 2

	vertices := []float32{
		// Positions                        // Texture coords (unused)
		float32(x - halfSize), float32(y + halfSize), 0.0, 0.0, 0.0,
		float32(x + halfSize), float32(y + halfSize), 0.0, 1.0, 0.0,
		float32(x + halfSize), float32(y - halfSize), 0.0, 1.0, 1.0,
		float32(x - halfSize), float32(y - halfSize), 0.0, 0.0, 1.0,
	}

	// Upload vertices
	gl.BindBuffer(gl.ARRAY_BUFFER, r.quadVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STREAM_DRAW)

	// Draw quad
	gl.BindVertexArray(r.quadVAO)
	gl.DrawArrays(gl.TRIANGLE_FAN, 0, 4)
}

// renderPostProcessed renders the low-resolution framebuffer to the screen with post-processing
func (r *PixelRenderer) renderPostProcessed(winWidth, winHeight int) {
	// Use post-processing shader
	gl.UseProgram(r.effectsShader)

	// Bind the low-resolution texture
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, r.pixelTexture)
	gl.Uniform1i(gl.GetUniformLocation(r.effectsShader, gl.Str("screenTexture\x00")), 0)

	// Set uniforms for effects
	currentTime := float32(time.Since(time.Time{}).Seconds())

	// Update glitch effect (fade out)
	currentGlitch := r.glitchAmount
	if r.glitchDuration > 0 {
		elapsed := float32(time.Since(r.glitchStartTime).Seconds())
		if elapsed < r.glitchDuration {
			// Gradually decrease effect
			fadeOut := 1.0 - (elapsed / r.glitchDuration)
			currentGlitch = r.glitchAmount * fadeOut
		} else {
			currentGlitch = 0.0
		}
	}

	// Set shader uniforms
	gl.Uniform1f(r.timeLocation, currentTime)
	gl.Uniform1f(r.glitchLocation, currentGlitch)
	gl.Uniform1f(r.vignetteLocation, r.vignetteAmount)
	gl.Uniform1f(r.noiseLocation, r.noiseAmount)
	gl.Uniform2f(r.resolutionLocation, float32(winWidth), float32(winHeight))

	// Set pixelation variables
	gl.Uniform1i(gl.GetUniformLocation(r.effectsShader, gl.Str("pixelSize\x00")), int32(r.pixelSize))

	// Draw fullscreen quad
	gl.BindVertexArray(r.quadVAO)
	gl.DrawArrays(gl.TRIANGLE_FAN, 0, 4)
	gl.BindVertexArray(0)
}

// Helper function to get window dimensions
func (r *PixelRenderer) getWindowDimensions() (int, int) {
	a := glfw.GetCurrentContext()
	if a != nil {
		if win := glfw.GetCurrentContext(); win != nil {
			width, height := win.GetSize()
			return width, height
		}
	}
	return r.width, r.height
}

// Helper function to resize the framebuffer if window size changed
func (r *PixelRenderer) resizeFramebufferIfNeeded(width, height int) {
	if r.width != width || r.height != height {
		// Update internal dimensions
		r.width = width
		r.height = height

		// Calculate new render resolution
		renderWidth := width / r.pixelSize
		renderHeight := height / r.pixelSize

		// Resize low-resolution texture
		gl.BindTexture(gl.TEXTURE_2D, r.pixelTexture)
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(renderWidth), int32(renderHeight), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)

		// Resize renderbuffer
		gl.BindRenderbuffer(gl.RENDERBUFFER, r.rbo)
		gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH24_STENCIL8, int32(renderWidth), int32(renderHeight))

		// Resize screen texture
		gl.BindTexture(gl.TEXTURE_2D, r.screenTexture)
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	}
}

// Helper function to convert world direction to screen coordinates
func (r *PixelRenderer) worldToScreen(dir Vector3, size float64, width, height int, fov, aspectRatio float64) (float64, float64, float64) {
	// Simple perspective projection
	// Convert direction to normalized device coordinates
	dirLength := math.Sqrt(dir.X*dir.X + dir.Y*dir.Y + dir.Z*dir.Z)
	if dirLength < 0.001 {
		return 0, 0, 0 // Invalid direction
	}

	normalizedDir := Vector3{
		X: dir.X / dirLength,
		Y: dir.Y / dirLength,
		Z: dir.Z / dirLength,
	}

	// Calculate screen position (perspective projection)
	// Assuming Z is forward, X is right, Y is up
	screenX := normalizedDir.X/(normalizedDir.Z*math.Tan(fov/2))*float64(width)/2 + float64(width)/2
	screenY := -normalizedDir.Y/(normalizedDir.Z*math.Tan(fov/2)/aspectRatio)*float64(height)/2 + float64(height)/2

	// Calculate screen size based on distance (simple perspective)
	screenSize := size / normalizedDir.Z * float64(height) / float64(2)

	return screenX, screenY, screenSize
}

// UpdateResolution updates the resolution of the renderer
func (r *PixelRenderer) UpdateResolution(width, height int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.width == width && r.height == height {
		return
	}

	r.width = width
	r.height = height

	// Resize framebuffers
	r.resizeFramebufferIfNeeded(width, height)
}

// ApplyGlitchEffect applies a glitch visual effect for the specified duration
func (r *PixelRenderer) ApplyGlitchEffect(amount, duration float32) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.glitchAmount = amount
	r.glitchDuration = duration
	r.glitchStartTime = time.Now()
}

// SetVignetteAmount sets the vignette effect intensity
func (r *PixelRenderer) SetVignetteAmount(amount float32) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.vignetteAmount = float32(math.Max(0, math.Min(1, float64(amount))))
}

// SetNoiseAmount sets the noise effect intensity
func (r *PixelRenderer) SetNoiseAmount(amount float32) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.noiseAmount = float32(math.Max(0, math.Min(1, float64(amount))))
}

// TogglePostProcessing enables or disables post-processing effects
func (r *PixelRenderer) TogglePostProcessing() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.usePostProcess = !r.usePostProcess

	// Adjust effect settings based on post-processing state
	if !r.usePostProcess {
		r.noiseAmount = 0.0
		r.vignetteAmount = 0.1 // Minimal vignette
	} else {
		r.noiseAmount = 0.03
		r.vignetteAmount = 0.4
	}
}

// SetPixelSize sets the size of rendered pixels (higher = more pixelated)
func (r *PixelRenderer) SetPixelSize(size int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if size < 1 {
		size = 1
	} else if size > 16 {
		size = 16
	}

	if r.pixelSize != size {
		r.pixelSize = size
		// Force framebuffer resize
		r.resizeFramebufferIfNeeded(r.width, r.height)
	}
}

// Close releases all renderer resources
func (r *PixelRenderer) Close() {
	// Delete OpenGL resources
	gl.DeleteVertexArrays(1, &r.quadVAO)
	gl.DeleteBuffers(1, &r.quadVBO)
	gl.DeleteTextures(1, &r.pixelTexture)
	gl.DeleteTextures(1, &r.screenTexture)
	gl.DeleteTextures(1, &r.paletteTexture)
	gl.DeleteRenderbuffers(1, &r.rbo)
	gl.DeleteFramebuffers(1, &r.fbo)
	gl.DeleteProgram(r.shaderProgram)
	gl.DeleteProgram(r.effectsShader)

	// Delete sprite sheets
	for _, textureID := range r.spriteSheets {
		gl.DeleteTextures(1, &textureID)
	}
}

// Helper function to sort objects by distance
func sortObjectsByDistance(objects []*SceneObject) {
	for i := 0; i < len(objects)-1; i++ {
		for j := i + 1; j < len(objects); j++ {
			if objects[i].Distance < objects[j].Distance {
				objects[i], objects[j] = objects[j], objects[i]
			}
		}
	}
}
