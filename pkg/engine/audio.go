package engine

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"

	noise "nightmare/internal/math"
	"nightmare/pkg/config"
)

const (
	sampleRate      = 44100
	framesPerBuffer = 1024
	numChannels     = 2
)

// AudioEngine handles all audio operations
type AudioEngine struct {
	config        config.AudioConfig
	noiseGen      *noise.NoiseGenerator
	stream        *portaudio.Stream
	micStream     *portaudio.Stream
	audioBuffer   []float32
	micBuffer     []float32
	volume        float32
	effectsVolume float32
	masterMutex   sync.Mutex
	isRunning     bool
	activeSounds  map[string]*Sound
	micEnabled    bool
	micAnalyzer   *MicrophoneAnalyzer
}

// Sound represents a sound that can be played
type Sound struct {
	ID       string
	Samples  []float32
	Position float32
	Volume   float32
	Pan      float32
	Loop     bool
	Playing  bool
	Metadata map[string]float64
	Seed     int64
}

// MicrophoneAnalyzer analyzes microphone input
type MicrophoneAnalyzer struct {
	Buffer            []float32
	BufferSize        int
	ProcessedBuffer   []float32
	LastVolume        float32
	VolumeThreshold   float32
	HasSpokenRecently bool
	LastSpeakTime     time.Time
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
			Buffer:            make([]float32, framesPerBuffer*10), // Store 10 buffers worth of data
			ProcessedBuffer:   make([]float32, framesPerBuffer*10),
			BufferSize:        framesPerBuffer * 10,
			VolumeThreshold:   0.02, // Threshold for detecting speech
			HasSpokenRecently: false,
			LastSpeakTime:     time.Now(),
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
		sum += float32(math.Abs(float64(sample)))
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
			bufferIndex := (int(time.Now().UnixNano())/1000000 + i) % ae.micAnalyzer.BufferSize
			ae.micAnalyzer.Buffer[bufferIndex] = sample

			// Apply "creepy" effects to the sample
			processedSample := sample

			// Apply reverb (simple delay with feedback)
			if i > 100 {
				delayIndex := (bufferIndex - 100) % ae.micAnalyzer.BufferSize
				if delayIndex < 0 {
					delayIndex += ae.micAnalyzer.BufferSize
				}
				processedSample += ae.micAnalyzer.Buffer[delayIndex] * 0.6
			}

			// Apply pitch shift (crude implementation)
			pitchShiftIndex := int(float32(i)*0.8) % len(in)
			if pitchShiftIndex < len(in) {
				processedSample = in[pitchShiftIndex]
			}

			// Apply distortion
			processedSample = float32(math.Tanh(float64(processedSample) * 2.0))

			// Store the processed sample
			ae.micAnalyzer.ProcessedBuffer[bufferIndex] = processedSample
		}
	} else {
		// Check if we should stop considering the user as speaking
		timeSinceLastSpeak := time.Since(ae.micAnalyzer.LastSpeakTime)
		if timeSinceLastSpeak > 2*time.Second {
			ae.micAnalyzer.HasSpokenRecently = false
		}
	}
}

// Update updates the audio engine state
func (ae *AudioEngine) Update(deltaTime float64) {
	// Check if microphone has detected speech and update effects accordingly
	if ae.micEnabled && ae.micAnalyzer.HasSpokenRecently {
		// Maybe trigger some additional creepy sounds when the player speaks
		if rand.Float32() < 0.05 { // 5% chance per update
			ae.PlayProceduralSound("response", 1.0, 0.0, map[string]float64{
				"atmosphere.fear":    0.8,
				"atmosphere.tension": 0.9,
				"visuals.distorted":  0.7,
			})
		}
	}

	// Update any time-based effects
}

// PlaySound plays a sound with the given parameters
func (ae *AudioEngine) PlaySound(id string, samples []float32, volume, pan float32, loop bool, metadata map[string]float64) {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	// Create sound
	sound := &Sound{
		ID:       id,
		Samples:  samples,
		Position: 0,
		Volume:   volume,
		Pan:      pan,
		Loop:     loop,
		Playing:  true,
		Metadata: metadata,
		Seed:     time.Now().UnixNano(),
	}

	// Add to active sounds
	ae.activeSounds[id] = sound
}

// StopSound stops the sound with the given ID
func (ae *AudioEngine) StopSound(id string) {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	if sound, ok := ae.activeSounds[id]; ok {
		sound.Playing = false
		delete(ae.activeSounds, id)
	}
}

