package engine

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	noise "nightmare/internal/math"
	"nightmare/internal/util"
	"nightmare/pkg/config"
)

// ProceduralObject represents a procedurally generated object
type ProceduralObject struct {
	ID         int
	Type       string             // Type of object (tree, rock, terrain, etc.)
	Position   Vector3            // Position in world space
	Scale      Vector3            // Scale of the object
	Rotation   Vector3            // Rotation of the object (in radians)
	Metadata   map[string]float64 // Hierarchical metadata with weights
	Seed       int64              // Seed for reproducibility
	ASCIIModel *ASCIITexture      // ASCII representation for advanced rendering
}

// ProceduralScene represents the current procedural scene
type ProceduralScene struct {
	Objects     []*ProceduralObject
	Terrain     *HeightMap
	TimeOfDay   float64            // 0.0-1.0, 0 = midnight, 0.5 = noon
	Weather     map[string]float64 // Weather conditions (fog, rain, etc.) with weights
	Seed        int64              // Scene seed
	BiomeType   string             // Type of biome (forest, mountains, etc.)
	Atmosphere  map[string]float64 // Atmospheric conditions
	LevelOfFear float64            // General fear level of the scene
}

// HeightMap represents terrain elevation data
type HeightMap struct {
	Width     int
	Height    int
	Data      [][]float64
	Materials [][]int
	Humidity  [][]float64  // Влажность почвы
	Regions   [][]string   // Регионы (лес, поляна, болото и т.д.)
	Mutex     sync.RWMutex // Для безопасного доступа из разных потоков
}

// BiomeParams содержит параметры для генерации определенного биома
type BiomeParams struct {
	BaseElevation    float64            // Базовая высота ландшафта
	Roughness        float64            // Неровность поверхности
	TreeDensity      float64            // Плотность деревьев
	RockDensity      float64            // Плотность камней
	StrangeDensity   float64            // Плотность странных объектов
	TreeTypes        []string           // Типы деревьев
	GroundMaterials  []int              // Типы материалов земли
	FearLevel        float64            // Базовый уровень страха
	WeatherSettings  map[string]float64 // Настройки погоды
	AtmosphereParams map[string]float64 // Атмосфера
}

// ProceduralGenerator handles procedural generation of content
type ProceduralGenerator struct {
	config       config.ProceduralConfig
	currentScene *ProceduralScene
	noiseGen     *noise.NoiseGenerator
	time         float64
	mutex        sync.RWMutex

	// Биомы и регионы
	biomes map[string]BiomeParams
}

// NewProceduralGenerator creates a new procedural generator
func NewProceduralGenerator(config config.ProceduralConfig) (*ProceduralGenerator, error) {
	// Используем указанный в конфигурации seed, или случайный, если seed=0
	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	gen := &ProceduralGenerator{
		config:   config,
		noiseGen: noise.NewNoiseGenerator(seed),
		time:     0,
		biomes:   make(map[string]BiomeParams),
	}

	// Инициализируем настройки биомов
	gen.initBiomes()

	return gen, nil
}

// initBiomes инициализирует параметры различных биомов
func (pg *ProceduralGenerator) initBiomes() {
	// Темный лес - основной биом
	pg.biomes["dark_forest"] = BiomeParams{
		BaseElevation:   0.5,
		Roughness:       0.6,
		TreeDensity:     8.0,
		RockDensity:     3.0,
		StrangeDensity:  0.5,
		TreeTypes:       []string{"pine", "dead_tree", "twisted_tree"},
		GroundMaterials: []int{2, 2, 2, 3}, // Преимущественно земля и камни
		FearLevel:       0.7,
		WeatherSettings: map[string]float64{
			"fog":  0.7,
			"mist": 0.5,
			"wind": 0.3,
		},
		AtmosphereParams: map[string]float64{
			"atmosphere.fear":     0.7,
			"atmosphere.ominous":  0.8,
			"atmosphere.dread":    0.6,
			"visuals.dark":        0.7,
			"conditions.shadow":   0.8,
			"conditions.darkness": 0.6,
		},
	}

	// Болотистая местность
	pg.biomes["swamp"] = BiomeParams{
		BaseElevation:   0.3,
		Roughness:       0.4,
		TreeDensity:     5.0,
		RockDensity:     1.0,
		StrangeDensity:  0.8,
		TreeTypes:       []string{"dead_tree", "twisted_tree", "thin_tree"},
		GroundMaterials: []int{1, 2, 1, 1}, // Преимущественно вода и земля
		FearLevel:       0.8,
		WeatherSettings: map[string]float64{
			"fog":  0.9,
			"mist": 0.8,
			"wind": 0.1,
		},
		AtmosphereParams: map[string]float64{
			"atmosphere.fear":      0.8,
			"atmosphere.ominous":   0.9,
			"atmosphere.dread":     0.8,
			"visuals.dark":         0.6,
			"visuals.distorted":    0.7,
			"conditions.fog":       0.9,
			"conditions.darkness":  0.5,
			"conditions.unnatural": 0.6,
		},
	}

	// Горы и утесы
	pg.biomes["mountains"] = BiomeParams{
		BaseElevation:   0.7,
		Roughness:       0.8,
		TreeDensity:     3.0,
		RockDensity:     7.0,
		StrangeDensity:  0.3,
		TreeTypes:       []string{"pine", "small_pine"},
		GroundMaterials: []int{3, 3, 3, 4}, // Преимущественно камни и снег
		FearLevel:       0.6,
		WeatherSettings: map[string]float64{
			"fog":  0.4,
			"mist": 0.3,
			"wind": 0.9,
		},
		AtmosphereParams: map[string]float64{
			"atmosphere.fear":    0.6,
			"atmosphere.tension": 0.7,
			"atmosphere.ominous": 0.5,
			"visuals.dark":       0.4,
			"conditions.shadow":  0.7,
		},
	}

	// Поляна (относительно безопасная зона)
	pg.biomes["clearing"] = BiomeParams{
		BaseElevation:   0.4,
		Roughness:       0.3,
		TreeDensity:     1.0,
		RockDensity:     1.0,
		StrangeDensity:  0.1,
		TreeTypes:       []string{"pine", "small_pine", "bush"},
		GroundMaterials: []int{2, 2, 2, 2}, // Преимущественно земля
		FearLevel:       0.3,
		WeatherSettings: map[string]float64{
			"fog":  0.3,
			"mist": 0.2,
			"wind": 0.4,
		},
		AtmosphereParams: map[string]float64{
			"atmosphere.fear":    0.3,
			"atmosphere.tension": 0.4,
			"visuals.dark":       0.3,
		},
	}
}

// GenerateInitialWorld creates the initial world
func (pg *ProceduralGenerator) GenerateInitialWorld() {
	pg.mutex.Lock()
	defer pg.mutex.Unlock()

	fmt.Println("Starting world generation...") // This will print to console even if logger isn't working

	// Create a new scene with seed from config or random
	seed := pg.config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	fmt.Println("Creating scene with seed:", seed)

	pg.currentScene = &ProceduralScene{
		Objects:     make([]*ProceduralObject, 0),
		TimeOfDay:   0.2, // Early morning
		Weather:     map[string]float64{"fog": 0.7, "mist": 0.3},
		Seed:        seed,
		BiomeType:   "dark_forest", // Start in a dark forest
		Atmosphere:  make(map[string]float64),
		LevelOfFear: 0.5,
	}

	fmt.Println("Scene created, initializing atmosphere...")

	// Initialize atmosphere from biome
	biomeParams := pg.biomes[pg.currentScene.BiomeType]
	for k, v := range biomeParams.AtmosphereParams {
		pg.currentScene.Atmosphere[k] = v
	}
	pg.currentScene.LevelOfFear = biomeParams.FearLevel

	// Weather based on biome
	for k, v := range biomeParams.WeatherSettings {
		pg.currentScene.Weather[k] = v
	}

	fmt.Println("Generating terrain...")
	// Generate terrain
	pg.currentScene.Terrain = pg.generateTerrain(seed, pg.currentScene.BiomeType)
	fmt.Println("Terrain generation completed")

	fmt.Println("Populating scene with objects...")
	// Generate initial objects based on the terrain
	pg.populateScene()
	fmt.Println("Scene population completed")

	fmt.Println("World generation completed")
}

// Update updates the procedural generation based on time
func (pg *ProceduralGenerator) Update(deltaTime float64) {
	pg.mutex.Lock()
	defer pg.mutex.Unlock()

	if pg.currentScene == nil {
		return
	}

	// Update internal time
	pg.time += deltaTime

	// Update time of day (complete cycle in 15 minutes real time)
	cycleDuration := 15 * 60.0 // 15 minutes in seconds
	pg.currentScene.TimeOfDay = math.Mod(pg.time/cycleDuration, 1.0)

	// Every few seconds, potentially update some aspects of the scene
	if int(pg.time*10)%30 == 0 { // Every 3 seconds
		pg.evolveScene()
	}
}

