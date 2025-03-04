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

	"nightmare/pkg/config"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.0/glfw"
)

// OpenGLRenderer handles rendering the scene using OpenGL
type OpenGLRenderer struct {
	config        config.RendererConfig
	width         int
	height        int
	vertexArray   uint32
	vertexBuffer  uint32
	elementBuffer uint32
	shaderProgram uint32
	textureID     uint32
	fontTexWidth  int
	fontTexHeight int

	// ASCII texture
	asciiTexture uint32
	asciiCharset []rune
	asciiImage   *image.RGBA
	charWidth    int
	charHeight   int

	// Post-processing effects
	effectsShader uint32
	fbo           uint32
	rbo           uint32
	screenTexture uint32
	quadVAO       uint32
	quadVBO       uint32

	// Effect parameters
	glitchAmount    float32
	glitchDuration  float32
	glitchStartTime time.Time
	vignetteAmount  float32
	noiseAmount     float32
	usePostProcess  bool

	// Shader uniforms
	timeLocation       int32
	glitchLocation     int32
	vignetteLocation   int32
	noiseLocation      int32
	resolutionLocation int32

	// State for dynamic effects
	lastRenderTime time.Time

	// Color palette for rendering
	baseFgColor    [3]float32
	baseColorDark  [3]float32
	baseColorLight [3]float32

	// Thread safety
	mutex sync.Mutex
}

// NewOpenGLRenderer creates a new OpenGL renderer
func NewOpenGLRenderer(config config.RendererConfig) (*OpenGLRenderer, error) {
	renderer := &OpenGLRenderer{
		config:         config,
		width:          config.Width,
		height:         config.Height,
		charWidth:      16,
		charHeight:     16,
		fontTexWidth:   256, // 16x16 grid of ASCII characters
		fontTexHeight:  256,
		glitchAmount:   0.0,
		glitchDuration: 0.0,
		vignetteAmount: 0.4,  // Default vignette intensity
		noiseAmount:    0.03, // Default noise intensity
		usePostProcess: true, // Enable post-processing by default
		lastRenderTime: time.Now(),
		baseFgColor:    [3]float32{0.7, 0.85, 0.7}, // Default foreground color (pale green)
		baseColorDark:  [3]float32{0.1, 0.1, 0.1},  // Dark color for color palette
		baseColorLight: [3]float32{0.9, 1.0, 0.9},  // Light color for color palette
	}

	// ASCII charset from dark to light
	renderer.asciiCharset = []rune{' ', '.', '\'', '`', ',', ':', ';', '"', '-', '+', '=', '*', '#', '%', '@', '$'}

	// Initialize OpenGL
	if err := renderer.initOpenGL(); err != nil {
		return nil, err
	}

	// Create ASCII font texture
	if err := renderer.createASCIITexture(); err != nil {
		return nil, err
	}

	// Initialize framebuffer for post-processing
	if err := renderer.setupFramebuffer(); err != nil {
		return nil, err
	}

	return renderer, nil
}

// initOpenGL initializes OpenGL resources
func (r *OpenGLRenderer) initOpenGL() error {
	// Initialize basic GL settings
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LEQUAL)
	gl.ClearColor(0.0, 0.0, 0.0, 1.0)

	// Create basic rendering shader program
	var err error
	if r.shaderProgram, err = r.createShaderProgram(vertexShaderSource, fragmentShaderSource); err != nil {
		return err
	}

	// Create effects shader program
	if r.effectsShader, err = r.createShaderProgram(postProcessVertexShader, postProcessFragmentShader); err != nil {
		return err
	}

	// Get uniform locations
	gl.UseProgram(r.effectsShader)
	r.timeLocation = gl.GetUniformLocation(r.effectsShader, gl.Str("time\x00"))
	r.glitchLocation = gl.GetUniformLocation(r.effectsShader, gl.Str("glitchAmount\x00"))
	r.vignetteLocation = gl.GetUniformLocation(r.effectsShader, gl.Str("vignetteAmount\x00"))
	r.noiseLocation = gl.GetUniformLocation(r.effectsShader, gl.Str("noiseAmount\x00"))
	r.resolutionLocation = gl.GetUniformLocation(r.effectsShader, gl.Str("resolution\x00"))

	// Create VAO for ASCII grid
	gl.GenVertexArrays(1, &r.vertexArray)
	gl.BindVertexArray(r.vertexArray)

	// Create the buffer for vertices
	gl.GenBuffers(1, &r.vertexBuffer)
	r.updateVertexBuffer()

	// Create index buffer
	gl.GenBuffers(1, &r.elementBuffer)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, r.elementBuffer)
	indices := []uint32{0, 1, 2, 2, 3, 0}
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

	// Setup screen quad for post-processing
	r.setupScreenQuad()

	return nil
}

