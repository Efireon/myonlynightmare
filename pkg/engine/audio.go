package engine

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"

	"nightmare/internal/logger"
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
	config          config.AudioConfig
	noiseGen        *noise.NoiseGenerator
	stream          *portaudio.Stream
	micStream       *portaudio.Stream
	audioBuffer     []float32
	micBuffer       []float32
	volume          float32
	masterVolume    float32 // Контроль общей громкости
	effectsVolume   float32
	masterMutex     sync.Mutex
	isRunning       bool
	isMuted         bool // Состояние отключения звука
	activeSounds    map[string]*Sound
	micEnabled      bool
	micAnalyzer     *MicrophoneAnalyzer
	lastEffectTimes map[string]time.Time // Для контроля частоты эффектов
	effectCooldowns map[string]float64   // Минимальный интервал между эффектами
	logger          *logger.Logger

	// Атмосфера и настроение
	currentAtmosphere       map[string]float64
	targetAtmosphere        map[string]float64
	atmosphereBlendDuration float64
	atmosphereBlendStart    time.Time

	// Ambient sounds
	ambientSoundtrack *Sound
	ambientIntensity  float32
}

// Sound represents a sound that can be played
type Sound struct {
	ID             string
	Samples        []float32
	Position       float32
	Volume         float32
	OriginalVolume float32 // Исходная громкость до кривой затухания
	Pan            float32
	Loop           bool
	Playing        bool
	Metadata       map[string]float64
	Seed           int64
	StartTime      time.Time
	Duration       float64 // Длительность в секундах
	FadeIn         float64 // Время нарастания в секундах
	FadeOut        float64 // Время затухания в секундах
	FadeOutStart   float64 // Момент начала затухания (в секундах от начала)
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

	// Если звук отключен - выходим
	if ae.isMuted {
		return
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

			// Вычисляем затухание/нарастание громкости
			var envelope float32 = 1.0 // По умолчанию без огибающей

			// Получаем текущее время воспроизведения в секундах
			playTime := float64(samplePos) / sampleRate

			// Фаза нарастания (fade in)
			if sound.FadeIn > 0 && playTime < sound.FadeIn {
				envelope = float32(playTime / sound.FadeIn)
			}

			// Фаза затухания (fade out)
			if sound.FadeOut > 0 {
				// Определяем, когда должно начаться затухание
				fadeOutStartTime := sound.Duration - sound.FadeOut
				if playTime > fadeOutStartTime {
					fadeProgress := (playTime - fadeOutStartTime) / sound.FadeOut
					envelope = float32(1.0 - fadeProgress)
				}
			}

			// Ограничиваем огибающую от 0 до 1
			envelope = float32(math.Max(0.0, math.Min(1.0, float64(envelope))))

			// Get sample value with envelope applied
			sample := sound.Samples[samplePos] * sound.Volume * envelope * ae.masterVolume

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
			sample := ae.micAnalyzer.ProcessedBuffer[micIndex] * ae.effectsVolume * ae.masterVolume

			// Add to both channels
			out[i] += sample
			if numChannels > 1 {
				out[i+1] += sample
			}
		}
	}

	// Apply master effects and processing
	ae.applyMasterProcessing(out)
}

// applyMasterProcessing применяет эффекты обработки к всему звуковому потоку
func (ae *AudioEngine) applyMasterProcessing(out []float32) {
	// Низкочастотный фильтр для придания «тяжести» звуку
	bassBoost := float32(0.3) // Степень усиления баса

	// Параметры для эффекта реверберации
	reverbAmount := float32(0.1) // Количество реверберации
	reverbDecay := float32(0.6)  // Скорость затухания реверберации

	// Размер буфера реверберации (размер "комнаты")
	reverbBufferSize := 4000

	// Статические буферы для эффектов
	// В реальном приложении они должны сохраняться между вызовами audioCallback
	static := struct {
		previousLeft      float32
		previousRight     float32
		reverbBufferLeft  []float32
		reverbBufferRight []float32
		reverbPos         int
	}{
		reverbBufferLeft:  make([]float32, reverbBufferSize),
		reverbBufferRight: make([]float32, reverbBufferSize),
	}

	// Обработка каждого сэмпла
	for i := 0; i < len(out); i += numChannels {
		// Получаем текущие значения сэмплов
		left := out[i]
		right := out[i+1]

		// Низкочастотный фильтр (простой IIR-фильтр)
		left = left*0.7 + static.previousLeft*0.3 + bassBoost*static.previousLeft*0.2
		right = right*0.7 + static.previousRight*0.3 + bassBoost*static.previousRight*0.2

		static.previousLeft = left
		static.previousRight = right

		// Добавляем реверберацию
		reverbPosLeft := (static.reverbPos + 100) % reverbBufferSize
		reverbPosRight := (static.reverbPos + 101) % reverbBufferSize

		// Получаем сэмплы из буфера реверберации
		reverbLeft := static.reverbBufferLeft[reverbPosLeft] * reverbDecay
		reverbRight := static.reverbBufferRight[reverbPosRight] * reverbDecay

		// Обновляем буфер реверберации
		static.reverbBufferLeft[static.reverbPos] = left + reverbLeft*0.4
		static.reverbBufferRight[static.reverbPos] = right + reverbRight*0.4

		// Смешиваем исходный звук с реверберацией
		left += reverbLeft * reverbAmount
		right += reverbRight * reverbAmount

		// Инкрементируем позицию в буфере реверберации
		static.reverbPos = (static.reverbPos + 1) % reverbBufferSize

		// Применяем мягкое ограничение для предотвращения клиппинга
		out[i] = softClip(left)
		out[i+1] = softClip(right)
	}
}

