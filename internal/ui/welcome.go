package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/hailam/chessplay/internal/storage"
)

// Welcome screen dimensions
const (
	WelcomeWidth  = 400
	WelcomeHeight = 380
	WelcomePadX   = 32
	WelcomePadY   = 24
)

// WelcomeScreen is shown on first launch.
type WelcomeScreen struct {
	visible      bool
	needsCapture bool // Set true when opening to capture background

	// Position (centered on screen)
	x, y int

	// Widgets
	nameInput     *TextInput
	evalModeRadio *RadioGroup
	startBtn      *ModalButton

	// Callback
	onComplete func(name string, evalMode storage.EvalMode)
}

// NewWelcomeScreen creates a new welcome screen.
func NewWelcomeScreen() *WelcomeScreen {
	ws := &WelcomeScreen{}
	ws.calculatePosition()
	ws.createWidgets()
	return ws
}

// calculatePosition centers the screen.
func (ws *WelcomeScreen) calculatePosition() {
	ws.x = (ScreenWidth - WelcomeWidth) / 2
	ws.y = (ScreenHeight - WelcomeHeight) / 2
}

// createWidgets initializes all welcome screen widgets.
func (ws *WelcomeScreen) createWidgets() {
	contentX := ws.x + WelcomePadX
	contentW := WelcomeWidth - WelcomePadX*2

	// Name input
	inputY := ws.y + 140
	ws.nameInput = NewTextInput(contentX, inputY, contentW, 40, "Enter your name", 20)

	// Eval mode radio
	radioY := inputY + 80
	ws.evalModeRadio = NewRadioGroup(contentX, radioY, []RadioOption{
		{Label: "Classical Evaluation", Value: int(storage.EvalClassical)},
		{Label: "NNUE (Neural Network)", Value: int(storage.EvalNNUE)},
	}, 0)

	// Start button
	btnW := 160
	btnH := 44
	btnX := ws.x + (WelcomeWidth-btnW)/2
	btnY := ws.y + WelcomeHeight - WelcomePadY - btnH
	ws.startBtn = NewModalButton(btnX, btnY, btnW, btnH, "Start Playing", true, nil)
}

// Show displays the welcome screen.
func (ws *WelcomeScreen) Show(onComplete func(name string, evalMode storage.EvalMode)) {
	ws.visible = true
	ws.needsCapture = true // Capture background on first draw
	ws.onComplete = onComplete
	ws.nameInput.Value = ""
	ws.evalModeRadio.Selected = 0
	ws.startBtn.OnClick = ws.handleStart
}

// Hide closes the welcome screen.
func (ws *WelcomeScreen) Hide() {
	ws.visible = false
	ws.nameInput.SetFocused(false)
}

// IsVisible returns true if the screen is visible.
func (ws *WelcomeScreen) IsVisible() bool {
	return ws.visible
}

// handleStart handles the start button click.
func (ws *WelcomeScreen) handleStart() {
	name := ws.nameInput.Value
	if name == "" {
		name = "Player"
	}
	evalMode := storage.EvalMode(ws.evalModeRadio.Selected)

	if ws.onComplete != nil {
		ws.onComplete(name, evalMode)
	}
	ws.Hide()
}

// Update handles input for the welcome screen.
func (ws *WelcomeScreen) Update(input *InputHandler) bool {
	if !ws.visible {
		return false
	}

	// Handle enter key to start
	if IsKeyJustPressed(ebiten.KeyEnter) && !ws.nameInput.IsFocused() {
		ws.handleStart()
		return true
	}

	// Update widgets
	ws.nameInput.Update(input)
	ws.evalModeRadio.Update(input)
	ws.startBtn.Update(input)

	// Welcome screen consumes all input
	return true
}

// AnyButtonHovered returns true if any button in the screen is hovered.
func (ws *WelcomeScreen) AnyButtonHovered() bool {
	if !ws.visible {
		return false
	}
	return ws.startBtn.IsHovered() || ws.evalModeRadio.hovered >= 0
}

