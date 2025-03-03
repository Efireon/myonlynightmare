package engine

import (
	"math"
	"time"

	noise "nightmare/internal/math"
	"nightmare/pkg/config"
)

// ProceduralObject represents a procedurally generated object
type ProceduralObject struct {
	ID       int
	Type     string             // Type of object (tree, rock, terrain, etc.)
	Position Vector3            // Position in world space
	Scale    Vector3            // Scale of the object
	Rotation Vector3            // Rotation of the object (in radians)
	Metadata map[string]float64 // Hierarchical metadata with weights
	Seed     int64              // Seed for reproducibility
}

// ProceduralScene represents the current procedural scene
type ProceduralScene struct {
	Objects   []*ProceduralObject
	Terrain   *HeightMap
	TimeOfDay float64            // 0.0-1.0, 0 = midnight, 0.5 = noon
	Weather   map[string]float64 // Weather conditions (fog, rain, etc.) with weights
	Seed      int64              // Scene seed
}

// HeightMap represents terrain elevation data
type HeightMap struct {
	Width     int
	Height    int
	Data      [][]float64
	Materials [][]int
}

// ProceduralGenerator handles procedural generation of content
type ProceduralGenerator struct {
	config       config.ProceduralConfig
	currentScene *ProceduralScene
	noiseGen     *noise.NoiseGenerator
	time         float64
}

// NewProceduralGenerator creates a new procedural generator
func NewProceduralGenerator(config config.ProceduralConfig) (*ProceduralGenerator, error) {
	gen := &ProceduralGenerator{
		config:   config,
		noiseGen: noise.NewNoiseGenerator(time.Now().UnixNano()),
		time:     0,
	}

	return gen, nil
}

// GenerateInitialWorld creates the initial world
func (pg *ProceduralGenerator) GenerateInitialWorld() {
	// Create a new scene with a random seed
	seed := time.Now().UnixNano()
	pg.currentScene = &ProceduralScene{
		Objects:   make([]*ProceduralObject, 0),
		TimeOfDay: 0.2, // Early morning
		Weather:   map[string]float64{"fog": 0.7, "mist": 0.3},
		Seed:      seed,
	}

	// Generate terrain
	pg.currentScene.Terrain = pg.generateTerrain(seed)

	// Generate initial objects
	pg.populateScene()
}

// Update updates the procedural generation based on time
func (pg *ProceduralGenerator) Update(deltaTime float64) {
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
	return pg.currentScene
}

// generateTerrain generates a new terrain heightmap
func (pg *ProceduralGenerator) generateTerrain(seed int64) *HeightMap {
	width := pg.config.TerrainSize
	height := pg.config.TerrainSize

	// Create heightmap
	heightMap := &HeightMap{
		Width:     width,
		Height:    height,
		Data:      make([][]float64, height),
		Materials: make([][]int, height),
	}

	// Set up noise parameters
	scale := 0.1
	octaves := 4
	persistence := 0.5
	lacunarity := 2.0

	// Generate heightmap data
	for y := 0; y < height; y++ {
		heightMap.Data[y] = make([]float64, width)
		heightMap.Materials[y] = make([]int, width)

		for x := 0; x < width; x++ {
			// Calculate world position
			worldX := float64(x - width/2)
			worldZ := float64(y - height/2)

			// Generate elevation using multiple octaves of Perlin noise
			elevation := 0.0
			amplitude := 1.0
			frequency := 1.0

			for i := 0; i < octaves; i++ {
				sampleX := worldX * scale * frequency
				sampleZ := worldZ * scale * frequency

				// Add noise value to elevation
				noiseValue := pg.noiseGen.Perlin2D(sampleX, sampleZ, seed+int64(i))
				elevation += noiseValue * amplitude

				// Update amplitude and frequency for next octave
				amplitude *= persistence
				frequency *= lacunarity
			}

			// Normalize to 0-1 range
			elevation = (elevation + 1.0) * 0.5

			// Apply terrain features
			elevation = pg.applyTerrainFeatures(elevation, worldX, worldZ, scale, seed)

			// Store in heightmap
			heightMap.Data[y][x] = elevation

			// Determine material based on elevation
			heightMap.Materials[y][x] = pg.determineMaterial(elevation, worldX, worldZ, seed)
		}
	}

	return heightMap
}

// applyTerrainFeatures applies additional features to terrain
func (pg *ProceduralGenerator) applyTerrainFeatures(baseElevation, x, z, scale float64, seed int64) float64 {
	elevation := baseElevation

	// Add a central depression (like a valley or dried lake)
	distFromCenter := math.Sqrt(x*x+z*z) * 0.01
	valleyDepression := math.Max(0, 1.0-distFromCenter)
	valleyDepression = math.Pow(valleyDepression, 3) * 0.3 // Cubic falloff, scaled

	elevation -= valleyDepression

	// Add some ridges
	ridgeNoise := pg.noiseGen.Ridge2D(x*scale*1.5, z*scale*1.5, seed+123)
	ridgeInfluence := 0.2 // How much ridges affect the terrain
	elevation += ridgeNoise * ridgeInfluence

	// Ensure elevation is in 0-1 range
	elevation = math.Max(0.0, math.Min(1.0, elevation))

	return elevation
}

