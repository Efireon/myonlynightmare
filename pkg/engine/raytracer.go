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

// Cross calculates the cross product of two vectors
func (v Vector3) Cross(other Vector3) Vector3 {
	return Vector3{
		X: v.Y*other.Z - v.Z*other.Y,
		Y: v.Z*other.X - v.X*other.Z,
		Z: v.X*other.Y - v.Y*other.X,
	}
}

// Length returns the length of the vector
func (v Vector3) Length() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

// Normalize returns a normalized (unit) vector
func (v Vector3) Normalize() Vector3 {
	len := v.Length()
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
	ObjectType string
	MaterialID int
	Color      Vector3
	Intensity  float64 // Used for ASCII intensity
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
		// Добавляем поддержку ограничения поворота камеры вверх/вниз
		Pitch float64 // вертикальный угол (в радианах)
		Yaw   float64 // горизонтальный угол (в радианах)
	}
	scene  *ProceduralScene
	width  int
	height int
	mutex  sync.Mutex
}

// NewRaytracer creates a new raytracer with the given configuration
func NewRaytracer(config config.RaytracerConfig) (*Raytracer, error) {
	rt := &Raytracer{
		config: config,
		width:  config.Width,
		height: config.Height,
	}

	// Setup default camera
	rt.camera.Position = Vector3{X: 0, Y: 1.7, Z: -5} // высота глаз человека ~1.7м
	rt.camera.Forward = Vector3{X: 0, Y: 0, Z: 1}.Normalize()
	rt.camera.Up = Vector3{X: 0, Y: 1, Z: 0}.Normalize()
	rt.camera.Right = Vector3{X: 1, Y: 0, Z: 0}.Normalize()
	rt.camera.FOV = 60.0 * (math.Pi / 180.0) // Convert to radians
	rt.camera.Pitch = 0.0                    // смотрим прямо
	rt.camera.Yaw = 0.0

	return rt, nil
}

// UpdateResolution updates the resolution of the raytracer
func (rt *Raytracer) UpdateResolution(width, height int) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	rt.width = width
	rt.height = height
	rt.config.Width = width
	rt.config.Height = height
}

// SetScene sets the current scene to trace
func (rt *Raytracer) SetScene(scene *ProceduralScene) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	rt.scene = scene
}

// SetCameraPosition sets the camera position
func (rt *Raytracer) SetCameraPosition(position Vector3) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	rt.camera.Position = position
}

// GetCameraPosition returns the current camera position
func (rt *Raytracer) GetCameraPosition() Vector3 {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	return rt.camera.Position
}

// GetCameraDirection returns the current forward direction of the camera
func (rt *Raytracer) GetCameraDirection() Vector3 {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	return rt.camera.Forward
}

// GetCameraRight returns the current right vector of the camera
func (rt *Raytracer) GetCameraRight() Vector3 {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	return rt.camera.Right
}

// RotateCamera rotates the camera by the given angles (in radians)
func (rt *Raytracer) RotateCamera(yawDelta, pitchDelta float64) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	// Обновляем углы камеры
	rt.camera.Yaw += yawDelta
	rt.camera.Pitch += pitchDelta

	// Ограничиваем углы наклона (pitch), чтобы не перевернуть камеру
	const maxPitch = math.Pi/2.0 - 0.1 // Немного меньше 90 градусов
	if rt.camera.Pitch > maxPitch {
		rt.camera.Pitch = maxPitch
	} else if rt.camera.Pitch < -maxPitch {
		rt.camera.Pitch = -maxPitch
	}

	// Вычисляем новые векторы направления
	// Сначала вычисляем вектор направления (Forward)
	rt.camera.Forward.X = math.Cos(rt.camera.Pitch) * math.Sin(rt.camera.Yaw)
	rt.camera.Forward.Y = math.Sin(rt.camera.Pitch)
	rt.camera.Forward.Z = math.Cos(rt.camera.Pitch) * math.Cos(rt.camera.Yaw)
	rt.camera.Forward = rt.camera.Forward.Normalize()

	// Пересчитываем правый вектор (Right) как перпендикулярный Forward и мировому Up
	worldUp := Vector3{X: 0, Y: 1, Z: 0}
	rt.camera.Right = worldUp.Cross(rt.camera.Forward).Normalize()

	// Пересчитываем вектор Up как перпендикулярный Forward и Right
	rt.camera.Up = rt.camera.Forward.Cross(rt.camera.Right).Normalize()
}

