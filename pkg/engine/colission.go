package engine

import (
	"math"
)

// CollisionShape представляет форму коллизии
type CollisionShape interface {
	// ClosestPoint возвращает ближайшую точку на форме к заданной точке
	ClosestPoint(point Vector3) Vector3

	// Contains проверяет, находится ли точка внутри формы
	Contains(point Vector3) bool

	// Distance возвращает дистанцию от точки до формы (отрицательная, если внутри)
	Distance(point Vector3) float64
}

// SphereCollider представляет сферическую зону коллизии
type SphereCollider struct {
	Center Vector3
	Radius float64
}

// ClosestPoint возвращает ближайшую точку на сфере к заданной точке
func (sc *SphereCollider) ClosestPoint(point Vector3) Vector3 {
	direction := point.Sub(sc.Center)
	distance := direction.Length()

	if distance <= 0.0001 {
		// Точка в центре, возвращаем произвольную точку на сфере
		return sc.Center.Add(Vector3{X: sc.Radius, Y: 0, Z: 0})
	}

	// Нормализуем направление и масштабируем до радиуса
	direction = direction.Mul(1.0 / distance)
	return sc.Center.Add(direction.Mul(sc.Radius))
}

// Contains проверяет, находится ли точка внутри сферы
func (sc *SphereCollider) Contains(point Vector3) bool {
	distance := point.Sub(sc.Center).Length()
	return distance <= sc.Radius
}

// Distance возвращает дистанцию от точки до сферы (отрицательная, если внутри)
func (sc *SphereCollider) Distance(point Vector3) float64 {
	distance := point.Sub(sc.Center).Length()
	return distance - sc.Radius
}

// CylinderCollider представляет цилиндрическую зону коллизии
type CylinderCollider struct {
	Base   Vector3 // Центр нижнего основания
	Top    Vector3 // Центр верхнего основания
	Radius float64
}

// ClosestPoint возвращает ближайшую точку на цилиндре к заданной точке
func (cc *CylinderCollider) ClosestPoint(point Vector3) Vector3 {
	// Вычисляем ось цилиндра
	axis := cc.Top.Sub(cc.Base).Normalize()

	// Вектор от базовой точки до тестируемой точки
	toPoint := point.Sub(cc.Base)

	// Проекция на ось
	axisProj := toPoint.Dot(axis)

	// Ограничиваем проекцию высотой цилиндра
	height := cc.Top.Sub(cc.Base).Length()
	axisProj = math.Max(0, math.Min(height, axisProj))

	// Точка на оси
	axisPoint := cc.Base.Add(axis.Mul(axisProj))

	// Направление от оси к точке
	radialDir := point.Sub(axisPoint)
	radialDist := radialDir.Length()

	if radialDist < 0.0001 {
		// Точка на оси, возвращаем точку на поверхности
		// Используем произвольное направление, перпендикулярное оси
		perpAxis := Vector3{X: 1, Y: 0, Z: 0}
		if math.Abs(axis.Dot(perpAxis)) > 0.9 {
			// Если ось почти параллельна X, используем Z
			perpAxis = Vector3{X: 0, Y: 0, Z: 1}
		}

		// Вычисляем перпендикулярное направление и нормализуем
		radialDir = axis.Cross(perpAxis).Normalize()
	} else {
		// Нормализуем радиальное направление
		radialDir = radialDir.Mul(1.0 / radialDist)
	}

	// Возвращаем точку на поверхности цилиндра
	return axisPoint.Add(radialDir.Mul(cc.Radius))
}

// Contains проверяет, находится ли точка внутри цилиндра
func (cc *CylinderCollider) Contains(point Vector3) bool {
	// Проверяем расстояние до оси
	axis := cc.Top.Sub(cc.Base).Normalize()
	toPoint := point.Sub(cc.Base)

	// Проекция на ось
	axisProj := toPoint.Dot(axis)

	// Проверяем, что точка между основаниями
	height := cc.Top.Sub(cc.Base).Length()
	if axisProj < 0 || axisProj > height {
		return false
	}

	// Точка на оси
	axisPoint := cc.Base.Add(axis.Mul(axisProj))

	// Расстояние от точки до оси
	radialDist := point.Sub(axisPoint).Length()

	// Точка внутри, если расстояние до оси меньше радиуса
	return radialDist <= cc.Radius
}