// setupScreenQuad creates a full-screen quad for post-processing
func (r *OpenGLRenderer) setupScreenQuad() {
	vertices := []float32{
		// Positions   // Texture coords
		-1.0, -1.0, 0.0, 0.0, 1.0,
		1.0, -1.0, 0.0, 1.0, 1.0,
		1.0, 1.0, 0.0, 1.0, 0.0,
		-1.0, 1.0, 0.0, 0.0, 0.0,
	}

	gl.GenVertexArrays(1, &r.quadVAO)
	gl.GenBuffers(1, &r.quadVBO)
	gl.BindVertexArray(r.quadVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.quadVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	// Position attribute
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 5*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)
	// Texture coord attribute
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 5*4, gl.PtrOffset(3*4))
	gl.EnableVertexAttribArray(1)

	gl.BindVertexArray(0)
}

// setupFramebuffer initializes the framebuffer for post-processing
func (r *OpenGLRenderer) setupFramebuffer() error {
	// Generate framebuffer
	gl.GenFramebuffers(1, &r.fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)

	// Create texture for framebuffer
	gl.GenTextures(1, &r.screenTexture)
	gl.BindTexture(gl.TEXTURE_2D, r.screenTexture)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(r.width), int32(r.height), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, r.screenTexture, 0)

	// Create renderbuffer for depth and stencil
	gl.GenRenderbuffers(1, &r.rbo)
	gl.BindRenderbuffer(gl.RENDERBUFFER, r.rbo)
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH24_STENCIL8, int32(r.width), int32(r.height))
	gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_STENCIL_ATTACHMENT, gl.RENDERBUFFER, r.rbo)

	// Check if framebuffer is complete
	if gl.CheckFramebufferStatus(gl.FRAMEBUFFER) != gl.FRAMEBUFFER_COMPLETE {
		return fmt.Errorf("framebuffer not complete")
	}

	// Unbind framebuffer
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)

	return nil
}

// updateVertexBuffer updates the vertex buffer based on current dimensions
func (r *OpenGLRenderer) updateVertexBuffer() {
	gl.BindBuffer(gl.ARRAY_BUFFER, r.vertexBuffer)

	// Create fullscreen quad vertices
	vertices := []float32{
		// Position    // Texture coordinates
		-1.0, -1.0, 0.0, 0.0, 1.0, // Bottom left
		1.0, -1.0, 0.0, 1.0, 1.0, // Bottom right
		1.0, 1.0, 0.0, 1.0, 0.0, // Top right
		-1.0, 1.0, 0.0, 0.0, 0.0, // Top left
	}

	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	// Position attribute
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 5*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)
	// Texture coord attribute
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 5*4, gl.PtrOffset(3*4))
	gl.EnableVertexAttribArray(1)
}

// createASCIITexture creates a texture with ASCII characters
func (r *OpenGLRenderer) createASCIITexture() error {
	// Create a texture with ASCII characters arranged in a grid
	r.asciiImage = image.NewRGBA(image.Rect(0, 0, r.fontTexWidth, r.fontTexHeight))

	// Fill with black
	draw.Draw(r.asciiImage, r.asciiImage.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)

	// Draw characters to the image - we're creating a simple font atlas
	// In a real implementation, this would load from a proper font file
	for i, char := range r.asciiCharset {
		// Calculate position in the texture atlas
		x := (i % 4) * r.charWidth
		y := (i / 4) * r.charHeight

		// Draw a simple representation of the character
		r.drawCharToImage(char, x, y)
	}

	// Create OpenGL texture from the image
	gl.GenTextures(1, &r.asciiTexture)
	gl.BindTexture(gl.TEXTURE_2D, r.asciiTexture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	// Upload texture data
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(r.fontTexWidth),
		int32(r.fontTexHeight),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		gl.Ptr(r.asciiImage.Pix),
	)

	return nil
}