// GetCurrentScene returns the current scene
func (pg *ProceduralGenerator) GetCurrentScene() *ProceduralScene {
	pg.mutex.RLock()
	defer pg.mutex.RUnlock()

	return pg.currentScene
}

// GetTerrainHeightAt возвращает высоту ландшафта в указанной точке
func (pg *ProceduralGenerator) GetTerrainHeightAt(x, z float64) float64 {
	pg.mutex.RLock()
	defer pg.mutex.RUnlock()

	if pg.currentScene == nil || pg.currentScene.Terrain == nil {
		return 0.0
	}

	terrain := pg.currentScene.Terrain

	// Блокируем чтение данных ландшафта для потокобезопасности
	terrain.Mutex.RLock()
	defer terrain.Mutex.RUnlock()

	// Преобразуем мировые координаты в координаты сетки ландшафта
	terrainWidth := float64(terrain.Width)
	terrainHeight := float64(terrain.Height)

	// Смещение для центрирования ландшафта
	offsetX := terrainWidth / 2.0
	offsetZ := terrainHeight / 2.0

	// Координаты в сетке ландшафта
	gridX := x + offsetX
	gridZ := z + offsetZ

	// Проверяем, что координаты находятся в пределах ландшафта
	if gridX < 0 || gridX >= terrainWidth || gridZ < 0 || gridZ >= terrainHeight {
		return 0.0 // За пределами ландшафта
	}

	// Координаты ближайших точек сетки
	x0 := int(math.Floor(gridX))
	z0 := int(math.Floor(gridZ))
	x1 := x0 + 1
	z1 := z0 + 1

	// Ограничиваем индексы границами массива
	x0 = clamp(x0, 0, terrain.Width-1)
	z0 = clamp(z0, 0, terrain.Height-1)
	x1 = clamp(x1, 0, terrain.Width-1)
	z1 = clamp(z1, 0, terrain.Height-1)

	// Веса для билинейной интерполяции
	wx := gridX - float64(x0)
	wz := gridZ - float64(z0)

	// Извлекаем высоты в ближайших точках
	h00 := terrain.Data[z0][x0]
	h10 := terrain.Data[z0][x1]
	h01 := terrain.Data[z1][x0]
	h11 := terrain.Data[z1][x1]

	// Билинейная интерполяция
	h0 := h00*(1-wx) + h10*wx
	h1 := h01*(1-wx) + h11*wx
	height := h0*(1-wz) + h1*wz

	// Масштабируем высоту для получения реалистичных значений
	// (в зависимости от вашей системы координат)
	heightScale := 20.0 // Коэффициент масштабирования высоты

	return height * heightScale
}

// GetBiomeAt возвращает тип биома в указанной точке
func (pg *ProceduralGenerator) GetBiomeAt(x, z float64) string {
	pg.mutex.RLock()
	defer pg.mutex.RUnlock()

	if pg.currentScene == nil || pg.currentScene.Terrain == nil {
		return "unknown"
	}

	terrain := pg.currentScene.Terrain

	// Блокируем чтение данных ландшафта для потокобезопасности
	terrain.Mutex.RLock()
	defer terrain.Mutex.RUnlock()

	// Преобразуем мировые координаты в координаты сетки ландшафта
	terrainWidth := float64(terrain.Width)
	terrainHeight := float64(terrain.Height)

	// Смещение для центрирования ландшафта
	offsetX := terrainWidth / 2.0
	offsetZ := terrainHeight / 2.0

	// Координаты в сетке ландшафта
	gridX := x + offsetX
	gridZ := z + offsetZ

	// Проверяем, что координаты находятся в пределах ландшафта
	if gridX < 0 || gridX >= terrainWidth || gridZ < 0 || gridZ >= terrainHeight {
		return "unknown" // За пределами ландшафта
	}

	// Получаем индексы ближайшей точки
	x0 := int(math.Floor(gridX))
	z0 := int(math.Floor(gridZ))

	// Ограничиваем индексы границами массива
	x0 = clamp(x0, 0, terrain.Width-1)
	z0 = clamp(z0, 0, terrain.Height-1)

	// Возвращаем регион или биом по умолчанию, если нет определенного региона
	if terrain.Regions != nil && z0 < len(terrain.Regions) && x0 < len(terrain.Regions[z0]) {
		region := terrain.Regions[z0][x0]
		if region != "" {
			return region
		}
	}

	return pg.currentScene.BiomeType
}

// generateTerrain generates a new terrain heightmap
func (pg *ProceduralGenerator) generateTerrain(seed int64, biomeType string) *HeightMap {
	// Получаем параметры биома
	biomeParams, ok := pg.biomes[biomeType]
	if !ok {
		biomeParams = pg.biomes["dark_forest"] // По умолчанию
	}

	width := pg.config.TerrainSize
	height := pg.config.TerrainSize

	// Create heightmap
	heightMap := &HeightMap{
		Width:     width,
		Height:    height,
		Data:      make([][]float64, height),
		Materials: make([][]int, height),
		Humidity:  make([][]float64, height),
		Regions:   make([][]string, height),
	}

	// Set up noise parameters
	baseScale := 0.03
	detailScale := 0.1

	// Параметры шума в зависимости от биома
	roughness := biomeParams.Roughness
	baseElevation := biomeParams.BaseElevation

	// Generate heightmap data
	for y := 0; y < height; y++ {
		heightMap.Data[y] = make([]float64, width)
		heightMap.Materials[y] = make([]int, width)
		heightMap.Humidity[y] = make([]float64, width)
		heightMap.Regions[y] = make([]string, width)

		for x := 0; x < width; x++ {
			// Calculate world position relative to center
			worldX := float64(x - width/2)
			worldZ := float64(y - height/2)

			// Generate base elevation using octaves of noise for a more natural look
			elevation := 0.0

			// Крупномасштабный шум для общей формы ландшафта
			largeScale := pg.noiseGen.FBM2D(worldX*baseScale*0.5, worldZ*baseScale*0.5,
				3, 2.0, 0.5, seed)

			// Среднемасштабный шум для холмов и долин
			mediumScale := pg.noiseGen.FBM2D(worldX*baseScale, worldZ*baseScale,
				4, 2.0, 0.5, seed+1234)

			// Мелкомасштабный шум для деталей
			smallScale := pg.noiseGen.FBM2D(worldX*detailScale, worldZ*detailScale,
				3, 2.0, 0.5, seed+5678)

			// Комбинируем разные масштабы шума
			elevation = largeScale*0.6 + mediumScale*0.3 + smallScale*0.1

			// Применяем параметры биома
			elevation = elevation*roughness + baseElevation

			// Создаем особенности рельефа
			elevation = pg.applyTerrainFeatures(elevation, worldX, worldZ, baseScale, seed, biomeType)

			// Ограничиваем значения от 0 до 1
			elevation = math.Max(0.01, math.Min(0.99, elevation))

			// Store in heightmap
			heightMap.Data[y][x] = elevation

			// Генерируем влажность почвы
			// Используем другой набор шумов для независимости от высоты
			humidityNoise := pg.noiseGen.FBM2D(worldX*baseScale*1.5, worldZ*baseScale*1.5,
				3, 2.0, 0.5, seed+9999)

			// Нормализуем и корректируем в зависимости от биома
			humidity := (humidityNoise + 1.0) * 0.5
			if biomeType == "swamp" {
				humidity = humidity*0.3 + 0.7 // Высокая влажность в болоте
			} else if biomeType == "mountains" {
				humidity = humidity * 0.6 // Более сухо в горах
			}
			heightMap.Humidity[y][x] = humidity

			// Determine material based on elevation and humidity
			heightMap.Materials[y][x] = pg.determineMaterial(elevation, humidity, biomeType)

			// Определяем регионы (подбиомы)
			heightMap.Regions[y][x] = pg.determineRegion(elevation, humidity, worldX, worldZ, seed)
		}
	}

	// Пост-обработка для создания интересных особенностей
	pg.postProcessTerrain(heightMap, seed, biomeType)

	return heightMap
}

