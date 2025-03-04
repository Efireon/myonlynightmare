package engine

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"

	"nightmare/internal/logger"
	"nightmare/pkg/config"
)

// Engine represents the main game engine
type Engine struct {
	window      *glfw.Window
	config      *config.Config
	logger      *logger.Logger
	raytracer   *Raytracer
	renderer    *ASCIIRenderer
	procedural  *ProceduralGenerator
	audioEngine *AudioEngine
	isRunning   bool
	lastUpdate  time.Time
	frameRate   int
	input       *InputHandler
	// Новые поля для отслеживания изменений размера
	windowWidth  int
	windowHeight int
}

// NewEngine creates a new game engine instance
func NewEngine(cfg *config.Config, log *logger.Logger) (*Engine, error) {
	runtime.LockOSThread()

	if err := glfw.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize GLFW: %v", err)
	}

	// Set window hints
	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	// Create window
	window, err := glfw.CreateWindow(
		cfg.Graphics.Width,
		cfg.Graphics.Height,
		"Nightmare - ASCII Horror",
		nil,
		nil,
	)
	if err != nil {
		glfw.Terminate()
		return nil, fmt.Errorf("failed to create GLFW window: %v", err)
	}

	window.MakeContextCurrent()

	// Создаем движок
	engine := &Engine{
		window:       window,
		config:       cfg,
		logger:       log,
		isRunning:    false,
		frameRate:    cfg.Graphics.FrameRate,
		windowWidth:  cfg.Graphics.Width,
		windowHeight: cfg.Graphics.Height,
	}

	// Добавляем обработчик изменения размера окна
	window.SetSizeCallback(engine.resizeCallback)

	// Создаем обработчик ввода
	engine.input = NewInputHandler(window)

	// Initialize components
	raytracer, err := NewRaytracer(cfg.Raytracer)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize raytracer: %v", err)
	}
	engine.raytracer = raytracer

	renderer, err := NewASCIIRenderer(cfg.Renderer)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ASCII renderer: %v", err)
	}
	engine.renderer = renderer

	procedural, err := NewProceduralGenerator(cfg.Procedural)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize procedural generator: %v", err)
	}
	engine.procedural = procedural

	audioEngine, err := NewAudioEngine(cfg.Audio)
	if err != nil {
		log.Warn("Failed to initialize audio engine: %v. Running without audio.", err)
		// Создаем заглушку для аудио (null object pattern)
		audioEngine = &AudioEngine{
			isRunning: false,
		}
	}
	engine.audioEngine = audioEngine

	return engine, nil
}

// resizeCallback обрабатывает изменение размера окна
func (e *Engine) resizeCallback(_ *glfw.Window, width int, height int) {
	e.logger.Info("Window resized to %dx%d", width, height)
	e.windowWidth = width
	e.windowHeight = height

	// Обновляем размеры в конфигурации
	e.config.Graphics.Width = width
	e.config.Graphics.Height = height

	// Пересчитываем размеры для рендеринга, сохраняя пропорции ASCII символов
	aspectRatio := float64(width) / float64(height)
	terminalAspectRatio := 2.0 // ширина:высота для ASCII символов (примерно 2:1)

	newTerminalWidth := int(float64(e.config.Renderer.Height) * aspectRatio * terminalAspectRatio)
	if newTerminalWidth != e.config.Renderer.Width {
		e.config.Renderer.Width = newTerminalWidth
		e.logger.Debug("Adjusted ASCII rendering grid to %dx%d",
			e.config.Renderer.Width, e.config.Renderer.Height)

		// Обновляем настройки рендеринга
		e.renderer.UpdateResolution(e.config.Renderer.Width, e.config.Renderer.Height)

		// Обновляем настройки рейтрейсера
		e.raytracer.UpdateResolution(e.config.Renderer.Width, e.config.Renderer.Height)
	}
}

