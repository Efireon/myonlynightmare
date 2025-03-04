package engine

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"

	"nightmare/internal/logger"
	"nightmare/pkg/config"
)

// In engine.go

type Engine struct {
	window      *glfw.Window
	config      *config.Config
	logger      *logger.Logger
	raytracer   *Raytracer
	renderer    Renderer // Using Renderer interface
	procedural  *ProceduralGenerator
	audioEngine *AudioEngine
	physics     *PhysicsSystem
	isRunning   bool
	lastUpdate  time.Time
	frameRate   int
	input       *InputHandler
	// Window dimensions
	windowWidth  int
	windowHeight int
	// Performance monitoring
	frameCount     int
	lastFpsCheck   time.Time
	currentFps     int
	framesPerCheck int
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

	// Initialize OpenGL - Add this line
	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenGL: %v", err)
	}

	// Create engine
	engine := &Engine{
		window:         window,
		config:         cfg,
		logger:         log,
		isRunning:      false,
		frameRate:      cfg.Graphics.FrameRate,
		windowWidth:    cfg.Graphics.Width,
		windowHeight:   cfg.Graphics.Height,
		frameCount:     0,
		lastFpsCheck:   time.Now(),
		framesPerCheck: 30,
	}

	// Add resize

	window.SetSizeCallback(glfw.SizeCallback(func(w *glfw.Window, width, height int) {
		engine.resizeCallback(w, width, height)
	}))

	// Create input handler
	engine.input = NewInputHandler(window)

	// Initialize components
	raytracer, err := NewRaytracer(cfg.Raytracer)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize raytracer: %v", err)
	}
	engine.raytracer = raytracer

	// Create OpenGL renderer
	renderer, err := NewOpenGLRenderer(cfg.Renderer)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenGL renderer: %v", err)
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
		// Create a dummy audio engine
		audioEngine = &AudioEngine{
			isRunning: false,
		}
	}
	engine.audioEngine = audioEngine

	// Initialize physics system
	engine.physics = NewPhysicsSystem()

	return engine, nil
}