// Distance возвращает дистанцию от точки до цилиндра (отрицательная, если внутри)
func (cc *CylinderCollider) Distance(point Vector3) float64 {
	// Вычисляем ось цилиндра
	axis := cc.Top.Sub(cc.Base).Normalize()
	toPoint := point.Sub(cc.Base)

	// Проекция на ось
	axisProj := toPoint.Dot(axis)

	// Высота цилиндра
	height := cc.Top.Sub(cc.Base).Length()

	// Расстояние до ближайшего основания, если точка за пределами цилиндра по высоте
	if axisProj < 0 {
		// Точка ниже нижнего основания
		basePoint := cc.Base
		dist := point.Sub(basePoint).Length()
		return dist - cc.Radius
	} else if axisProj > height {
		// Точка выше верхнего основания
		topPoint := cc.Top
		dist := point.Sub(topPoint).Length()
		return dist - cc.Radius
	}

	// Точка между основаниями, вычисляем расстояние до оси
	axisPoint := cc.Base.Add(axis.Mul(axisProj))
	radialDist := point.Sub(axisPoint).Length()

	// Возвращаем расстояние до поверхности (отрицательное, если внутри)
	return radialDist - cc.Radius
}

// BoxCollider представляет коллизию в форме прямоугольного параллелепипеда (box)
type BoxCollider struct {
	Center     Vector3
	HalfExtent Vector3
	Rotation   Vector3 // Углы поворота в радианах (X, Y, Z)
}

// ClosestPoint возвращает ближайшую точку на поверхности бокса к заданной точке
func (bc *BoxCollider) ClosestPoint(point Vector3) Vector3 {
	// Преобразуем точку в локальное пространство бокса
	localPoint := bc.worldToLocal(point)

	// Ближайшая точка в локальном пространстве (ограничиваем по размерам бокса)
	closestLocal := Vector3{
		X: math.Max(-bc.HalfExtent.X, math.Min(bc.HalfExtent.X, localPoint.X)),
		Y: math.Max(-bc.HalfExtent.Y, math.Min(bc.HalfExtent.Y, localPoint.Y)),
		Z: math.Max(-bc.HalfExtent.Z, math.Min(bc.HalfExtent.Z, localPoint.Z)),
	}

	// Если точка внутри бокса, нужно найти ближайшую грань
	if closestLocal.X == localPoint.X && closestLocal.Y == localPoint.Y && closestLocal.Z == localPoint.Z {
		// Находим расстояния до каждой грани
		distToXPos := bc.HalfExtent.X - localPoint.X
		distToXNeg := localPoint.X + bc.HalfExtent.X
		distToYPos := bc.HalfExtent.Y - localPoint.Y
		distToYNeg := localPoint.Y + bc.HalfExtent.Y
		distToZPos := bc.HalfExtent.Z - localPoint.Z
		distToZNeg := localPoint.Z + bc.HalfExtent.Z

		// Находим минимальное расстояние
		minDist := math.Min(distToXPos, math.Min(distToXNeg,
			math.Min(distToYPos, math.Min(distToYNeg,
				math.Min(distToZPos, distToZNeg)))))

		// Устанавливаем координату для ближайшей грани
		if minDist == distToXPos {
			closestLocal.X = bc.HalfExtent.X
		} else if minDist == distToXNeg {
			closestLocal.X = -bc.HalfExtent.X
		} else if minDist == distToYPos {
			closestLocal.Y = bc.HalfExtent.Y
		} else if minDist == distToYNeg {
			closestLocal.Y = -bc.HalfExtent.Y
		} else if minDist == distToZPos {
			closestLocal.Z = bc.HalfExtent.Z
		} else {
			closestLocal.Z = -bc.HalfExtent.Z
		}
	}

	// Преобразуем обратно в мировое пространство
	return bc.localToWorld(closestLocal)
}