// TraceScene performs ray tracing for the entire scene
func (rt *Raytracer) TraceScene() *SceneData {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	// Создаем пустую сцену
	sceneData := &SceneData{
		Width:          rt.width,
		Height:         rt.height,
		Pixels:         make([][]TracedPixel, rt.height),
		SpecialEffects: make(map[string]float64),
	}

	// Инициализируем каждую строку пикселей
	for y := 0; y < rt.height; y++ {
		sceneData.Pixels[y] = make([]TracedPixel, rt.width)
	}

	// Если сцена не задана, просто возвращаем пустую инициализированную сцену
	if rt.scene == nil {
		return sceneData
	}

	// Заполняем информацию о специальных эффектах из сцены
	if rt.scene.Weather != nil {
		if fogLevel, ok := rt.scene.Weather["fog"]; ok {
			sceneData.SpecialEffects["fog"] = fogLevel
		}
	}

	// Вычисляем значение темноты в зависимости от времени суток
	timeOfDay := rt.scene.TimeOfDay
	if timeOfDay < 0.25 || timeOfDay > 0.75 { // ночь или вечер
		// Определяем интенсивность темноты
		darknessIntensity := 0.0
		if timeOfDay < 0.25 { // ночь
			// 0.0 (полночь) -> 1.0, 0.25 (утро) -> 0.0
			darknessIntensity = 1.0 - (timeOfDay / 0.25 * 4.0)
		} else { // вечер
			// 0.75 (вечер) -> 0.0, 1.0 (полночь) -> 1.0
			darknessIntensity = (timeOfDay - 0.75) / 0.25 * 4.0
		}
		sceneData.SpecialEffects["darkness"] = darknessIntensity
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
						X:          x,
						Y:          y,
						Intensity:  calculateIntensity(hitInfo),
						ObjectID:   hitInfo.ObjectID,
						ObjectType: hitInfo.ObjectType,
						Depth:      hitInfo.Distance,
						Normal:     hitInfo.Normal,
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
		ObjectType: "none",
		MaterialID: -1,
		Intensity:  0,
	}

	// If we have a scene, query it
	if rt.scene != nil {
		// Проверяем пересечение с ландшафтом
		terrainHit := rt.traceTerrainIntersection(ray)

		// Если есть пересечение, обновляем информацию о ближайшем объекте
		if terrainHit.ObjectID != -1 && terrainHit.Distance < hitInfo.Distance {
			hitInfo = terrainHit
		}

		// Проверяем пересечение с объектами сцены
		for _, obj := range rt.scene.Objects {
			objHit := rt.traceObjectIntersection(ray, obj)

			// Если есть пересечение и оно ближе текущего, обновляем
			if objHit.ObjectID != -1 && objHit.Distance < hitInfo.Distance {
				hitInfo = objHit
			}
		}

		// Добавляем эффект тумана
		if fogAmount, ok := rt.scene.Weather["fog"]; ok && fogAmount > 0 {
			// Применяем туман только если был хит
			if hitInfo.ObjectID != -1 {
				// Рассчитываем затухание по расстоянию (экспоненциальный туман)
				fogDensity := 0.05 * fogAmount
				fogFactor := math.Exp(-fogDensity * hitInfo.Distance)

				// Ограничиваем до [0, 1]
				fogFactor = math.Max(0.0, math.Min(1.0, fogFactor))

				// Смешиваем интенсивность с туманом
				// Чем дальше объект, тем больше влияния тумана (меньше контраста)
				fogIntensity := 0.2 // Туман имеет базовую видимость
				hitInfo.Intensity = hitInfo.Intensity*fogFactor + fogIntensity*(1.0-fogFactor)
			}
		}
	}

	return hitInfo
}

