package engine

import (
	"math"
	"sync"

	"nightmare/pkg/config"
)

// Vector3 represents a 3D vector
type Vector3 struct {
	X, Y, Z float64
}

// Add adds two vectors
func (v Vector3) Add(other Vector3) Vector3 {
	return Vector3{
		X: v.X + other.X,
		Y: v.Y + other.Y,
		Z: v.Z + other.Z,
	}
}

// Sub subtracts a vector from another
func (v Vector3) Sub(other Vector3) Vector3 {
	return Vector3{
		X: v.X - other.X,
		Y: v.Y - other.Y,
		Z: v.Z - other.Z,
	}
}

// Mul multiplies a vector by a scalar
func (v Vector3) Mul(scalar float64) Vector3 {
	return Vector3{
		X: v.X * scalar,
		Y: v.Y * scalar,
		Z: v.Z * scalar,
	}
}

// Dot calculates the dot product of two vectors
func (v Vector3) Dot(other Vector3) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

// Normalize returns a normalized (unit) vector
func (v Vector3) Normalize() Vector3 {
	len := math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
	if len == 0 {
		return v
	}
	return Vector3{
		X: v.X / len,
		Y: v.Y / len,
		Z: v.Z / len,
	}
}

// Ray represents a ray in 3D space
type Ray struct {
	Origin    Vector3
	Direction Vector3
}

// HitInfo contains information about a ray hit
type HitInfo struct {
	Distance   float64
	Position   Vector3
	Normal     Vector3
	ObjectID   int
	MaterialID int
	Density    float64 // Used for ASCII intensity
}

// TracedPixel represents the result of a ray trace
type TracedPixel struct {
	X, Y      int
	Intensity float64 // 0.0-1.0 intensity for ASCII rendering
	ObjectID  int     // ID of the hit object
	Depth     float64 // Depth from camera
}

// SceneData contains the full result of tracing the scene
type SceneData struct {
	Pixels [][]TracedPixel
	Width  int
	Height int
}

// Raytracer handles ray tracing operations
type Raytracer struct {
	config config.RaytracerConfig
	camera struct {
		Position Vector3
		Forward  Vector3
		Up       Vector3
		Right    Vector3
		FOV      float64
	}
	scene  *ProceduralScene
	width  int
	height int
}

// NewRaytracer creates a new raytracer with the given configuration
func NewRaytracer(config config.RaytracerConfig) (*Raytracer, error) {
	rt := &Raytracer{
		config: config,
		width:  config.Width,
		height: config.Height,
	}

	// Setup default camera
	rt.camera.Position = Vector3{X: 0, Y: 0, Z: -5}
	rt.camera.Forward = Vector3{X: 0, Y: 0, Z: 1}.Normalize()
	rt.camera.Up = Vector3{X: 0, Y: 1, Z: 0}.Normalize()
	rt.camera.Right = Vector3{X: 1, Y: 0, Z: 0}.Normalize()
	rt.camera.FOV = 60.0 * (math.Pi / 180.0) // Convert to radians

	return rt, nil
}

// SetScene sets the current scene to trace
func (rt *Raytracer) SetScene(scene *ProceduralScene) {
	rt.scene = scene
}