// determineMaterial determines the material type based on elevation and position
func (pg *ProceduralGenerator) determineMaterial(elevation, x, z float64, seed int64) int {
	// Simple material assignment based on elevation
	if elevation < 0.3 {
		return 1 // Water/low ground
	} else if elevation < 0.5 {
		return 2 // Dirt/grass
	} else if elevation < 0.7 {
		return 3 // Rock
	} else {
		return 4 // Snow/peaks
	}
}

// populateScene populates the scene with objects
func (pg *ProceduralGenerator) populateScene() {
	if pg.currentScene == nil || pg.currentScene.Terrain == nil {
		return
	}

	// Get terrain dimensions
	terrainWidth := pg.currentScene.Terrain.Width
	terrainHeight := pg.currentScene.Terrain.Height

	// Generate trees
	numTrees := int(pg.config.TreeDensity) * terrainWidth * terrainHeight / 10000

	for i := 0; i < int(numTrees); i++ {
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

		// Only place trees at certain elevations
		if elevation > 0.3 && elevation < 0.7 {
			// Create a tree object
			treeHeight := 2.0 + pg.noiseGen.RandomFloat()*3.0

			// Create hierarchical metadata
			metadata := map[string]float64{
				"atmosphere.fear":       0.3 + pg.noiseGen.RandomFloat()*0.3,
				"atmosphere.ominous":    0.2 + pg.noiseGen.RandomFloat()*0.4,
				"visuals.distorted":     pg.noiseGen.RandomFloat() * 0.5,
				"visuals.dark":          0.3 + pg.noiseGen.RandomFloat()*0.4,
				"conditions.silhouette": 0.2 + pg.noiseGen.RandomFloat()*0.7,
			}

			tree := &ProceduralObject{
				ID:       len(pg.currentScene.Objects) + 1,
				Type:     "tree",
				Position: Vector3{X: x, Y: elevation * 20, Z: z}, // Scale elevation
				Scale:    Vector3{X: 1.0, Y: treeHeight, Z: 1.0},
				Rotation: Vector3{X: 0, Y: pg.noiseGen.RandomFloat() * 2 * math.Pi, Z: 0},
				Metadata: metadata,
				Seed:     pg.currentScene.Seed + int64(i),
			}

			pg.currentScene.Objects = append(pg.currentScene.Objects, tree)
		}
	}

	// Generate rocks
	numRocks := int(pg.config.RockDensity) * terrainWidth * terrainHeight / 10000

	for i := 0; i < int(numRocks); i++ {
		// Similar logic as for trees
		x := pg.noiseGen.RandomFloat()*float64(terrainWidth) - float64(terrainWidth)/2
		z := pg.noiseGen.RandomFloat()*float64(terrainHeight) - float64(terrainHeight)/2

		terrainX := int(x + float64(terrainWidth)/2)
		terrainZ := int(z + float64(terrainHeight)/2)

		if terrainX < 0 || terrainX >= terrainWidth || terrainZ < 0 || terrainZ >= terrainHeight {
			continue
		}

		elevation := pg.currentScene.Terrain.Data[terrainZ][terrainX]

		// Rocks can be at more places than trees
		if elevation > 0.2 {
			rockSize := 0.5 + pg.noiseGen.RandomFloat()*1.5

			// Create hierarchical metadata
			metadata := map[string]float64{
				"atmosphere.ominous": 0.1 + pg.noiseGen.RandomFloat()*0.3,
				"visuals.rough":      0.4 + pg.noiseGen.RandomFloat()*0.4,
				"conditions.shadow":  0.3 + pg.noiseGen.RandomFloat()*0.3,
			}

			rock := &ProceduralObject{
				ID:       len(pg.currentScene.Objects) + 1,
				Type:     "rock",
				Position: Vector3{X: x, Y: elevation * 20, Z: z},
				Scale:    Vector3{X: rockSize, Y: rockSize, Z: rockSize},
				Rotation: Vector3{X: pg.noiseGen.RandomFloat(), Y: pg.noiseGen.RandomFloat() * 2 * math.Pi, Z: pg.noiseGen.RandomFloat()},
				Metadata: metadata,
				Seed:     pg.currentScene.Seed + int64(i+1000), // Different seed range than trees
			}

			pg.currentScene.Objects = append(pg.currentScene.Objects, rock)
		}
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
}

// updateWeather updates the weather conditions
func (pg *ProceduralGenerator) updateWeather() {
	// Get time of day factor (0 = midnight, 1 = noon)
	timeOfDay := pg.currentScene.TimeOfDay

	// Fog is thicker at night and early morning
	nightFactor := 1.0 - math.Sin(timeOfDay*math.Pi)
	pg.currentScene.Weather["fog"] = 0.3 + nightFactor*0.5

	// Add some randomness
	randomFactor := pg.noiseGen.Perlin1D(pg.time*0.01, pg.currentScene.Seed)
	pg.currentScene.Weather["fog"] += randomFactor * 0.2

	// Clamp between 0 and 1
	pg.currentScene.Weather["fog"] = math.Max(0.0, math.Min(1.0, pg.currentScene.Weather["fog"]))

	// Update other weather conditions...
	pg.currentScene.Weather["mist"] = pg.currentScene.Weather["fog"] * 0.7
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
		}
	}
}

// addRandomObject adds a random object to the scene
func (pg *ProceduralGenerator) addRandomObject() {
	if pg.currentScene.Terrain == nil {
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
	if len(pg.currentScene.Objects) == 0 {
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
