package engine

import (
	"github.com/go-gl/glfw/v3.3/glfw"
)

// InputHandler управляет вводом с клавиатуры и мыши
type InputHandler struct {
	window            *glfw.Window
	currentKeys       map[glfw.Key]bool
	previousKeys      map[glfw.Key]bool
	currentMousePos   [2]float64
	previousMousePos  [2]float64
	currentMouseBtns  map[glfw.MouseButton]bool
	previousMouseBtns map[glfw.MouseButton]bool
	mouseDelta        [2]float64
	mouseWheelDelta   float64
}

// NewInputHandler создает новый обработчик ввода
func NewInputHandler(window *glfw.Window) *InputHandler {
	handler := &InputHandler{
		window:            window,
		currentKeys:       make(map[glfw.Key]bool),
		previousKeys:      make(map[glfw.Key]bool),
		currentMouseBtns:  make(map[glfw.MouseButton]bool),
		previousMouseBtns: make(map[glfw.MouseButton]bool),
	}

	// Установка обработчика колесика мыши
	window.SetScrollCallback(func(_ *glfw.Window, _, yoffset float64) {
		handler.mouseWheelDelta += yoffset
	})

	return handler
}

// Update обновляет состояние ввода
func (ih *InputHandler) Update() {
	// Копируем текущее состояние клавиш в предыдущее
	ih.previousKeys = make(map[glfw.Key]bool)
	for k, v := range ih.currentKeys {
		ih.previousKeys[k] = v
	}

	// Копируем текущее состояние кнопок мыши в предыдущее
	ih.previousMouseBtns = make(map[glfw.MouseButton]bool)
	for b, v := range ih.currentMouseBtns {
		ih.previousMouseBtns[b] = v
	}

	// Сохраняем текущую позицию мыши как предыдущую
	ih.previousMousePos = ih.currentMousePos

	// Обновляем текущую позицию мыши
	x, y := ih.window.GetCursorPos()
	ih.currentMousePos = [2]float64{x, y}

	// Вычисляем дельту перемещения мыши
	ih.mouseDelta[0] = ih.currentMousePos[0] - ih.previousMousePos[0]
	ih.mouseDelta[1] = ih.currentMousePos[1] - ih.previousMousePos[1]

	// Сканируем все отслеживаемые клавиши
	for key := glfw.KeyUnknown; key <= glfw.KeyLast; key++ {
		ih.currentKeys[key] = ih.window.GetKey(key) == glfw.Press
	}

	// Сканируем все кнопки мыши
	for btn := glfw.MouseButton1; btn <= glfw.MouseButtonLast; btn++ {
		ih.currentMouseBtns[btn] = ih.window.GetMouseButton(btn) == glfw.Press
	}
}

// IsKeyDown проверяет, нажата ли клавиша в данный момент
func (ih *InputHandler) IsKeyDown(key glfw.Key) bool {
	return ih.currentKeys[key]
}

// IsKeyPressed проверяет, была ли клавиша нажата в этом кадре
func (ih *InputHandler) IsKeyPressed(key glfw.Key) bool {
	return ih.currentKeys[key] && !ih.previousKeys[key]
}

// IsKeyReleased проверяет, была ли клавиша отпущена в этом кадре
func (ih *InputHandler) IsKeyReleased(key glfw.Key) bool {
	return !ih.currentKeys[key] && ih.previousKeys[key]
}

// IsMouseButtonDown проверяет, нажата ли кнопка мыши в данный момент
func (ih *InputHandler) IsMouseButtonDown(button glfw.MouseButton) bool {
	return ih.currentMouseBtns[button]
}

// IsMouseButtonPressed проверяет, была ли кнопка мыши нажата в этом кадре
func (ih *InputHandler) IsMouseButtonPressed(button glfw.MouseButton) bool {
	return ih.currentMouseBtns[button] && !ih.previousMouseBtns[button]
}

// IsMouseButtonReleased проверяет, была ли кнопка мыши отпущена в этом кадре
func (ih *InputHandler) IsMouseButtonReleased(button glfw.MouseButton) bool {
	return !ih.currentMouseBtns[button] && ih.previousMouseBtns[button]
}

// GetMousePosition возвращает текущую позицию курсора мыши
func (ih *InputHandler) GetMousePosition() [2]float64 {
	return ih.currentMousePos
}

// GetMouseDelta возвращает перемещение мыши с прошлого кадра
func (ih *InputHandler) GetMouseDelta() [2]float64 {
	return ih.mouseDelta
}

// GetMouseWheelDelta возвращает изменение колесика мыши с последнего вызова
func (ih *InputHandler) GetMouseWheelDelta() float64 {
	delta := ih.mouseWheelDelta
	ih.mouseWheelDelta = 0 // сбрасываем после получения
	return delta
}