// applyTerrainFeatures applies additional features to terrain
func (pg *ProceduralGenerator) applyTerrainFeatures(baseElevation, x, z, scale float64, seed int64, biomeType string) float64 {
	elevation := baseElevation

	// Различные особенности для разных биомов
	switch biomeType {
	case "dark_forest":
		// Добавляем небольшие холмы и впадины
		hillNoise := pg.noiseGen.Ridge2D(x*scale*2.0, z*scale*2.0, seed+123)
		elevation += (hillNoise - 0.5) * 0.15

	case "swamp":
		// Создаем плоские болотистые участки с водоемами
		swampFlat := pg.noiseGen.FBM2D(x*scale*3.0, z*scale*3.0, 2, 2.0, 0.5, seed+456)

		// Если значение шума ниже порога, делаем эту область болотом
		if swampFlat < 0.4 {
			// Сглаживаем и снижаем высоту для создания болота
			flatFactor := (0.4 - swampFlat) / 0.4
			elevation = elevation*(1.0-flatFactor*0.7) + 0.2*flatFactor
		}

	case "mountains":
		// Добавляем хребты для создания горной местности
		ridgeNoise := pg.noiseGen.Ridge2D(x*scale*1.5, z*scale*1.5, seed+789)

		// Усиливаем ридж-шум для создания более крутых склонов
		ridgeNoise = math.Pow(ridgeNoise, 2.0) * 1.5
		elevation += ridgeNoise * 0.3

		// Местами добавляем резкие вершины
		peakNoise := pg.noiseGen.Perlin2D(x*scale*5.0, z*scale*5.0, seed+101112)
		if peakNoise > 0.7 {
			peakFactor := (peakNoise - 0.7) / 0.3
			elevation += peakFactor * 0.2
		}

	case "clearing":
		// Делаем поляны более плоскими
		// Используем дистанцию от центра для создания круглой поляны
		distFromCenter := math.Sqrt(x*x+z*z) * scale
		if distFromCenter < 0.05 {
			// Внутри радиуса поляны сглаживаем рельеф
			flatFactor := (0.05 - distFromCenter) / 0.05

			// Целевая высота для поляны - чуть выше среднего
			targetHeight := 0.45
			elevation = elevation*(1.0-flatFactor) + targetHeight*flatFactor
		}
	}

	// Добавляем реки и ручьи (подходит для всех биомов)
	riverNoise := pg.noiseGen.Perlin2D(x*scale*10.0, z*scale*10.0, seed+131415)
	riverPathNoise := pg.noiseGen.Perlin2D(x*scale*2.0, z*scale*2.0, seed+161718)

	// Создаем реки там, где riverPathNoise ближе к 0
	riverPathValue := math.Abs(riverPathNoise)
	if riverPathValue < 0.05 && riverNoise > 0 {
		// Глубина реки зависит от того, насколько близко мы к "идеальному" пути реки
		riverDepth := (0.05 - riverPathValue) / 0.05 * 0.15

		// Особый случай для болот - более широкие и мелкие водоемы
		if biomeType == "swamp" {
			riverDepth *= 0.7
			riverPathValue *= 0.7
		}

		// Снижаем высоту для создания русла реки
		elevation -= riverDepth
	}

	// Ensure elevation is in 0-1 range
	elevation = math.Max(0.01, math.Min(0.99, elevation))

	return elevation
}

// determineMaterial determines the material type based on elevation and other factors
func (pg *ProceduralGenerator) determineMaterial(elevation, humidity float64, biomeType string) int {
	// Получаем доступные материалы для биома
	biomeParams, ok := pg.biomes[biomeType]
	if !ok {
		biomeParams = pg.biomes["dark_forest"] // По умолчанию
	}

	availableMaterials := biomeParams.GroundMaterials

	// По умолчанию, просто выбираем материал на основе высоты
	var materialID int

	// Базовое определение по высоте
	if elevation < 0.3 {
		materialID = 1 // Water/low ground
	} else if elevation < 0.55 {
		materialID = 2 // Dirt/grass
	} else if elevation < 0.8 {
		materialID = 3 // Rock
	} else {
		materialID = 4 // Snow/peaks
	}

	// Учитываем влажность
	if materialID == 2 && humidity > 0.7 {
		// Очень влажная земля, может быть болотом
		if rand.Float64() < 0.3 {
			materialID = 1 // Больше шансов на воду в очень влажных местах
		}
	}

	// Если есть доступные материалы для биома, делаем финальный выбор
	if len(availableMaterials) > 0 {
		// Создаем веса для разных материалов на основе их представленности
		// и подходящести для текущей высоты/влажности
		weights := make([]float64, len(availableMaterials))
		totalWeight := 0.0

		for i, material := range availableMaterials {
			// Базовый вес
			weight := 1.0

			// Если материал совпадает с определенным по высоте, увеличиваем его вес
			if material == materialID {
				weight *= 3.0
			}

			// Корректируем на основе совместимости
			if material == 1 && elevation >= 0.4 {
				weight *= 0.2 // Вода маловероятна на большой высоте
			}
			if material == 4 && elevation < 0.7 {
				weight *= 0.1 // Снег маловероятен на низкой высоте
			}

			weights[i] = weight
			totalWeight += weight
		}

		// Нормализуем веса
		for i := range weights {
			weights[i] /= totalWeight
		}

		// Выбираем материал с учетом весов
		selection := rand.Float64()
		cumulativeWeight := 0.0

		for i, weight := range weights {
			cumulativeWeight += weight
			if selection <= cumulativeWeight {
				return availableMaterials[i]
			}
		}

		// Если что-то пошло не так, возвращаем первый материал из списка
		return availableMaterials[0]
	}

	return materialID
}

// determineRegion определяет регион (подбиом) на основе характеристик местности
func (pg *ProceduralGenerator) determineRegion(elevation, humidity, x, z float64, seed int64) string {
	// Используем шум для создания плавных переходов между регионами
	regionNoise := pg.noiseGen.FBM2D(x*0.01, z*0.01, 2, 2.0, 0.5, seed+191919)

	// Нормализуем шум
	regionValue := (regionNoise + 1.0) * 0.5

	// Создаем комбинированное значение, учитывающее высоту, влажность и шум
	// Определяем регион на основе комбинированного значения
	// и дополнительных условий

	// Водоемы
	if elevation < 0.25 {
		if humidity > 0.7 {
			return "swamp" // Болото
		} else {
			return "pond" // Пруд/озеро
		}
	}

	// Низины
	if elevation < 0.4 {
		if humidity > 0.6 {
			return "swamp" // Заболоченная низина
		} else if regionValue < 0.4 {
			return "clearing" // Поляна
		} else {
			return "low_forest" // Низинный лес
		}
	}

	// Средние высоты
	if elevation < 0.7 {
		if regionValue < 0.3 {
			return "clearing" // Поляна
		} else if regionValue < 0.7 {
			return "dark_forest" // Темный лес
		} else {
			return "dense_forest" // Густой лес
		}
	}

	// Высокие районы
	if elevation < 0.85 {
		if humidity < 0.4 {
			return "rocky_hills" // Каменистые холмы
		} else {
			return "mountains" // Горы
		}
	}

	// Самые высокие пики
	return "mountain_peak" // Горный пик
}

// postProcessTerrain выполняет пост-обработку ландшафта
func (pg *ProceduralGenerator) postProcessTerrain(terrain *HeightMap, seed int64, biomeType string) {
	// Создаем несколько значимых особенностей рельефа

	// Для темного леса - поляны и густые заросли
	if biomeType == "dark_forest" {
		pg.createClearings(terrain, 3, seed)
		pg.createDenseGroves(terrain, 5, seed)
	}

	// Для болота - топи и островки
	if biomeType == "swamp" {
		pg.createSwampPits(terrain, 4, seed)
		pg.createSmallIslands(terrain, 6, seed)
	}

	// Для гор - пики и ущелья
	if biomeType == "mountains" {
		pg.createMountainPeaks(terrain, 3, seed)
		pg.createRavines(terrain, 2, seed)
	}

	// Общие элементы для всех биомов
	pg.createPaths(terrain, 1, seed) // Тропинки

	// Сглаживаем ландшафт для более естественного вида
	pg.smoothTerrain(terrain, 1)
}

// Различные функции для создания особенностей рельефа

// createClearings создает поляны в лесу
func (pg *ProceduralGenerator) createClearings(terrain *HeightMap, count int, seed int64) {
	rand.Seed(seed)
	width := terrain.Width
	height := terrain.Height

	for i := 0; i < count; i++ {
		// Выбираем случайную позицию для центра поляны
		centerX := rand.Intn(width)
		centerY := rand.Intn(height)

		// Размер поляны
		radius := 5 + rand.Intn(10)

		// Создаем поляну
		for y := centerY - radius; y <= centerY+radius; y++ {
			for x := centerX - radius; x <= centerX+radius; x++ {
				// Проверяем, что координаты в пределах карты
				if x < 0 || x >= width || y < 0 || y >= height {
					continue
				}

				// Расстояние от центра
				dx := x - centerX
				dy := y - centerY
				dist := math.Sqrt(float64(dx*dx + dy*dy))

				// Если внутри радиуса поляны
				if dist <= float64(radius) {
					// Сглаживаем края поляны
					factor := (1.0 - dist/float64(radius)) * 0.8

					// Выравниваем высоту
					targetHeight := 0.4 + rand.Float64()*0.1
					terrain.Data[y][x] = terrain.Data[y][x]*(1.0-factor) + targetHeight*factor

					// Устанавливаем регион
					terrain.Regions[y][x] = "clearing"

					// Устанавливаем материал
					terrain.Materials[y][x] = 2 // Трава
				}
			}
		}
	}
}