// Contains проверяет, находится ли точка внутри бокса
func (bc *BoxCollider) Contains(point Vector3) bool {
	// Преобразуем точку в локальное пространство бокса
	localPoint := bc.worldToLocal(point)

	// Проверяем, что точка внутри границ по всем осям
	return localPoint.X >= -bc.HalfExtent.X && localPoint.X <= bc.HalfExtent.X &&
		localPoint.Y >= -bc.HalfExtent.Y && localPoint.Y <= bc.HalfExtent.Y &&
		localPoint.Z >= -bc.HalfExtent.Z && localPoint.Z <= bc.HalfExtent.Z
}

// Distance возвращает дистанцию от точки до бокса (отрицательная, если внутри)
func (bc *BoxCollider) Distance(point Vector3) float64 {
	// Преобразуем точку в локальное пространство бокса
	localPoint := bc.worldToLocal(point)

	// Если точка внутри бокса, найдем ближайшую грань
	if bc.Contains(point) {
		// Расстояния до граней
		distToXPos := bc.HalfExtent.X - localPoint.X
		distToXNeg := localPoint.X + bc.HalfExtent.X
		distToYPos := bc.HalfExtent.Y - localPoint.Y
		distToYNeg := localPoint.Y + bc.HalfExtent.Y
		distToZPos := bc.HalfExtent.Z - localPoint.Z
		distToZNeg := localPoint.Z + bc.HalfExtent.Z

		// Минимальное расстояние до грани (это будет отрицательное расстояние до бокса)
		minDist := math.Min(distToXPos, math.Min(distToXNeg,
			math.Min(distToYPos, math.Min(distToYNeg,
				math.Min(distToZPos, distToZNeg)))))

		return -minDist
	}

	// Если точка снаружи, найдем ближайшую точку на поверхности
	closest := bc.ClosestPoint(point)
	return point.Sub(closest).Length()
}

// worldToLocal преобразует точку из мирового пространства в локальное пространство бокса
func (bc *BoxCollider) worldToLocal(worldPoint Vector3) Vector3 {
	// Сначала вычитаем позицию центра
	translated := worldPoint.Sub(bc.Center)

	// Матрица поворота слишком сложна для прямой реализации здесь
	// Упрощенная версия для Y-поворота (наиболее распространенный случай)
	cosY := math.Cos(bc.Rotation.Y)
	sinY := math.Sin(bc.Rotation.Y)

	// Поворот вокруг оси Y (наиболее частый случай для игровых объектов)
	return Vector3{
		X: translated.X*cosY + translated.Z*sinY,
		Y: translated.Y,
		Z: -translated.X*sinY + translated.Z*cosY,
	}
}

// localToWorld преобразует точку из локального пространства бокса в мировое пространство
func (bc *BoxCollider) localToWorld(localPoint Vector3) Vector3 {
	// Выполняем обратное преобразование
	cosY := math.Cos(bc.Rotation.Y)
	sinY := math.Sin(bc.Rotation.Y)

	rotated := Vector3{
		X: localPoint.X*cosY - localPoint.Z*sinY,
		Y: localPoint.Y,
		Z: localPoint.X*sinY + localPoint.Z*cosY,
	}

	// Добавляем позицию центра
	return rotated.Add(bc.Center)
}

// CollisionResolver отвечает за разрешение коллизий
type CollisionResolver struct {
	playerRadius float64          // Радиус коллизии игрока
	playerHeight float64          // Высота коллизии игрока
	colliders    []CollisionShape // Все коллайдеры в сцене
}

// NewCollisionResolver создает новый решатель коллизий
func NewCollisionResolver(playerRadius, playerHeight float64) *CollisionResolver {
	return &CollisionResolver{
		playerRadius: playerRadius,
		playerHeight: playerHeight,
		colliders:    make([]CollisionShape, 0),
	}
}

// AddSphereCollider добавляет сферический коллайдер
func (cr *CollisionResolver) AddSphereCollider(center Vector3, radius float64) *SphereCollider {
	collider := &SphereCollider{
		Center: center,
		Radius: radius,
	}
	cr.colliders = append(cr.colliders, collider)
	return collider
}

