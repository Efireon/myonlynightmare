package engine

import (
	"math"
	"math/rand"
	"time"

	noise "nightmare/internal/math"
)

// AudioPattern represents a type of procedural audio pattern
type AudioPattern string

const (
	AudioPatternAmbient    AudioPattern = "ambient"
	AudioPatternFootstep   AudioPattern = "footstep"
	AudioPatternCreature   AudioPattern = "creature"
	AudioPatternMechanical AudioPattern = "mechanical"
	AudioPatternWhisper    AudioPattern = "whisper"
	AudioPatternGlitch     AudioPattern = "glitch"
)

// ProceduralAudioGenerator generates procedural audio
type ProceduralAudioGenerator struct {
	noiseGen   *noise.NoiseGenerator
	sampleRate int
}

// NewProceduralAudioGenerator creates a new procedural audio generator
func NewProceduralAudioGenerator(sampleRate int) *ProceduralAudioGenerator {
	return &ProceduralAudioGenerator{
		noiseGen:   noise.NewNoiseGenerator(time.Now().UnixNano()),
		sampleRate: sampleRate,
	}
}

// GenerateAudio generates audio samples based on the given parameters
func (pag *ProceduralAudioGenerator) GenerateAudio(pattern AudioPattern, durationSeconds float64, metadata map[string]float64, seed int64) []float32 {
	// Use a new noise generator with the provided seed for deterministic output
	ng := noise.NewNoiseGenerator(seed)

	// Calculate number of samples
	numSamples := int(durationSeconds * float64(pag.sampleRate))
	samples := make([]float32, numSamples)

	// Generate appropriate audio pattern
	switch pattern {
	case AudioPatternAmbient:
		pag.generateAmbient(samples, metadata, ng)
	case AudioPatternFootstep:
		pag.generateFootstep(samples, metadata, ng)
	case AudioPatternCreature:
		pag.generateCreature(samples, metadata, ng)
	case AudioPatternMechanical:
		pag.generateMechanical(samples, metadata, ng)
	case AudioPatternWhisper:
		pag.generateWhisper(samples, metadata, ng)
	case AudioPatternGlitch:
		pag.generateGlitch(samples, metadata, ng)
	default:
		pag.generateAmbient(samples, metadata, ng) // Default to ambient
	}

	// Normalize the samples to avoid clipping
	pag.normalizeAudio(samples)

	return samples
}

// generateAmbient generates ambient background sounds
func (pag *ProceduralAudioGenerator) generateAmbient(samples []float32, metadata map[string]float64, ng *noise.NoiseGenerator) {
	numSamples := len(samples)

	// Extract relevant metadata values
	fear := getMetadataValue(metadata, "atmosphere.fear", 0.3)
	tension := getMetadataValue(metadata, "atmosphere.tension", 0.2)
	ominous := getMetadataValue(metadata, "atmosphere.ominous", 0.4)
	fog := getMetadataValue(metadata, "conditions.fog", 0.0)
	darkness := getMetadataValue(metadata, "conditions.darkness", 0.0)

	// Base frequency for the drone
	baseFreq := 50.0 + fear*100.0 // 50-150 Hz

	// Secondary frequencies
	secondFreq := baseFreq * 1.5
	thirdFreq := baseFreq * (1.0 + fear*0.5) // More dissonant with higher fear

	// LFO (Low Frequency Oscillator) rate
	lfoRate := 0.1 + tension*0.4 // 0.1-0.5 Hz

	// Wind noise amount (increased by fog)
	windAmount := 0.1 + fog*0.3

	for i := 0; i < numSamples; i++ {
		time := float64(i) / float64(pag.sampleRate)

		// LFO for amplitude modulation
		lfo := 0.5 + 0.5*math.Sin(2.0*math.Pi*lfoRate*time)

		// Base drone
		sample := float32(math.Sin(2.0*math.Pi*baseFreq*time) * 0.3)

		// Add secondary tones
		sample += float32(math.Sin(2.0*math.Pi*secondFreq*time) * 0.15 * lfo)
		sample += float32(math.Sin(2.0*math.Pi*thirdFreq*time) * 0.1 * (1.0 - lfo))

		// Wind noise (filtered using perlin noise)
		wind := float32(ng.Perlin1D(time*2.0, 12345) * windAmount)

		// Random occasional creaks
		if ng.RandomFloat() < 0.0001*ominous {
			creak := float32(math.Sin(2.0*math.Pi*200.0*time) * math.Exp(-time*20.0))
			sample += creak
		}

		// Combine
		sample += wind

		// Make audio darker when darkness is high
		if darkness > 0.5 {
			sample *= float32(1.0 - (darkness-0.5)*0.5) // Reduce volume by up to 25%
		}

		samples[i] = sample
	}
}

