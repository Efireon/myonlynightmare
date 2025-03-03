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
// Raytracer handles ray tracing operations
type Raytracer struct {
	config config.RaytracerConfig
	camera struct {
		Position Vector3
		Forward  Vector3
		Up       Vector3
		Right    Vector3
		FOV      float64
		Yaw      float64 // Горизонтальный угол поворота в радианах
		Pitch    float64 // Вертикальный угол поворота в радианах
	}
	scene  *ProceduralScene
	width  int
	height int
}

// UpdateCamera обновляет положение и ориентацию камеры
func (rt *Raytracer) UpdateCamera(positionDelta Vector3, yawDelta, pitchDelta float64) {
	// Обновляем углы поворота
	rt.camera.Yaw += yawDelta
	rt.camera.Pitch += pitchDelta

	// Ограничиваем угол наклона (pitch) чтобы избежать переворота камеры
	if rt.camera.Pitch > 1.5 {
		rt.camera.Pitch = 1.5
	}
	if rt.camera.Pitch < -1.5 {
		rt.camera.Pitch = -1.5
	}

	// Пересчитываем векторы направления камеры
	rt.camera.Forward = Vector3{
		X: math.Cos(rt.camera.Yaw) * math.Cos(rt.camera.Pitch),
		Y: math.Sin(rt.camera.Pitch),
		Z: math.Sin(rt.camera.Yaw) * math.Cos(rt.camera.Pitch),
	}.Normalize()

	// Пересчитываем правый вектор, используя мировой "вверх"
	worldUp := Vector3{X: 0, Y: 1, Z: 0}
	rt.camera.Right = worldUp.Cross(rt.camera.Forward).Normalize()

	// Пересчитываем верхний вектор, используя правый и вперед
	rt.camera.Up = rt.camera.Forward.Cross(rt.camera.Right).Normalize()

	// Обновляем позицию камеры
	// Для перемещения мы используем направление камеры, а не мировые оси
	rt.camera.Position = rt.camera.Position.Add(
		rt.camera.Forward.Mul(positionDelta.Z).Add(
			rt.camera.Right.Mul(positionDelta.X)).Add(
			worldUp.Mul(positionDelta.Y)))
}

