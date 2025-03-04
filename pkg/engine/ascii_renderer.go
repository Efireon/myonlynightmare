package engine

import (
	"math/rand"
	"sync"
	"time"

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

	// Поддержка эффектов
	glitchAmount    float32
	glitchDuration  float32
	glitchStartTime time.Time

	// Параметры шейдера
	colorLocation  int32
	timeLocation   int32
	glitchLocation int32

	// Блокировка для потокобезопасности
	mutex sync.Mutex
}

// NewASCIIRenderer creates a new ASCII renderer
func NewASCIIRenderer(config config.RendererConfig) (*ASCIIRenderer, error) {
	// Initialize renderer
	renderer := &ASCIIRenderer{
		config:         config,
		width:          config.Width,
		height:         config.Height,
		fontTexWidth:   16, // ASCII character grid width
		fontTexHeight:  16, // ASCII character grid height
		glitchAmount:   0.0,
		glitchDuration: 0.0,
	}

	// Set up ASCII gradient from darkest to lightest
	// Добавляем больше символов для более плавного градиента
	renderer.asciiGradient = []rune{' ', '.', '\'', '`', ',', ':', ';', '"', '-', '+', '=', '*', '#', '%', '@', '$'}

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

	// Enable alpha blending
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Generate vertex array
	gl.GenVertexArrays(1, &r.vertexArray)
	gl.BindVertexArray(r.vertexArray)

	// Generate vertex buffer
	gl.GenBuffers(1, &r.vertexBuffer)
	r.updateVertexBuffer()

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

	// Compile shader program
	r.shaderProgram = r.compileShaderProgram()

	// Get uniform locations
	r.colorLocation = gl.GetUniformLocation(r.shaderProgram, gl.Str("asciiColor\x00"))
	r.timeLocation = gl.GetUniformLocation(r.shaderProgram, gl.Str("time\x00"))
	r.glitchLocation = gl.GetUniformLocation(r.shaderProgram, gl.Str("glitchAmount\x00"))

	// Create textures
	gl.GenTextures(1, &r.textureID)

	// Initialize texture
	r.updateTexture()

	return nil
}

// updateVertexBuffer updates the vertex buffer with current dimensions
func (r *ASCIIRenderer) updateVertexBuffer() {
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

	// Set up vertex attributes
	// Position attribute
	gl.VertexAttribPointer(0, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)
	// Texture coordinate attribute
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(2*4))
	gl.EnableVertexAttribArray(1)
}