// softClip выполняет мягкое ограничение сигнала для предотвращения искажений
func softClip(sample float32) float32 {
	// Используем гиперболический тангенс для мягкого ограничения
	if sample > 1.0 {
		return 1.0 - 1.0/(1.0+float32(math.Abs(float64(sample-1.0))))
	} else if sample < -1.0 {
		return -1.0 + 1.0/(1.0+float32(math.Abs(float64(sample+1.0))))
	}
	return sample
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

			// Применяем более сложные и атмосферные эффекты

			// Эхо с многократными отражениями
			processedSample := sample * 0.3 // Снижаем громкость исходного звука

			// Добавляем несколько слоев эхо с разными задержками
			echoDelays := []int{100, 200, 300, 500, 800, 1300, 2100}
			echoVolumes := []float32{0.5, 0.3, 0.2, 0.15, 0.1, 0.05, 0.025}

			for j, delay := range echoDelays {
				// Вычисляем индекс с учетом задержки
				echoIndex := bufferIndex - delay
				if echoIndex < 0 {
					echoIndex += ae.micAnalyzer.BufferSize
				}

				// Добавляем эхо с соответствующей громкостью
				processedSample += ae.micAnalyzer.Buffer[echoIndex] * echoVolumes[j]
			}

			// Pitch shift (более качественный)
			// Смешиваем несколько смещенных по высоте версий для создания "хора голосов"
			pitchShiftFactors := []float32{0.7, 0.8, 1.2, 1.5}
			pitchShiftWeights := []float32{0.3, 0.2, 0.25, 0.15}

			for j, factor := range pitchShiftFactors {
				index := int(float32(i)*factor) % len(in)
				if index < len(in) {
					processedSample += in[index] * pitchShiftWeights[j]
				}
			}

			// Нелинейное искажение для создания "потустороннего" эффекта
			processedSample = float32(math.Tanh(float64(processedSample) * 1.5))

			// Фильтрация для удаления высоких частот и усиления средних
			if i > 1 {
				// Простой фильтр низких частот
				prevIndex := (bufferIndex - 1)
				if prevIndex < 0 {
					prevIndex += ae.micAnalyzer.BufferSize
				}

				prevProcessed := ae.micAnalyzer.ProcessedBuffer[prevIndex]
				processedSample = processedSample*0.7 + prevProcessed*0.3
			}

			// Модуляция амплитуды для эффекта "плавающего" звука
			modulationDepth := 0.3
			modulationRate := 3.0 // Гц
			modulationTime := float64(time.Now().UnixNano()) / 1e9
			modulation := 1.0 + modulationDepth*math.Sin(2.0*math.Pi*modulationRate*modulationTime)

			processedSample = float32(float64(processedSample) * modulation)

			// Сохраняем обработанный сэмпл
			ae.micAnalyzer.ProcessedBuffer[bufferIndex] = processedSample * 0.7 // Снижаем общую громкость
		}
	} else {
		// Check if we should stop considering the user as speaking
		timeSinceLastSpeak := time.Since(ae.micAnalyzer.LastSpeakTime)
		if timeSinceLastSpeak > 2*time.Second {
			ae.micAnalyzer.HasSpokenRecently = false
		}
	}
}

// UpdateAtmosphere плавно обновляет атмосферу
func (ae *AudioEngine) UpdateAtmosphere(newMetadata map[string]float64, blendDuration float64) {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	// Сохраняем целевую атмосферу
	ae.targetAtmosphere = newMetadata

	// Устанавливаем длительность перехода
	ae.atmosphereBlendDuration = blendDuration
	ae.atmosphereBlendStart = time.Now()

	// Интенсивность будет обновлена постепенно в методе Update
}