// createDenseGroves создает участки густого леса
func (pg *ProceduralGenerator) createDenseGroves(terrain *HeightMap, count int, seed int64) {
	rand.Seed(seed + 1000)
	width := terrain.Width
	height := terrain.Height

	for i := 0; i < count; i++ {
		// Выбираем случайную позицию для центра рощи
		centerX := rand.Intn(width)
		centerY := rand.Intn(height)

		// Размер рощи
		radius := 8 + rand.Intn(12)

		// Создаем рощу
		for y := centerY - radius; y <= centerY+radius; y++ {
			for x := centerX - radius; x <= centerX+radius; x++ {
				// Проверяем, что координаты в пределах карты
				if x < 0 || x >= width || y < 0 || y >= height {
					continue
				}

				// Расстояние от центра
				dx := x - centerX
				dy := y - centerY
				dist := math.Sqrt(float64(dx*dx + dy*dy))

				// Если внутри радиуса рощи
				if dist <= float64(radius) {
					// Сглаживаем края
					factor := (1.0 - dist/float64(radius)) * 0.7

					// Устанавливаем регион
					if rand.Float64() < factor {
						terrain.Regions[y][x] = "dense_forest"
					}
				}
			}
		}
	}
}

// createSwampPits создает болотные ямы
func (pg *ProceduralGenerator) createSwampPits(terrain *HeightMap, count int, seed int64) {
	rand.Seed(seed + 2000)
	width := terrain.Width
	height := terrain.Height

	for i := 0; i < count; i++ {
		// Выбираем случайную позицию для центра ямы
		centerX := rand.Intn(width)
		centerY := rand.Intn(height)

		// Размер ямы
		radius := 4 + rand.Intn(8)

		// Создаем яму
		for y := centerY - radius; y <= centerY+radius; y++ {
			for x := centerX - radius; x <= centerX+radius; x++ {
				// Проверяем, что координаты в пределах карты
				if x < 0 || x >= width || y < 0 || y >= height {
					continue
				}

				// Расстояние от центра
				dx := x - centerX
				dy := y - centerY
				dist := math.Sqrt(float64(dx*dx + dy*dy))

				// Если внутри радиуса ямы
				if dist <= float64(radius) {
					// Сглаживаем края
					factor := (1.0 - dist/float64(radius)) * 0.9

					// Понижаем высоту для создания ямы
					targetHeight := 0.15 + rand.Float64()*0.1
					terrain.Data[y][x] = terrain.Data[y][x]*(1.0-factor) + targetHeight*factor

					// Устанавливаем регион
					terrain.Regions[y][x] = "swamp_pit"

					// Устанавливаем материал (вода)
					terrain.Materials[y][x] = 1

					// Повышаем влажность
					terrain.Humidity[y][x] = math.Min(1.0, terrain.Humidity[y][x]+factor*0.3)
				}
			}
		}
	}
}

// createSmallIslands создает маленькие островки в болоте
func (pg *ProceduralGenerator) createSmallIslands(terrain *HeightMap, count int, seed int64) {
	rand.Seed(seed + 3000)
	width := terrain.Width
	height := terrain.Height

	for i := 0; i < count; i++ {
		// Выбираем случайную позицию для центра острова
		centerX := rand.Intn(width)
		centerY := rand.Intn(height)

		// Размер острова
		radius := 2 + rand.Intn(4)

		// Создаем остров
		for y := centerY - radius; y <= centerY+radius; y++ {
			for x := centerX - radius; x <= centerX+radius; x++ {
				// Проверяем, что координаты в пределах карты
				if x < 0 || x >= width || y < 0 || y >= height {
					continue
				}

				// Расстояние от центра
				dx := x - centerX
				dy := y - centerY
				dist := math.Sqrt(float64(dx*dx + dy*dy))

				// Если внутри радиуса острова
				if dist <= float64(radius) {
					// Сглаживаем края
					factor := (1.0 - dist/float64(radius)) * 0.8

					// Повышаем высоту для создания острова
					targetHeight := 0.35 + rand.Float64()*0.1
					terrain.Data[y][x] = terrain.Data[y][x]*(1.0-factor) + targetHeight*factor

					// Устанавливаем регион
					terrain.Regions[y][x] = "small_island"

					// Устанавливаем материал (земля)
					terrain.Materials[y][x] = 2
				}
			}
		}
	}
}

// createMountainPeaks создает горные вершины
func (pg *ProceduralGenerator) createMountainPeaks(terrain *HeightMap, count int, seed int64) {
	rand.Seed(seed + 4000)
	width := terrain.Width
	height := terrain.Height

	// Ищем высокие участки для размещения пиков
	highSpots := make([][2]int, 0)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if terrain.Data[y][x] > 0.7 {
				highSpots = append(highSpots, [2]int{x, y})
			}
		}
	}

	// Если нет высоких участков, выходим
	if len(highSpots) == 0 {
		return
	}

	// Перемешиваем высокие участки
	rand.Shuffle(len(highSpots), func(i, j int) {
		highSpots[i], highSpots[j] = highSpots[j], highSpots[i]
	})

	// Берем первые count участков как центры пиков
	peakCount := min(count, len(highSpots))

	for i := 0; i < peakCount; i++ {
		centerX := highSpots[i][0]
		centerY := highSpots[i][1]

		// Радиус влияния пика
		radius := 3 + rand.Intn(5)

		// Создаем пик
		for y := centerY - radius; y <= centerY+radius; y++ {
			for x := centerX - radius; x <= centerX+radius; x++ {
				// Проверяем, что координаты в пределах карты
				if x < 0 || x >= width || y < 0 || y >= height {
					continue
				}

				// Расстояние от центра
				dx := x - centerX
				dy := y - centerY
				dist := math.Sqrt(float64(dx*dx + dy*dy))

				// Если внутри радиуса пика
				if dist <= float64(radius) {
					// Функция для создания пика (экспоненциальное убывание)
					peakFactor := math.Exp(-dist * dist / (float64(radius) * float64(radius) * 0.5))

					// Повышаем высоту
					peakHeight := 0.9 + rand.Float64()*0.1
					terrain.Data[y][x] = math.Max(terrain.Data[y][x],
						terrain.Data[y][x]*(1.0-peakFactor)+peakHeight*peakFactor)

					// Устанавливаем регион для центральной части
					if dist < float64(radius)*0.5 {
						terrain.Regions[y][x] = "mountain_peak"

						// Пик горы может быть снежным
						if terrain.Data[y][x] > 0.85 {
							terrain.Materials[y][x] = 4 // Снег
						} else {
							terrain.Materials[y][x] = 3 // Камень
						}
					}
				}
			}
		}
	}
}

// createRavines создает ущелья
func (pg *ProceduralGenerator) createRavines(terrain *HeightMap, count int, seed int64) {
	rand.Seed(seed + 5000)
	width := terrain.Width
	height := terrain.Height

	for i := 0; i < count; i++ {
		// Выбираем случайную начальную точку
		startX := rand.Intn(width)
		startY := rand.Intn(height)

		// Выбираем случайное направление
		angle := rand.Float64() * 2 * math.Pi
		length := 20 + rand.Intn(30)

		// Ширина ущелья
		ravineWidth := 2 + rand.Intn(4)

		// Создаем ущелье путем прокладывания криволинейного пути
		curX, curY := float64(startX), float64(startY)
		angleVariation := 0.3 // Максимальное изменение угла на каждом шаге

		for step := 0; step < length; step++ {
			// Небольшое изменение направления для естественности
			angle += (rand.Float64()*2.0 - 1.0) * angleVariation

			// Перемещаемся в новом направлении
			curX += math.Cos(angle)
			curY += math.Sin(angle)

			// Проверяем, что мы все еще в пределах карты
			if int(curX) < 0 || int(curX) >= width || int(curY) < 0 || int(curY) >= height {
				break
			}

			// Высекаем ущелье
			for dy := -ravineWidth; dy <= ravineWidth; dy++ {
				for dx := -ravineWidth; dx <= ravineWidth; dx++ {
					x := int(curX) + dx
					y := int(curY) + dy

					// Проверяем, что координаты в пределах карты
					if x < 0 || x >= width || y < 0 || y >= height {
						continue
					}

					// Расстояние от центра ущелья
					dist := math.Sqrt(float64(dx*dx + dy*dy))

					// Если внутри радиуса ущелья
					if dist <= float64(ravineWidth) {
						// Степень влияния (сильнее в центре, слабее по краям)
						factor := (1.0 - dist/float64(ravineWidth)) * 0.7

						// Понижаем высоту
						targetHeight := terrain.Data[y][x] * 0.6 // Понижаем на 40%
						terrain.Data[y][x] = terrain.Data[y][x]*(1.0-factor) + targetHeight*factor

						// Устанавливаем регион
						if factor > 0.5 {
							terrain.Regions[y][x] = "ravine"
							terrain.Materials[y][x] = 3 // Камень
						}
					}
				}
			}
		}
	}
}

