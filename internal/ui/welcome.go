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
	visible bool

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

// Draw renders the welcome screen.
func (ws *WelcomeScreen) Draw(screen *ebiten.Image) {
	if !ws.visible {
		return
	}

	// Semi-transparent overlay
	vector.DrawFilledRect(screen, 0, 0, float32(ScreenWidth), float32(ScreenHeight), modalOverlay, false)

	// Modal background
	vector.DrawFilledRect(screen, float32(ws.x), float32(ws.y), float32(WelcomeWidth), float32(WelcomeHeight), modalBg, false)

	// Modal border
	vector.StrokeRect(screen, float32(ws.x), float32(ws.y), float32(WelcomeWidth), float32(WelcomeHeight), 2, modalBorder, false)

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
	centerX := float32(ws.x + WelcomeWidth/2)
	y := float32(ws.y + 28)

	iconColor := accentColor

	// Simple crown/king icon using circles and rectangles
	vector.DrawFilledCircle(screen, centerX, y+8, 6, iconColor, false)
	vector.DrawFilledRect(screen, centerX-8, y+10, 16, 14, iconColor, false)

	// Cross on top
	vector.DrawFilledRect(screen, centerX-1, y-2, 3, 10, iconColor, false)
	vector.DrawFilledRect(screen, centerX-4, y+2, 9, 3, iconColor, false)
}

// drawTitle draws the main title.
func (ws *WelcomeScreen) drawTitle(screen *ebiten.Image) {
	face := GetFaceWithSize(24)
	if face == nil {
		return
	}

	title := "CHESSPLAY"
	w, _ := MeasureText(title, face)
	centerX := float64(ws.x) + float64(WelcomeWidth)/2 - w/2

	op := &text.DrawOptions{}
	op.GeoM.Translate(centerX, float64(ws.y+64))
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
	centerX := float64(ws.x) + float64(WelcomeWidth)/2 - w/2

	op := &text.DrawOptions{}
	op.GeoM.Translate(centerX, float64(ws.y+96))
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
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(color.RGBA{160, 165, 175, 255})
	text.Draw(screen, label, face, op)
}