// Run starts the main game loop
func (e *Engine) Run() {
	e.isRunning = true
	e.lastUpdate = time.Now()

	// Setup initial world
	e.procedural.GenerateInitialWorld()

	// Установим начальное положение камеры
	cameraHeight := 1.7 // высота камеры в условных единицах (рост человека)
	e.raytracer.SetCameraPosition(Vector3{X: 0, Y: cameraHeight, Z: -5})

	// Генерируем начальную атмосферу
	metadata := map[string]float64{
		"atmosphere.fear":    0.2,
		"atmosphere.ominous": 0.3,
		"visuals.dark":       0.5,
		"conditions.fog":     0.3,
	}
	e.audioEngine.GenerateAtmosphere(metadata)

	// Main game loop
	for e.isRunning && !e.window.ShouldClose() {
		currentTime := time.Now()
		deltaTime := currentTime.Sub(e.lastUpdate).Seconds()
		e.lastUpdate = currentTime

		// Обработка ввода
		e.input.Update()
		e.processInput(deltaTime)

		// Update game state
		e.update(deltaTime)

		// Render frame
		e.render()

		// Swap buffers and poll events
		e.window.SwapBuffers()
		glfw.PollEvents()

		// Cap the frame rate
		if e.frameRate > 0 {
			frameTime := time.Now().Sub(currentTime)
			targetFrameTime := time.Second / time.Duration(e.frameRate)
			if frameTime < targetFrameTime {
				time.Sleep(targetFrameTime - frameTime)
			}
		}
	}

	e.cleanup()
}

// processInput handles user input
func (e *Engine) processInput(deltaTime float64) {
	// Close the game when ESC is pressed
	if e.input.IsKeyPressed(glfw.KeyEscape) {
		e.isRunning = false
		return
	}

	// Получение текущей позиции и ориентации камеры
	camPos := e.raytracer.GetCameraPosition()
	camDir := e.raytracer.GetCameraDirection()
	camRight := e.raytracer.GetCameraRight()

	// Скорость передвижения и вращения камеры
	moveSpeed := 2.0 * deltaTime
	rotateSpeed := 1.5 * deltaTime

	// Обработка движения (WASD)
	if e.input.IsKeyDown(glfw.KeyW) {
		// Движение вперед (по направлению взгляда)
		forwardXZ := Vector3{
			X: camDir.X,
			Y: 0, // Сохраняем Y-координату (высоту)
			Z: camDir.Z,
		}.Normalize()
		camPos = camPos.Add(forwardXZ.Mul(moveSpeed))
	}
	if e.input.IsKeyDown(glfw.KeyS) {
		// Движение назад
		backwardXZ := Vector3{
			X: -camDir.X,
			Y: 0,
			Z: -camDir.Z,
		}.Normalize()
		camPos = camPos.Add(backwardXZ.Mul(moveSpeed))
	}
	if e.input.IsKeyDown(glfw.KeyA) {
		// Движение влево
		leftXZ := Vector3{
			X: -camRight.X,
			Y: 0,
			Z: -camRight.Z,
		}.Normalize()
		camPos = camPos.Add(leftXZ.Mul(moveSpeed))
	}
	if e.input.IsKeyDown(glfw.KeyD) {
		// Движение вправо
		rightXZ := Vector3{
			X: camRight.X,
			Y: 0,
			Z: camRight.Z,
		}.Normalize()
		camPos = camPos.Add(rightXZ.Mul(moveSpeed))
	}

	// Вращение камеры (стрелки)
	if e.input.IsKeyDown(glfw.KeyLeft) {
		e.raytracer.RotateCamera(-rotateSpeed, 0)
	}
	if e.input.IsKeyDown(glfw.KeyRight) {
		e.raytracer.RotateCamera(rotateSpeed, 0)
	}
	if e.input.IsKeyDown(glfw.KeyUp) {
		e.raytracer.RotateCamera(0, -rotateSpeed)
	}
	if e.input.IsKeyDown(glfw.KeyDown) {
		e.raytracer.RotateCamera(0, rotateSpeed)
	}

	// Проверка столкновений и взаимодействие с окружением
	// Получаем информацию о ландшафте от процедурного генератора
	if e.procedural != nil && e.procedural.currentScene != nil {
		terrain := e.procedural.currentScene.Terrain

		if terrain != nil {
			// Получаем высоту ландшафта в текущей позиции
			terrainHeight := e.procedural.GetTerrainHeightAt(camPos.X, camPos.Z)

			// Учитываем минимальную высоту над поверхностью
			minHeightAboveTerrain := 1.7 // рост человека
			if camPos.Y < terrainHeight+minHeightAboveTerrain {
				camPos.Y = terrainHeight + minHeightAboveTerrain
			}

			// Проверка столкновений с объектами
			for _, obj := range e.procedural.currentScene.Objects {
				// Простая проверка дистанции до объекта
				dx := obj.Position.X - camPos.X
				dz := obj.Position.Z - camPos.Z
				distance := dx*dx + dz*dz

				// Размер колизии объекта зависит от его типа и масштаба
				collisionRadius := 0.5
				if obj.Type == "tree" {
					collisionRadius = 0.8 * obj.Scale.X
				} else if obj.Type == "rock" {
					collisionRadius = 1.0 * obj.Scale.X
				}

				// Если слишком близко - отталкиваем игрока
				if distance < collisionRadius*collisionRadius {
					// Вектор направления от объекта к игроку
					pushDir := Vector3{X: camPos.X - obj.Position.X, Y: 0, Z: camPos.Z - obj.Position.Z}.Normalize()
					pushAmount := collisionRadius - math.Sqrt(distance)
					camPos = camPos.Add(pushDir.Mul(pushAmount))
				}
			}
		}
	}

	// Обновляем позицию камеры
	e.raytracer.SetCameraPosition(camPos)

	// Интерактивные действия
	if e.input.IsKeyPressed(glfw.KeySpace) {
		// Действие при нажатии пробела (например, взаимодействие)
		e.logger.Debug("Player interaction triggered at position %v", camPos)

		// Генерируем "испуганный" звук при взаимодействии
		interactMeta := map[string]float64{
			"atmosphere.fear":    0.5,
			"atmosphere.tension": 0.6,
			"visuals.distorted":  0.3,
		}
		e.audioEngine.PlayProceduralSound("interact", 0.7, 0.0, interactMeta)
	}

	// Управление громкостью
	if e.input.IsKeyPressed(glfw.KeyEqual) || e.input.IsKeyPressed(glfw.KeyKPAdd) {
		e.audioEngine.IncreaseVolume(0.1)
		e.logger.Info("Volume increased")
	}
	if e.input.IsKeyPressed(glfw.KeyMinus) || e.input.IsKeyPressed(glfw.KeyKPSubtract) {
		e.audioEngine.DecreaseVolume(0.1)
		e.logger.Info("Volume decreased")
	}
	if e.input.IsKeyPressed(glfw.KeyM) {
		e.audioEngine.ToggleMute()
		e.logger.Info("Audio mute toggled")
	}
}