// createPaths создает тропинки
func (pg *ProceduralGenerator) createPaths(terrain *HeightMap, count int, seed int64) {
	rand.Seed(seed + 6000)
	width := terrain.Width
	height := terrain.Height

	for i := 0; i < count; i++ {
		// Выбираем случайную начальную точку на краю карты
		var startX, startY int

		// Выбираем случайную сторону
		side := rand.Intn(4)
		switch side {
		case 0: // Top
			startX = rand.Intn(width)
			startY = 0
		case 1: // Right
			startX = width - 1
			startY = rand.Intn(height)
		case 2: // Bottom
			startX = rand.Intn(width)
			startY = height - 1
		case 3: // Left
			startX = 0
			startY = rand.Intn(height)
		}

		// Создаем противоположную точку назначения (примерно)
		var endX, endY int
		switch side {
		case 0: // Top -> Bottom
			endX = rand.Intn(width)
			endY = height - 1
		case 1: // Right -> Left
			endX = 0
			endY = rand.Intn(height)
		case 2: // Bottom -> Top
			endX = rand.Intn(width)
			endY = 0
		case 3: // Left -> Right
			endX = width - 1
			endY = rand.Intn(height)
		}

		// Создаем путь с использованием A* или другого алгоритма поиска пути
		path := pg.findPath(terrain, startX, startY, endX, endY)

		// Если путь найден, создаем тропинку
		if len(path) > 0 {
			// Ширина тропинки
			pathWidth := 1 + rand.Intn(2)

			// Проходим по всем точкам пути
			for _, point := range path {
				x, y := point[0], point[1]

				// Создаем тропинку вокруг точки
				for dy := -pathWidth; dy <= pathWidth; dy++ {
					for dx := -pathWidth; dx <= pathWidth; dx++ {
						checkX := x + dx
						checkY := y + dy

						// Проверяем, что координаты в пределах карты
						if checkX < 0 || checkX >= width || checkY < 0 || checkY >= height {
							continue
						}

						// Расстояние от центра тропинки
						dist := math.Sqrt(float64(dx*dx + dy*dy))

						// Если внутри радиуса тропинки
						if dist <= float64(pathWidth) {
							// Степень влияния (сильнее в центре, слабее по краям)
							factor := (1.0 - dist/float64(pathWidth)) * 0.6

							// Сглаживаем высоту тропинки
							// Немного снижаем, но не слишком сильно
							currentHeight := terrain.Data[checkY][checkX]
							targetHeight := currentHeight * 0.95
							terrain.Data[checkY][checkX] = currentHeight*(1.0-factor) + targetHeight*factor

							// Устанавливаем регион, только для центральной части
							if factor > 0.5 {
								terrain.Regions[checkY][checkX] = "path"
								terrain.Materials[checkY][checkX] = 2 // Земля/тропа
							}
						}
					}
				}
			}
		}
	}
}

// findPath находит путь между двумя точками
// Упрощенная версия A* алгоритма для поиска пути
func (pg *ProceduralGenerator) findPath(terrain *HeightMap, startX, startY, endX, endY int) [][2]int {
	// Максимальное количество шагов для предотвращения бесконечного поиска
	maxSteps := terrain.Width * terrain.Height / 10

	// Определяем направления движения (4 направления)
	dirs := [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}}

	// Текущая точка
	curX, curY := startX, startY

	// Путь
	path := make([][2]int, 0)
	path = append(path, [2]int{curX, curY})

	// Поиск пути (очень упрощенный)
	for step := 0; step < maxSteps; step++ {
		// Если достигли цели, завершаем
		if curX == endX && curY == endY {
			break
		}

		// Выбираем направление, которое приближает нас к цели
		bestDir := -1
		bestDist := math.MaxFloat64

		for i, dir := range dirs {
			nextX := curX + dir[0]
			nextY := curY + dir[1]

			// Проверяем, что координаты в пределах карты
			if nextX < 0 || nextX >= terrain.Width || nextY < 0 || nextY >= terrain.Height {
				continue
			}

			// Расстояние до цели
			dist := math.Sqrt(float64((nextX-endX)*(nextX-endX) + (nextY-endY)*(nextY-endY)))

			// Высота в новой точке
			height := terrain.Data[nextY][nextX]

			// Пенализируем крутые склоны
			heightDiff := math.Abs(height - terrain.Data[curY][curX])
			heightPenalty := heightDiff * 10.0

			// Итоговая оценка (меньше - лучше)
			score := dist + heightPenalty

			// Если лучше предыдущего, запоминаем
			if score < bestDist {
				bestDist = score
				bestDir = i
			}
		}

		// Если не нашли направления, выходим
		if bestDir == -1 {
			break
		}

		// Перемещаемся в выбранном направлении
		curX += dirs[bestDir][0]
		curY += dirs[bestDir][1]

		// Добавляем точку в путь
		path = append(path, [2]int{curX, curY})

		// Добавляем немного случайности
		if rand.Float64() < 0.2 { // 20% шанс на случайное направление
			randomDir := rand.Intn(len(dirs))
			nx := curX + dirs[randomDir][0]
			ny := curY + dirs[randomDir][1]

			// Проверяем, что координаты в пределах карты
			if nx >= 0 && nx < terrain.Width && ny >= 0 && ny < terrain.Height {
				curX = nx
				curY = ny
				path = append(path, [2]int{curX, curY})
			}
		}
	}

	return path
}

// smoothTerrain сглаживает ландшафт для более естественного вида
func (pg *ProceduralGenerator) smoothTerrain(terrain *HeightMap, passes int) {
	width := terrain.Width
	height := terrain.Height

	// Создаем временный буфер для данных
	tempData := make([][]float64, height)
	for y := 0; y < height; y++ {
		tempData[y] = make([]float64, width)
		copy(tempData[y], terrain.Data[y])
	}

	// Выполняем несколько проходов сглаживания
	for pass := 0; pass < passes; pass++ {
		for y := 1; y < height-1; y++ {
			for x := 1; x < width-1; x++ {
				// Среднее значение по окрестности 3x3
				sum := 0.0
				count := 0

				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx == 0 && dy == 0 {
							continue // Пропускаем центральную точку
						}

						sum += terrain.Data[y+dy][x+dx]
						count++
					}
				}

				// Вычисляем среднее
				average := sum / float64(count)

				// Сглаживаем, но сохраняем некоторые особенности
				blendFactor := 0.3 // 30% от среднего, 70% от исходного
				tempData[y][x] = terrain.Data[y][x]*(1.0-blendFactor) + average*blendFactor
			}
		}

		// Копируем временные данные обратно
		for y := 0; y < height; y++ {
			copy(terrain.Data[y], tempData[y])
		}
	}
}