// Run starts the main game loop
func (e *Engine) Run() {
	e.logger.Info("Starting engine.Run()")
	e.isRunning = true
	e.lastUpdate = time.Now()

	// Set up world
	e.logger.Info("Starting world generation")
	e.procedural.GenerateInitialWorld()
	e.logger.Info("World generation completed")

	// Set up physics
	e.physics.SetScene(e.procedural.GetCurrentScene())

	// Set up camera
	e.logger.Info("Setting up camera")
	cameraHeight := 1.7 // Eye height
	e.raytracer.SetCameraPosition(Vector3{X: 0, Y: cameraHeight, Z: -5})
	e.logger.Info("Camera setup completed")

	// Initialize atmosphere
	e.logger.Info("Generating atmosphere")
	metadata := map[string]float64{
		"atmosphere.fear":    0.2,
		"atmosphere.ominous": 0.3,
		"visuals.dark":       0.5,
		"conditions.fog":     0.3,
	}
	e.audioEngine.GenerateAtmosphere(metadata)
	e.logger.Info("Atmosphere generation completed")

	frameCount := 0
	lastFpsTime := time.Now()

	e.logger.Info("Entering main game loop")
	for e.isRunning && !e.window.ShouldClose() {
		frameCount++
		currentTime := time.Now()

		// Display FPS every second
		if currentTime.Sub(lastFpsTime) >= time.Second {
			e.logger.Info("FPS: %d", frameCount)
			frameCount = 0
			lastFpsTime = currentTime
		}

		deltaTime := currentTime.Sub(e.lastUpdate).Seconds()
		e.lastUpdate = currentTime

		// Process input
		e.input.Update()
		e.processInput(deltaTime)

		// Update physics
		e.physics.Update(deltaTime)

		// Update camera from physics
		playerPos := e.physics.GetPlayer().Position
		viewDir := e.physics.GetPlayer().Direction
		e.raytracer.SetCameraPosition(playerPos)

		// Calculate yaw and pitch from direction vector
		yaw := math.Atan2(viewDir.X, viewDir.Z)
		pitch := math.Asin(math.Max(-0.99, math.Min(0.99, viewDir.Y)))
		e.raytracer.RotateCamera(yaw, pitch)

		// Update game state
		e.update(deltaTime)

		// Render frame
		e.render()

		// Swap buffers and poll events
		e.window.SwapBuffers()
		glfw.PollEvents()

		// Cap frame rate
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

// Vector3Distance calculates the distance between two Vector3 points
func Vector3Distance(a, b Vector3) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// render renders the current frame
func (e *Engine) render() {
	// Generate scene using raytracer
	scene := e.raytracer.TraceScene()

	// Check for valid scene
	if scene == nil || len(scene.Pixels) == 0 {
		e.logger.Warn("Empty scene returned from raytracer")
	}

	// Render scene
	e.renderer.Render(scene)

	// Update FPS counter
	e.frameCount++
	if e.frameCount >= e.framesPerCheck {
		currentTime := time.Now()
		elapsed := currentTime.Sub(e.lastFpsCheck).Seconds()
		e.currentFps = int(float64(e.frameCount) / elapsed)

		// Log if FPS is low
		if e.currentFps < 30 {
			e.logger.Warn("Low frame rate detected: %d FPS", e.currentFps)
		}

		// Reset counters
		e.frameCount = 0
		e.lastFpsCheck = currentTime
	}
}

// cleanup performs cleanup before exiting
func (e *Engine) cleanup() {
	e.logger.Info("Shutting down engine...")
	e.audioEngine.Shutdown()
	e.renderer.Close()
	glfw.Terminate()
}

// Missing functions in engine.go

// analyzeEnvironment analyzes the environment around the player
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

	// If scene is not initialized, return base values
	if e.procedural == nil || e.procedural.currentScene == nil {
		return result
	}

	scene := e.procedural.currentScene

	// Time of day influence
	timeOfDay := scene.TimeOfDay
	isDark := timeOfDay < 0.25 || timeOfDay > 0.75 // night or evening

	if isDark {
		result["conditions.darkness"] = 0.7
		result["atmosphere.fear"] = 0.5
		result["visuals.dark"] = 0.7
	}

	// Weather influence
	if weather := scene.Weather; weather != nil {
		if fogLevel, ok := weather["fog"]; ok && fogLevel > 0.5 {
			result["conditions.fog"] = fogLevel
			result["atmosphere.ominous"] += 0.2
			result["visuals.distorted"] += 0.1
		}
	}

	// Influence of nearby objects
	const detectionRadius = 15.0
	nearbyObjects := 0
	totalMood := make(map[string]float64)

	for _, obj := range scene.Objects {
		dist := Vector3Distance(playerPos, obj.Position)

		if dist < detectionRadius {
			nearbyObjects++

			// Influence decreases with distance
			influence := 1.0 - (dist / detectionRadius)

			// Accumulate object metadata
			for key, value := range obj.Metadata {
				totalMood[key] += value * influence
			}

			// Special handling for "strange" objects
			if obj.Type == "strange" {
				result["atmosphere.fear"] += 0.2 * influence
				result["conditions.unnatural"] += 0.3 * influence
				result["visuals.distorted"] += 0.2 * influence
			}
		}
	}

	// Average object influence
	if nearbyObjects > 0 {
		for key, value := range totalMood {
			if _, exists := result[key]; exists {
				result[key] = (result[key] + (value / float64(nearbyObjects))) / 2.0
			} else {
				result[key] = value / float64(nearbyObjects)
			}
		}
	}

	// Limit values to 0-1 range
	for key, value := range result {
		result[key] = math.Max(0, math.Min(1, value))
	}

	return result
}

// processEnvironmentTriggers processes environment triggers
func (e *Engine) processEnvironmentTriggers(playerPos Vector3, deltaTime float64) {
	// Skip if scene is not initialized
	if e.procedural == nil || e.procedural.currentScene == nil {
		return
	}

	scene := e.procedural.currentScene

	// Check distance to "strange" objects for fear triggers
	for _, obj := range scene.Objects {
		if obj.Type == "strange" {
			dist := Vector3Distance(playerPos, obj.Position)

			// Close object triggers reaction
			if dist < 5.0 && e.audioEngine.CanPlayEffect("scare") {
				intensity := 0.5 + (5.0-dist)/5.0*0.5 // 0.5-1.0 based on distance

				scareMeta := map[string]float64{
					"atmosphere.fear":      0.8,
					"atmosphere.tension":   0.9,
					"visuals.distorted":    0.7,
					"conditions.unnatural": 0.8,
				}

				e.audioEngine.PlayProceduralSound("scare", float32(intensity), 0.0, scareMeta)
				e.logger.Debug("Scare triggered by strange object at distance %.2f", dist)

				// Random image distortion when scared
				if intensity > 0.7 {
					e.renderer.ApplyGlitchEffect(0.5, 0.3)
				}
			}
		}
	}

	// Wandering sounds in darkness
	if scene.TimeOfDay < 0.25 || scene.TimeOfDay > 0.75 { // night or evening
		if e.audioEngine.CanPlayEffect("ambient") && rand.Float64() < 0.01*deltaTime {
			// Random direction for sound
			angle := rand.Float64() * 2 * math.Pi
			distance := 5.0 + rand.Float64()*10.0

			// Calculate sound pan (-1 to 1)
			pan := float32(math.Sin(angle))

			// Play sound with appropriate parameters
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

// Fixed processInput method to remove unused variables
func (e *Engine) processInput(deltaTime float64) {
	// Close game on ESC
	if e.input.IsKeyPressed(glfw.KeyEscape) {
		e.isRunning = false
		return
	}

	// Process movement (WASD)
	if e.input.IsKeyDown(glfw.KeyW) {
		// Move forward
		e.physics.MoveForward(e.input.IsKeyDown(glfw.KeyLeftShift))
	}
	if e.input.IsKeyDown(glfw.KeyS) {
		// Move backward
		e.physics.MoveBackward()
	}
	if e.input.IsKeyDown(glfw.KeyA) {
		// Strafe left
		e.physics.MoveLeft()
	}
	if e.input.IsKeyDown(glfw.KeyD) {
		// Strafe right
		e.physics.MoveRight()
	}

	// Rotation (arrow keys)
	rotateSpeed := 1.5 * deltaTime
	if e.input.IsKeyDown(glfw.KeyLeft) {
		e.physics.RotateLeft(deltaTime)
	}
	if e.input.IsKeyDown(glfw.KeyRight) {
		e.physics.RotateRight(deltaTime)
	}
	if e.input.IsKeyDown(glfw.KeyUp) {
		// Look up - use raytracer directly
		e.raytracer.RotateCamera(0, -rotateSpeed)
	}
	if e.input.IsKeyDown(glfw.KeyDown) {
		// Look down - use raytracer directly
		e.raytracer.RotateCamera(0, rotateSpeed)
	}

	// Jump
	if e.input.IsKeyPressed(glfw.KeySpace) {
		e.physics.Jump()

		// Generate interaction sound
		interactMeta := map[string]float64{
			"atmosphere.fear":    0.5,
			"atmosphere.tension": 0.6,
			"visuals.distorted":  0.3,
		}
		e.audioEngine.PlayProceduralSound("interact", 0.7, 0.0, interactMeta)
	}

	// Toggle post-processing effects
	if e.input.IsKeyPressed(glfw.KeyP) {
		e.renderer.TogglePostProcessing()
		e.logger.Info("Post-processing toggled")
	}

	// Audio volume controls
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

// Fixed update method to handle scene.SpecialEffects access
func (e *Engine) update(deltaTime float64) {
	// Update procedural generation
	e.procedural.Update(deltaTime)

	// Pass current scene to raytracer
	if e.procedural.currentScene != nil {
		e.raytracer.SetScene(e.procedural.currentScene)
	}

	// Update audio
	e.audioEngine.Update(deltaTime)

	// Get player position
	playerPos := e.raytracer.GetCameraPosition()

	// Analyze environment around player
	environmentMood := e.analyzeEnvironment(playerPos)

	// Update atmosphere based on environment
	if deltaTime > 0 && int(e.lastUpdate.Second())%10 == 0 {
		e.audioEngine.UpdateAtmosphere(environmentMood, 5.0) // Smooth transition over 5 seconds
	}

	// Process environment triggers
	e.processEnvironmentTriggers(playerPos, deltaTime)

	// Update renderer effects based on scene conditions
	scene := e.procedural.GetCurrentScene()
	if scene != nil {
		// Create a map for special effects derived from atmospherics
		specialEffects := make(map[string]float64)

		// Add fog from weather
		if scene.Weather != nil {
			if fogAmount, ok := scene.Weather["fog"]; ok {
				specialEffects["fog"] = fogAmount

				// Lower noise in foggy scenes for better visibility
				e.renderer.SetNoiseAmount(float32(0.02 * (1.0 - fogAmount*0.5)))
				// Increase vignette in foggy scenes
				e.renderer.SetVignetteAmount(float32(0.4 + fogAmount*0.2))
			}
		}

		// Add darkness based on time of day
		timeOfDay := scene.TimeOfDay
		if timeOfDay < 0.25 || timeOfDay > 0.75 { // night or evening
			var darkness float64
			if timeOfDay < 0.25 { // night
				darkness = 1.0 - (timeOfDay / 0.25 * 4.0)
			} else { // evening
				darkness = (timeOfDay - 0.75) / 0.25 * 4.0
			}

			specialEffects["darkness"] = darkness

			// Increase vignette in dark scenes
			if darkness > 0.5 {
				e.renderer.SetVignetteAmount(float32(0.4 + (darkness-0.5)*0.3))
			}
		}

		// Add fear based on atmosphere
		if scene.Atmosphere != nil {
			if fear, ok := scene.Atmosphere["atmosphere.fear"]; ok {
				specialEffects["fear"] = fear
			}
		}
	}
}