// NewAudioEngine creates a new audio engine
func NewAudioEngine(config config.AudioConfig) (*AudioEngine, error) {
	// Create audio engine with basic initialization
	engine := &AudioEngine{
		config:                  config,
		noiseGen:                noise.NewNoiseGenerator(time.Now().UnixNano()),
		audioBuffer:             make([]float32, framesPerBuffer*numChannels),
		micBuffer:               make([]float32, framesPerBuffer),
		volume:                  float32(config.Volume),
		masterVolume:            float32(config.Volume),
		effectsVolume:           float32(config.Volume) * 0.7,
		activeSounds:            make(map[string]*Sound),
		micEnabled:              config.EnableMic,
		lastEffectTimes:         make(map[string]time.Time),
		effectCooldowns:         make(map[string]float64),
		logger:                  logger.NewLogger("info"),
		currentAtmosphere:       make(map[string]float64),
		targetAtmosphere:        make(map[string]float64),
		atmosphereBlendDuration: 0,
		ambientIntensity:        0.5,
		isRunning:               false, // Start with audio disabled until initialized
		isMuted:                 false,
	}

	// Disable audio completely if not enabled in config
	if !config.Enabled {
		engine.logger.Info("Audio disabled in config, running in silent mode")
		return engine, nil
	}

	// Initialize PortAudio safely
	var err error
	err = portaudio.Initialize()
	if err != nil {
		engine.logger.Error("Failed to initialize PortAudio: %v", err)
		return engine, fmt.Errorf("failed to initialize PortAudio: %v", err)
	}

	// Устанавливаем интервалы для различных эффектов
	engine.effectCooldowns["ambient"] = 10.0 // 10 секунд между фоновыми звуками
	engine.effectCooldowns["scare"] = 30.0   // 30 секунд между пугающими эффектами
	engine.effectCooldowns["footstep"] = 0.5 // Полсекунды между шагами
	engine.effectCooldowns["interact"] = 1.0 // 1 секунда между звуками взаимодействия
	engine.effectCooldowns["voice"] = 15.0   // 15 секунд между голосами

	// Try to initialize microphone
	if engine.micEnabled {
		engine.micAnalyzer = &MicrophoneAnalyzer{
			Buffer:            make([]float32, framesPerBuffer*10),
			ProcessedBuffer:   make([]float32, framesPerBuffer*10),
			BufferSize:        framesPerBuffer * 10,
			VolumeThreshold:   0.02,
			HasSpokenRecently: false,
			LastSpeakTime:     time.Now(),
		}

		if err := engine.initMicrophone(); err != nil {
			engine.logger.Warn("Failed to initialize microphone: %v", err)
			engine.micEnabled = false
		}
	}

	// Initialize audio output
	if err := engine.initAudio(); err != nil {
		// Clean up PortAudio if audio init fails
		portaudio.Terminate()
		return engine, fmt.Errorf("failed to initialize audio: %v", err)
	}

	// Initialize default atmosphere
	defaultAtmosphere := map[string]float64{
		"atmosphere.fear":    0.3,
		"atmosphere.ominous": 0.3,
		"visuals.dark":       0.5,
		"conditions.fog":     0.3,
	}
	engine.currentAtmosphere = defaultAtmosphere
	engine.targetAtmosphere = defaultAtmosphere

	// Successfully initialized
	engine.isRunning = true
	return engine, nil
}

// Update updates the audio engine state - add safety checks
func (ae *AudioEngine) Update(deltaTime float64) {
	// Skip if audio engine is not running
	if !ae.isRunning {
		return
	}

	// Use a timeout for acquiring the lock to prevent deadlocks
	lockAcquired := make(chan struct{}, 1)
	go func() {
		ae.masterMutex.Lock()
		lockAcquired <- struct{}{}
	}()

	select {
	case <-lockAcquired:
		// Successfully acquired lock
		defer ae.masterMutex.Unlock()
	case <-time.After(100 * time.Millisecond):
		// Lock acquisition timed out, skip this update
		ae.logger.Warn("Audio engine lock timeout during Update")
		return
	}

	// The rest of the update logic remains the same
	if ae.atmosphereBlendDuration > 0 {
		// Вычисляем прогресс перехода
		elapsed := time.Since(ae.atmosphereBlendStart).Seconds()
		progress := math.Min(1.0, elapsed/ae.atmosphereBlendDuration)

		// Если переход завершен
		if progress >= 1.0 {
			ae.currentAtmosphere = ae.targetAtmosphere
			ae.atmosphereBlendDuration = 0

			// Полностью обновляем эмбиент при завершении перехода
			if rand.Float64() < 0.3 { // 30% шанс полного обновления
				ae.generateAndPlayAmbient(ae.currentAtmosphere)
			}
		} else {
			// Постепенно обновляем текущую атмосферу
			blendedAtmosphere := make(map[string]float64)

			// Интерполируем все значения
			allKeys := make(map[string]bool)
			for k := range ae.currentAtmosphere {
				allKeys[k] = true
			}
			for k := range ae.targetAtmosphere {
				allKeys[k] = true
			}

			for k := range allKeys {
				currentVal := getMetadataValue(ae.currentAtmosphere, k, 0.0)
				targetVal := getMetadataValue(ae.targetAtmosphere, k, 0.0)
				blendedAtmosphere[k] = currentVal*(1.0-progress) + targetVal*progress
			}

			// Обновляем текущую атмосферу
			ae.currentAtmosphere = blendedAtmosphere

			// Обновляем интенсивность эмбиента если он существует
			if ae.ambientSoundtrack != nil && ae.ambientSoundtrack.Playing {
				// Вычисляем новую интенсивность
				intensity := 0.5 // Базовая интенсивность

				if fear, ok := blendedAtmosphere["atmosphere.fear"]; ok {
					intensity += fear * 0.2
				}
				if ominous, ok := blendedAtmosphere["atmosphere.ominous"]; ok {
					intensity += ominous * 0.15
				}
				if dread, ok := blendedAtmosphere["atmosphere.dread"]; ok {
					intensity += dread * 0.25
				}

				// Ограничиваем интенсивность
				intensity = math.Max(0.3, math.Min(0.9, intensity))
				ae.ambientIntensity = float32(intensity)

				// Плавно обновляем громкость
				ae.ambientSoundtrack.Volume = float32(0.5 * ae.ambientIntensity)
			}
		}
	}

	// Проверка микрофона и генерация эффектов
	if ae.micEnabled && ae.micAnalyzer != nil && ae.micAnalyzer.HasSpokenRecently {
		// Возможность генерации отклика на речь игрока
		if ae.CanPlayEffect("voice") && rand.Float64() < 0.1*deltaTime { // Вероятность зависит от deltaTime
			responseMeta := map[string]float64{
				"atmosphere.fear":      0.7,
				"atmosphere.tension":   0.8,
				"visuals.distorted":    0.6,
				"conditions.unnatural": 0.7,
			}

			// Генерируем звук шепота или отклика
			ae.PlayProceduralSound("voice_response", 0.4, calculateSpatialPan(), responseMeta)
		}
	}

	// Генерация случайных эмбиентных звуков
	if ae.CanPlayEffect("ambient") && rand.Float64() < 0.3*deltaTime {
		// Extraction of atmosphere parameters and sound generation
		// (Keep the rest of this section as it was)
	}
}