// populateScene populates the scene with objects
func (pg *ProceduralGenerator) populateScene() {
	if pg.currentScene == nil || pg.currentScene.Terrain == nil {
		return
	}

	// Получаем параметры биома
	biomeParams, ok := pg.biomes[pg.currentScene.BiomeType]
	if !ok {
		biomeParams = pg.biomes["dark_forest"] // По умолчанию
	}

	// Get terrain dimensions
	terrain := pg.currentScene.Terrain
	terrainWidth := terrain.Width
	terrainHeight := terrain.Height

	// Генерируем деревья
	numTrees := int(biomeParams.TreeDensity) * terrainWidth * terrainHeight / 1000

	// Массив для хранения занятых позиций
	occupiedPositions := make(map[string]bool)

	for i := 0; i < numTrees; i++ {
		// Pick a random position on the terrain
		x := pg.noiseGen.RandomFloat()*float64(terrainWidth) - float64(terrainWidth)/2
		z := pg.noiseGen.RandomFloat()*float64(terrainHeight) - float64(terrainHeight)/2

		// Find the terrain height at this position
		terrainX := int(x + float64(terrainWidth)/2)
		terrainZ := int(z + float64(terrainHeight)/2)

		// Ensure within bounds
		if terrainX < 0 || terrainX >= terrainWidth || terrainZ < 0 || terrainZ >= terrainHeight {
			continue
		}

		elevation := pg.currentScene.Terrain.Data[terrainZ][terrainX]
		region := pg.currentScene.Terrain.Regions[terrainZ][terrainX]

		// Only place trees at certain elevations and not in water
		if elevation > 0.3 && elevation < 0.8 && pg.currentScene.Terrain.Materials[terrainZ][terrainX] != 1 {
			// Check if position is already occupied
			posKey := fmt.Sprintf("%d,%d", terrainX, terrainZ)
			if occupiedPositions[posKey] {
				continue // Skip if position is already occupied
			}

			// Mark position as occupied
			occupiedPositions[posKey] = true

			// Определяем тип дерева в зависимости от региона и случайности
			treeType := "pine" // По умолчанию

			// Доступные типы деревьев для биома
			availableTypes := biomeParams.TreeTypes
			if len(availableTypes) > 0 {
				treeType = availableTypes[rand.Intn(len(availableTypes))]
			}

			// Коррекции на основе региона
			if region == "swamp" || region == "swamp_pit" {
				treeType = "dead_tree"
			} else if region == "dense_forest" {
				// В густом лесу больше вероятность искривленных деревьев
				if rand.Float64() < 0.4 {
					treeType = "twisted_tree"
				}
			} else if region == "clearing" || region == "path" {
				// На полянах и тропах меньше деревьев
				if rand.Float64() < 0.7 {
					continue // 70% шанс пропустить дерево
				}

				// На полянах чаще встречаются низкие деревья и кусты
				if rand.Float64() < 0.5 {
					treeType = "small_pine"
				} else if rand.Float64() < 0.3 {
					treeType = "bush"
				}
			}

			// Различные параметры в зависимости от типа дерева
			var treeHeight, treeWidth float64
			var treeMeta map[string]float64

			switch treeType {
			case "pine":
				treeHeight = 3.0 + pg.noiseGen.RandomFloat()*2.0
				treeWidth = 0.8 + pg.noiseGen.RandomFloat()*0.4
				treeMeta = map[string]float64{
					"atmosphere.fear":       0.3 + pg.noiseGen.RandomFloat()*0.3,
					"atmosphere.ominous":    0.2 + pg.noiseGen.RandomFloat()*0.4,
					"visuals.distorted":     pg.noiseGen.RandomFloat() * 0.5,
					"visuals.dark":          0.3 + pg.noiseGen.RandomFloat()*0.4,
					"conditions.silhouette": 0.2 + pg.noiseGen.RandomFloat()*0.7,
				}

			case "dead_tree":
				treeHeight = 2.5 + pg.noiseGen.RandomFloat()*1.5
				treeWidth = 0.6 + pg.noiseGen.RandomFloat()*0.3
				treeMeta = map[string]float64{
					"atmosphere.fear":       0.5 + pg.noiseGen.RandomFloat()*0.3,
					"atmosphere.ominous":    0.4 + pg.noiseGen.RandomFloat()*0.4,
					"atmosphere.dread":      0.3 + pg.noiseGen.RandomFloat()*0.4,
					"visuals.distorted":     0.2 + pg.noiseGen.RandomFloat()*0.3,
					"visuals.dark":          0.5 + pg.noiseGen.RandomFloat()*0.3,
					"conditions.silhouette": 0.5 + pg.noiseGen.RandomFloat()*0.5,
				}

			case "twisted_tree":
				treeHeight = 2.0 + pg.noiseGen.RandomFloat()*2.5
				treeWidth = 0.7 + pg.noiseGen.RandomFloat()*0.5
				treeMeta = map[string]float64{
					"atmosphere.fear":       0.4 + pg.noiseGen.RandomFloat()*0.4,
					"atmosphere.ominous":    0.5 + pg.noiseGen.RandomFloat()*0.3,
					"atmosphere.dread":      0.4 + pg.noiseGen.RandomFloat()*0.3,
					"visuals.distorted":     0.4 + pg.noiseGen.RandomFloat()*0.4,
					"visuals.twisted":       0.6 + pg.noiseGen.RandomFloat()*0.4,
					"conditions.silhouette": 0.4 + pg.noiseGen.RandomFloat()*0.4,
				}

			case "small_pine":
				treeHeight = 1.5 + pg.noiseGen.RandomFloat()*1.0
				treeWidth = 0.6 + pg.noiseGen.RandomFloat()*0.3
				treeMeta = map[string]float64{
					"atmosphere.fear":       0.2 + pg.noiseGen.RandomFloat()*0.2,
					"atmosphere.ominous":    0.1 + pg.noiseGen.RandomFloat()*0.3,
					"visuals.dark":          0.2 + pg.noiseGen.RandomFloat()*0.3,
					"conditions.silhouette": 0.1 + pg.noiseGen.RandomFloat()*0.5,
				}

			case "bush":
				treeHeight = 0.7 + pg.noiseGen.RandomFloat()*0.5
				treeWidth = 0.8 + pg.noiseGen.RandomFloat()*0.4
				treeMeta = map[string]float64{
					"atmosphere.fear": 0.1 + pg.noiseGen.RandomFloat()*0.2,
					"visuals.dark":    0.2 + pg.noiseGen.RandomFloat()*0.2,
				}
			}

			// Создаем дерево с соответствующими параметрами
			tree := &ProceduralObject{
				ID:       len(pg.currentScene.Objects) + 1,
				Type:     "tree",
				Position: Vector3{X: x, Y: elevation * 20, Z: z}, // Scale elevation
				Scale:    Vector3{X: treeWidth, Y: treeHeight, Z: treeWidth},
				Rotation: Vector3{X: 0, Y: pg.noiseGen.RandomFloat() * 2 * math.Pi, Z: 0},
				Metadata: treeMeta,
				Seed:     pg.currentScene.Seed + int64(i),
			}

			// Добавляем случайный наклон для некоторых деревьев
			if treeType == "twisted_tree" || treeType == "dead_tree" {
				tree.Rotation.X = (pg.noiseGen.RandomFloat()*2.0 - 1.0) * 0.2 // Наклон до 0.2 радиан
				tree.Rotation.Z = (pg.noiseGen.RandomFloat()*2.0 - 1.0) * 0.2
			}

			pg.currentScene.Objects = append(pg.currentScene.Objects, tree)
		}
	}

	// Генерируем камни
	numRocks := int(biomeParams.RockDensity) * terrainWidth * terrainHeight / 1000

	for i := 0; i < numRocks; i++ {
		// Similar logic as for trees
		x := pg.noiseGen.RandomFloat()*float64(terrainWidth) - float64(terrainWidth)/2
		z := pg.noiseGen.RandomFloat()*float64(terrainHeight) - float64(terrainHeight)/2

		terrainX := int(x + float64(terrainWidth)/2)
		terrainZ := int(z + float64(terrainHeight)/2)

		if terrainX < 0 || terrainX >= terrainWidth || terrainZ < 0 || terrainZ >= terrainHeight {
			continue
		}

		elevation := pg.currentScene.Terrain.Data[terrainZ][terrainX]
		region := pg.currentScene.Terrain.Regions[terrainZ][terrainX]

		// Rocks can be at more places than trees, but not in water
		if elevation > 0.2 && pg.currentScene.Terrain.Materials[terrainZ][terrainX] != 1 {
			// Check if position is already occupied
			posKey := fmt.Sprintf("%d,%d", terrainX, terrainZ)
			if occupiedPositions[posKey] {
				continue // Skip if position is already occupied
			}

			// Mark position as occupied
			occupiedPositions[posKey] = true

			// Определяем тип камня и размер
			rockSize := 0.5 + pg.noiseGen.RandomFloat()*1.5
			rockType := "rock"

			// Модификации на основе региона
			if region == "mountain_peak" || region == "mountains" || region == "rocky_hills" {
				// Больше и разнообразнее камни в горах
				rockSize = 1.0 + pg.noiseGen.RandomFloat()*2.0

				if rand.Float64() < 0.3 {
					rockType = "boulder"
				}
			} else if region == "ravine" {
				// В ущельях больше узких и острых камней
				rockType = "sharp_rock"
				rockSize = 0.7 + pg.noiseGen.RandomFloat()*1.2
			} else if region == "path" || region == "clearing" {
				// На тропах и полянах меньше камней
				if rand.Float64() < 0.7 {
					continue // 70% шанс пропустить камень
				}

				// Небольшие камни
				rockSize = 0.3 + pg.noiseGen.RandomFloat()*0.5
			}

			// Метаданные для камней
			rockMeta := map[string]float64{
				"atmosphere.ominous": 0.1 + pg.noiseGen.RandomFloat()*0.3,
				"visuals.rough":      0.4 + pg.noiseGen.RandomFloat()*0.4,
				"conditions.shadow":  0.3 + pg.noiseGen.RandomFloat()*0.3,
			}

			// Специфические метаданные для разных типов камней
			if rockType == "boulder" {
				rockMeta["atmosphere.dread"] = 0.2 + pg.noiseGen.RandomFloat()*0.2
				rockMeta["visuals.dark"] = 0.3 + pg.noiseGen.RandomFloat()*0.3
			} else if rockType == "sharp_rock" {
				rockMeta["atmosphere.tension"] = 0.3 + pg.noiseGen.RandomFloat()*0.3
				rockMeta["visuals.distorted"] = 0.2 + pg.noiseGen.RandomFloat()*0.2
			}

			// Create a rock object
			rock := &ProceduralObject{
				ID:       len(pg.currentScene.Objects) + 1,
				Type:     "rock",
				Position: Vector3{X: x, Y: elevation * 20, Z: z},
				Scale:    Vector3{X: rockSize, Y: rockSize * 0.7, Z: rockSize},
				Rotation: Vector3{X: pg.noiseGen.RandomFloat() * 0.3, Y: pg.noiseGen.RandomFloat() * 2 * math.Pi, Z: pg.noiseGen.RandomFloat() * 0.3},
				Metadata: rockMeta,
				Seed:     pg.currentScene.Seed + int64(i+1000), // Different seed range than trees
			}

			pg.currentScene.Objects = append(pg.currentScene.Objects, rock)
		}
	}

	// Генерируем странные объекты
	numStrange := int(biomeParams.StrangeDensity * 10)

	for i := 0; i < numStrange; i++ {
		// Выбираем позицию для странного объекта
		// Стараемся поместить их в места, которые усилят атмосферу страха

		// Изначально случайная позиция
		x := pg.noiseGen.RandomFloat()*float64(terrainWidth) - float64(terrainWidth)/2
		z := pg.noiseGen.RandomFloat()*float64(terrainHeight) - float64(terrainHeight)/2

		// Если возможно, помещаем их в зловещие регионы
		darkRegions := []string{"swamp", "dense_forest", "ravine", "swamp_pit"}

		// Пытаемся найти подходящий регион
		attempts := 0
		maxAttempts := 10

		for attempts < maxAttempts {
			terrainX := int(x + float64(terrainWidth)/2)
			terrainZ := int(z + float64(terrainHeight)/2)

			if terrainX >= 0 && terrainX < terrainWidth && terrainZ >= 0 && terrainZ < terrainHeight {
				region := pg.currentScene.Terrain.Regions[terrainZ][terrainX]

				// Проверяем, подходит ли регион
				isGoodRegion := false
				for _, darkRegion := range darkRegions {
					if region == darkRegion {
						isGoodRegion = true
						break
					}
				}

				if isGoodRegion {
					break // Нашли хорошее место
				}
			}

			// Пробуем новую случайную позицию
			x = pg.noiseGen.RandomFloat()*float64(terrainWidth) - float64(terrainWidth)/2
			z = pg.noiseGen.RandomFloat()*float64(terrainHeight) - float64(terrainHeight)/2
			attempts++
		}

		terrainX := int(x + float64(terrainWidth)/2)
		terrainZ := int(z + float64(terrainHeight)/2)

		// Проверяем, что координаты в пределах ландшафта
		if terrainX < 0 || terrainX >= terrainWidth || terrainZ < 0 || terrainZ >= terrainHeight {
			continue
		}

		elevation := pg.currentScene.Terrain.Data[terrainZ][terrainX]

		// Проверяем, что объект не будет в воде
		if pg.currentScene.Terrain.Materials[terrainZ][terrainX] == 1 {
			continue
		}

		// Check if position is already occupied
		posKey := fmt.Sprintf("%d,%d", terrainX, terrainZ)
		if occupiedPositions[posKey] {
			continue // Skip if position is already occupied
		}

		// Mark position as occupied
		occupiedPositions[posKey] = true

		// Выбираем тип странного объекта
		strangeTypes := []string{"obelisk", "strange_tree", "anomaly", "ritual_stones"}
		strangeType := strangeTypes[rand.Intn(len(strangeTypes))]

		// Параметры объекта в зависимости от типа
		var strangeSize, strangeHeight float64

		switch strangeType {
		case "obelisk":
			strangeSize = 0.5 + pg.noiseGen.RandomFloat()*0.5
			strangeHeight = 3.0 + pg.noiseGen.RandomFloat()*2.0

		case "strange_tree":
			strangeSize = 0.8 + pg.noiseGen.RandomFloat()*0.8
			strangeHeight = 4.0 + pg.noiseGen.RandomFloat()*3.0

		case "anomaly":
			strangeSize = 1.0 + pg.noiseGen.RandomFloat()*1.5
			strangeHeight = strangeSize

		case "ritual_stones":
			strangeSize = 1.2 + pg.noiseGen.RandomFloat()*0.8
			strangeHeight = 1.5 + pg.noiseGen.RandomFloat()*1.0
		}

		// Метаданные для странных объектов - высокие значения страха и неестественности
		strangeMeta := map[string]float64{
			"atmosphere.fear":       0.7 + pg.noiseGen.RandomFloat()*0.3,
			"atmosphere.dread":      0.8 + pg.noiseGen.RandomFloat()*0.2,
			"visuals.distorted":     0.6 + pg.noiseGen.RandomFloat()*0.4,
			"visuals.twisted":       0.7 + pg.noiseGen.RandomFloat()*0.3,
			"conditions.silhouette": 0.8 + pg.noiseGen.RandomFloat()*0.2,
			"conditions.unnatural":  0.9 + pg.noiseGen.RandomFloat()*0.1,
		}

		// Создаем странный объект
		strange := &ProceduralObject{
			ID:       len(pg.currentScene.Objects) + 1,
			Type:     "strange",
			Position: Vector3{X: x, Y: elevation * 20, Z: z},
			Scale:    Vector3{X: strangeSize, Y: strangeHeight, Z: strangeSize},
			Rotation: Vector3{X: 0, Y: pg.noiseGen.RandomFloat() * 2 * math.Pi, Z: 0},
			Metadata: strangeMeta,
			Seed:     pg.currentScene.Seed + int64(i+5000),
		}

		pg.currentScene.Objects = append(pg.currentScene.Objects, strange)
	}
}