// generateFootstep generates footstep sounds
func (pag *ProceduralAudioGenerator) generateFootstep(samples []float32, metadata map[string]float64, ng *noise.NoiseGenerator) {
	numSamples := len(samples)

	// Extract relevant metadata values
	fear := getMetadataValue(metadata, "atmosphere.fear", 0.3)
	unnatural := getMetadataValue(metadata, "conditions.unnatural", 0.0)

	// Determine surface type based on metadata
	wetness := getMetadataValue(metadata, "conditions.wetness", 0.0)
	material := getMetadataValue(metadata, "surface.hardness", 0.5)

	// Attack and decay times
	attackTime := 0.01              // 10 milliseconds
	decayTime := 0.1 + material*0.2 // Harder surfaces have shorter decay

	// Main frequency
	baseFreq := 100.0 + material*200.0 // 100-300 Hz

	// Modify parameters for unnatural footsteps
	if unnatural > 0.5 {
		baseFreq *= 0.7  // Lower pitch
		decayTime *= 2.0 // Longer decay
	}

	for i := 0; i < numSamples; i++ {
		time := float64(i) / float64(pag.sampleRate)

		// Calculate envelope
		envelope := 0.0
		if time < attackTime {
			envelope = time / attackTime
		} else {
			envelope = math.Exp(-(time - attackTime) / decayTime)
		}

		// Base impact sound
		impact := math.Sin(2.0*math.Pi*baseFreq*time) * envelope

		// Add noise for texture
		noise := ng.RandomFloat()*2.0 - 1.0

		// Filter noise based on surface properties
		if wetness > 0.5 {
			// More high-frequency noise for wet surfaces
			noise *= math.Sin(2.0*math.Pi*1000.0*time) * envelope
		}

		// Add crunch for some surfaces
		crunch := 0.0
		if material < 0.3 { // Soft materials like leaves or gravel
			crunch = (ng.RandomFloat()*2.0 - 1.0) * envelope * 0.7
		}

		// Combine all components
		sample := impact + noise*0.3 + crunch

		// Apply fear factor
		if fear > 0.7 && i%2 == 0 {
			// Add slight distortion for high fear
			sample *= 1.1
		}

		samples[i] = float32(sample * envelope)
	}
}

// generateCreature generates creature sounds (growls, roars, etc.)
func (pag *ProceduralAudioGenerator) generateCreature(samples []float32, metadata map[string]float64, ng *noise.NoiseGenerator) {
	numSamples := len(samples)

	// Extract relevant metadata values
	fear := getMetadataValue(metadata, "atmosphere.fear", 0.5)
	unnatural := getMetadataValue(metadata, "conditions.unnatural", 0.3)
	tension := getMetadataValue(metadata, "atmosphere.tension", 0.4)

	// Creature characteristics
	size := getMetadataValue(metadata, "creature.size", 0.5)
	aggression := getMetadataValue(metadata, "creature.aggression", fear) // Default to fear level

	// Base frequency (lower = larger creature)
	baseFreq := 300.0 - size*200.0 // 100-300 Hz

	// Modulation parameters
	modRate := 4.0 + aggression*8.0 // 4-12 Hz
	modDepth := 0.3 + tension*0.4   // 0.3-0.7

	// Duration of different phases
	attackTime := 0.05
	sustainTime := 0.2 + aggression*0.3
	releaseTime := 0.1 + size*0.3

	// Total sound duration
	totalDuration := attackTime + sustainTime + releaseTime

	for i := 0; i < numSamples; i++ {
		time := float64(i) / float64(pag.sampleRate)

		// Stop generating after sound duration
		if time > totalDuration {
			break
		}

		// Calculate envelope
		envelope := 0.0
		if time < attackTime {
			envelope = time / attackTime
		} else if time < attackTime+sustainTime {
			envelope = 1.0
		} else {
			releaseProgress := (time - (attackTime + sustainTime)) / releaseTime
			envelope = 1.0 - releaseProgress
		}

		// Frequency modulation for growl effect
		frequencyMod := 1.0 + modDepth*math.Sin(2.0*math.Pi*modRate*time)
		currentFreq := baseFreq * frequencyMod

		// Generate base growl
		growl := math.Sin(2.0 * math.Pi * currentFreq * time)

		// Add harmonics
		growl += 0.5 * math.Sin(2.0*math.Pi*currentFreq*1.5*time)
		growl += 0.25 * math.Sin(2.0*math.Pi*currentFreq*2.0*time)

		// Add noise for texture
		noise := (ng.RandomFloat()*2.0 - 1.0) * 0.2

		// Add sub-frequency for larger creatures
		subBass := 0.0
		if size > 0.6 {
			subBass = math.Sin(2.0*math.Pi*(baseFreq*0.5)*time) * size * 0.4
		}

		// Add unnatural effects
		unnatural_effect := 0.0
		if unnatural > 0.5 {
			// Add weird artifacts that don't sound natural
			unnatural_effect = math.Sin(2.0*math.Pi*1200.0*time) *
				math.Sin(2.0*math.Pi*1.5*time) * unnatural * 0.3
		}

		// Combine all components
		sample := (growl + noise + subBass + unnatural_effect) * envelope

		// Apply distortion for aggression
		if aggression > 0.6 {
			sample = math.Tanh(sample * (1.0 + aggression))
		}

		samples[i] = float32(sample)
	}
}