// traceTerrainIntersection проверяет пересечение луча с ландшафтом
func (rt *Raytracer) traceTerrainIntersection(ray Ray) HitInfo {
	hitInfo := HitInfo{
		Distance:   math.MaxFloat64,
		ObjectID:   -1,
		ObjectType: "terrain",
		MaterialID: 0,
		Intensity:  0,
	}

	// Пропускаем, если сцены или ландшафта нет
	if rt.scene == nil || rt.scene.Terrain == nil {
		return hitInfo
	}

	terrain := rt.scene.Terrain

	// Определяем размеры ландшафта
	terrainSizeX := float64(terrain.Width)
	terrainSizeZ := float64(terrain.Height)
	terrainScale := 1.0 // масштаб по Y (высота)

	// Центрируем ландшафт
	terrainOffset := Vector3{
		X: -terrainSizeX / 2.0,
		Y: 0,
		Z: -terrainSizeZ / 2.0,
	}

	// Проверяем, может ли луч вообще пересечь плоскость ландшафта
	// Луч параллелен плоскости Y=0 если ray.Direction.Y почти 0
	if math.Abs(ray.Direction.Y) < 0.0001 {
		return hitInfo
	}

	// Проверяем грубое пересечение с плоскостью ландшафта
	// Находим расстояние до плоскости Y=0
	t := -ray.Origin.Y / ray.Direction.Y

	// Только положительные расстояния имеют смысл
	if t <= 0 {
		return hitInfo
	}

	// Находим точку пересечения с плоскостью
	planeHit := ray.Origin.Add(ray.Direction.Mul(t))

	// Проверяем, попадает ли точка в границы ландшафта
	localX := planeHit.X - terrainOffset.X
	localZ := planeHit.Z - terrainOffset.Z

	if localX < 0 || localX >= terrainSizeX || localZ < 0 || localZ >= terrainSizeZ {
		return hitInfo
	}

	// Теперь выполняем более точную проверку по высоте ландшафта
	// Используем билинейную интерполяцию для более точного значения высоты
	gridX := int(localX)
	gridZ := int(localZ)

	// Ограничиваем индексы
	if gridX >= terrain.Width-1 {
		gridX = terrain.Width - 2
	}
	if gridZ >= terrain.Height-1 {
		gridZ = terrain.Height - 2
	}

	// Получаем высоты в четырех ближайших точках
	h00 := terrain.Data[gridZ][gridX] * terrainScale
	h10 := terrain.Data[gridZ][gridX+1] * terrainScale
	h01 := terrain.Data[gridZ+1][gridX] * terrainScale
	h11 := terrain.Data[gridZ+1][gridX+1] * terrainScale

	// Используем дробную часть для интерполяции
	fracX := localX - float64(gridX)
	fracZ := localZ - float64(gridZ)

	// Билинейная интерполяция
	h0 := h00*(1-fracX) + h10*fracX
	h1 := h01*(1-fracX) + h11*fracX
	terrainHeight := h0*(1-fracZ) + h1*fracZ

	// Теперь проверяем пересечение луча с этой высотой
	// Вычисляем время, за которое луч достигнет этой высоты
	tHeight := (terrainHeight - ray.Origin.Y) / ray.Direction.Y

	if tHeight <= 0 {
		return hitInfo
	}

	// Находим точку пересечения
	hitPoint := ray.Origin.Add(ray.Direction.Mul(tHeight))

	// Проверяем, находится ли точка в границах ландшафта
	hitLocalX := hitPoint.X - terrainOffset.X
	hitLocalZ := hitPoint.Z - terrainOffset.Z

	if hitLocalX < 0 || hitLocalX >= terrainSizeX || hitLocalZ < 0 || hitLocalZ >= terrainSizeZ {
		return hitInfo
	}

	// Вычисляем нормаль к поверхности ландшафта
	// Градиент по X и Z дает нам направление склона
	gradX := (h10-h00)*(1-fracZ) + (h11-h01)*fracZ
	gradZ := (h01-h00)*(1-fracX) + (h11-h10)*fracX

	// Нормаль - это вектор, перпендикулярный поверхности
	normal := Vector3{X: -gradX, Y: 1.0, Z: -gradZ}.Normalize()

	// Получаем материал в точке
	materialID := 0
	if gridZ < len(terrain.Materials) && gridX < len(terrain.Materials[gridZ]) {
		materialID = terrain.Materials[gridZ][gridX]
	}

	// Определяем интенсивность в зависимости от материала и освещения
	intensity := computeTerrainIntensity(normal, materialID, rt.scene.TimeOfDay)

	// Заполняем информацию о пересечении
	hitInfo.Distance = tHeight
	hitInfo.Position = hitPoint
	hitInfo.Normal = normal
	hitInfo.ObjectID = 0 // ID ландшафта = 0
	hitInfo.MaterialID = materialID
	hitInfo.Intensity = intensity

	return hitInfo
}

