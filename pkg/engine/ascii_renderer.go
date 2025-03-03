package engine

import (
	"github.com/go-gl/gl/v4.1-core/gl"

	"nightmare/pkg/config"
)

// ASCIIRenderer renders the scene using ASCII characters
type ASCIIRenderer struct {
	config        config.RendererConfig
	asciiGradient []rune // Characters from dark to light
	width         int
	height        int
	vertexArray   uint32
	vertexBuffer  uint32
	elementBuffer uint32
	textureID     uint32
	shaderProgram uint32
	fontTexWidth  int
	fontTexHeight int
}

// NewASCIIRenderer creates a new ASCII renderer
func NewASCIIRenderer(config config.RendererConfig) (*ASCIIRenderer, error) {
	// Initialize renderer
	renderer := &ASCIIRenderer{
		config:        config,
		width:         config.Width,
		height:        config.Height,
		fontTexWidth:  16, // ASCII character grid width
		fontTexHeight: 16, // ASCII character grid height
	}

	// Set up ASCII gradient from darkest to lightest
	// Space is darkest (no character), @ or # is brightest
	renderer.asciiGradient = []rune{' ', '.', ':', '-', '=', '+', '*', '#', '%', '@'}

	// Initialize OpenGL resources
	if err := renderer.initResources(); err != nil {
		return nil, err
	}

	return renderer, nil
}

// initResources initializes OpenGL resources needed for rendering
func (r *ASCIIRenderer) initResources() error {
	// Initialize OpenGL
	if err := gl.Init(); err != nil {
		return err
	}

	// Generate vertex array
	gl.GenVertexArrays(1, &r.vertexArray)
	gl.BindVertexArray(r.vertexArray)

	// Generate vertex buffer
	gl.GenBuffers(1, &r.vertexBuffer)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.vertexBuffer)

	// Define vertices for a fullscreen quad
	vertices := []float32{
		// Position    // Texture coordinates
		-1.0, -1.0, 0.0, 1.0, // Bottom left
		1.0, -1.0, 1.0, 1.0, // Bottom right
		1.0, 1.0, 1.0, 0.0, // Top right
		-1.0, 1.0, 0.0, 0.0, // Top left
	}

	// Upload vertex data
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	// Generate element buffer
	gl.GenBuffers(1, &r.elementBuffer)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, r.elementBuffer)

	// Define indices
	indices := []uint32{
		0, 1, 2, // First triangle
		2, 3, 0, // Second triangle
	}

	// Upload index data
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

	// Set up vertex attributes
	// Position attribute
	gl.VertexAttribPointer(0, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)
	// Texture coordinate attribute
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(2*4))
	gl.EnableVertexAttribArray(1)

	// Compile shader program
	r.shaderProgram = r.compileShaderProgram()

	// Create textures
	gl.GenTextures(1, &r.textureID)

	// Generate empty texture for ASCII display
	gl.BindTexture(gl.TEXTURE_2D, r.textureID)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	// Initialize with empty texture
	data := make([]byte, r.width*r.height)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RED, int32(r.width), int32(r.height), 0, gl.RED, gl.UNSIGNED_BYTE, gl.Ptr(data))

	return nil
}