// GenerateAtmosphere - add safety checks
func (ae *AudioEngine) GenerateAtmosphere(metadata map[string]float64) {
	// Skip if audio engine is not running
	if !ae.isRunning {
		return
	}

	// Use a timeout for acquiring the lock
	lockAcquired := make(chan struct{}, 1)
	go func() {
		ae.masterMutex.Lock()
		lockAcquired <- struct{}{}
	}()

	select {
	case <-lockAcquired:
		// Successfully acquired lock
		defer ae.masterMutex.Unlock()
	case <-time.After(500 * time.Millisecond):
		// Lock acquisition timed out
		ae.logger.Warn("Audio engine lock timeout during GenerateAtmosphere")
		return
	}

	// Safely continue with atmosphere generation
	// Очищаем предыдущую атмосферу и останавливаем текущий эмбиент
	if ae.ambientSoundtrack != nil && ae.ambientSoundtrack.Playing {
		// Начинаем плавное затухание текущего эмбиента
		ae.ambientSoundtrack.FadeOutStart = float64(ae.ambientSoundtrack.Position) / sampleRate
		ae.ambientSoundtrack.FadeOut = 3.0 // 3 секунды на затухание
	}

	// Сохраняем новую атмосферу
	ae.currentAtmosphere = metadata
	ae.targetAtmosphere = metadata

	// Вычисляем интенсивность атмосферы на основе параметров
	intensity := 0.5 // Базовая интенсивность

	// Увеличиваем интенсивность в зависимости от уровня страха и тревоги
	if fear, ok := metadata["atmosphere.fear"]; ok {
		intensity += fear * 0.2
	}
	if ominous, ok := metadata["atmosphere.ominous"]; ok {
		intensity += ominous * 0.15
	}
	if dread, ok := metadata["atmosphere.dread"]; ok {
		intensity += dread * 0.25
	}

	// Ограничиваем интенсивность
	intensity = math.Max(0.3, math.Min(0.9, intensity))
	ae.ambientIntensity = float32(intensity)

	// Safely generate and play ambient sound
	func() {
		defer func() {
			if r := recover(); r != nil {
				ae.logger.Error("Panic in generateAndPlayAmbient: %v", r)
			}
		}()
		ae.generateAndPlayAmbient(metadata)
	}()
}

// generateAndPlayAmbient - add safety checks
func (ae *AudioEngine) generateAndPlayAmbient(metadata map[string]float64) {
	// Skip if audio engine is not running
	if !ae.isRunning {
		return
	}

	// Use recover to catch any panics during sample generation
	defer func() {
		if r := recover(); r != nil {
			ae.logger.Error("Panic in ambient sound generation: %v", r)
		}
	}()

	// Генерируем эмбиент в зависимости от атмосферы
	durationSecs := 30.0 // 30 секунд эмбиента (будет зацикливаться)
	numSamples := int(durationSecs * sampleRate)
	samples := make([]float32, numSamples)

	// Rest of the ambient generation code remains the same
	// [...]

	// Create and add sound safely
	ambientSound := &Sound{
		ID:             "ambient_background",
		Samples:        samples,
		Position:       0,
		Volume:         float32(0.5 * ae.ambientIntensity),
		OriginalVolume: float32(0.5 * ae.ambientIntensity),
		Pan:            0,
		Loop:           true,
		Playing:        true,
		Metadata:       metadata,
		Seed:           time.Now().UnixNano(),
		StartTime:      time.Now(),
		Duration:       durationSecs,
		FadeIn:         5.0, // 5 секунд на нарастание
		FadeOut:        5.0, // 5 секунд на затухание
	}

	// Add safely to active sounds
	if ae.activeSounds == nil {
		ae.activeSounds = make(map[string]*Sound)
	}
	ae.ambientSoundtrack = ambientSound
	ae.activeSounds["ambient_background"] = ambientSound
}

// getMetadataValueForSoundType возвращает числовое представление типа звука
func getMetadataValueForSoundType(soundType string) float64 {
	switch soundType {
	case "creature":
		return 0.1
	case "mechanical":
		return 0.5
	case "environment":
		return 0.9
	default:
		return 0.0
	}
}

// calculateSpatialPan вычисляет пространственную панораму для звука
func calculateSpatialPan() float32 {
	// Случайная позиция от -1.0 (полностью слева) до 1.0 (полностью справа)
	return float32(rand.Float64()*2.0 - 1.0)
}