// TraceScene performs ray tracing for the entire scene
func (rt *Raytracer) TraceScene() *SceneData {
	// Создаем пустую сцену
	sceneData := &SceneData{
		Width:  rt.width,
		Height: rt.height,
		Pixels: make([][]TracedPixel, rt.height),
	}

	// Инициализируем каждую строку пикселей
	for y := 0; y < rt.height; y++ {
		sceneData.Pixels[y] = make([]TracedPixel, rt.width)
	}

	// Если сцена не задана, просто возвращаем пустую инициализированную сцену
	if rt.scene == nil {
		return sceneData
	}

	// Use goroutines for parallel ray tracing
	var wg sync.WaitGroup

	// Determine the number of goroutines based on CPU cores
	numGoroutines := rt.config.NumThreads
	rowsPerGoroutine := rt.height / numGoroutines

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)

		// Calculate row range for this goroutine
		startRow := g * rowsPerGoroutine
		endRow := startRow + rowsPerGoroutine
		if g == numGoroutines-1 {
			endRow = rt.height // Make sure to cover all rows
		}

		go func(startRow, endRow int) {
			defer wg.Done()

			// Process each row assigned to this goroutine
			for y := startRow; y < endRow; y++ {
				for x := 0; x < rt.width; x++ {
					// Convert pixel coordinates to normalized device coordinates (-1 to 1)
					ndcX := (2.0 * float64(x) / float64(rt.width)) - 1.0
					ndcY := 1.0 - (2.0 * float64(y) / float64(rt.height))

					// Calculate aspect ratio
					aspectRatio := float64(rt.width) / float64(rt.height)

					// Calculate the ray direction in camera space
					rayDirX := ndcX * math.Tan(rt.camera.FOV/2) * aspectRatio
					rayDirY := ndcY * math.Tan(rt.camera.FOV/2)

					// Transform to world space
					rayDir := rt.camera.Forward.Add(
						rt.camera.Right.Mul(rayDirX).Add(
							rt.camera.Up.Mul(rayDirY),
						),
					).Normalize()

					// Create the ray
					ray := Ray{
						Origin:    rt.camera.Position,
						Direction: rayDir,
					}

					// Trace the ray
					hitInfo := rt.trace(ray)

					// Record the result
					sceneData.Pixels[y][x] = TracedPixel{
						X:         x,
						Y:         y,
						Intensity: calculateIntensity(hitInfo),
						ObjectID:  hitInfo.ObjectID,
						Depth:     hitInfo.Distance,
					}
				}
			}
		}(startRow, endRow)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	return sceneData
}

// trace traces a single ray and returns hit information
func (rt *Raytracer) trace(ray Ray) HitInfo {
	// For now, just do a simple sphere test as placeholder
	// In a real implementation, this would query the scene's objects

	// No hit by default
	hitInfo := HitInfo{
		Distance:   math.MaxFloat64,
		ObjectID:   -1,
		MaterialID: -1,
		Density:    0,
	}

	// If we have a scene, query it
	if rt.scene != nil {
		// This is where the actual scene tracing would happen
		// For now, just a placeholder
		sphereCenter := Vector3{X: 0, Y: 0, Z: 0}
		sphereRadius := 2.0

		// Check for sphere intersection
		oc := ray.Origin.Sub(sphereCenter)
		a := ray.Direction.Dot(ray.Direction)
		b := 2.0 * oc.Dot(ray.Direction)
		c := oc.Dot(oc) - sphereRadius*sphereRadius
		discriminant := b*b - 4*a*c

		if discriminant > 0 {
			t := (-b - math.Sqrt(discriminant)) / (2.0 * a)
			if t > 0.001 {
				hitInfo.Distance = t
				hitInfo.Position = ray.Origin.Add(ray.Direction.Mul(t))
				hitInfo.Normal = hitInfo.Position.Sub(sphereCenter).Normalize()
				hitInfo.ObjectID = 1 // Arbitrary ID for the sphere
				hitInfo.MaterialID = 1

				// For density, calculate based on normal and view angle
				hitInfo.Density = math.Abs(hitInfo.Normal.Dot(ray.Direction.Mul(-1)))
			}
		}
	}

	return hitInfo
}

// calculateIntensity converts hit information to a normalized intensity value for ASCII rendering
func calculateIntensity(hit HitInfo) float64 {
	if hit.ObjectID == -1 {
		return 0 // No hit, darkness
	}

	// Start with the density from the hit
	intensity := hit.Density

	// Could add more factors here like:
	// - Material properties
	// - Distance falloff
	// - Lighting calculations

	// Ensure it's in 0-1 range
	if intensity < 0 {
		intensity = 0
	} else if intensity > 1 {
		intensity = 1
	}

	return intensity
}