// generateMechanical generates mechanical/industrial sounds
func (pag *ProceduralAudioGenerator) generateMechanical(samples []float32, metadata map[string]float64, ng *noise.NoiseGenerator) {
	numSamples := len(samples)

	// Extract relevant metadata values
	ominous := getMetadataValue(metadata, "atmosphere.ominous", 0.4)
	unnatural := getMetadataValue(metadata, "conditions.unnatural", 0.2)

	// Mechanical characteristics
	metallic := getMetadataValue(metadata, "mechanical.metallic", 0.7)
	rhythm := getMetadataValue(metadata, "mechanical.rhythm", 0.5)

	// Base frequency for metallic tones
	baseFreq := 200.0 + metallic*400.0 // 200-600 Hz

	// Rhythm parameters
	rhythmRate := 2.0 + rhythm*6.0 // 2-8 Hz

	for i := 0; i < numSamples; i++ {
		time := float64(i) / float64(pag.sampleRate)

		// Rhythmic pulses
		rhythmPulse := math.Pow(0.5+0.5*math.Sin(2.0*math.Pi*rhythmRate*time), 4.0) // Sharper pulse

		// Metallic resonances
		metal := math.Sin(2.0*math.Pi*baseFreq*time) * 0.3
		metal += math.Sin(2.0*math.Pi*(baseFreq*1.414)*time) * 0.2 // 2nd harmonic at sqrt(2)
		metal += math.Sin(2.0*math.Pi*(baseFreq*1.732)*time) * 0.1 // 3rd harmonic at sqrt(3)

		// Industrial noise
		noise := (ng.RandomFloat()*2.0 - 1.0) * 0.2

		// Combine with rhythm
		sample := metal*(0.3+0.7*rhythmPulse) + noise*rhythmPulse

		// Add ominous sub-frequency
		if ominous > 0.5 {
			subFreq := math.Sin(2.0*math.Pi*40.0*time) * ominous * 0.4
			sample += subFreq
		}

		// Add unnatural artifacts
		if unnatural > 0.4 && ng.RandomFloat() < 0.01 {
			// Random digital artifacts
			glitch := math.Sin(2.0*math.Pi*2000.0*time) * math.Exp(-time*100.0)
			sample += glitch * unnatural
		}

		samples[i] = float32(sample)
	}
}

