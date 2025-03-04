package engine

import (
	"sync"
	"time"

	"nightmare/pkg/config"
)

// HybridRenderer combines ASCII and OpenGL rendering for better performance
type HybridRenderer struct {
	config          config.RendererConfig
	asciiRenderer   *ASCIIRenderer
	openglRenderer  *OpenGLRenderer
	useOpenGL       bool
	usePostProcess  bool
	frameCounter    int
	performanceMode bool
	mutex           sync.Mutex
	lastSwitch      time.Time
}

// NewHybridRenderer creates a new hybrid renderer
func NewHybridRenderer(config config.RendererConfig) (*HybridRenderer, error) {
	// Create ASCII renderer
	asciiRenderer, err := NewASCIIRenderer(config)
	if err != nil {
		return nil, err
	}

	// Create OpenGL renderer
	openglRenderer, err := NewOpenGLRenderer(config)
	if err != nil {
		return nil, err
	}

	return &HybridRenderer{
		config:          config,
		asciiRenderer:   asciiRenderer,
		openglRenderer:  openglRenderer,
		useOpenGL:       true, // Start with OpenGL rendering
		usePostProcess:  true, // Enable post-processing by default
		frameCounter:    0,
		performanceMode: false, // Start in quality mode
		lastSwitch:      time.Now(),
	}, nil
}

// UpdateResolution updates the resolution of both renderers
func (hr *HybridRenderer) UpdateResolution(width, height int) {
	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	hr.asciiRenderer.UpdateResolution(width, height)
	hr.openglRenderer.UpdateResolution(width, height)
}

// SetRenderingMode sets the rendering mode (OpenGL vs ASCII)
func (hr *HybridRenderer) SetRenderingMode(useOpenGL bool) {
	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	hr.useOpenGL = useOpenGL
}

// TogglePostProcessing enables or disables post-processing effects
func (hr *HybridRenderer) TogglePostProcessing() {
	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	hr.usePostProcess = !hr.usePostProcess

	// When disabling post-processing, disable some effects
	if !hr.usePostProcess {
		hr.openglRenderer.SetNoiseAmount(0.0)
		hr.openglRenderer.SetVignetteAmount(0.1) // Minimal vignette
	} else {
		hr.openglRenderer.SetNoiseAmount(0.03)
		hr.openglRenderer.SetVignetteAmount(0.4)
	}
}

// TogglePerformanceMode toggles between performance and quality modes
func (hr *HybridRenderer) TogglePerformanceMode() {
	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	hr.performanceMode = !hr.performanceMode
}

// ApplyGlitchEffect applies a glitch effect to both renderers
func (hr *HybridRenderer) ApplyGlitchEffect(amount, duration float32) {
	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	hr.asciiRenderer.ApplyGlitchEffect(amount, duration)
	hr.openglRenderer.ApplyGlitchEffect(amount, duration)
}

// Render renders the scene using the currently active renderer
func (hr *HybridRenderer) Render(scene *SceneData) {
	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	hr.frameCounter++

	// Auto-switch between renderers in performance mode
	if hr.performanceMode {
		// Check if we should switch renderers
		if time.Since(hr.lastSwitch) >= 5*time.Second {
			// In performance mode, alternate between OpenGL and ASCII
			// to maintain higher frame rates during intensive scenes
			hr.useOpenGL = !hr.useOpenGL
			hr.lastSwitch = time.Now()
		}
	}

	// Render with the active renderer
	if hr.useOpenGL {
		// Apply progressive enhancement for special effects
		if scene != nil && len(scene.SpecialEffects) > 0 {
			// Adjust effect intensity based on scene conditions
			if fogAmount, ok := scene.SpecialEffects["fog"]; ok {
				// Lower noise in foggy scenes for better visibility
				hr.openglRenderer.SetNoiseAmount(float32(0.02 * (1.0 - fogAmount*0.5)))
				// Increase vignette in foggy scenes
				hr.openglRenderer.SetVignetteAmount(float32(0.4 + fogAmount*0.2))
			}

			// Increase vignette in dark scenes
			if darkness, ok := scene.SpecialEffects["darkness"]; ok && darkness > 0.5 {
				hr.openglRenderer.SetVignetteAmount(float32(0.4 + (darkness-0.5)*0.3))
			}
		}

		hr.openglRenderer.Render(scene)
	} else {
		hr.asciiRenderer.Render(scene)
	}
}

// Close releases all resources used by the renderers
func (hr *HybridRenderer) Close() {
	hr.asciiRenderer.Close()
	hr.openglRenderer.Close()
}