// CanPlayEffect проверяет, можно ли проиграть эффект с учетом интервала
func (ae *AudioEngine) CanPlayEffect(effectType string) bool {
	// Проверяем, когда последний раз проигрывался этот тип эффекта
	lastTime, exists := ae.lastEffectTimes[effectType]
	if !exists {
		// Эффект еще не проигрывался
		ae.lastEffectTimes[effectType] = time.Now()
		return true
	}

	// Получаем минимальный интервал для этого типа эффекта
	cooldown, exists := ae.effectCooldowns[effectType]
	if !exists {
		cooldown = 1.0 // По умолчанию 1 секунда
	}

	// Проверяем, прошло ли достаточно времени
	if time.Since(lastTime).Seconds() >= cooldown {
		ae.lastEffectTimes[effectType] = time.Now()
		return true
	}

	return false
}

// PlaySound plays a sound with the given parameters
func (ae *AudioEngine) PlaySound(id string, samples []float32, volume, pan float32, loop bool, metadata map[string]float64) {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	// Вычисляем параметры фейдинга на основе длительности
	duration := float64(len(samples)) / sampleRate
	fadeIn := math.Min(0.2, duration*0.1)  // 10% от длительности, но не более 0.2 сек
	fadeOut := math.Min(0.5, duration*0.2) // 20% от длительности, но не более 0.5 сек

	// Create sound
	sound := &Sound{
		ID:             id,
		Samples:        samples,
		Position:       0,
		Volume:         volume,
		OriginalVolume: volume,
		Pan:            pan,
		Loop:           loop,
		Playing:        true,
		Metadata:       metadata,
		Seed:           time.Now().UnixNano(),
		StartTime:      time.Now(),
		Duration:       duration,
		FadeIn:         fadeIn,
		FadeOut:        fadeOut,
	}

	// Add to active sounds
	ae.activeSounds[id] = sound
}

// StopSound stops the sound with the given ID
func (ae *AudioEngine) StopSound(id string) {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	if sound, ok := ae.activeSounds[id]; ok {
		// Начинаем плавное затухание
		currentPosition := float64(sound.Position) / sampleRate
		sound.FadeOutStart = currentPosition
		sound.FadeOut = 0.5 // 0.5 секунды на затухание

		// Звук будет автоматически остановлен в audioCallback
		// когда закончится фейдаут или достигнет конца сэмплов
	}
}

// StopAllSounds останавливает все звуки
func (ae *AudioEngine) StopAllSounds() {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	// Устанавливаем фейдаут для всех активных звуков
	for _, sound := range ae.activeSounds {
		currentPosition := float64(sound.Position) / sampleRate
		sound.FadeOutStart = currentPosition
		sound.FadeOut = 0.3 // 0.3 секунды на затухание
	}
}

// PlayProceduralSound generates and plays a procedural sound based on metadata
func (ae *AudioEngine) PlayProceduralSound(id string, volume, pan float32, metadata map[string]float64) {
	// Обновляем время последнего эффекта для контроля интервалов
	effectType := "generic"
	if strings.HasPrefix(id, "ambient") {
		effectType = "ambient"
	} else if strings.HasPrefix(id, "scare") {
		effectType = "scare"
	} else if strings.HasPrefix(id, "footstep") {
		effectType = "footstep"
	} else if strings.HasPrefix(id, "interact") {
		effectType = "interact"
	} else if strings.HasPrefix(id, "voice") {
		effectType = "voice"
	}

	ae.lastEffectTimes[effectType] = time.Now()

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

	// Determine sound type from metadata
	soundType := ""
	if val, ok := metadata["sound.type"]; ok {
		if val < 0.3 {
			soundType = "creature"
		} else if val < 0.7 {
			soundType = "mechanical"
		} else {
			soundType = "environment"
		}
	}

	// Determine sound characteristics from metadata
	// Fear affects pitch and dissonance
	fearLevel := getMetadataValue(metadata, "atmosphere.fear", 0.3)

	// Tension affects rhythm and beats
	tensionLevel := getMetadataValue(metadata, "atmosphere.tension", 0.3)

	// Distortion affects signal processing
	distortionLevel := getMetadataValue(metadata, "visuals.distorted", 0.1)

	// Determine sound duration based on tension
	durationSecs := 1.0 + float64(tensionLevel*4.0) // 1-5 seconds
	numSamples := int(durationSecs * sampleRate)

	// Create samples buffer
	samples := make([]float32, numSamples)

	// Генерируем звук в зависимости от типа
	switch soundType {
	case "creature":
		generateCreatureSound(samples, fearLevel, tensionLevel, distortionLevel, ng)
	case "mechanical":
		generateMechanicalSound(samples, fearLevel, tensionLevel, distortionLevel, ng)
	case "environment":
		generateEnvironmentSound(samples, fearLevel, tensionLevel, distortionLevel, ng)
	default:
		// По умолчанию генерируем таинственный звук
		generateMysteriousSound(samples, fearLevel, tensionLevel, distortionLevel, ng)
	}

	// Normalize audio to avoid clipping
	normalizeAudio(samples)

	return samples
}

