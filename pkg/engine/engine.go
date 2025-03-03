package engine

import (
	"fmt"
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
}

// NewEngine creates a new game engine instance
func NewEngine(cfg *config.Config, log *logger.Logger) (*Engine, error) {
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

	// Initialize components
	raytracer, err := NewRaytracer(cfg.Raytracer)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize raytracer: %v", err)
	}

	renderer, err := NewASCIIRenderer(cfg.Renderer)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ASCII renderer: %v", err)
	}

	procedural, err := NewProceduralGenerator(cfg.Procedural)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize procedural generator: %v", err)
	}

	audioEngine, err := NewAudioEngine(cfg.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize audio engine: %v", err)
	}

	engine := &Engine{
		window:      window,
		config:      cfg,
		logger:      log,
		raytracer:   raytracer,
		renderer:    renderer,
		procedural:  procedural,
		audioEngine: audioEngine,
		isRunning:   false,
		frameRate:   cfg.Graphics.FrameRate,
	}

	return engine, nil
}

// Run starts the main game loop
func (e *Engine) Run() {
	e.isRunning = true
	e.lastUpdate = time.Now()

	// Setup initial world
	e.procedural.GenerateInitialWorld()

	// Установка сцены для рейтрейсера
	e.raytracer.SetScene(e.procedural.GetCurrentScene())

	// Main game loop
	for e.isRunning && !e.window.ShouldClose() {
		currentTime := time.Now()
		deltaTime := currentTime.Sub(e.lastUpdate).Seconds()
		e.lastUpdate = currentTime

		// Check for input
		e.processInput()

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
// processInput handles user input
func (e *Engine) processInput() {
	// Close the game when ESC is pressed
	if e.window.GetKey(glfw.KeyEscape) == glfw.Press {
		e.isRunning = false
	}

	// Параметры движения камеры
	moveSpeed := 0.2    // Скорость движения
	rotateSpeed := 0.02 // Скорость поворота

	// Создаем вектор движения
	movement := Vector3{X: 0, Y: 0, Z: 0}

	// Обработка клавиш движения
	if e.window.GetKey(glfw.KeyW) == glfw.Press {
		movement.Z += moveSpeed // Вперед
	}
	if e.window.GetKey(glfw.KeyS) == glfw.Press {
		movement.Z -= moveSpeed // Назад
	}
	if e.window.GetKey(glfw.KeyA) == glfw.Press {
		movement.X -= moveSpeed // Влево
	}
	if e.window.GetKey(glfw.KeyD) == glfw.Press {
		movement.X += moveSpeed // Вправо
	}
	if e.window.GetKey(glfw.KeySpace) == glfw.Press {
		movement.Y += moveSpeed // Вверх
	}
	if e.window.GetKey(glfw.KeyLeftControl) == glfw.Press || e.window.GetKey(glfw.KeyRightControl) == glfw.Press {
		movement.Y -= moveSpeed // Вниз
	}

	// Обработка клавиш поворота
	yawDelta := 0.0
	pitchDelta := 0.0

	if e.window.GetKey(glfw.KeyRight) == glfw.Press {
		yawDelta += rotateSpeed // Поворот вправо
	}
	if e.window.GetKey(glfw.KeyLeft) == glfw.Press {
		yawDelta -= rotateSpeed // Поворот влево
	}
	if e.window.GetKey(glfw.KeyUp) == glfw.Press {
		pitchDelta -= rotateSpeed // Поворот вверх
	}
	if e.window.GetKey(glfw.KeyDown) == glfw.Press {
		pitchDelta += rotateSpeed // Поворот вниз
	}

	// Обновляем камеру
	e.raytracer.UpdateCamera(movement, yawDelta, pitchDelta)
}

// update updates the game state
func (e *Engine) update(deltaTime float64) {
	// Update procedural generation
	e.procedural.Update(deltaTime)

	// Обновляем сцену в рейтрейсере после изменений процедурной генерации
	e.raytracer.SetScene(e.procedural.GetCurrentScene())

	// Update audio
	e.audioEngine.Update(deltaTime)

	// Other updates...
}

// render renders the current frame
// render renders the current frame
func (e *Engine) render() {
	// Clear the screen
	// gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// Generate scene using raytracer
	scene := e.raytracer.TraceScene()

	// Убедимся, что сцена была создана корректно
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
	glfw.Terminate()
}