// PlayProceduralSound generates and plays a procedural sound based on metadata
func (ae *AudioEngine) PlayProceduralSound(id string, volume, pan float32, metadata map[string]float64) {
	// Generate procedural sound based on metadata
	seed := time.Now().UnixNano()
	samples := ae.generateProceduralSound(seed, metadata)

	// Play the sound
	ae.PlaySound(id, samples, volume, pan, false, metadata)
}

// generateProceduralSound generates a procedural sound based on metadata
func (ae *AudioEngine) generateProceduralSound(seed int64, metadata map[string]float64) []float32 {
	// Create a local noise generator with the given seed
	ng := noise.NewNoiseGenerator(seed)

	// Determine sound characteristics from metadata
	// Fear affects pitch and dissonance
	fearLevel := float32(metadata["atmosphere.fear"])
	if fearLevel == 0 {
		fearLevel = 0.2 // Default value
	}

	// Tension affects rhythm and beats
	tensionLevel := float32(metadata["atmosphere.tension"])
	if tensionLevel == 0 {
		tensionLevel = 0.3 // Default value
	}

	// Distortion affects signal processing
	distortionLevel := float32(metadata["visuals.distorted"])
	if distortionLevel == 0 {
		distortionLevel = 0.1 // Default value
	}

	// Determine sound duration based on tension
	durationSecs := 1.0 + float64(tensionLevel*3.0) // 1-4 seconds
	numSamples := int(durationSecs * sampleRate)

	// Create samples buffer
	samples := make([]float32, numSamples)

	// Generate base sound
	baseFreq := 100.0 + float64(fearLevel*400.0) // 100-500 Hz

	// Add multiple sin waves with slight detuning for dissonance
	for i := 0; i < numSamples; i++ {
		t := float64(i) / sampleRate

		// Base tone
		sample := float32(math.Sin(2.0 * math.Pi * baseFreq * t))

		// Add detuned oscillators for dissonance
		if fearLevel > 0.3 {
			detune := 1.0 + float64(fearLevel*0.1)
			sample += float32(math.Sin(2.0*math.Pi*baseFreq*detune*t)) * 0.3
		}

		if fearLevel > 0.6 {
			detune := 1.0 - float64(fearLevel*0.2)
			sample += float32(math.Sin(2.0*math.Pi*baseFreq*detune*t)) * 0.2
		}

		// Add noise for texture
		noise := float32(ng.RandomFloat()*2.0-1.0) * distortionLevel * 0.4
		sample += noise

		// Apply amplitude envelope (attack-decay-sustain-release)
		normalizedTime := float64(i) / float64(numSamples)
		envelope := float32(0)

		// Calculate ADSR envelope
		attackTime := 0.1
		decayTime := 0.2
		sustainLevel := 0.6
		releaseTime := 0.3

		if normalizedTime < attackTime {
			// Attack phase
			envelope = float32(normalizedTime / attackTime)
		} else if normalizedTime < attackTime+decayTime {
			// Decay phase
			decayProgress := (normalizedTime - attackTime) / decayTime
			envelope = float32(1.0 - (1.0-sustainLevel)*decayProgress)
		} else if normalizedTime < 1.0-releaseTime {
			// Sustain phase
			envelope = float32(sustainLevel)
		} else {
			// Release phase
			releaseProgress := (normalizedTime - (1.0 - releaseTime)) / releaseTime
			envelope = float32(sustainLevel * (1.0 - releaseProgress))
		}

		// Apply tremolo (amplitude modulation) if tension is high
		if tensionLevel > 0.5 {
			tremolo := float32(math.Sin(2.0*math.Pi*5.0*t)) * tensionLevel * 0.3
			envelope *= (1.0 + tremolo)
		}

		// Apply envelope
		sample *= envelope

		// Apply distortion
		if distortionLevel > 0.2 {
			sample = float32(math.Tanh(float64(sample*distortionLevel*5.0))) * 0.7
		}

		// Store the sample
		samples[i] = sample
	}

	// Normalize audio to avoid clipping
	maxAmp := float32(0)
	for _, sample := range samples {
		if math.Abs(float64(sample)) > float64(maxAmp) {
			maxAmp = float32(math.Abs(float64(sample)))
		}
	}

	if maxAmp > 0 {
		for i := range samples {
			samples[i] /= maxAmp // Normalize to -1.0 to 1.0 range
		}
	}

	return samples
}

// Shutdown shuts down the audio engine
func (ae *AudioEngine) Shutdown() {
	if ae.stream != nil {
		ae.stream.Stop()
		ae.stream.Close()
	}

	if ae.micStream != nil {
		ae.micStream.Stop()
		ae.micStream.Close()
	}

	portaudio.Terminate()
}