// drawCharToImage draws a simple representation of a character to the ASCII texture
func (r *OpenGLRenderer) drawCharToImage(char rune, x, y int) {
	// Simple representation - actually draw the shape based on the character
	// For a real implementation, this would render from a font
	intensity := float64(0)

	// Map character to intensity
	for i, c := range r.asciiCharset {
		if c == char {
			intensity = float64(i) / float64(len(r.asciiCharset)-1)
			break
		}
	}

	// Calculate color based on intensity
	bright := uint8(intensity * 255)
	pixelColor := color.RGBA{bright, bright, bright, 255}

	// Fill a box in the texture with this color
	for dy := 0; dy < r.charHeight; dy++ {
		for dx := 0; dx < r.charWidth; dx++ {
			// Create a shape based on the character
			drawPixel := false

			// Very simple shape rules
			switch char {
			case ' ':
				drawPixel = false
			case '.':
				drawPixel = dx > r.charWidth/3 && dx < 2*r.charWidth/3 &&
					dy > r.charHeight/3 && dy < 2*r.charHeight/3
			case '*':
				// Star shape
				drawPixel = (dx == r.charWidth/2 || dy == r.charHeight/2 ||
					(dx-r.charWidth/2)*(dx-r.charWidth/2)+(dy-r.charHeight/2)*(dy-r.charHeight/2) < 16)
			case '#':
				// Grid shape
				drawPixel = dx%4 == 0 || dy%4 == 0
			case '@':
				// Circle shape
				centerX, centerY := r.charWidth/2, r.charHeight/2
				dist := math.Sqrt(float64((dx-centerX)*(dx-centerX) + (dy-centerY)*(dy-centerY)))
				drawPixel = dist < float64(r.charWidth)/3
			default:
				// For other chars, just fill based on intensity
				drawPixel = float64(dx*dy)/float64(r.charWidth*r.charHeight) < intensity
			}

			if drawPixel {
				r.asciiImage.Set(x+dx, y+dy, pixelColor)
			}
		}
	}
}

// createShaderProgram compiles and links a shader program from source
func (r *OpenGLRenderer) createShaderProgram(vertexSource, fragmentSource string) (uint32, error) {
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

	// Detach and delete shaders since they're linked to the program now
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

// UpdateResolution updates the renderer resolution
func (r *OpenGLRenderer) UpdateResolution(width, height int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.width == width && r.height == height {
		return
	}

	r.width = width
	r.height = height

	// Update framebuffer size
	gl.BindTexture(gl.TEXTURE_2D, r.screenTexture)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)

	gl.BindRenderbuffer(gl.RENDERBUFFER, r.rbo)
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH24_STENCIL8, int32(width), int32(height))
}

// ApplyGlitchEffect applies a glitch visual effect for the specified duration
func (r *OpenGLRenderer) ApplyGlitchEffect(amount, duration float32) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.glitchAmount = amount
	r.glitchDuration = duration
	r.glitchStartTime = time.Now()
}

// SetVignetteAmount sets the vignette effect intensity
func (r *OpenGLRenderer) SetVignetteAmount(amount float32) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.vignetteAmount = float32(math.Max(0, math.Min(1, float64(amount))))
}

// SetNoiseAmount sets the noise effect intensity
func (r *OpenGLRenderer) SetNoiseAmount(amount float32) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.noiseAmount = float32(math.Max(0, math.Min(1, float64(amount))))
}

// TogglePostProcessing enables or disables post-processing effects
func (r *OpenGLRenderer) TogglePostProcessing() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.usePostProcess = !r.usePostProcess

	// When disabling post-processing, disable some effects
	if !r.usePostProcess {
		r.noiseAmount = 0.0
		r.vignetteAmount = 0.1 // Minimal vignette
	} else {
		r.noiseAmount = 0.03
		r.vignetteAmount = 0.4
	}
}

// getColorForScene determines the text color based on scene mood
func (r *OpenGLRenderer) getColorForScene(scene *SceneData) [3]float32 {
	// Start with base color
	result := r.baseFgColor

	// Apply fog effect (blue tint)
	if fogAmount, ok := scene.SpecialEffects["fog"]; ok && fogAmount > 0 {
		result[0] *= float32(1.0 - 0.3*fogAmount) // Reduce red
		result[1] *= float32(1.0 - 0.1*fogAmount) // Reduce green slightly
		result[2] *= float32(1.0 + 0.2*fogAmount) // Increase blue
	}

	// Darkness reduces overall brightness
	if darkness, ok := scene.SpecialEffects["darkness"]; ok && darkness > 0 {
		factor := float32(1.0 - 0.5*darkness)
		result[0] *= factor
		result[1] *= factor
		result[2] *= factor
	}

	// Fear increases red component
	if fear, ok := scene.SpecialEffects["fear"]; ok && fear > 0.7 {
		result[0] *= float32(1.0 + 0.2*(fear-0.7))
		result[1] *= float32(1.0 - 0.1*(fear-0.7))
		result[2] *= float32(1.0 - 0.1*(fear-0.7))
	}

	// Add subtle flicker
	flicker := float32(0.98 + 0.04*rand.Float64())
	result[0] *= flicker
	result[1] *= flicker
	result[2] *= flicker

	return result
}