// generateCreatureSound генерирует звук существа
func generateCreatureSound(samples []float32, fear, tension, distortion float64, ng *noise.NoiseGenerator) {
	numSamples := len(samples)
	sampleRateF := float64(sampleRate)

	// Параметры звука существа
	baseFreq := 100.0 + fear*200.0
	growlRate := 2.0 + tension*8.0
	growlDepth := 0.4 + fear*0.4

	// Создаем огибающую ADSR (Attack, Decay, Sustain, Release)
	attackTime := 0.05 + tension*0.05
	decayTime := 0.1 + fear*0.2
	sustainLevel := 0.7 - tension*0.3
	releaseTime := 0.2 + fear*0.3

	totalTime := float64(numSamples) / sampleRateF

	// Длительности фаз
	attackEnd := attackTime
	decayEnd := attackEnd + decayTime
	sustainEnd := totalTime - releaseTime

	// Гармоники для создания более богатого звука
	harmonics := []float64{1.0, 1.5, 2.0, 2.5, 3.0}
	harmonicWeights := []float64{1.0, 0.5, 0.25, 0.125, 0.0625}

	for i := 0; i < numSamples; i++ {
		t := float64(i) / sampleRateF

		// Вычисляем огибающую
		envelope := 0.0
		if t < attackEnd {
			envelope = t / attackTime
		} else if t < decayEnd {
			envelope = 1.0 - (1.0-sustainLevel)*(t-attackEnd)/decayTime
		} else if t < sustainEnd {
			envelope = sustainLevel
		} else {
			envelope = sustainLevel * (1.0 - (t-sustainEnd)/releaseTime)
		}

		// Модуляция частоты для эффекта рычания
		growlMod := 1.0 + growlDepth*math.Sin(2.0*math.Pi*growlRate*t)
		currentFreq := baseFreq * growlMod

		// Генерируем основной тон с гармониками
		sample := 0.0
		for j, harmonic := range harmonics {
			sample += math.Sin(2.0*math.Pi*currentFreq*harmonic*t) * harmonicWeights[j]
		}

		// Добавляем шум для текстуры
		noise := (ng.RandomFloat()*2.0 - 1.0) * 0.2 * fear

		// Применяем огибающую
		value := (sample + noise) * envelope

		// Нелинейное искажение для агрессивного звучания
		if distortion > 0.3 {
			value = math.Tanh(value*(1.0+distortion*3.0)) * 0.8
		}

		samples[i] = float32(value)
	}
}

// generateMechanicalSound генерирует механический звук
func generateMechanicalSound(samples []float32, fear, tension, distortion float64, ng *noise.NoiseGenerator) {
	numSamples := len(samples)
	sampleRateF := float64(sampleRate)

	// Параметры механического звука
	baseFreq := 200.0 + fear*100.0
	clickRate := 3.0 + tension*10.0
	metallic := 0.5 + fear*0.3

	// Гармоники для металлического звука
	harmonics := []float64{1.0, 1.414, 1.732, 2.0, 2.236, 2.449, 2.646}
	harmonicWeights := []float64{1.0, 0.7, 0.5, 0.4, 0.3, 0.2, 0.1}

	for i := 0; i < numSamples; i++ {
		t := float64(i) / sampleRateF

		// Создаем ритмичный паттерн (клики/щелчки)
		clickPattern := math.Pow(0.5+0.5*math.Sin(2.0*math.Pi*clickRate*t), 4.0)

		// Генерируем металлический звук с гармониками
		sample := 0.0
		for j, harmonic := range harmonics {
			// Небольшая разница в фазе для каждой гармоники
			phase := t + float64(j)*0.01
			sample += math.Sin(2.0*math.Pi*baseFreq*harmonic*phase) * harmonicWeights[j] * metallic
		}

		// Добавляем шум для индустриального звучания
		noise := (ng.RandomFloat()*2.0 - 1.0) * 0.15

		// Модулируем звук ритмичным паттерном
		value := (sample*0.7 + noise*0.3) * (0.3 + 0.7*clickPattern)

		// Добавляем эхо
		echoDelay := int(0.1 * sampleRateF) // 100 мс задержка
		if i > echoDelay {
			value += float64(samples[i-echoDelay]) * 0.3
		}

		// Нелинейное искажение
		if distortion > 0.2 {
			value = math.Tanh(value*(1.0+distortion*2.0)) * 0.7
		}

		samples[i] = float32(value)
	}
}