// updateTexture initializes or resizes the texture
func (r *ASCIIRenderer) updateTexture() {
	gl.BindTexture(gl.TEXTURE_2D, r.textureID)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	// Initialize with empty texture
	data := make([]byte, r.width*r.height)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RED, int32(r.width), int32(r.height), 0, gl.RED, gl.UNSIGNED_BYTE, gl.Ptr(data))
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

	// Fragment shader source code - улучшенный шейдер с эффектами
	fragmentShaderSource := `
		#version 410 core
		in vec2 TexCoord;
		out vec4 FragColor;
		
		uniform sampler2D asciiTexture;
		uniform vec3 asciiColor;
		uniform float time;
		uniform float glitchAmount;
		
		// Функция случайного шума
		float rand(vec2 co) {
			return fract(sin(dot(co.xy, vec2(12.9898, 78.233))) * 43758.5453);
		}
		
		void main() {
			// Базовые искаженные координаты текстуры
			vec2 texCoord = TexCoord;
			
			// Применяем эффект глитча если нужно
			if (glitchAmount > 0.0) {
				// Горизонтальные полосы глитча
				float lineNoise = floor(texCoord.y * 10.0) / 10.0 * time;
				
				// Случайное смещение по X в некоторых строках
				if (rand(vec2(lineNoise, time)) < glitchAmount * 0.8) {
					float shift = (rand(vec2(lineNoise, time * 0.1)) - 0.5) * 0.1 * glitchAmount;
					texCoord.x += shift;
				}
				
				// Цветовой шум
				float noiseVal = rand(texCoord + time) * glitchAmount * 0.1;
				
				// Получаем базовую интенсивность из текстуры
				float intensity = texture(asciiTexture, texCoord).r;
				
				// Добавляем шум к интенсивности
				intensity = clamp(intensity + noiseVal - glitchAmount * 0.05, 0.0, 1.0);
				
				// Создаем цвет с эффектом глитча
				vec3 color = asciiColor + vec3(noiseVal * 3.0 - 1.5, -noiseVal, noiseVal) * glitchAmount;
				FragColor = vec4(color * intensity, 1.0);
			} else {
				// Стандартный рендеринг без глитча
				float intensity = texture(asciiTexture, texCoord).r;
				FragColor = vec4(asciiColor * intensity, 1.0);
			}
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

// UpdateResolution updates the rendering resolution
func (r *ASCIIRenderer) UpdateResolution(width, height int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.width == width && r.height == height {
		return // Размер не изменился
	}

	r.width = width
	r.height = height
	r.config.Width = width
	r.config.Height = height

	// Обновляем текстуру с новыми размерами
	r.updateTexture()
}

// ApplyGlitchEffect применяет эффект глитча на заданное время
func (r *ASCIIRenderer) ApplyGlitchEffect(amount, duration float32) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.glitchAmount = amount
	r.glitchDuration = duration
	r.glitchStartTime = time.Now()
}

// Render renders the scene using ASCII characters
func (r *ASCIIRenderer) Render(scene *SceneData) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

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

	// Обновляем время для анимаций и эффектов
	gl.Uniform1f(r.timeLocation, float32(time.Since(time.Unix(0, 0)).Seconds()))

	// Обновляем значение глитча
	currentGlitch := float32(0.0)
	if r.glitchDuration > 0 {
		elapsed := float32(time.Since(r.glitchStartTime).Seconds())
		if elapsed < r.glitchDuration {
			// Постепенно уменьшаем эффект к концу длительности
			fadeOut := 1.0 - (elapsed / r.glitchDuration)
			currentGlitch = r.glitchAmount * fadeOut
		} else {
			r.glitchDuration = 0 // Эффект закончился
		}
	}
	gl.Uniform1f(r.glitchLocation, currentGlitch)

	// Dynamically select color based on mood
	// По умолчанию светло-серый
	r.setColorBasedOnScene(scene)

	// Bind VAO and draw
	gl.BindVertexArray(r.vertexArray)
	gl.DrawElements(gl.TRIANGLES, 6, gl.UNSIGNED_INT, nil)
}

// setColorBasedOnScene устанавливает цвет в зависимости от тона сцены
func (r *ASCIIRenderer) setColorBasedOnScene(scene *SceneData) {
	// Базовый цвет (приглушенный зеленый для ретро-эффекта)
	baseR, baseG, baseB := 0.7, 0.85, 0.7

	// Если доступна сцена с особыми эффектами, меняем цвет
	if scene != nil && scene.SpecialEffects != nil {
		// Эффект тумана делает цвет более голубоватым
		if fogAmount, ok := scene.SpecialEffects["fog"]; ok && fogAmount > 0 {
			baseR *= 1.0 - 0.3*fogAmount
			baseG *= 1.0 - 0.1*fogAmount
			baseB *= 1.0 + 0.2*fogAmount
		}

		// Темнота снижает все компоненты
		if darkness, ok := scene.SpecialEffects["darkness"]; ok && darkness > 0 {
			factor := 1.0 - 0.5*darkness
			baseR *= factor
			baseG *= factor
			baseB *= factor
		}

		// Страх добавляет краснота
		if fear, ok := scene.SpecialEffects["fear"]; ok && fear > 0.7 {
			baseR *= 1.0 + 0.2*(fear-0.7)
			baseG *= 1.0 - 0.1*(fear-0.7)
			baseB *= 1.0 - 0.1*(fear-0.7)
		}
	}

	// Добавляем небольшое случайное колебание для "живости"
	flicker := 0.98 + 0.04*rand.Float64()
	baseR *= flicker
	baseG *= flicker
	baseB *= flicker

	// Устанавливаем цвет в шейдере
	gl.Uniform3f(r.colorLocation, float32(baseR), float32(baseG), float32(baseB))
}

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

	// Адаптируем размеры сцены к нашему буферу
	sceneWidth := scene.Width
	sceneHeight := scene.Height

	// Фактор масштабирования для подгонки сцены к размеру рендерера
	scaleX := float64(sceneWidth) / float64(r.width)
	scaleY := float64(sceneHeight) / float64(r.height)

	// Более продвинутый алгоритм сэмплирования с билинейной интерполяцией
	for y := 0; y < r.height; y++ {
		for x := 0; x < r.width; x++ {
			// Вычисляем координаты в пространстве сцены (с дробной частью)
			sceneX := float64(x) * scaleX
			sceneY := float64(y) * scaleY

			// Координаты четырех ближайших пикселей
			x0, y0 := int(sceneX), int(sceneY)
			x1, y1 := min(x0+1, sceneWidth-1), min(y0+1, sceneHeight-1)

			// Веса для билинейной интерполяции
			wx := sceneX - float64(x0)
			wy := sceneY - float64(y0)

			// Получаем интенсивность из четырех точек
			var intensity00, intensity01, intensity10, intensity11 float64

			if y0 < len(scene.Pixels) && x0 < len(scene.Pixels[y0]) {
				intensity00 = scene.Pixels[y0][x0].Intensity
			}
			if y0 < len(scene.Pixels) && x1 < len(scene.Pixels[y0]) {
				intensity10 = scene.Pixels[y0][x1].Intensity
			}
			if y1 < len(scene.Pixels) && x0 < len(scene.Pixels[y1]) {
				intensity01 = scene.Pixels[y1][x0].Intensity
			}
			if y1 < len(scene.Pixels) && x1 < len(scene.Pixels[y1]) {
				intensity11 = scene.Pixels[y1][x1].Intensity
			}

			// Билинейная интерполяция
			intensityX0 := intensity00*(1-wx) + intensity10*wx
			intensityX1 := intensity01*(1-wx) + intensity11*wx
			intensity := intensityX0*(1-wy) + intensityX1*wy

			// Применяем тональное отображение для лучшей видимости
			intensity = toneMap(intensity)

			// Конвертируем в байтовое значение (0-255)
			byteValue := byte(intensity * 255)

			// Сохраняем в результат
			result[y*r.width+x] = byteValue
		}
	}

	return result
}

// toneMap применяет тональное отображение для улучшения видимости
func toneMap(intensity float64) float64 {
	// Используем простую кривую гаммы для увеличения контраста
	gamma := 1.2
	return pow(intensity, 1.0/gamma)
}

// pow реализует возведение в степень для float64
func pow(x, y float64) float64 {
	if x <= 0 {
		return 0
	}
	return float64(float32(x) * float32(y)) // использование оператора ** для степени
}

// Close releases all resources
func (r *ASCIIRenderer) Close() {
	gl.DeleteVertexArrays(1, &r.vertexArray)
	gl.DeleteBuffers(1, &r.vertexBuffer)
	gl.DeleteBuffers(1, &r.elementBuffer)
	gl.DeleteTextures(1, &r.textureID)
	gl.DeleteProgram(r.shaderProgram)
}
