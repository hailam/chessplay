package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// InputHandler manages mouse and keyboard input.
type InputHandler struct {
	mouseX, mouseY   int // Logical coordinates (unscaled)
	leftPressed      bool
	leftJustPressed  bool
	leftJustReleased bool
}

// NewInputHandler creates a new input handler.
func NewInputHandler() *InputHandler {
	return &InputHandler{}
}

// Update updates the input state. Call this once per frame.
func (ih *InputHandler) Update() {
	// Get raw cursor position (in scaled space)
	rawX, rawY := ebiten.CursorPosition()

	// Convert to logical coordinates by dividing by scale
	scale := UIScale
	if scale < 1.0 {
		scale = 1.0
	}
	ih.mouseX = int(float64(rawX) / scale)
	ih.mouseY = int(float64(rawY) / scale)

	ih.leftJustPressed = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	ih.leftJustReleased = inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
	ih.leftPressed = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
}

// MousePosition returns the current mouse position in logical coordinates.
func (ih *InputHandler) MousePosition() (int, int) {
	return ih.mouseX, ih.mouseY
}

// MouseX returns the current mouse X position.
func (ih *InputHandler) MouseX() int {
	return ih.mouseX
}

// MouseY returns the current mouse Y position.
func (ih *InputHandler) MouseY() int {
	return ih.mouseY
}

// IsLeftJustPressed returns true if the left mouse button was just pressed.
func (ih *InputHandler) IsLeftJustPressed() bool {
	return ih.leftJustPressed
}

// IsLeftJustReleased returns true if the left mouse button was just released.
func (ih *InputHandler) IsLeftJustReleased() bool {
	return ih.leftJustReleased
}

// IsLeftPressed returns true if the left mouse button is currently pressed.
func (ih *InputHandler) IsLeftPressed() bool {
	return ih.leftPressed
}

// IsInBounds returns true if the mouse is within the given rectangle.
func (ih *InputHandler) IsInBounds(x, y, w, h int) bool {
	return ih.mouseX >= x && ih.mouseX < x+w && ih.mouseY >= y && ih.mouseY < y+h
}

// ClickedInBounds returns true if the mouse was just clicked within the given rectangle.
func (ih *InputHandler) ClickedInBounds(x, y, w, h int) bool {
	return ih.leftJustPressed && ih.IsInBounds(x, y, w, h)
}

// ReleasedInBounds returns true if the mouse was just released within the given rectangle.
func (ih *InputHandler) ReleasedInBounds(x, y, w, h int) bool {
	return ih.leftJustReleased && ih.IsInBounds(x, y, w, h)
}

// IsKeyJustPressed returns true if the specified key was just pressed.
func IsKeyJustPressed(key ebiten.Key) bool {
	return inpututil.IsKeyJustPressed(key)
}

// IsKeyPressed returns true if the specified key is currently pressed.
func IsKeyPressed(key ebiten.Key) bool {
	return ebiten.IsKeyPressed(key)
}