// traceObjectIntersection проверяет пересечение луча с объектом сцены
func (rt *Raytracer) traceObjectIntersection(ray Ray, obj *ProceduralObject) HitInfo {
	hitInfo := HitInfo{
		Distance:   math.MaxFloat64,
		ObjectID:   -1,
		ObjectType: "none",
		MaterialID: -1,
		Intensity:  0,
	}

	// В зависимости от типа объекта проверяем разные примитивы
	switch obj.Type {
	case "tree":
		// Упрощаем дерево как цилиндр для ствола и сферу для кроны
		trunkHit := rt.traceCylinderIntersection(ray,
			obj.Position,
			obj.Position.Add(Vector3{X: 0, Y: obj.Scale.Y * 0.6, Z: 0}),
			obj.Scale.X*0.3)

		crownCenter := obj.Position.Add(Vector3{X: 0, Y: obj.Scale.Y * 0.7, Z: 0})
		crownHit := rt.traceSphereIntersection(ray, crownCenter, obj.Scale.X*0.8)

		// Выбираем ближайшее пересечение
		if trunkHit.ObjectID != -1 && (crownHit.ObjectID == -1 || trunkHit.Distance < crownHit.Distance) {
			hitInfo = trunkHit
			hitInfo.ObjectID = obj.ID
			hitInfo.ObjectType = "tree_trunk"
			hitInfo.Intensity = 0.6 // Ствол темнее
		} else if crownHit.ObjectID != -1 {
			hitInfo = crownHit
			hitInfo.ObjectID = obj.ID
			hitInfo.ObjectType = "tree_crown"
			hitInfo.Intensity = 0.4 // Крона дерева темнее
		}

	case "rock":
		// Упрощаем камень как эллипсоид
		rockHit := rt.traceEllipsoidIntersection(ray, obj.Position, obj.Scale)

		if rockHit.ObjectID != -1 {
			hitInfo = rockHit
			hitInfo.ObjectID = obj.ID
			hitInfo.ObjectType = "rock"
			hitInfo.Intensity = 0.7 // Камни светлее
		}

	case "strange":
		// Странные объекты как искаженные сферы
		strangeHit := rt.traceDistortedSphereIntersection(ray, obj.Position, obj.Scale.X, obj.Seed)

		if strangeHit.ObjectID != -1 {
			hitInfo = strangeHit
			hitInfo.ObjectID = obj.ID
			hitInfo.ObjectType = "strange"
			hitInfo.Intensity = 0.2 // Странные объекты темные
		}

	default:
		// Для прочих объектов используем сферу
		sphereHit := rt.traceSphereIntersection(ray, obj.Position, obj.Scale.X)

		if sphereHit.ObjectID != -1 {
			hitInfo = sphereHit
			hitInfo.ObjectID = obj.ID
			hitInfo.ObjectType = obj.Type
			hitInfo.Intensity = 0.5 // Средняя интенсивность
		}
	}

	return hitInfo
}

// traceSphereIntersection проверяет пересечение луча со сферой
func (rt *Raytracer) traceSphereIntersection(ray Ray, center Vector3, radius float64) HitInfo {
	hitInfo := HitInfo{
		Distance:   math.MaxFloat64,
		ObjectID:   -1,
		MaterialID: 1, // Материал по умолчанию для сферы
		Intensity:  0,
	}

	// Вектор от начала луча до центра сферы
	oc := ray.Origin.Sub(center)

	// Коэффициенты квадратного уравнения t^2*dot(d,d) + 2*t*dot(oc,d) + dot(oc,oc) - r^2 = 0
	a := ray.Direction.Dot(ray.Direction)
	b := 2.0 * oc.Dot(ray.Direction)
	c := oc.Dot(oc) - radius*radius

	discriminant := b*b - 4*a*c

	if discriminant >= 0 {
		// Корни квадратного уравнения
		t := (-b - math.Sqrt(discriminant)) / (2.0 * a)

		// Проверяем, находится ли пересечение впереди луча
		if t > 0.001 {
			hitInfo.Distance = t
			hitInfo.Position = ray.Origin.Add(ray.Direction.Mul(t))
			hitInfo.Normal = hitInfo.Position.Sub(center).Normalize()
			hitInfo.ObjectID = 1 // Временный ID

			// Интенсивность зависит от угла между нормалью и вектором к наблюдателю
			hitInfo.Intensity = math.Abs(hitInfo.Normal.Dot(ray.Direction.Mul(-1)))
		}
	}

	return hitInfo
}