// generateEnvironmentSound генерирует звуки окружения
func generateEnvironmentSound(samples []float32, fear, tension, distortion float64, ng *noise.NoiseGenerator) {
	numSamples := len(samples)
	sampleRateF := float64(sampleRate)

	// Определяем тип звука окружения
	envType := ""
	randVal := ng.RandomFloat()

	if randVal < 0.25 {
		envType = "wind"
	} else if randVal < 0.5 {
		envType = "creaking"
	} else if randVal < 0.75 {
		envType = "rustling"
	} else {
		envType = "distant"
	}

	// Создаем огибающую в зависимости от типа звука
	attackTime := 0.0
	releaseTime := 0.0

	switch envType {
	case "wind":
		attackTime = 0.3 + ng.RandomFloat()*0.7  // 0.3-1.0 сек
		releaseTime = 0.5 + ng.RandomFloat()*0.5 // 0.5-1.0 сек
	case "creaking":
		attackTime = 0.05 + ng.RandomFloat()*0.1 // 0.05-0.15 сек
		releaseTime = 0.2 + ng.RandomFloat()*0.3 // 0.2-0.5 сек
	case "rustling":
		attackTime = 0.01 + ng.RandomFloat()*0.05 // 0.01-0.06 сек
		releaseTime = 0.1 + ng.RandomFloat()*0.2  // 0.1-0.3 сек
	case "distant":
		attackTime = 0.2 + ng.RandomFloat()*0.3  // 0.2-0.5 сек
		releaseTime = 0.4 + ng.RandomFloat()*0.6 // 0.4-1.0 сек
	}

	totalTime := float64(numSamples) / sampleRateF
	sustainTime := totalTime - (attackTime + releaseTime)
	if sustainTime < 0 {
		// Корректируем, если сигнал слишком короткий
		sustainTime = totalTime * 0.5
		attackTime = totalTime * 0.25
		releaseTime = totalTime * 0.25
	}

	// Генерируем звук в зависимости от типа
	for i := 0; i < numSamples; i++ {
		t := float64(i) / sampleRateF

		// Вычисляем огибающую
		envelope := 0.0
		if t < attackTime {
			envelope = t / attackTime
		} else if t < attackTime+sustainTime {
			envelope = 1.0
		} else {
			envelope = 1.0 - (t-(attackTime+sustainTime))/releaseTime
		}
		envelope = math.Max(0.0, math.Min(1.0, envelope))

		// Генерируем звук в зависимости от типа
		var value float64

		switch envType {
		case "wind":
			// Шум с фильтрацией низких частот
			noise := ng.RandomFloat()*2.0 - 1.0

			// Простая фильтрация
			if i > 1 {
				noise = noise*0.2 + float64(samples[i-1])*0.5 + float64(samples[i-2])*0.3
			}

			// Модуляция для эффекта порывов ветра
			windMod := 0.7 + 0.3*math.Sin(2.0*math.Pi*0.2*t)
			value = noise * windMod * 0.7

		case "creaking":
			// Скрипящий звук (комбинация синусоид с резонансами)
			baseFreq := 200.0 + 100.0*math.Sin(2.0*math.Pi*0.5*t)
			creak := math.Sin(2.0*math.Pi*baseFreq*t) * 0.3

			// Добавляем резонансы для скрипучего звука
			for j := 1; j <= 5; j++ {
				resonance := math.Sin(2.0*math.Pi*baseFreq*float64(j)*1.3*t) * 0.1 / float64(j)
				creak += resonance
			}

			// Добавляем случайность для естественности
			noise := (ng.RandomFloat()*2.0 - 1.0) * 0.1
			value = creak + noise

		case "rustling":
			// Шорох (быстро меняющийся отфильтрованный шум)
			noise := ng.RandomFloat()*2.0 - 1.0

			// Фильтрация для эффекта шороха
			if i > 0 {
				// Менее сильная фильтрация для сохранения высоких частот
				noise = noise*0.6 + float64(samples[i-1])*0.4
			}

			// Модуляция громкости для создания паттерна
			rustleMod := 0.5 + 0.5*math.Sin(2.0*math.Pi*10.0*t+ng.RandomFloat()*10.0)
			value = noise * rustleMod * 0.5

		case "distant":
			// Далекий звук (низкочастотные тоны с эхом)
			baseFreq := 100.0 + 50.0*math.Sin(2.0*math.Pi*0.2*t)
			tone := math.Sin(2.0*math.Pi*baseFreq*t) * 0.3

			// Добавляем эхо
			echoDelay := int(0.2 * sampleRateF) // 200 мс задержка
			if i > echoDelay {
				tone += float64(samples[i-echoDelay]) * 0.4
			}

			// Добавляем атмосферный шум
			noise := (ng.RandomFloat()*2.0 - 1.0) * 0.05
			value = tone + noise
		}

		// Применяем огибающую и добавляем эффект страха/напряжения
		value *= envelope * (0.7 + fear*0.3)

		// Добавляем искажения если требуется
		if distortion > 0.3 {
			value = math.Tanh(value * (1.0 + distortion))
		}

		samples[i] = float32(value)
	}
}

