package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/hailam/chessplay/internal/storage"
)

// Settings modal dimensions
const (
	SettingsWidth  = 380
	SettingsHeight = 480
	SettingsPadX   = 24
	SettingsPadY   = 20
)

// Settings modal colors
var (
	modalOverlay = color.RGBA{0, 0, 0, 180}
	modalBg      = color.RGBA{38, 40, 45, 255}
	modalHeader  = color.RGBA{48, 52, 58, 255}
	modalBorder  = color.RGBA{58, 62, 68, 255}
)

// SettingsModal is the settings configuration screen.
type SettingsModal struct {
	visible bool

	// Position (centered on screen)
	x, y int

	// Widgets
	usernameInput  *TextInput
	evalModeRadio  *RadioGroup
	difficultyBtns *ButtonGroup
	soundCheckbox  *Checkbox
	saveBtn        *ModalButton
	cancelBtn      *ModalButton

	// Callbacks
	onSave   func(prefs *storage.UserPreferences)
	onCancel func()

	// Original values (for cancel)
	originalPrefs *storage.UserPreferences
}

// NewSettingsModal creates a new settings modal.
func NewSettingsModal() *SettingsModal {
	sm := &SettingsModal{}
	sm.calculatePosition()
	sm.createWidgets()
	return sm
}

// calculatePosition centers the modal on screen.
func (sm *SettingsModal) calculatePosition() {
	sm.x = (ScreenWidth - SettingsWidth) / 2
	sm.y = (ScreenHeight - SettingsHeight) / 2
}

// createWidgets initializes all settings widgets.
func (sm *SettingsModal) createWidgets() {
	contentX := sm.x + SettingsPadX
	contentW := SettingsWidth - SettingsPadX*2

	// Username input (below header)
	inputY := sm.y + 60
	sm.usernameInput = NewTextInput(contentX, inputY, contentW, 36, "Enter your name", 20)

	// Eval mode radio group
	radioY := inputY + 70
	sm.evalModeRadio = NewRadioGroup(contentX, radioY, []RadioOption{
		{Label: "Classical Evaluation", Value: int(storage.EvalClassical)},
		{Label: "NNUE (Neural Network)", Value: int(storage.EvalNNUE)},
	}, 0)

	// Difficulty buttons
	diffY := radioY + 90
	btnW := contentW / 3
	sm.difficultyBtns = NewButtonGroup(contentX, diffY, []string{"Easy", "Medium", "Hard"}, 1, btnW, 34)

	// Sound checkbox
	checkY := diffY + 70
	sm.soundCheckbox = NewCheckbox(contentX, checkY, "Sound Effects", true)

	// Buttons at bottom
	btnW = 100
	btnH := 38
	btnY := sm.y + SettingsHeight - SettingsPadY - btnH
	btnSpacing := 12

	sm.cancelBtn = NewModalButton(
		sm.x+SettingsWidth-SettingsPadX-btnW*2-btnSpacing,
		btnY, btnW, btnH, "Cancel", false, nil,
	)
	sm.saveBtn = NewModalButton(
		sm.x+SettingsWidth-SettingsPadX-btnW,
		btnY, btnW, btnH, "Save", true, nil,
	)
}

// Show displays the settings modal with the given preferences.
func (sm *SettingsModal) Show(prefs *storage.UserPreferences, onSave func(*storage.UserPreferences), onCancel func()) {
	sm.visible = true
	sm.onSave = onSave
	sm.onCancel = onCancel

	// Store original for cancel
	sm.originalPrefs = &storage.UserPreferences{
		Username:     prefs.Username,
		Difficulty:   prefs.Difficulty,
		EvalMode:     prefs.EvalMode,
		SoundEnabled: prefs.SoundEnabled,
	}

	// Load current values into widgets
	sm.usernameInput.Value = prefs.Username
	sm.evalModeRadio.Selected = int(prefs.EvalMode)
	sm.difficultyBtns.Selected = int(prefs.Difficulty)
	sm.soundCheckbox.Checked = prefs.SoundEnabled

	// Set button callbacks
	sm.saveBtn.OnClick = sm.handleSave
	sm.cancelBtn.OnClick = sm.handleCancel
}

// Hide closes the settings modal.
func (sm *SettingsModal) Hide() {
	sm.visible = false
	sm.usernameInput.SetFocused(false)
}

// IsVisible returns true if the modal is visible.
func (sm *SettingsModal) IsVisible() bool {
	return sm.visible
}

// handleSave saves settings and closes the modal.
func (sm *SettingsModal) handleSave() {
	prefs := &storage.UserPreferences{
		Username:     sm.usernameInput.Value,
		Difficulty:   storage.Difficulty(sm.difficultyBtns.Selected),
		EvalMode:     storage.EvalMode(sm.evalModeRadio.Selected),
		SoundEnabled: sm.soundCheckbox.Checked,
	}

	// Use default name if empty
	if prefs.Username == "" {
		prefs.Username = "Player"
	}

	if sm.onSave != nil {
		sm.onSave(prefs)
	}
	sm.Hide()
}