// Close releases all OpenGL resources
func (r *OpenGLRenderer) Close() {
	gl.DeleteVertexArrays(1, &r.vertexArray)
	gl.DeleteBuffers(1, &r.vertexBuffer)
	gl.DeleteBuffers(1, &r.elementBuffer)
	gl.DeleteVertexArrays(1, &r.quadVAO)
	gl.DeleteBuffers(1, &r.quadVBO)
	gl.DeleteTextures(1, &r.asciiTexture)
	gl.DeleteTextures(1, &r.screenTexture)
	gl.DeleteRenderbuffers(1, &r.rbo)
	gl.DeleteFramebuffers(1, &r.fbo)
	gl.DeleteProgram(r.shaderProgram)
	gl.DeleteProgram(r.effectsShader)
}

// Fix for OpenGLRenderer viewport issue

// Update the OpenGLRenderer.Render method to properly set the viewport
func (r *OpenGLRenderer) Render(scene *SceneData) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Get window dimensions instead of using internal dimensions
	winWidth, winHeight := r.getWindowDimensions()

	// If scene is nil, just clear the screen
	if scene == nil || len(scene.Pixels) == 0 {
		gl.Viewport(0, 0, int32(winWidth), int32(winHeight))
		gl.ClearColor(0, 0, 0, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		return
	}

	if r.usePostProcess {
		// Make sure framebuffer is properly sized for window
		r.resizeFramebufferIfNeeded(winWidth, winHeight)

		// Bind framebuffer for offscreen rendering
		gl.BindFramebuffer(gl.FRAMEBUFFER, r.fbo)
	}

	// Set viewport to window dimensions
	gl.Viewport(0, 0, int32(winWidth), int32(winHeight))
	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// Render ASCII scene
	r.renderASCIIScene(scene)

	// Apply post-processing if enabled
	if r.usePostProcess {
		// Unbind framebuffer to render to screen with post-processing
		gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
		gl.Viewport(0, 0, int32(winWidth), int32(winHeight))
		gl.ClearColor(0, 0, 0, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		// Render with post-processing effects
		r.renderPostProcess()
	}

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

// Add this helper function to get current window dimensions
func (r *OpenGLRenderer) getWindowDimensions() (int, int) {
	a, _ := glfw.GetCurrentContext()
	if a != nil {
		if win, _ := glfw.GetCurrentContext(); win != nil {
			width, height := win.GetSize()
			return width, height
		}
	}
	return r.width, r.height
}

// Add this helper function to resize the framebuffer
func (r *OpenGLRenderer) resizeFramebufferIfNeeded(width, height int) {
	if r.width != width || r.height != height {
		// Update internal dimensions
		r.width = width
		r.height = height

		// Resize framebuffer texture
		gl.BindTexture(gl.TEXTURE_2D, r.screenTexture)
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)

		// Resize renderbuffer
		gl.BindRenderbuffer(gl.RENDERBUFFER, r.rbo)
		gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH24_STENCIL8, int32(width), int32(height))
	}
}

