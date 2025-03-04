package engine

// SceneData содержит результат трассировки сцены и дополнительные метаданные
type SceneData struct {
	Pixels         [][]TracedPixel    // Трассированные пиксели
	Width          int                // Ширина сцены
	Height         int                // Высота сцены
	SpecialEffects map[string]float64 // Карта специальных эффектов и их интенсивности
	Atmosphere     map[string]float64 // Атмосферные параметры сцены
	TimeOfDay      float64            // Время суток (0.0-1.0, 0 = полночь, 0.5 = полдень)
	ObjectsInView  []*SceneObject     // Объекты в поле зрения
	PlayerPosition Vector3            // Текущая позиция игрока
	ViewDirection  Vector3            // Направление взгляда
}

// SceneObject представляет собой объект в поле зрения
type SceneObject struct {
	Type       string             // Тип объекта
	ID         int                // Идентификатор объекта
	Distance   float64            // Расстояние до объекта
	Direction  Vector3            // Направление к объекту
	Size       float64            // Примерный размер объекта в поле зрения
	Metadata   map[string]float64 // Метаданные объекта
	Visibility float64            // Видимость объекта (0.0-1.0)
}

// TracedPixel представляет результат трассировки для одного пикселя
type TracedPixel struct {
	X          int     // Координата X
	Y          int     // Координата Y
	Intensity  float64 // Интенсивность (0.0-1.0) для ASCII рендеринга
	ObjectID   int     // ID объекта, в который попал луч
	ObjectType string  // Тип объекта
	Depth      float64 // Глубина от камеры
	Normal     Vector3 // Поверхностная нормаль в точке пересечения
}

// NewSceneData создает новый пустой экземпляр SceneData
func NewSceneData(width, height int) *SceneData {
	scene := &SceneData{
		Width:          width,
		Height:         height,
		Pixels:         make([][]TracedPixel, height),
		SpecialEffects: make(map[string]float64),
		Atmosphere:     make(map[string]float64),
		TimeOfDay:      0.5,
		ObjectsInView:  make([]*SceneObject, 0),
	}

	// Инициализируем массив пикселей
	for y := 0; y < height; y++ {
		scene.Pixels[y] = make([]TracedPixel, width)
	}

	return scene
}

// AddObjectInView добавляет объект в список видимых объектов
func (sd *SceneData) AddObjectInView(obj *SceneObject) {
	sd.ObjectsInView = append(sd.ObjectsInView, obj)
}

// SetSpecialEffect устанавливает значение специального эффекта
func (sd *SceneData) SetSpecialEffect(effect string, value float64) {
	sd.SpecialEffects[effect] = value
}

// GetSpecialEffect возвращает значение специального эффекта
func (sd *SceneData) GetSpecialEffect(effect string) float64 {
	if value, ok := sd.SpecialEffects[effect]; ok {
		return value
	}
	return 0.0
}

// SetAtmosphereValue устанавливает значение атмосферного параметра
func (sd *SceneData) SetAtmosphereValue(param string, value float64) {
	sd.Atmosphere[param] = value
}

// GetAtmosphereValue возвращает значение атмосферного параметра
func (sd *SceneData) GetAtmosphereValue(param string) float64 {
	if value, ok := sd.Atmosphere[param]; ok {
		return value
	}
	return 0.0
}

// GetNearestObject возвращает ближайший объект указанного типа
func (sd *SceneData) GetNearestObject(objectType string) *SceneObject {
	var nearest *SceneObject
	minDistance := 1000000.0 // Очень большое число

	for _, obj := range sd.ObjectsInView {
		if obj.Type == objectType && obj.Distance < minDistance {
			nearest = obj
			minDistance = obj.Distance
		}
	}

	return nearest
}

// GetObjectsByType возвращает список объектов указанного типа
func (sd *SceneData) GetObjectsByType(objectType string) []*SceneObject {
	result := make([]*SceneObject, 0)

	for _, obj := range sd.ObjectsInView {
		if obj.Type == objectType {
			result = append(result, obj)
		}
	}

	return result
}

// GetObjectByID возвращает объект с указанным ID
func (sd *SceneData) GetObjectByID(id int) *SceneObject {
	for _, obj := range sd.ObjectsInView {
		if obj.ID == id {
			return obj
		}
	}

	return nil
}