// Draw renders the welcome screen.
func (ws *WelcomeScreen) Draw(screen *ebiten.Image, glass *GlassEffect) {
	if !ws.visible {
		return
	}

	// Capture background once when modal first opens (fixes flicker)
	if ws.needsCapture && glass != nil && glass.IsEnabled() {
		glass.CaptureForModal(screen, 3.0) // sigma=3.0 blur
		ws.needsCapture = false
	}

	// Draw blurred, dimmed background (simple blur + dim, NOT glass material)
	if glass != nil && glass.IsEnabled() {
		glass.DrawModalBackground(screen, 0.4) // 40% dimming
	} else {
		// Fallback: semi-transparent overlay
		vector.DrawFilledRect(screen, 0, 0, scaleF(ScreenWidth), scaleF(ScreenHeight), modalOverlay, false)
	}

	// Modal background
	vector.DrawFilledRect(screen, scaleF(ws.x), scaleF(ws.y), scaleF(WelcomeWidth), scaleF(WelcomeHeight), modalBg, false)

	// Modal border
	vector.StrokeRect(screen, scaleF(ws.x), scaleF(ws.y), scaleF(WelcomeWidth), scaleF(WelcomeHeight), float32(UIScale*2), modalBorder, false)

	// Draw chess piece icon (king)
	ws.drawChessIcon(screen)

	// Draw title
	ws.drawTitle(screen)

	// Draw subtitle
	ws.drawSubtitle(screen)

	// Section label for name
	contentX := ws.x + WelcomePadX
	ws.drawSectionLabel(screen, "Your Name", contentX, ws.nameInput.Y-20)

	// Section label for eval mode
	ws.drawSectionLabel(screen, "Engine Mode", contentX, ws.evalModeRadio.Y-20)

	// Draw widgets
	ws.nameInput.Draw(screen)
	ws.evalModeRadio.Draw(screen)
	ws.startBtn.Draw(screen)
}

// drawChessIcon draws a decorative chess icon.
func (ws *WelcomeScreen) drawChessIcon(screen *ebiten.Image) {
	// Draw a simple crown-like shape for the king
	centerX := scaleF(ws.x + WelcomeWidth/2)
	y := scaleF(ws.y + 28)

	iconColor := accentColor

	// Simple crown/king icon using circles and rectangles
	vector.DrawFilledCircle(screen, centerX, y+scaleF(8), scaleF(6), iconColor, false)
	vector.DrawFilledRect(screen, centerX-scaleF(8), y+scaleF(10), scaleF(16), scaleF(14), iconColor, false)

	// Cross on top
	vector.DrawFilledRect(screen, centerX-scaleF(1), y-scaleF(2), scaleF(3), scaleF(10), iconColor, false)
	vector.DrawFilledRect(screen, centerX-scaleF(4), y+scaleF(2), scaleF(9), scaleF(3), iconColor, false)
}

// drawTitle draws the main title.
func (ws *WelcomeScreen) drawTitle(screen *ebiten.Image) {
	face := GetFaceWithSize(24)
	if face == nil {
		return
	}

	title := "CHESSPLAY"
	w, _ := MeasureText(title, face)
	centerX := scaleD(ws.x) + scaleD(WelcomeWidth)/2 - w/2

	op := &text.DrawOptions{}
	op.GeoM.Translate(centerX, scaleD(ws.y+64))
	op.ColorScale.ScaleWithColor(textPrimary)
	text.Draw(screen, title, face, op)
}

// drawSubtitle draws the subtitle.
func (ws *WelcomeScreen) drawSubtitle(screen *ebiten.Image) {
	face := GetRegularFace()
	if face == nil {
		return
	}

	subtitle := "Welcome! Set up your preferences."
	w, _ := MeasureText(subtitle, face)
	centerX := scaleD(ws.x) + scaleD(WelcomeWidth)/2 - w/2

	op := &text.DrawOptions{}
	op.GeoM.Translate(centerX, scaleD(ws.y+96))
	op.ColorScale.ScaleWithColor(textSecondary)
	text.Draw(screen, subtitle, face, op)
}

// drawSectionLabel draws a section label.
func (ws *WelcomeScreen) drawSectionLabel(screen *ebiten.Image, label string, x, y int) {
	face := GetRegularFace()
	if face == nil {
		return
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(scaleD(x), scaleD(y))
	op.ColorScale.ScaleWithColor(color.RGBA{160, 165, 175, 255})
	text.Draw(screen, label, face, op)
}
