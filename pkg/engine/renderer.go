package engine

// Renderer defines the interface for all renderers
type Renderer interface {
	// Render renders the scene
	Render(scene *SceneData)

	// UpdateResolution updates the rendering resolution
	UpdateResolution(width, height int)

	// ApplyGlitchEffect applies a glitch visual effect
	ApplyGlitchEffect(amount, duration float32)

	// SetVignetteAmount sets the vignette effect intensity
	SetVignetteAmount(amount float32)

	// SetNoiseAmount sets the noise effect intensity
	SetNoiseAmount(amount float32)

	// TogglePostProcessing enables or disables post-processing effects
	TogglePostProcessing()

	// Close releases resources
	Close()
}