// generateWhisper generates whisper/voice-like sounds
func (pag *ProceduralAudioGenerator) generateWhisper(samples []float32, metadata map[string]float64, ng *noise.NoiseGenerator) {
	numSamples := len(samples)

	// Extract relevant metadata values
	fear := getMetadataValue(metadata, "atmosphere.fear", 0.6)
	unnatural := getMetadataValue(metadata, "conditions.unnatural", 0.4)

	// Voice characteristics
	pitch := getMetadataValue(metadata, "voice.pitch", 0.5)
	intensity := getMetadataValue(metadata, "voice.intensity", fear) // Default to fear level

	// Base formant frequencies for whispers
	f1 := 500.0 - pitch*200.0  // First formant: 300-500 Hz
	f2 := 1800.0 + pitch*400.0 // Second formant: 1800-2200 Hz
	f3 := 2500.0 + pitch*500.0 // Third formant: 2500-3000 Hz

	// Modulation rates
	ampModRate := 4.0 + intensity*8.0     // 4-12 Hz
	formantModRate := 1.0 + intensity*3.0 // 1-4 Hz

	// Generate several whisper "syllables"
	numSyllables := 2 + int(intensity*4) // 2-6 syllables
	syllableDuration := float64(numSamples) / float64(pag.sampleRate) / float64(numSyllables)

	for i := 0; i < numSamples; i++ {
		time := float64(i) / float64(pag.sampleRate)

		// Determine which syllable we're in
		syllableIndex := int(time / syllableDuration)
		if syllableIndex >= numSyllables {
			syllableIndex = numSyllables - 1
		}

		// Time within the current syllable
		syllableTime := time - float64(syllableIndex)*syllableDuration

		// Syllable envelope
		attackTime := 0.1 * syllableDuration
		releaseTime := 0.2 * syllableDuration
		envelope := 0.0

		if syllableTime < attackTime {
			envelope = syllableTime / attackTime
		} else if syllableTime > (syllableDuration - releaseTime) {
			envelope = (syllableDuration - syllableTime) / releaseTime
		} else {
			envelope = 1.0
		}

		// Base noise for whisper
		noise := (ng.RandomFloat()*2.0 - 1.0)

		// Create seed for consistent syllables
		syllableSeed := int64(syllableIndex + 1234)

		// Modulate formant frequencies to simulate speech
		formantMod := 0.8 + 0.2*math.Sin(2.0*math.Pi*formantModRate*(time+float64(syllableSeed)))

		// Filter noise through formants
		filtered := 0.0

		// First formant (low frequency)
		filtered += noise * 0.6 * math.Exp(-math.Pow((baseFreqToAngular(f1*formantMod)-baseFreqToAngular(100.0+ng.Perlin1D(time*3.0+float64(syllableSeed), syllableSeed)*300.0)), 2.0)/10000.0)

		// Second formant (mid frequency)
		filtered += noise * 0.3 * math.Exp(-math.Pow((baseFreqToAngular(f2*formantMod)-baseFreqToAngular(1500.0+ng.Perlin1D(time*4.0+100.0+float64(syllableSeed), syllableSeed+1)*500.0)), 2.0)/20000.0)

		// Third formant (high frequency)
		filtered += noise * 0.1 * math.Exp(-math.Pow((baseFreqToAngular(f3*formantMod)-baseFreqToAngular(2500.0+ng.Perlin1D(time*5.0+200.0+float64(syllableSeed), syllableSeed+2)*500.0)), 2.0)/40000.0)

		// Amplitude modulation
		ampMod := 0.7 + 0.3*math.Sin(2.0*math.Pi*ampModRate*time)

		// Add unnatural effects
		if unnatural > 0.6 {
			// Reversed fragments
			reverseFactor := math.Sin(2.0 * math.Pi * 0.5 * time)
			if reverseFactor > 0.7 {
				timeIndex := numSamples - i
				if timeIndex >= 0 && timeIndex < numSamples && i > 0 {
					filtered = filtered*0.5 + float64(samples[timeIndex])*0.5*unnatural
				}
			}

			// Octave jump artifacts
			if ng.RandomFloat() < 0.005*unnatural {
				filtered *= 2.0
			}
		}

		// Combine everything
		sample := filtered * envelope * ampMod

		// Apply intensity
		sample *= (0.5 + intensity*0.5)

		samples[i] = float32(sample)
	}
}