// evolveScene evolves the scene over time
func (pg *ProceduralGenerator) evolveScene() {
	if pg.currentScene == nil {
		return
	}

	// Update weather conditions
	pg.updateWeather()

	// Randomly modify some objects
	pg.modifyObjects()

	// Occasionally add new objects or remove existing ones
	if pg.noiseGen.RandomFloat() < 0.1 { // 10% chance each evolution cycle
		if pg.noiseGen.RandomFloat() < 0.5 {
			pg.addRandomObject()
		} else {
			pg.removeRandomObject()
		}
	}

	// Случайно обновляем уровень страха
	if pg.noiseGen.RandomFloat() < 0.05 { // 5% chance
		// Небольшие случайные колебания для естественности
		randomChange := (pg.noiseGen.RandomFloat()*2.0 - 1.0) * 0.1 // ±0.1
		pg.currentScene.LevelOfFear = util.Clamp(pg.currentScene.LevelOfFear+randomChange, 0.1, 1.0)
	}
}

// updateWeather updates the weather conditions
func (pg *ProceduralGenerator) updateWeather() {
	// Get time of day factor (0 = midnight, 1 = noon)
	timeOfDay := pg.currentScene.TimeOfDay

	// Fog is thicker at night and early morning
	nightFactor := 1.0 - math.Sin(timeOfDay*math.Pi)
	baseFogValue := 0.3 + nightFactor*0.5

	// Add some randomness
	randomFactor := pg.noiseGen.Perlin1D(pg.time*0.01, pg.currentScene.Seed)
	fogValue := baseFogValue + randomFactor*0.2

	// Сглаживаем изменения тумана для естественности
	currentFog := pg.currentScene.Weather["fog"]
	pg.currentScene.Weather["fog"] = currentFog*0.9 + fogValue*0.1 // Плавный переход

	// Clamp between 0 and 1
	pg.currentScene.Weather["fog"] = math.Max(0.0, math.Min(1.0, pg.currentScene.Weather["fog"]))

	// Update other weather conditions...
	pg.currentScene.Weather["mist"] = pg.currentScene.Weather["fog"] * 0.7

	// Обновляем ветер - случайные порывы
	if pg.noiseGen.RandomFloat() < 0.2 { // 20% шанс изменения ветра
		// Базовый ветер + случайные порывы
		baseWind := 0.3 + randomFactor*0.3

		// Сильнее ветер в сумерках и на закате/рассвете
		twilightFactor := math.Sin((timeOfDay-0.25)*2.0*math.Pi)*0.5 + 0.5
		windValue := baseWind + twilightFactor*0.2

		// Сглаживаем изменения
		currentWind := pg.currentScene.Weather["wind"]
		pg.currentScene.Weather["wind"] = currentWind*0.8 + windValue*0.2

		// Clamp
		pg.currentScene.Weather["wind"] = math.Max(0.0, math.Min(1.0, pg.currentScene.Weather["wind"]))
	}
}