// generateMysteriousSound генерирует загадочный/таинственный звук
func generateMysteriousSound(samples []float32, fear, tension, distortion float64, ng *noise.NoiseGenerator) {
	numSamples := len(samples)
	sampleRateF := float64(sampleRate)

	// Параметры звука
	baseFreq := 80.0 + fear*150.0                // 80-230 Гц
	secondFreq := baseFreq * (1.5 + tension*0.5) // Создает диссонанс при высоком напряжении

	// Модуляционные параметры
	modRate := 0.2 + tension*0.8 // 0.2-1.0 Гц
	modDepth := 0.3 + fear*0.4   // 0.3-0.7

	// Огибающая
	attackTime := 0.1 + fear*0.1     // 0.1-0.2 сек
	releaseTime := 0.3 + tension*0.7 // 0.3-1.0 сек

	totalTime := float64(numSamples) / sampleRateF
	sustainTime := totalTime - (attackTime + releaseTime)
	if sustainTime < 0 {
		sustainTime = totalTime * 0.5
		attackTime = totalTime * 0.25
		releaseTime = totalTime * 0.25
	}

	// Генерируем звук
	for i := 0; i < numSamples; i++ {
		t := float64(i) / sampleRateF

		// Огибающая
		envelope := 0.0
		if t < attackTime {
			envelope = t / attackTime
		} else if t < attackTime+sustainTime {
			envelope = 1.0
		} else {
			envelope = 1.0 - (t-(attackTime+sustainTime))/releaseTime
		}
		envelope = math.Max(0.0, math.Min(1.0, envelope))

		// Модуляция частоты
		freqMod := 1.0 + modDepth*math.Sin(2.0*math.Pi*modRate*t)

		// Основной звук - комбинация тонов с модуляцией
		mainTone := math.Sin(2.0*math.Pi*baseFreq*freqMod*t) * 0.4
		secondTone := math.Sin(2.0*math.Pi*secondFreq*t*(1.0+modDepth*0.1*math.Sin(2.0*math.Pi*modRate*2.0*t))) * 0.3

		// Добавляем гармоники
		harmonics := 0.0
		for j := 2; j <= 5; j++ {
			amplitude := 0.15 / float64(j)
			phase := t + float64(j)*0.01*tension // Сдвиг фазы для напряженности
			harmonics += math.Sin(2.0*math.Pi*baseFreq*float64(j)*phase) * amplitude
		}

		// Добавляем атмосферный шум
		noise := (ng.RandomFloat()*2.0 - 1.0) * 0.1 * fear

		// Случайные щелчки/аномалии
		glitch := 0.0
		if ng.RandomFloat() < 0.01*tension {
			glitch = (ng.RandomFloat()*2.0 - 1.0) * 0.5
		}

		// Объединяем все компоненты
		value := (mainTone + secondTone + harmonics + noise + glitch) * envelope

		// Применяем нелинейное искажение
		if distortion > 0.2 {
			value = math.Tanh(value*(1.0+distortion*2.0)) * 0.8
		}

		samples[i] = float32(value)
	}
}

// normalizeAudio нормализует аудиосэмплы для предотвращения клиппинга
func normalizeAudio(samples []float32) {
	// Находим максимальную амплитуду
	maxAmp := float32(0)
	for _, sample := range samples {
		if math.Abs(float64(sample)) > float64(maxAmp) {
			maxAmp = float32(math.Abs(float64(sample)))
		}
	}

	// Нормализуем если нужно
	if maxAmp > 1.0 {
		// Нормализуем к уровню 0.95 для запаса по клиппингу
		targetLevel := float32(0.95)
		for i := range samples {
			samples[i] = samples[i] / maxAmp * targetLevel
		}
	}

	// Если максимальная амплитуда слишком низкая, усиливаем её
	if maxAmp < 0.2 {
		gain := float32(0.7) / maxAmp // Цель - примерно 70% от максимума
		for i := range samples {
			samples[i] *= gain
		}
	}

	// Применяем мягкое ограничение для защиты от клиппинга
	for i := range samples {
		samples[i] = softClip(samples[i])
	}
}

// getMetadataValue safely gets a value from metadata with a default fallback
func getMetadataValue(metadata map[string]float64, key string, defaultValue float64) float64 {
	if value, ok := metadata[key]; ok {
		return value
	}
	return defaultValue
}

// IncreaseVolume increases the volume by the specified amount
func (ae *AudioEngine) IncreaseVolume(amount float32) {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	ae.masterVolume = float32(math.Min(1.0, float64(ae.masterVolume+amount)))
}

// DecreaseVolume decreases the volume by the specified amount
func (ae *AudioEngine) DecreaseVolume(amount float32) {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	ae.masterVolume = float32(math.Max(0.0, float64(ae.masterVolume-amount)))
}

// ToggleMute toggles the mute state
func (ae *AudioEngine) ToggleMute() {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	ae.isMuted = !ae.isMuted
}

// IsMuted returns the current mute state
func (ae *AudioEngine) IsMuted() bool {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	return ae.isMuted
}

// GetVolume returns the current volume
func (ae *AudioEngine) GetVolume() float32 {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	return ae.masterVolume
}

// SetVolume sets the volume to the specified level
func (ae *AudioEngine) SetVolume(volume float32) {
	ae.masterMutex.Lock()
	defer ae.masterMutex.Unlock()

	ae.masterVolume = float32(math.Max(0.0, math.Min(1.0, float64(volume))))
}

// Shutdown shuts down the audio engine
func (ae *AudioEngine) Shutdown() {
	ae.masterMutex.Lock()

	// Stop all sounds first with a short fade-out
	for _, sound := range ae.activeSounds {
		sound.FadeOut = 0.1
		sound.FadeOutStart = float64(sound.Position) / sampleRate
	}

	// Give a short time for fade-outs to complete
	ae.masterMutex.Unlock()
	time.Sleep(200 * time.Millisecond)
	ae.masterMutex.Lock()

	if ae.stream != nil {
		ae.stream.Stop()
		ae.stream.Close()
	}

	if ae.micStream != nil {
		ae.micStream.Stop()
		ae.micStream.Close()
	}

	ae.masterMutex.Unlock()

	portaudio.Terminate()
}