// generateGlitch generates digital glitch/error sounds
func (pag *ProceduralAudioGenerator) generateGlitch(samples []float32, metadata map[string]float64, ng *noise.NoiseGenerator) {
	numSamples := len(samples)

	// Extract relevant metadata values
	glitchy := getMetadataValue(metadata, "visuals.glitchy", 0.7)
	distorted := getMetadataValue(metadata, "visuals.distorted", 0.5)

	// Glitch characteristics
	density := getMetadataValue(metadata, "glitch.density", glitchy)     // Default to glitchy level
	severity := getMetadataValue(metadata, "glitch.severity", distorted) // Default to distorted level

	// Number of glitch events
	numGlitches := 3 + int(density*10) // 3-13 glitches

	// Create base samples (silence)
	for i := range samples {
		samples[i] = 0
	}

	// Generate glitch events
	for g := 0; g < numGlitches; g++ {
		// Determine glitch position and duration
		glitchStart := ng.RandomFloat() * 0.8                  // First 80% of the sample
		glitchDuration := 0.01 + ng.RandomFloat()*0.1*severity // 10-110ms

		startSample := int(glitchStart * float64(numSamples))
		durationSamples := int(glitchDuration * float64(pag.sampleRate))
		endSample := startSample + durationSamples
		if endSample > numSamples {
			endSample = numSamples
		}

		// Choose glitch type
		glitchType := rand.Intn(5)

		switch glitchType {
		case 0: // Digital noise burst
			for i := startSample; i < endSample; i++ {
				samples[i] = float32(ng.RandomFloat()*2.0-1.0) * float32(severity)
			}

		case 1: // Sine wave artifact
			freq := 500.0 + ng.RandomFloat()*1500.0
			for i := startSample; i < endSample; i++ {
				time := float64(i) / float64(pag.sampleRate)
				samples[i] = float32(math.Sin(2.0*math.Pi*freq*time)) * float32(severity)
			}

		case 2: // Sample repeat/stutter
			if startSample > 100 {
				stutter := 10 + int(severity*20) // 10-30 samples
				for i := startSample; i < endSample; i++ {
					sampleIndex := startSample - stutter + (i-startSample)%stutter
					if sampleIndex >= 0 && sampleIndex < numSamples {
						samples[i] = samples[sampleIndex]
					}
				}
			}

		case 3: // Bit crush effect
			bitDepth := 2 + int(8*(1.0-severity)) // 2-10 bits
			quantizationLevels := float32(math.Pow(2, float64(bitDepth)))
			for i := startSample; i < endSample; i++ {
				if i > 0 {
					// Quantize the previous sample
					samples[i] = float32(math.Floor(float64(samples[i-1])*float64(quantizationLevels))) / quantizationLevels
				}
			}

		case 4: // Frequency shift
			shift := 500.0 + ng.RandomFloat()*1000.0
			for i := startSample; i < endSample; i++ {
				time := float64(i) / float64(pag.sampleRate)
				if i > 0 {
					// Apply frequency shift by multiplying by a complex exponential
					carrier := math.Sin(2.0 * math.Pi * shift * time)
					samples[i] = samples[i] * float32(carrier)
				}
			}
		}

		// Apply envelope to avoid clicks
		fadeTime := int(0.005 * float64(pag.sampleRate)) // 5ms fade
		for i := 0; i < fadeTime; i++ {
			// Fade in
			if startSample+i < numSamples {
				fade := float32(i) / float32(fadeTime)
				samples[startSample+i] *= fade
			}

			// Fade out
			if endSample-fadeTime+i < numSamples {
				fade := float32(fadeTime-i) / float32(fadeTime)
				samples[endSample-fadeTime+i] *= fade
			}
		}
	}

	// Add background digital noise
	noiseLevel := 0.05 + density*0.1
	for i := range samples {
		if samples[i] == 0 && ng.RandomFloat() < density*0.1 {
			samples[i] = float32((ng.RandomFloat()*2.0 - 1.0) * noiseLevel)
		}
	}
}

// normalizeAudio normalizes audio samples to avoid clipping
func (pag *ProceduralAudioGenerator) normalizeAudio(samples []float32) {
	// Find maximum amplitude
	maxAmp := float32(0)
	for _, sample := range samples {
		if math.Abs(float64(sample)) > float64(maxAmp) {
			maxAmp = float32(math.Abs(float64(sample)))
		}
	}

	// Normalize if needed
	if maxAmp > 1.0 {
		for i := range samples {
			samples[i] /= maxAmp
		}
	}

	// If maximum amplitude is too low, boost it
	if maxAmp < 0.1 {
		gain := 0.7 / maxAmp
		for i := range samples {
			samples[i] *= gain
		}
	}
}

// baseFreqToAngular converts a frequency in Hz to angular frequency
func baseFreqToAngular(freqHz float64) float64 {
	return 2.0 * math.Pi * freqHz
}