// traceCylinderIntersection проверяет пересечение луча с цилиндром
func (rt *Raytracer) traceCylinderIntersection(ray Ray, baseCenter, topCenter Vector3, radius float64) HitInfo {
	hitInfo := HitInfo{
		Distance:   math.MaxFloat64,
		ObjectID:   -1,
		MaterialID: 2, // Материал по умолчанию для цилиндра
		Intensity:  0,
	}

	// Ось цилиндра
	axis := topCenter.Sub(baseCenter).Normalize()
	height := topCenter.Sub(baseCenter).Length()

	// Проекция направления луча на ось цилиндра
	dirDotAxis := ray.Direction.Dot(axis)

	// Вектор от начала луча до центра нижнего основания
	oc := ray.Origin.Sub(baseCenter)

	// Проекция oc на ось
	ocDotAxis := oc.Dot(axis)

	// Вычисляем коэффициенты для квадратного уравнения
	a := 1.0 - dirDotAxis*dirDotAxis
	b := 2.0 * (oc.Dot(ray.Direction) - ocDotAxis*dirDotAxis)
	c := oc.Dot(oc) - ocDotAxis*ocDotAxis - radius*radius

	// Решаем квадратное уравнение
	discriminant := b*b - 4*a*c

	if discriminant >= 0 && a != 0 {
		// Находим ближайший корень
		t := (-b - math.Sqrt(discriminant)) / (2.0 * a)

		if t > 0.001 {
			// Точка пересечения с бесконечным цилиндром
			hitPos := ray.Origin.Add(ray.Direction.Mul(t))

			// Проверяем, находится ли точка между основаниями
			hitPosRelative := hitPos.Sub(baseCenter)
			hitHeight := hitPosRelative.Dot(axis)

			if hitHeight >= 0 && hitHeight <= height {
				hitInfo.Distance = t
				hitInfo.Position = hitPos

				// Нормаль к поверхности цилиндра
				projectionOnAxis := axis.Mul(hitHeight)
				hitInfo.Normal = hitPos.Sub(baseCenter.Add(projectionOnAxis)).Normalize()
				hitInfo.ObjectID = 1 // Временный ID

				// Интенсивность зависит от угла между нормалью и вектором к наблюдателю
				hitInfo.Intensity = math.Abs(hitInfo.Normal.Dot(ray.Direction.Mul(-1)))
			}
		}
	}

	// Проверяем пересечение с нижним основанием
	tBase := intersectPlane(ray, baseCenter, axis.Mul(-1))
	if tBase > 0.001 && tBase < hitInfo.Distance {
		// Точка пересечения
		hitPos := ray.Origin.Add(ray.Direction.Mul(tBase))

		// Проверяем, находится ли точка внутри круга основания
		distanceFromBase := hitPos.Sub(baseCenter)
		distanceFromBase = distanceFromBase.Sub(axis.Mul(distanceFromBase.Dot(axis)))

		if distanceFromBase.Length() <= radius {
			hitInfo.Distance = tBase
			hitInfo.Position = hitPos
			hitInfo.Normal = axis.Mul(-1)
			hitInfo.ObjectID = 1 // Временный ID
			hitInfo.Intensity = math.Abs(hitInfo.Normal.Dot(ray.Direction.Mul(-1)))
		}
	}

	// Проверяем пересечение с верхним основанием
	tTop := intersectPlane(ray, topCenter, axis)
	if tTop > 0.001 && tTop < hitInfo.Distance {
		// Точка пересечения
		hitPos := ray.Origin.Add(ray.Direction.Mul(tTop))

		// Проверяем, находится ли точка внутри круга основания
		distanceFromTop := hitPos.Sub(topCenter)
		distanceFromTop = distanceFromTop.Sub(axis.Mul(distanceFromTop.Dot(axis)))

		if distanceFromTop.Length() <= radius {
			hitInfo.Distance = tTop
			hitInfo.Position = hitPos
			hitInfo.Normal = axis
			hitInfo.ObjectID = 1 // Временный ID
			hitInfo.Intensity = math.Abs(hitInfo.Normal.Dot(ray.Direction.Mul(-1)))
		}
	}

	return hitInfo
}