// update updates the game state
func (e *Engine) update(deltaTime float64) {
	// Update procedural generation
	e.procedural.Update(deltaTime)

	// Передаем текущую сцену в рейтрейсер
	if e.procedural.currentScene != nil {
		e.raytracer.SetScene(e.procedural.currentScene)
	}

	// Update audio
	e.audioEngine.Update(deltaTime)

	// Получаем текущее положение игрока
	playerPos := e.raytracer.GetCameraPosition()

	// Анализируем окружение вокруг игрока
	environmentMood := e.analyzeEnvironment(playerPos)

	// Обновляем атмосферу на основе окружения
	if deltaTime > 0 && int(e.lastUpdate.Second())%10 == 0 { // каждые 10 секунд
		e.audioEngine.UpdateAtmosphere(environmentMood, 5.0) // плавный переход за 5 секунд
	}

	// Обработка специальных события по триггерам окружения
	e.processEnvironmentTriggers(playerPos, deltaTime)
}

// analyzeEnvironment анализирует окружение вокруг игрока для создания соответствующей атмосферы
func (e *Engine) analyzeEnvironment(playerPos Vector3) map[string]float64 {
	result := map[string]float64{
		"atmosphere.fear":      0.3,
		"atmosphere.ominous":   0.3,
		"atmosphere.tension":   0.2,
		"atmosphere.dread":     0.2,
		"visuals.dark":         0.4,
		"visuals.distorted":    0.2,
		"visuals.glitchy":      0.1,
		"conditions.fog":       0.3,
		"conditions.darkness":  0.3,
		"conditions.unnatural": 0.1,
	}

	// Если сцена не инициализирована, возвращаем базовые значения
	if e.procedural == nil || e.procedural.currentScene == nil {
		return result
	}

	scene := e.procedural.currentScene

	// Влияние времени суток
	timeOfDay := scene.TimeOfDay
	isDark := timeOfDay < 0.25 || timeOfDay > 0.75 // ночь или вечер

	if isDark {
		result["conditions.darkness"] = 0.7
		result["atmosphere.fear"] = 0.5
		result["visuals.dark"] = 0.7
	}

	// Влияние погоды
	if fogLevel, ok := scene.Weather["fog"]; ok && fogLevel > 0.5 {
		result["conditions.fog"] = fogLevel
		result["atmosphere.ominous"] += 0.2
		result["visuals.distorted"] += 0.1
	}

	// Влияние близких объектов - собираем все объекты в радиусе видимости
	const detectionRadius = 15.0
	nearbyObjects := 0
	totalMood := map[string]float64{}

	for _, obj := range scene.Objects {
		dist := Vector3Distance(playerPos, obj.Position)

		if dist < detectionRadius {
			nearbyObjects++

			// Влияние объекта уменьшается с расстоянием
			influence := 1.0 - (dist / detectionRadius)

			// Аккумулируем метаданные объекта
			for key, value := range obj.Metadata {
				totalMood[key] += value * influence
			}

			// Особая обработка для "странных" объектов
			if obj.Type == "strange" {
				result["atmosphere.fear"] += 0.2 * influence
				result["conditions.unnatural"] += 0.3 * influence
				result["visuals.distorted"] += 0.2 * influence
			}
		}
	}

	// Усредняем влияние объектов
	if nearbyObjects > 0 {
		for key, value := range totalMood {
			if _, exists := result[key]; exists {
				result[key] = (result[key] + (value / float64(nearbyObjects))) / 2.0
			} else {
				result[key] = value / float64(nearbyObjects)
			}
		}
	}

	// Ограничиваем значения диапазоном [0, 1]
	for key, value := range result {
		if value < 0 {
			result[key] = 0
		} else if value > 1 {
			result[key] = 1
		}
	}

	return result
}

