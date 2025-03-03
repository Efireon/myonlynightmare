package engine

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
	
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gordonklaus/portaudio"
	
	"nightmare/internal/math/noise"
	"nightmare/pkg/config"
)

const (
	sampleRate      = 44100
	framesPerBuffer = 1024
	numChannels     = 2
)

// AudioEngine handles all audio operations
type AudioEngine struct {
	config         config.AudioConfig
	noiseGen       *noise.NoiseGenerator
	stream         *portaudio.Stream
	micStream      *portaudio.Stream
	audioBuffer    []float32
	micBuffer      []float32
	volume         float32
	effectsVolume  float32
	masterMutex    sync.Mutex
	isRunning      bool
	activeSounds   map[string]*Sound
	micEnabled     bool
	micAnalyzer    *MicrophoneAnalyzer
}

// Sound represents a sound that can be played
type Sound struct {
	ID          string
	Samples     []float32
	Position    float32
	Volume      float32
	Pan         float32
	Loop        bool
	Playing     bool
	Metadata    map[string]float64
	Seed        int64
}

// MicrophoneAnalyzer analyzes microphone input
type MicrophoneAnalyzer struct {
	Buffer           []float32
	BufferSize       int
	ProcessedBuffer  []float32
	LastVolume       float32
	VolumeThreshold  float32
	HasSpokenRecently bool
	LastSpeakTime    time.Time
}

// NewAudioEngine creates a new audio engine
func NewAudioEngine(config config.AudioConfig) (*AudioEngine, error) {
	// Initialize PortAudio
	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize PortAudio: %v", err)
	}
	
	// Create audio engine
	engine := &AudioEngine{
		config:        config,
		noiseGen:      noise.NewNoiseGenerator(time.Now().UnixNano()),
		audioBuffer:   make([]float32, framesPerBuffer*numChannels),
		micBuffer:     make([]float32, framesPerBuffer),
		volume:        float32(config.Volume),
		effectsVolume: float32(config.Volume) * 0.8,
		activeSounds:  make(map[string]*Sound),
		micEnabled:    config.EnableMic,
	}
	
	// If mic is enabled, initialize microphone
	if engine.micEnabled {
		engine.micAnalyzer = &MicrophoneAnalyzer{
			Buffer:           make([]float32, framesPerBuffer*10), // Store 10 buffers worth of data
			ProcessedBuffer:  make([]float32, framesPerBuffer*10),
			BufferSize:       framesPerBuffer * 10,
			VolumeThreshold:  0.02, // Threshold for detecting speech
			HasSpokenRecently: false,
			LastSpeakTime:    time.Now(),
		}
		
		if err := engine.initMicrophone(); err != nil {
			fmt.Printf("Warning: Failed to initialize microphone: %v\n", err)
			engine.micEnabled = false
		}
	}
	
	// Initialize audio output
	if err := engine.initAudio(); err != nil {
		return nil, fmt.Errorf("failed to initialize audio: %v", err)
	}
	
	return engine, nil
}

// initAudio initializes the audio output
func (ae *AudioEngine) initAudio() error {
	var err error
	
	// Create audio stream
	ae.stream, err = portaudio.OpenDefaultStream(0, numChannels, sampleRate, framesPerBuffer, ae.audioCallback)
	if err != nil {
		return fmt.Errorf("failed to open audio stream: %v", err)
	}
	
	// Start audio stream
	if err := ae.stream.Start(); err != nil {
		return fmt.Errorf("failed to start audio stream: %v", err)
	}
	
	ae.isRunning = true
	return nil
}

// initMicrophone initializes the microphone input
func (ae *AudioEngine) initMicrophone() error {
	var err error
	
	// Create microphone stream
	ae.micStream, err = portaudio.OpenDefaultStream(1, 0, sampleRate, framesPerBuffer, ae.microphoneCallback)
	if err != nil {
		return fmt.Errorf("failed to open microphone stream: %v", err)
	}
	
	// Start microphone stream
	if err := ae.micStream.Start(); err != nil {
		return fmt.Errorf("failed to start microphone stream: %v", err)
	}
	
	return nil
}

// audioCallback is called by PortAudio to fill the audio buffer
func (ae *AudioEngine) audioCallback(out []float32) {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()
	
	// Clear buffer
	for i := range out {
		out[i] = 0
	}
	
	// Mix active sounds
	for id, sound := range ae.activeSounds {
		if !sound.Playing {
			continue
		}
		
		// Mix this sound into the output buffer
		for i := 0; i < len(out); i += numChannels {
			// Get sample position
			samplePos := int(sound.Position)
			
			// Check if we've reached the end of the sound
			if samplePos >= len(sound.Samples) {
				if sound.Loop {
					// Loop back to beginning
					sound.Position = 0
					samplePos = 0
				} else {
					// Sound finished playing
					sound.Playing = false
					delete(ae.activeSounds, id)
					break
				}
			}
			
			// Get sample value
			sample := sound.Samples[samplePos] * sound.Volume * ae.volume
			
			// Apply panning
			if numChannels == 2 {
				// Left channel
				out[i] += sample * (1 - sound.Pan)
				// Right channel
				out[i+1] += sample * (1 + sound.Pan)
			} else {
				// Mono output
				out[i] += sample
			}
			
			// Increment position
			sound.Position++
		}
	}
	
	// Process microphone effects if enabled
	if ae.micEnabled && ae.micAnalyzer.HasSpokenRecently {
		// Add processed microphone audio as an effect
		for i := 0; i < len(out); i += numChannels {
			// Get a processed sample from the microphone buffer
			micIndex := (i/numChannels + int(time.Now().UnixNano()/1000000)) % len(ae.micAnalyzer.ProcessedBuffer)
			sample := ae.micAnalyzer.ProcessedBuffer[micIndex] * ae.effectsVolume
			
			// Add to both channels
			out[i] += sample
			if numChannels > 1 {
				out[i+1] += sample
			}
		}
	}
	
	// Apply master effects
	for i := range out {
		// Apply some soft clipping to avoid harsh distortion
		if out[i] > 1.0 {
			out[i] = 1.0 - (1.0 / (1.0 + out[i] - 1.0))
		} else if out[i] < -1.0 {
			out[i] = -1.0 + (1.0 / (1.0 - out[i] - 1.0))
		}
	}
}

// microphoneCallback is called by PortAudio to process microphone input
func (ae *AudioEngine) microphoneCallback(in []float32) {
	if !ae.micEnabled || ae.micAnalyzer == nil {
		return
	}
	
	// Calculate average volume of this buffer
	sum := float32(0)
	for _, sample := range in {
		sum += math.Abs(float32(sample))
	}
	avgVolume := sum / float32(len(in))
	
	// Update analyzer
	ae.micAnalyzer.LastVolume = avgVolume
	
	// Check if the user is speaking
	if avgVolume > ae.micAnalyzer.VolumeThreshold {
		ae.micAnalyzer.HasSpokenRecently = true
		ae.micAnalyzer.LastSpeakTime = time.Now()
		
		// Process the microphone input to create a "creepy" version
		for i, sample := range in {
			// Copy to the circular buffer
			bufferIndex := (int(time.Now().Unix