// compileShaderProgram compiles and links the shader program
func (r *ASCIIRenderer) compileShaderProgram() uint32 {
	// Vertex shader source code
	vertexShaderSource := `
		#version 410 core
		layout (location = 0) in vec2 aPos;
		layout (location = 1) in vec2 aTexCoord;
		
		out vec2 TexCoord;
		
		void main() {
			gl_Position = vec4(aPos, 0.0, 1.0);
			TexCoord = aTexCoord;
		}
	`

	// Fragment shader source code
	fragmentShaderSource := `
		#version 410 core
		in vec2 TexCoord;
		out vec4 FragColor;
		
		uniform sampler2D asciiTexture;
		uniform vec3 asciiColor;
		
		void main() {
			float intensity = texture(asciiTexture, TexCoord).r;
			FragColor = vec4(asciiColor * intensity, 1.0);
		}
	`

	// Compile vertex shader
	vertexShader := gl.CreateShader(gl.VERTEX_SHADER)
	csource, free := gl.Strs(vertexShaderSource + "\x00")
	gl.ShaderSource(vertexShader, 1, csource, nil)
	free()
	gl.CompileShader(vertexShader)

	// Check for vertex shader compile errors
	var status int32
	gl.GetShaderiv(vertexShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(vertexShader, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetShaderInfoLog(vertexShader, logLength, nil, &log[0])
		panic("Vertex shader compilation failed: " + string(log))
	}

	// Compile fragment shader
	fragmentShader := gl.CreateShader(gl.FRAGMENT_SHADER)
	csource, free = gl.Strs(fragmentShaderSource + "\x00")
	gl.ShaderSource(fragmentShader, 1, csource, nil)
	free()
	gl.CompileShader(fragmentShader)

	// Check for fragment shader compile errors
	gl.GetShaderiv(fragmentShader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(fragmentShader, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetShaderInfoLog(fragmentShader, logLength, nil, &log[0])
		panic("Fragment shader compilation failed: " + string(log))
	}

	// Link shaders
	program := gl.CreateProgram()
	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	// Check for linking errors
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
		log := make([]byte, logLength)
		gl.GetProgramInfoLog(program, logLength, nil, &log[0])
		panic("Shader program linking failed: " + string(log))
	}

	// Delete shaders after linking
	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program
}

// Render renders the scene using ASCII characters
// Render renders the scene using ASCII characters
func (r *ASCIIRenderer) Render(scene *SceneData) {
	// Проверяем, что сцена существует и содержит данные
	if scene == nil {
		// Очищаем экран и выходим
		gl.ClearColor(0, 0, 0, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		return
	}

	// Convert scene data to ASCII representation
	asciiData := r.sceneToASCII(scene)

	// Clear the screen
	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	// Bind shader program
	gl.UseProgram(r.shaderProgram)

	// Update texture with new ASCII data
	gl.BindTexture(gl.TEXTURE_2D, r.textureID)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, int32(r.width), int32(r.height), gl.RED, gl.UNSIGNED_BYTE, gl.Ptr(asciiData))

	// Set ASCII color uniform (grayscale)
	colorLocation := gl.GetUniformLocation(r.shaderProgram, gl.Str("asciiColor\x00"))
	gl.Uniform3f(colorLocation, 0.8, 0.8, 0.8) // Light gray

	// Bind VAO and draw
	gl.BindVertexArray(r.vertexArray)
	gl.DrawElements(gl.TRIANGLES, 6, gl.UNSIGNED_INT, nil)
}

// sceneToASCII converts scene data to ASCII representation
// sceneToASCII converts scene data to ASCII representation
func (r *ASCIIRenderer) sceneToASCII(scene *SceneData) []byte {
	result := make([]byte, r.width*r.height)

	// Проверка, что scene не nil и содержит данные
	if scene == nil || len(scene.Pixels) == 0 {
		// Заполняем пустыми символами
		for i := range result {
			result[i] = 0 // Нулевая интенсивность (пустой символ)
		}
		return result
	}

	// Проверка размеров сцены для предотвращения выхода за границы
	height := min(scene.Height, len(scene.Pixels))

	for y := 0; y < height; y++ {
		// Проверяем, что слайс для данной строки существует и имеет соответствующую длину
		if y >= len(scene.Pixels) || scene.Pixels[y] == nil {
			continue
		}

		width := min(scene.Width, len(scene.Pixels[y]))

		for x := 0; x < width; x++ {
			// Get traced pixel data
			pixel := scene.Pixels[y][x]

			// Map intensity to ASCII character index
			asciiIndex := int(pixel.Intensity * float64(len(r.asciiGradient)-1))

			// Convert to grayscale intensity (0-255)
			// More dense character = higher intensity
			intensity := byte(asciiIndex * 255 / (len(r.asciiGradient) - 1))

			// Store in result buffer
			result[y*r.width+x] = intensity
		}
	}

	return result
}

// Вспомогательная функция для нахождения минимума из двух int
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Close releases all resources
func (r *ASCIIRenderer) Close() {
	gl.DeleteVertexArrays(1, &r.vertexArray)
	gl.DeleteBuffers(1, &r.vertexBuffer)
	gl.DeleteBuffers(1, &r.elementBuffer)
	gl.DeleteTextures(1, &r.textureID)
	gl.DeleteProgram(r.shaderProgram)
}