// processEnvironmentTriggers обрабатывает триггеры окружения
func (e *Engine) processEnvironmentTriggers(playerPos Vector3, deltaTime float64) {
	// Если сцена не инициализирована, пропускаем
	if e.procedural == nil || e.procedural.currentScene == nil {
		return
	}

	scene := e.procedural.currentScene

	// Проверяем расстояние до "странных" объектов для триггеров страха
	for _, obj := range scene.Objects {
		if obj.Type == "strange" {
			dist := Vector3Distance(playerPos, obj.Position)

			// Близкий объект вызывает реакцию
			if dist < 5.0 && e.audioEngine.CanPlayEffect("scare") {
				intensity := 0.5 + (5.0-dist)/5.0*0.5 // 0.5-1.0 в зависимости от расстояния

				scareMeta := map[string]float64{
					"atmosphere.fear":      0.8,
					"atmosphere.tension":   0.9,
					"visuals.distorted":    0.7,
					"conditions.unnatural": 0.8,
				}

				e.audioEngine.PlayProceduralSound("scare", float32(intensity), 0.0, scareMeta)
				e.logger.Debug("Scare triggered by strange object at distance %.2f", dist)

				// Случайное искажение изображения при испуге
				if intensity > 0.7 {
					e.renderer.ApplyGlitchEffect(0.5, 0.3)
				}
			}
		}
	}

	// Бродячие звуки в темноте
	if scene.TimeOfDay < 0.25 || scene.TimeOfDay > 0.75 { // ночь или вечер
		if e.audioEngine.CanPlayEffect("ambient") && rand.Float64() < 0.01*deltaTime {
			// Случайное направление для звука
			angle := rand.Float64() * 2 * math.Pi
			distance := 5.0 + rand.Float64()*10.0

			// Вычисляем панораму звука (-1 до 1)
			pan := float32(math.Sin(angle))

			// Воспроизводим звук с соответствующими параметрами
			ambientMeta := map[string]float64{
				"atmosphere.fear":    0.3 + rand.Float64()*0.4,
				"atmosphere.ominous": 0.4 + rand.Float64()*0.3,
				"atmosphere.dread":   0.2 + rand.Float64()*0.3,
			}

			e.audioEngine.PlayProceduralSound("ambient", 0.4, pan, ambientMeta)
			e.logger.Debug("Ambient sound generated at direction %.2f, distance %.2f", angle, distance)
		}
	}
}

// render renders the current frame
func (e *Engine) render() {
	// Generate scene using raytracer
	scene := e.raytracer.TraceScene()

	// Ensure the scene was created correctly
	if scene == nil || len(scene.Pixels) == 0 {
		e.logger.Warn("Empty scene returned from raytracer")
	}

	// Render ASCII representation
	e.renderer.Render(scene)
}

// cleanup performs necessary cleanup before exiting
func (e *Engine) cleanup() {
	e.logger.Info("Shutting down engine...")
	e.audioEngine.Shutdown()
	e.renderer.Close()
	glfw.Terminate()
}

// Vector3Distance calculates distance between two 3D points
func Vector3Distance(a, b Vector3) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}