// intersectPlane находит расстояние до пересечения с плоскостью
func intersectPlane(ray Ray, pointOnPlane Vector3, normal Vector3) float64 {
	denom := ray.Direction.Dot(normal)

	if math.Abs(denom) < 0.0001 {
		return math.MaxFloat64 // Луч параллелен плоскости
	}

	t := pointOnPlane.Sub(ray.Origin).Dot(normal) / denom
	return t
}

// traceEllipsoidIntersection проверяет пересечение луча с эллипсоидом
func (rt *Raytracer) traceEllipsoidIntersection(ray Ray, center Vector3, scale Vector3) HitInfo {
	hitInfo := HitInfo{
		Distance:   math.MaxFloat64,
		ObjectID:   -1,
		MaterialID: 3, // Материал по умолчанию для эллипсоида
		Intensity:  0,
	}

	// Преобразуем луч в пространство сферы, деля координаты на соответствующие масштабы
	invScale := Vector3{X: 1.0 / scale.X, Y: 1.0 / scale.Y, Z: 1.0 / scale.Z}

	// Преобразованный луч
	transformedOrigin := Vector3{
		X: (ray.Origin.X - center.X) * invScale.X,
		Y: (ray.Origin.Y - center.Y) * invScale.Y,
		Z: (ray.Origin.Z - center.Z) * invScale.Z,
	}

	transformedDir := Vector3{
		X: ray.Direction.X * invScale.X,
		Y: ray.Direction.Y * invScale.Y,
		Z: ray.Direction.Z * invScale.Z,
	}
	normFactor := 1.0 / transformedDir.Length()
	transformedDir = transformedDir.Normalize()

	// Теперь проверяем пересечение с единичной сферой
	a := transformedDir.Dot(transformedDir) // всегда 1 после нормализации
	b := 2.0 * transformedOrigin.Dot(transformedDir)
	c := transformedOrigin.Dot(transformedOrigin) - 1.0

	discriminant := b*b - 4*a*c

	if discriminant >= 0 {
		// Корни квадратного уравнения
		t := (-b - math.Sqrt(discriminant)) / (2.0 * a)

		// Корректируем t с учетом изменения длины направляющего вектора
		t *= normFactor

		// Проверяем, находится ли пересечение впереди луча
		if t > 0.001 {
			hitInfo.Distance = t
			hitInfo.Position = ray.Origin.Add(ray.Direction.Mul(t))

			// Вычисляем нормаль к эллипсоиду
			localPos := Vector3{
				X: (hitInfo.Position.X - center.X) / scale.X,
				Y: (hitInfo.Position.Y - center.Y) / scale.Y,
				Z: (hitInfo.Position.Z - center.Z) / scale.Z,
			}

			// Преобразование нормали обратно в мировое пространство
			normal := Vector3{
				X: localPos.X / scale.X,
				Y: localPos.Y / scale.Y,
				Z: localPos.Z / scale.Z,
			}.Normalize()

			hitInfo.Normal = normal
			hitInfo.ObjectID = 1 // Временный ID

			// Интенсивность зависит от угла между нормалью и вектором к наблюдателю
			hitInfo.Intensity = math.Abs(hitInfo.Normal.Dot(ray.Direction.Mul(-1)))
		}
	}

	return hitInfo
}