// NewRaytracer creates a new raytracer with the given configuration
// NewRaytracer creates a new raytracer with the given configuration
func NewRaytracer(config config.RaytracerConfig) (*Raytracer, error) {
	rt := &Raytracer{
		config: config,
		width:  config.Width,
		height: config.Height,
	}

	// Устанавливаем начальные углы для камеры
	rt.camera.Yaw = -math.Pi / 2 // Направление -Z (смотрим вперед)
	rt.camera.Pitch = -0.3       // Немного вниз

	// Задаем начальную позицию камеры
	rt.camera.Position = Vector3{X: 0, Y: 10, Z: -15}

	// Вычисляем направление "вперед" на основе углов
	rt.camera.Forward = Vector3{
		X: math.Cos(rt.camera.Yaw) * math.Cos(rt.camera.Pitch),
		Y: math.Sin(rt.camera.Pitch),
		Z: math.Sin(rt.camera.Yaw) * math.Cos(rt.camera.Pitch),
	}.Normalize()

	// Вычисляем правый вектор, используя мировой "вверх"
	worldUp := Vector3{X: 0, Y: 1, Z: 0}
	rt.camera.Right = worldUp.Cross(rt.camera.Forward).Normalize()

	// Вычисляем верхний вектор камеры
	rt.camera.Up = rt.camera.Forward.Cross(rt.camera.Right).Normalize()

	// Угол поля зрения
	rt.camera.FOV = 60.0 * (math.Pi / 180.0) // 60 градусов в радианах

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
	// No hit by default
	hitInfo := HitInfo{
		Distance:   math.MaxFloat64,
		ObjectID:   -1,
		MaterialID: -1,
		Density:    0,
	}

	// If we have a scene, query it
	if rt.scene == nil {
		return hitInfo
	}

	// Сперва пробуем пересечь объекты сцены
	// Проверяем каждый объект в сцене на пересечение с лучом
	closestDist := math.MaxFloat64

	// Если в сцене есть объекты
	if rt.scene.Objects != nil {
		for _, obj := range rt.scene.Objects {
			// Простая проверка пересечения с бокс-сферой объекта
			objPos := obj.Position
			objRadius := math.Max(obj.Scale.X, math.Max(obj.Scale.Y, obj.Scale.Z)) * 0.5

			// Проверяем пересечение с примитивной сферой
			oc := ray.Origin.Sub(objPos)
			a := ray.Direction.Dot(ray.Direction)
			b := 2.0 * oc.Dot(ray.Direction)
			c := oc.Dot(oc) - objRadius*objRadius
			discriminant := b*b - 4*a*c

			if discriminant > 0 {
				t := (-b - math.Sqrt(discriminant)) / (2.0 * a)
				if t > 0.001 && t < closestDist {
					closestDist = t
					hitInfo.Distance = t
					hitInfo.Position = ray.Origin.Add(ray.Direction.Mul(t))
					hitInfo.Normal = hitInfo.Position.Sub(objPos).Normalize()
					hitInfo.ObjectID = obj.ID

					// Материал зависит от типа объекта
					switch obj.Type {
					case "tree":
						hitInfo.MaterialID = 1
					case "rock":
						hitInfo.MaterialID = 2
					case "strange":
						hitInfo.MaterialID = 3
					default:
						hitInfo.MaterialID = 1
					}

					// Для плотности используем комбинацию нормали и угла просмотра
					viewAngle := math.Abs(hitInfo.Normal.Dot(ray.Direction.Mul(-1)))

					// Учитываем атмосферный страх, если он есть
					fear := 1.0
					if val, ok := obj.Metadata["atmosphere.fear"]; ok {
						fear = val
					}

					hitInfo.Density = viewAngle * (0.5 + fear*0.5)
				}
			}
		}
	}

	// Если не попали ни в один объект, и есть терраин, пробуем пересечь терраин
	if hitInfo.ObjectID == -1 && rt.scene.Terrain != nil {
		// Упрощенное пересечение с плоскостью земли
		if ray.Direction.Y < 0 {
			// Плоскость земли находится на y = 0
			t := -ray.Origin.Y / ray.Direction.Y
			if t > 0.001 && t < closestDist {
				hitPos := ray.Origin.Add(ray.Direction.Mul(t))

				// Определяем, находится ли точка пересечения в пределах ландшафта
				terrainWidth := float64(rt.scene.Terrain.Width)
				terrainHeight := float64(rt.scene.Terrain.Height)
				halfWidth := terrainWidth / 2
				halfHeight := terrainHeight / 2

				if hitPos.X >= -halfWidth && hitPos.X < halfWidth &&
					hitPos.Z >= -halfHeight && hitPos.Z < halfHeight {

					// Получаем координаты точки в пространстве терраина
					terrainX := int(hitPos.X + halfWidth)
					terrainZ := int(hitPos.Z + halfHeight)

					// Получаем высоту и материал в этой точке
					if terrainX >= 0 && terrainX < int(terrainWidth) &&
						terrainZ >= 0 && terrainZ < int(terrainHeight) {
						elevation := rt.scene.Terrain.Data[terrainZ][terrainX]
						materialID := rt.scene.Terrain.Materials[terrainZ][terrainX]

						// Обновляем информацию о попадании
						hitInfo.Distance = t
						hitInfo.Position = hitPos
						hitInfo.Normal = Vector3{X: 0, Y: 1, Z: 0} // Простая нормаль вверх
						hitInfo.ObjectID = 1000                    // Специальный ID для терраина
						hitInfo.MaterialID = materialID

						// Вычисляем плотность на основе высоты и других факторов
						hitInfo.Density = 0.3 + elevation*0.7
					}
				}
			}
		}
	}

	// Если всё ещё нет попадания, рисуем простой тестовый объект чтобы убедиться, что рендер работает
	if hitInfo.ObjectID == -1 {
		// Простая сфера для тестирования
		sphereCenter := Vector3{X: 0, Y: 0, Z: 5}
		sphereRadius := 2.0

		// Проверяем пересечение со сферой
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
				hitInfo.ObjectID = 999 // Специальный ID для тестовой сферы
				hitInfo.MaterialID = 0

				// Для плотности используем комбинацию нормали и угла просмотра
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

	// Начальное значение интенсивности на основе плотности
	intensity := hit.Density

	// Изменяем интенсивность в зависимости от типа объекта
	if hit.ObjectID >= 1000 {
		// Это терраин, немного увеличиваем базовую яркость
		intensity *= 0.8
	} else if hit.ObjectID == 999 {
		// Тестовая сфера, делаем её более яркой
		intensity = math.Min(intensity*1.5, 1.0)
	} else {
		// Обычные объекты, корректируем интенсивность в зависимости от материала
		switch hit.MaterialID {
		case 1: // Деревья
			intensity *= 0.7
		case 2: // Камни
			intensity *= 0.9
		case 3: // Странные объекты
			intensity *= 1.2 // Можно даже выйти за пределы 1.0 для особого эффекта
		}
	}

	// Добавим немного базовой яркости, чтобы не было полностью чёрных объектов
	intensity = 0.1 + 0.9*intensity

	// Применяем небольшую корректировку гаммы для лучшей видимости
	intensity = math.Pow(intensity, 0.8)

	// Ensure it's in 0-1 range
	if intensity < 0 {
		intensity = 0
	} else if intensity > 1 {
		intensity = 1
	}

	return intensity
}

// Cross вычисляет векторное произведение двух векторов
func (v Vector3) Cross(other Vector3) Vector3 {
	return Vector3{
		X: v.Y*other.Z - v.Z*other.Y,
		Y: v.Z*other.X - v.X*other.Z,
		Z: v.X*other.Y - v.Y*other.X,
	}
}