// modifyObjects modifies existing objects
func (pg *ProceduralGenerator) modifyObjects() {
	// Modify a random subset of objects
	for _, obj := range pg.currentScene.Objects {
		// Only modify with a small chance
		if pg.noiseGen.RandomFloat() < 0.05 { // 5% chance per object
			// Slightly modify metadata
			for key, value := range obj.Metadata {
				// Add small random variation
				delta := (pg.noiseGen.RandomFloat()*0.2 - 0.1) // -0.1 to +0.1
				obj.Metadata[key] = math.Max(0.0, math.Min(1.0, value+delta))
			}

			// Occasionally add a completely new metadata property
			if pg.noiseGen.RandomFloat() < 0.1 { // 10% chance
				// Choose a random metadata property
				categories := []string{"atmosphere", "visuals", "conditions"}
				properties := map[string][]string{
					"atmosphere": {"fear", "ominous", "dread", "tension"},
					"visuals":    {"distorted", "dark", "glitchy", "twisted"},
					"conditions": {"fog", "shadow", "silhouette", "darkness"},
				}

				category := categories[int(pg.noiseGen.RandomFloat()*float64(len(categories)))]
				propertyList := properties[category]
				property := propertyList[int(pg.noiseGen.RandomFloat()*float64(len(propertyList)))]

				// Set the property with a random value
				key := category + "." + property
				if _, exists := obj.Metadata[key]; !exists {
					obj.Metadata[key] = pg.noiseGen.RandomFloat()
				}
			}

			// Для странных объектов и деревьев иногда меняем положение
			if obj.Type == "strange" || obj.Type == "tree" {
				if pg.noiseGen.RandomFloat() < 0.05 { // 5% шанс
					// Небольшое случайное смещение
					offsetRange := 0.5
					obj.Position.X += (pg.noiseGen.RandomFloat()*2.0 - 1.0) * offsetRange
					obj.Position.Z += (pg.noiseGen.RandomFloat()*2.0 - 1.0) * offsetRange
				}
			}
		}
	}
}

// addRandomObject adds a random object to the scene
func (pg *ProceduralGenerator) addRandomObject() {
	if pg.currentScene == nil || pg.currentScene.Terrain == nil {
		return
	}

	// Decide on object type
	objectTypes := []string{"tree", "rock", "stump", "strange"}
	objectType := objectTypes[int(pg.noiseGen.RandomFloat()*float64(len(objectTypes)))]

	// Get terrain dimensions
	terrainWidth := pg.currentScene.Terrain.Width
	terrainHeight := pg.currentScene.Terrain.Height

	// Pick a random position
	x := pg.noiseGen.RandomFloat()*float64(terrainWidth) - float64(terrainWidth)/2
	z := pg.noiseGen.RandomFloat()*float64(terrainHeight) - float64(terrainHeight)/2

	// Find the terrain height at this position
	terrainX := int(x + float64(terrainWidth)/2)
	terrainZ := int(z + float64(terrainHeight)/2)

	// Ensure within bounds
	if terrainX < 0 || terrainX >= terrainWidth || terrainZ < 0 || terrainZ >= terrainHeight {
		return
	}

	elevation := pg.currentScene.Terrain.Data[terrainZ][terrainX]

	// Проверяем, что не в воде
	if pg.currentScene.Terrain.Materials[terrainZ][terrainX] == 1 {
		return
	}

	// Проверяем, что позиция не занята другим объектом
	for _, obj := range pg.currentScene.Objects {
		dist := math.Sqrt(math.Pow(obj.Position.X-x, 2) + math.Pow(obj.Position.Z-z, 2))
		if dist < 2.0 { // Минимальное расстояние между объектами
			return
		}
	}

	// Create object based on type
	var newObject *ProceduralObject

	switch objectType {
	case "tree":
		height := 2.0 + pg.noiseGen.RandomFloat()*3.0
		newObject = &ProceduralObject{
			ID:       len(pg.currentScene.Objects) + 1,
			Type:     "tree",
			Position: Vector3{X: x, Y: elevation * 20, Z: z},
			Scale:    Vector3{X: 1.0, Y: height, Z: 1.0},
			Rotation: Vector3{X: 0, Y: pg.noiseGen.RandomFloat() * 2 * math.Pi, Z: 0},
			Metadata: map[string]float64{
				"atmosphere.fear":       0.3 + pg.noiseGen.RandomFloat()*0.3,
				"atmosphere.ominous":    0.2 + pg.noiseGen.RandomFloat()*0.4,
				"visuals.distorted":     pg.noiseGen.RandomFloat() * 0.5,
				"visuals.dark":          0.3 + pg.noiseGen.RandomFloat()*0.4,
				"conditions.silhouette": 0.2 + pg.noiseGen.RandomFloat()*0.7,
			},
			Seed: pg.currentScene.Seed + int64(len(pg.currentScene.Objects)),
		}

	case "rock":
		size := 0.5 + pg.noiseGen.RandomFloat()*1.5
		newObject = &ProceduralObject{
			ID:       len(pg.currentScene.Objects) + 1,
			Type:     "rock",
			Position: Vector3{X: x, Y: elevation * 20, Z: z},
			Scale:    Vector3{X: size, Y: size, Z: size},
			Rotation: Vector3{X: pg.noiseGen.RandomFloat(), Y: pg.noiseGen.RandomFloat() * 2 * math.Pi, Z: pg.noiseGen.RandomFloat()},
			Metadata: map[string]float64{
				"atmosphere.ominous": 0.1 + pg.noiseGen.RandomFloat()*0.3,
				"visuals.rough":      0.4 + pg.noiseGen.RandomFloat()*0.4,
				"conditions.shadow":  0.3 + pg.noiseGen.RandomFloat()*0.3,
			},
			Seed: pg.currentScene.Seed + int64(len(pg.currentScene.Objects)),
		}

	case "stump":
		newObject = &ProceduralObject{
			ID:       len(pg.currentScene.Objects) + 1,
			Type:     "stump",
			Position: Vector3{X: x, Y: elevation * 20, Z: z},
			Scale:    Vector3{X: 0.8, Y: 0.5, Z: 0.8},
			Rotation: Vector3{X: 0, Y: pg.noiseGen.RandomFloat() * 2 * math.Pi, Z: 0},
			Metadata: map[string]float64{
				"atmosphere.dread":    0.4 + pg.noiseGen.RandomFloat()*0.4,
				"visuals.decay":       0.5 + pg.noiseGen.RandomFloat()*0.3,
				"conditions.darkness": 0.3 + pg.noiseGen.RandomFloat()*0.3,
			},
			Seed: pg.currentScene.Seed + int64(len(pg.currentScene.Objects)),
		}

	case "strange":
		// This is a special, more scary object that appears rarely
		size := 0.5 + pg.noiseGen.RandomFloat()
		newObject = &ProceduralObject{
			ID:       len(pg.currentScene.Objects) + 1,
			Type:     "strange",
			Position: Vector3{X: x, Y: elevation * 20, Z: z},
			Scale:    Vector3{X: size, Y: size * 3, Z: size},
			Rotation: Vector3{X: 0, Y: pg.noiseGen.RandomFloat() * 2 * math.Pi, Z: 0},
			Metadata: map[string]float64{
				"atmosphere.fear":       0.7 + pg.noiseGen.RandomFloat()*0.3,
				"atmosphere.dread":      0.8 + pg.noiseGen.RandomFloat()*0.2,
				"visuals.distorted":     0.6 + pg.noiseGen.RandomFloat()*0.4,
				"visuals.twisted":       0.7 + pg.noiseGen.RandomFloat()*0.3,
				"conditions.silhouette": 0.8 + pg.noiseGen.RandomFloat()*0.2,
				"conditions.unnatural":  0.9 + pg.noiseGen.RandomFloat()*0.1,
			},
			Seed: pg.currentScene.Seed + int64(len(pg.currentScene.Objects)),
		}
	}

	// Add the new object to the scene
	if newObject != nil {
		pg.currentScene.Objects = append(pg.currentScene.Objects, newObject)
	}
}

// removeRandomObject removes a random object from the scene
func (pg *ProceduralGenerator) removeRandomObject() {
	if pg.currentScene == nil || len(pg.currentScene.Objects) == 0 {
		return
	}

	// Choose a random object to remove
	indexToRemove := int(pg.noiseGen.RandomFloat() * float64(len(pg.currentScene.Objects)))

	// Remove the object
	pg.currentScene.Objects = append(
		pg.currentScene.Objects[:indexToRemove],
		pg.currentScene.Objects[indexToRemove+1:]...,
	)
}

// clamp ограничивает значение указанным диапазоном
func clamp(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
