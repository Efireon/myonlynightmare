package config

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Config represents the main configuration
type Config struct {
	Graphics   GraphicsConfig   `yaml:"graphics"`
	Audio      AudioConfig      `yaml:"audio"`
	Raytracer  RaytracerConfig  `yaml:"raytracer"`
	Renderer   RendererConfig   `yaml:"renderer"`
	Procedural ProceduralConfig `yaml:"procedural"`
	AI         AIConfig         `yaml:"ai"`
	Mods       ModsConfig       `yaml:"mods"`
}

// GraphicsConfig contains graphics-related configuration
type GraphicsConfig struct {
	Width       int    `yaml:"width"`
	Height      int    `yaml:"height"`
	Fullscreen  bool   `yaml:"fullscreen"`
	VSync       bool   `yaml:"vsync"`
	FrameRate   int    `yaml:"framerate"`
	DisplayMode string `yaml:"display_mode"` // ascii, opengl, hybrid
}

// AudioConfig contains audio-related configuration
type AudioConfig struct {
	Enabled        bool    `yaml:"enabled"`
	Volume         float64 `yaml:"volume"`
	EnableMic      bool    `yaml:"enable_mic"`
	MicSensitivity float64 `yaml:"mic_sensitivity"`
}

// RaytracerConfig contains raytracer configuration
type RaytracerConfig struct {
	Width          int  `yaml:"width"`
	Height         int  `yaml:"height"`
	NumThreads     int  `yaml:"num_threads"`
	MaxBounces     int  `yaml:"max_bounces"`
	ShadowsEnabled bool `yaml:"shadows_enabled"`
}

// RendererConfig contains renderer configuration
type RendererConfig struct {
	Width     int    `yaml:"width"`
	Height    int    `yaml:"height"`
	CharSet   string `yaml:"charset"`    // The set of ASCII characters to use for rendering
	ColorMode string `yaml:"color_mode"` // mono, grayscale, color
}

// ProceduralConfig contains procedural generation configuration
type ProceduralConfig struct {
	TerrainSize int     `yaml:"terrain_size"`
	TreeDensity float64 `yaml:"tree_density"`
	RockDensity float64 `yaml:"rock_density"`
	DetailLevel int     `yaml:"detail_level"`
	Seed        int64   `yaml:"seed"` // Optional: 0 means random
}

// AIConfig contains AI-related configuration
type AIConfig struct {
	Enabled          bool    `yaml:"enabled"`
	Difficulty       float64 `yaml:"difficulty"` // 0.0-1.0
	AdaptationRate   float64 `yaml:"adaptation_rate"`
	MicEnabled       bool    `yaml:"mic_enabled"`
	BehaviorAnalysis bool    `yaml:"behavior_analysis"`
}

// ModsConfig contains mod-related configuration
type ModsConfig struct {
	Enabled     bool     `yaml:"enabled"`
	ModsFolder  string   `yaml:"mods_folder"`
	EnabledMods []string `yaml:"enabled_mods"`
}

// MetadataConfig represents the hierarchical configuration for metadata
type MetadataConfig struct {
	Atmosphere struct {
		Fear    float64 `yaml:"fear"`
		Ominous float64 `yaml:"ominous"`
		Dread   float64 `yaml:"dread"`
		Tension float64 `yaml:"tension"`
	} `yaml:"atmosphere"`

	Visuals struct {
		Distorted float64 `yaml:"distorted"`
		Dark      float64 `yaml:"dark"`
		Glitchy   float64 `yaml:"glitchy"`
		Twisted   float64 `yaml:"twisted"`
	} `yaml:"visuals"`

	Conditions struct {
		Fog        float64 `yaml:"fog"`
		Shadow     float64 `yaml:"shadow"`
		Silhouette float64 `yaml:"silhouette"`
		Darkness   float64 `yaml:"darkness"`
		Unnatural  float64 `yaml:"unnatural"`
	} `yaml:"conditions"`
}