// handleCancel discards changes and closes the modal.
func (sm *SettingsModal) handleCancel() {
	if sm.onCancel != nil {
		sm.onCancel()
	}
	sm.Hide()
}

// Update handles input for the settings modal.
func (sm *SettingsModal) Update(input *InputHandler) bool {
	if !sm.visible {
		return false
	}

	// Handle escape key to close
	if IsKeyJustPressed(ebiten.KeyEscape) {
		sm.handleCancel()
		return true
	}

	// Handle enter key to save
	if IsKeyJustPressed(ebiten.KeyEnter) && !sm.usernameInput.IsFocused() {
		sm.handleSave()
		return true
	}

	// Update widgets
	sm.usernameInput.Update(input)
	sm.evalModeRadio.Update(input)
	sm.difficultyBtns.Update(input)
	sm.soundCheckbox.Update(input)
	sm.saveBtn.Update(input)
	sm.cancelBtn.Update(input)

	// Modal consumes all input
	return true
}

// AnyButtonHovered returns true if any button in the modal is hovered.
func (sm *SettingsModal) AnyButtonHovered() bool {
	if !sm.visible {
		return false
	}
	return sm.saveBtn.IsHovered() || sm.cancelBtn.IsHovered() ||
		sm.evalModeRadio.hovered >= 0 || sm.difficultyBtns.hovered >= 0 ||
		sm.soundCheckbox.hovered
}

// Draw renders the settings modal.
func (sm *SettingsModal) Draw(screen *ebiten.Image, glass *GlassEffect) {
	if !sm.visible {
		return
	}

	// Full-screen blur overlay with glass effect
	if glass != nil && glass.IsEnabled() {
		tint := color.RGBA{0, 0, 0, 100} // Dark tint for modal backdrop
		glass.DrawGlass(screen, 0, 0, scaleI(ScreenWidth), scaleI(ScreenHeight),
			tint, 3.0, 4.0) // sigma=3.0, refraction=4.0
	} else {
		// Fallback: semi-transparent overlay
		vector.DrawFilledRect(screen, 0, 0, scaleF(ScreenWidth), scaleF(ScreenHeight), modalOverlay, false)
	}

	// Modal background
	vector.DrawFilledRect(screen, scaleF(sm.x), scaleF(sm.y), scaleF(SettingsWidth), scaleF(SettingsHeight), modalBg, false)

	// Modal border
	vector.StrokeRect(screen, scaleF(sm.x), scaleF(sm.y), scaleF(SettingsWidth), scaleF(SettingsHeight), float32(UIScale*2), modalBorder, false)

	// Header background
	vector.DrawFilledRect(screen, scaleF(sm.x), scaleF(sm.y), scaleF(SettingsWidth), scaleF(44), modalHeader, false)

	// Header title
	sm.drawTitle(screen)

	// Section labels
	contentX := sm.x + SettingsPadX
	sm.drawSectionLabel(screen, "Player Name", contentX, sm.y+52)
	sm.drawSectionLabel(screen, "Engine Mode", contentX, sm.usernameInput.Y+sm.usernameInput.H+16)
	sm.drawSectionLabel(screen, "Difficulty", contentX, sm.evalModeRadio.Y+sm.evalModeRadio.ItemH*len(sm.evalModeRadio.Options)+8)
	sm.drawSectionLabel(screen, "Audio", contentX, sm.difficultyBtns.Y+sm.difficultyBtns.ButtonH+16)

	// Draw widgets
	sm.usernameInput.Draw(screen)
	sm.evalModeRadio.Draw(screen)
	sm.difficultyBtns.Draw(screen)
	sm.soundCheckbox.Draw(screen)
	sm.saveBtn.Draw(screen)
	sm.cancelBtn.Draw(screen)
}

// drawTitle draws the modal title.
func (sm *SettingsModal) drawTitle(screen *ebiten.Image) {
	face := GetBoldFace()
	if face == nil {
		return
	}

	title := "Settings"
	w, h := MeasureText(title, face)
	centerX := scaleD(sm.x) + scaleD(SettingsWidth)/2 - w/2
	centerY := scaleD(sm.y) + scaleD(22) - h/2

	op := &text.DrawOptions{}
	op.GeoM.Translate(centerX, centerY)
	op.ColorScale.ScaleWithColor(textPrimary)
	text.Draw(screen, title, face, op)
}

// drawSectionLabel draws a section label.
func (sm *SettingsModal) drawSectionLabel(screen *ebiten.Image, label string, x, y int) {
	face := GetRegularFace()
	if face == nil {
		return
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(scaleD(x), scaleD(y))
	op.ColorScale.ScaleWithColor(textMuted)
	text.Draw(screen, label, face, op)
}