// AddCylinderCollider добавляет цилиндрический коллайдер
func (cr *CollisionResolver) AddCylinderCollider(base, top Vector3, radius float64) *CylinderCollider {
	collider := &CylinderCollider{
		Base:   base,
		Top:    top,
		Radius: radius,
	}
	cr.colliders = append(cr.colliders, collider)
	return collider
}

// AddBoxCollider добавляет коллайдер в форме прямоугольного параллелепипеда
func (cr *CollisionResolver) AddBoxCollider(center, halfExtent, rotation Vector3) *BoxCollider {
	collider := &BoxCollider{
		Center:     center,
		HalfExtent: halfExtent,
		Rotation:   rotation,
	}
	cr.colliders = append(cr.colliders, collider)
	return collider
}

// ClearColliders очищает все коллайдеры
func (cr *CollisionResolver) ClearColliders() {
	cr.colliders = make([]CollisionShape, 0)
}

// AddSceneObjectsAsColliders добавляет объекты сцены как коллайдеры
func (cr *CollisionResolver) AddSceneObjectsAsColliders(objects []*ProceduralObject) {
	for _, obj := range objects {
		switch obj.Type {
		case "tree":
			// Для дерева используем цилиндр для ствола
			base := obj.Position
			top := Vector3{
				X: base.X,
				Y: base.Y + obj.Scale.Y*0.6, // 60% высоты для ствола
				Z: base.Z,
			}
			radius := obj.Scale.X * 0.3 // 30% ширины для ствола
			cr.AddCylinderCollider(base, top, radius)

			// Добавляем сферу для кроны дерева
			crownCenter := Vector3{
				X: base.X,
				Y: base.Y + obj.Scale.Y*0.7, // 70% высоты для центра кроны
				Z: base.Z,
			}
			crownRadius := obj.Scale.X * 0.8 // 80% ширины для кроны
			cr.AddSphereCollider(crownCenter, crownRadius)

		case "rock":
			// Для камня используем сферу
			cr.AddSphereCollider(obj.Position, obj.Scale.X)

		case "stump":
			// Для пня используем цилиндр
			base := obj.Position
			top := Vector3{
				X: base.X,
				Y: base.Y + obj.Scale.Y,
				Z: base.Z,
			}
			cr.AddCylinderCollider(base, top, obj.Scale.X)

		case "strange":
			// Для странных объектов используем сферу
			cr.AddSphereCollider(obj.Position, obj.Scale.X)
		}
	}
}

// ResolveCollision проверяет и разрешает коллизии для игрока
func (cr *CollisionResolver) ResolveCollision(position Vector3) Vector3 {
	// Создаем цилиндрический коллайдер для игрока
	for _, collider := range cr.colliders {
		// Находим ближайшую точку коллайдера к центру игрока
		closestPoint := collider.ClosestPoint(position)

		// Вычисляем вектор от ближайшей точки к игроку
		toPlayer := position.Sub(closestPoint)
		distance := toPlayer.Length()

		// Если есть перекрытие, разрешаем коллизию
		if distance < cr.playerRadius {
			// Если расстояние почти нулевое, выбираем произвольное направление
			if distance < 0.0001 {
				toPlayer = Vector3{X: 0, Y: 0, Z: 1}
				distance = 1.0
			}

			// Нормализуем направление
			toPlayer = toPlayer.Mul(1.0 / distance)

			// Вычисляем глубину проникновения
			penetration := cr.playerRadius - distance

			// Корректируем позицию, чтобы разрешить коллизию
			position = position.Add(toPlayer.Mul(penetration))
		}
	}

	return position
}

// IsPositionValid проверяет, является ли позиция допустимой (нет коллизий)
func (cr *CollisionResolver) IsPositionValid(position Vector3) bool {
	// Проверяем расстояние до всех коллайдеров
	for _, collider := range cr.colliders {
		closestPoint := collider.ClosestPoint(position)
		distance := position.Sub(closestPoint).Length()

		if distance < cr.playerRadius {
			return false
		}
	}

	return true
}