// DefaultConfig creates a default configuration
func DefaultConfig() *Config {
	return &Config{
		Graphics: GraphicsConfig{
			Width:       800,
			Height:      600,
			Fullscreen:  false,
			VSync:       true,
			FrameRate:   60,
			DisplayMode: "ascii",
		},
		Audio: AudioConfig{
			Enabled:        true,
			Volume:         0.8,
			EnableMic:      true,
			MicSensitivity: 0.7,
		},
		Raytracer: RaytracerConfig{
			Width:          120,
			Height:         60,
			NumThreads:     4,
			MaxBounces:     1,
			ShadowsEnabled: true,
		},
		Renderer: RendererConfig{
			Width:     120,
			Height:    60,
			CharSet:   " .:-=+*#%@",
			ColorMode: "grayscale",
		},
		Procedural: ProceduralConfig{
			TerrainSize: 128,
			TreeDensity: 5.0,
			RockDensity: 3.0,
			DetailLevel: 3,
			Seed:        0, // Random seed
		},
		AI: AIConfig{
			Enabled:          true,
			Difficulty:       0.5,
			AdaptationRate:   0.2,
			MicEnabled:       true,
			BehaviorAnalysis: true,
		},
		Mods: ModsConfig{
			Enabled:     true,
			ModsFolder:  "mods",
			EnabledMods: []string{},
		},
	}
}

// LoadConfig loads the configuration from a file
func LoadConfig(filePath string) (*Config, error) {
	// Create default config
	config := DefaultConfig()

	// Read file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return config, fmt.Errorf("config file not found, using defaults: %v", err)
	}

	// Parse YAML
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return config, fmt.Errorf("error parsing config: %v", err)
	}

	return config, nil
}

// SaveConfig saves the configuration to a file
func SaveConfig(config *Config, filePath string) error {
	// Convert to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("error serializing config: %v", err)
	}

	// Write file
	err = ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}

	return nil
}

// FlattenMetadata converts hierarchical metadata to a flat map
func FlattenMetadata(metadata *MetadataConfig) map[string]float64 {
	result := make(map[string]float64)

	// Atmosphere
	result["atmosphere.fear"] = metadata.Atmosphere.Fear
	result["atmosphere.ominous"] = metadata.Atmosphere.Ominous
	result["atmosphere.dread"] = metadata.Atmosphere.Dread
	result["atmosphere.tension"] = metadata.Atmosphere.Tension

	// Visuals
	result["visuals.distorted"] = metadata.Visuals.Distorted
	result["visuals.dark"] = metadata.Visuals.Dark
	result["visuals.glitchy"] = metadata.Visuals.Glitchy
	result["visuals.twisted"] = metadata.Visuals.Twisted

	// Conditions
	result["conditions.fog"] = metadata.Conditions.Fog
	result["conditions.shadow"] = metadata.Conditions.Shadow
	result["conditions.silhouette"] = metadata.Conditions.Silhouette
	result["conditions.darkness"] = metadata.Conditions.Darkness
	result["conditions.unnatural"] = metadata.Conditions.Unnatural

	return result
}

// ConvertFlatToMetadataConfig converts a flat metadata map to a hierarchical config
func ConvertFlatToMetadataConfig(flatMetadata map[string]float64) *MetadataConfig {
	result := &MetadataConfig{}

	// Atmosphere
	result.Atmosphere.Fear = flatMetadata["atmosphere.fear"]
	result.Atmosphere.Ominous = flatMetadata["atmosphere.ominous"]
	result.Atmosphere.Dread = flatMetadata["atmosphere.dread"]
	result.Atmosphere.Tension = flatMetadata["atmosphere.tension"]

	// Visuals
	result.Visuals.Distorted = flatMetadata["visuals.distorted"]
	result.Visuals.Dark = flatMetadata["visuals.dark"]
	result.Visuals.Glitchy = flatMetadata["visuals.glitchy"]
	result.Visuals.Twisted = flatMetadata["visuals.twisted"]

	// Conditions
	result.Conditions.Fog = flatMetadata["conditions.fog"]
	result.Conditions.Shadow = flatMetadata["conditions.shadow"]
	result.Conditions.Silhouette = flatMetadata["conditions.silhouette"]
	result.Conditions.Darkness = flatMetadata["conditions.darkness"]
	result.Conditions.Unnatural = flatMetadata["conditions.unnatural"]

	return result
}