// GenerateAmbientSoundscape generates a complete ambient soundscape
func (pag *ProceduralAudioGenerator) GenerateAmbientSoundscape(durationSeconds float64, metadata map[string]float64, ng *noise.NoiseGenerator) []float32 {
	// Create a layered soundscape with multiple elements
	numSamples := int(durationSeconds * float64(pag.sampleRate))
	result := make([]float32, numSamples)

	// Base ambient layer
	ambient := pag.GenerateAudio(AudioPatternAmbient, durationSeconds, metadata, time.Now().UnixNano())
	for i := range ambient {
		result[i] += ambient[i] * 0.7 // Main ambient at 70% volume
	}

	// Extract key mood values
	fear := getMetadataValue(metadata, "atmosphere.fear", 0.3)
	ominous := getMetadataValue(metadata, "atmosphere.ominous", 0.4)

	// Add mechanical sounds if ominous
	if ominous > 0.4 {
		// How many mechanical sounds to add
		numMechanical := 1 + int(ominous*3) // 1-4 sounds

		for i := 0; i < numMechanical; i++ {
			// Random start time
			startTime := ng.RandomFloat() * (durationSeconds * 0.7) // First 70% of the soundscape
			startSample := int(startTime * float64(pag.sampleRate))

			// Generate a short mechanical sound
			mech := pag.GenerateAudio(AudioPatternMechanical, 1.0+ominous*2.0, metadata, time.Now().UnixNano()+int64(i*1000))

			// Add it to the result
			for j := 0; j < len(mech) && startSample+j < numSamples; j++ {
				result[startSample+j] += mech[j] * 0.4 // Mechanical at 40% volume
			}
		}
	}

	// Add creature sounds if fearful
	if fear > 0.5 {
		// How many creature sounds to add
		numCreatures := 1 + int(fear*2) // 1-3 sounds

		for i := 0; i < numCreatures; i++ {
			// Random start time
			startTime := ng.RandomFloat() * (durationSeconds * 0.8) // First 80% of the soundscape
			startSample := int(startTime * float64(pag.sampleRate))

			// Generate a creature sound
			creature := pag.GenerateAudio(AudioPatternCreature, 0.5+fear, metadata, time.Now().UnixNano()+int64(i*2000))

			// Add it to the result
			for j := 0; j < len(creature) && startSample+j < numSamples; j++ {
				result[startSample+j] += creature[j] * 0.5 // Creature at 50% volume
			}
		}
	}

	// Add whispers for extreme fear
	if fear > 0.7 {
		// How many whispers to add
		numWhispers := 1 + int(fear*3) // 1-4 whispers

		for i := 0; i < numWhispers; i++ {
			// Random start time with preference for later in the soundscape
			startTime := 0.3*durationSeconds + ng.RandomFloat()*(durationSeconds*0.6)
			startSample := int(startTime * float64(pag.sampleRate))

			// Generate a whisper
			whisper := pag.GenerateAudio(AudioPatternWhisper, 0.3+fear*0.5, metadata, time.Now().UnixNano()+int64(i*3000))

			// Add it to the result
			for j := 0; j < len(whisper) && startSample+j < numSamples; j++ {
				result[startSample+j] += whisper[j] * float32(0.3+fear*0.3) // Whisper volume increases with fear
			}
		}
	}

	// Add glitches for visual distortion correlation
	glitchy := getMetadataValue(metadata, "visuals.glitchy", 0.0)
	if glitchy > 0.5 {
		// How many glitches to add
		numGlitches := 1 + int(glitchy*5) // 1-6 glitches

		for i := 0; i < numGlitches; i++ {
			// Random start time
			startTime := ng.RandomFloat() * durationSeconds
			startSample := int(startTime * float64(pag.sampleRate))

			// Generate a glitch
			glitch := pag.GenerateAudio(AudioPatternGlitch, 0.2+glitchy*0.3, metadata, time.Now().UnixNano()+int64(i*4000))

			// Add it to the result
			for j := 0; j < len(glitch) && startSample+j < numSamples; j++ {
				result[startSample+j] += glitch[j] * float32(0.4+glitchy*0.4) // Glitch volume increases with glitchiness
			}
		}
	}

	// Normalize the final result
	pag.normalizeAudio(result)

	return result
}