// traceDistortedSphereIntersection проверяет пересечение луча с искаженной сферой
func (rt *Raytracer) traceDistortedSphereIntersection(ray Ray, center Vector3, radius float64, seed int64) HitInfo {
	hitInfo := HitInfo{
		Distance:   math.MaxFloat64,
		ObjectID:   -1,
		MaterialID: 4, // Материал по умолчанию для искаженной сферы
		Intensity:  0,
	}

	// Сначала проверяем пересечение с обычной сферой
	sphereHit := rt.traceSphereIntersection(ray, center, radius)

	if sphereHit.ObjectID != -1 {
		hitInfo = sphereHit
		hitInfo.ObjectID = 1 // Временный ID

		// Теперь искажаем нормаль и позицию для создания неровной поверхности
		// Используем seed для воспроизводимости искажений
		distortAmount := 0.2

		// Преобразуем координаты хита в сферические для искажения
		localPos := hitInfo.Position.Sub(center)

		// Используем координаты как входы для шума
		distX := math.Sin((localPos.X+float64(seed))*0.5) * distortAmount
		distY := math.Sin((localPos.Y+float64(seed))*0.5) * distortAmount
		distZ := math.Sin((localPos.Z+float64(seed))*0.5) * distortAmount

		// Искажаем нормаль
		distortedNormal := Vector3{
			X: hitInfo.Normal.X + distX,
			Y: hitInfo.Normal.Y + distY,
			Z: hitInfo.Normal.Z + distZ,
		}.Normalize()

		hitInfo.Normal = distortedNormal

		// Делаем странные объекты более темными и контрастными
		hitInfo.Intensity = math.Abs(hitInfo.Normal.Dot(ray.Direction.Mul(-1))) * 0.7
	}

	return hitInfo
}

// computeTerrainIntensity вычисляет интенсивность точки на ландшафте
func computeTerrainIntensity(normal Vector3, materialID int, timeOfDay float64) float64 {
	// Направление света зависит от времени суток
	lightAngle := timeOfDay * 2.0 * math.Pi // полный круг за день
	lightDir := Vector3{
		X: math.Cos(lightAngle),
		Y: math.Sin(lightAngle), // Солнце высоко в полдень, низко утром/вечером
		Z: 0,
	}.Normalize()

	// Свет не светит снизу (ночью)
	if lightDir.Y < 0 {
		lightDir.Y = 0
		lightDir = lightDir.Normalize()
	}

	// Вычисляем базовую интенсивность как скалярное произведение нормали и направления света
	baseIntensity := normal.Dot(lightDir)

	// Ограничиваем минимальную интенсивность для имитации рассеянного света
	baseIntensity = math.Max(0.1, baseIntensity)

	// Различные материалы имеют разную базовую яркость
	materialBrightness := 0.5 // по умолчанию

	switch materialID {
	case 1: // Вода
		materialBrightness = 0.7
	case 2: // Земля
		materialBrightness = 0.5
	case 3: // Камень
		materialBrightness = 0.6
	case 4: // Снег
		materialBrightness = 0.9
	}

	// Ночью все темнее
	if timeOfDay < 0.25 || timeOfDay > 0.75 {
		nightFactor := 0.3 // базовая ночная освещенность

		// Интенсивность ночного эффекта
		var nightIntensity float64
		if timeOfDay < 0.25 { // ночь до рассвета
			nightIntensity = 1.0 - timeOfDay/0.25
		} else { // ночь после заката
			nightIntensity = (timeOfDay - 0.75) / 0.25
		}

		// Интерполируем между дневной и ночной интенсивностью
		baseIntensity = baseIntensity*(1.0-nightIntensity) + nightFactor*nightIntensity
	}

	// Учитываем материал
	intensity := baseIntensity * materialBrightness

	// Ограничиваем диапазон
	return math.Max(0.1, math.Min(1.0, intensity))
}

// calculateIntensity converts hit information to a normalized intensity value for ASCII rendering
func calculateIntensity(hit HitInfo) float64 {
	if hit.ObjectID == -1 {
		return 0 // No hit, darkness
	}

	// Use the intensity calculated during ray tracing
	intensity := hit.Intensity

	// Adjust based on object type for more artistic control
	switch hit.ObjectType {
	case "terrain":
		// Земля немного ярче для лучшей видимости
		intensity *= 1.1
	case "tree_trunk":
		// Стволы деревьев темные
		intensity *= 0.8
	case "tree_crown":
		// Кроны деревьев средней яркости
		intensity *= 0.9
	case "rock":
		// Камни чуть ярче для заметности
		intensity *= 1.2
	case "strange":
		// Странные объекты очень темные и контрастные
		intensity = math.Pow(intensity, 1.5) * 0.8
	}

	// Ensure it's in 0-1 range
	if intensity < 0 {
		intensity = 0
	} else if intensity > 1 {
		intensity = 1
	}

	return intensity
}