// Update the OpenGLRenderer.renderASCIIScene method to account for window size
func (r *OpenGLRenderer) renderASCIIScene(scene *SceneData) {
	// Use the basic shader program
	gl.UseProgram(r.shaderProgram)

	// Active texture slot 0
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, r.asciiTexture)

	// Set uniform for texture
	gl.Uniform1i(gl.GetUniformLocation(r.shaderProgram, gl.Str("asciiTexture\x00")), 0)

	// Get current window dimensions
	winWidth, winHeight := r.getWindowDimensions()

	// Calculate aspect ratio
	aspectRatio := float32(winWidth) / float32(winHeight)

	// Calculate grid dimensions for current window aspect ratio
	cellWidth := 2.0 / float32(scene.Width)
	cellHeight := 2.0 / float32(scene.Height)

	// Maintain aspect ratio by scaling the grid
	if aspectRatio > 1.0 {
		// Widescreen, scale horizontally
		cellWidth *= aspectRatio
	} else {
		// Tall screen, scale vertically
		cellHeight /= aspectRatio
	}

	// Get foreground color based on scene mood
	fgColor := r.getColorForScene(scene)
	gl.Uniform3f(gl.GetUniformLocation(r.shaderProgram, gl.Str("textColor\x00")), fgColor[0], fgColor[1], fgColor[2])

	// We'll iterate over each character position in our grid
	for y := 0; y < scene.Height; y++ {
		if y >= len(scene.Pixels) {
			continue
		}

		for x := 0; x < scene.Width; x++ {
			if x >= len(scene.Pixels[y]) {
				continue
			}

			// Get intensity from scene data
			intensity := scene.Pixels[y][x].Intensity

			// Map intensity to ASCII character
			charIndex := int(intensity * float64(len(r.asciiCharset)-1))
			charIndex = int(math.Min(float64(len(r.asciiCharset)-1), math.Max(0, float64(charIndex))))

			// Calculate texture coordinates for this character
			glyphX := (charIndex % 4) * r.charWidth
			glyphY := (charIndex / 4) * r.charHeight
			texCoordX := float32(glyphX) / float32(r.fontTexWidth)
			texCoordY := float32(glyphY) / float32(r.fontTexHeight)
			texWidth := float32(r.charWidth) / float32(r.fontTexWidth)
			texHeight := float32(r.charHeight) / float32(r.fontTexHeight)

			// Calculate position in normalized device coordinates
			// Center the grid in the window
			posX := -1.0 + float32(x)*cellWidth
			posY := 1.0 - float32(y)*cellHeight

			// Create vertices for this character quad
			vertices := []float32{
				// Position                  // Texture Coordinates
				posX, posY - cellHeight, 0.0, texCoordX, texCoordY + texHeight, // Bottom left
				posX + cellWidth, posY - cellHeight, 0.0, texCoordX + texWidth, texCoordY + texHeight, // Bottom right
				posX + cellWidth, posY, 0.0, texCoordX + texWidth, texCoordY, // Top right
				posX, posY, 0.0, texCoordX, texCoordY, // Top left
			}

			// Upload vertices
			gl.BindBuffer(gl.ARRAY_BUFFER, r.vertexBuffer)
			gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STREAM_DRAW)

			// Draw quad
			gl.BindVertexArray(r.vertexArray)
			gl.DrawElements(gl.TRIANGLES, 6, gl.UNSIGNED_INT, nil)
		}
	}
}

// Update the renderPostProcess method to use window dimensions
func (r *OpenGLRenderer) renderPostProcess() {
	// Use post-processing shader
	gl.UseProgram(r.effectsShader)

	// Bind the offscreen rendered texture
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, r.screenTexture)
	gl.Uniform1i(gl.GetUniformLocation(r.effectsShader, gl.Str("screenTexture\x00")), 0)

	// Get current window dimensions
	winWidth, winHeight := r.getWindowDimensions()

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

	// Draw fullscreen quad
	gl.BindVertexArray(r.quadVAO)
	gl.DrawArrays(gl.TRIANGLE_FAN, 0, 4)
	gl.BindVertexArray(0)
}

// Update this function to use the correct GLFW version
func (e *Engine) resizeCallback(_ *glfw.Window, width int, height int) {
	e.logger.Info("Window resized to %dx%d", width, height)
	e.windowWidth = width
	e.windowHeight = height

	// Update config
	e.config.Graphics.Width = width
	e.config.Graphics.Height = height

	// Update renderer resolution
	e.renderer.UpdateResolution(width, height)

	// Only update raytracer resolution if needed (maintain grid proportions)
	aspectRatio := float64(width) / float64(height)
	gridHeight := e.config.Raytracer.Height
	newGridWidth := int(float64(gridHeight) * aspectRatio)

	if newGridWidth != e.config.Raytracer.Width {
		e.config.Raytracer.Width = newGridWidth
		e.logger.Debug("Adjusted raytracer grid to %dx%d",
			e.config.Raytracer.Width, e.config.Raytracer.Height)

		// Update raytracer resolution
		e.raytracer.UpdateResolution(e.config.Raytracer.Width, e.config.Raytracer.Height)
	}
}
