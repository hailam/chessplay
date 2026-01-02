package ui

import (
	"image/color"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Widget colors (uses colors from panel.go: buttonBg, buttonHoverBg, accentColor, textPrimary, textSecondary)
var (
	widgetBg          = color.RGBA{48, 52, 58, 255}
	widgetBorder      = color.RGBA{68, 72, 78, 255}
	widgetFocusBorder = color.RGBA{76, 175, 120, 255}
	widgetHoverBg     = color.RGBA{65, 70, 78, 255}   // Brighter hover
	radioActive       = color.RGBA{76, 175, 120, 255}
	radioInactive     = color.RGBA{70, 75, 82, 255}   // Softer inactive
	checkboxCheck     = color.RGBA{76, 175, 120, 255}
	inputTextColor    = color.RGBA{240, 240, 245, 255}
	inputPlaceholder  = color.RGBA{120, 125, 135, 255}
)

// TextInput is an editable text field widget.
type TextInput struct {
	X, Y, W, H   int
	Value        string
	Placeholder  string
	MaxLength    int
	focused      bool
	hovered      bool
	cursorBlink  int
}

// NewTextInput creates a new text input widget.
func NewTextInput(x, y, w, h int, placeholder string, maxLen int) *TextInput {
	return &TextInput{
		X: x, Y: y, W: w, H: h,
		Placeholder: placeholder,
		MaxLength:   maxLen,
	}
}

// Update handles text input updates.
func (ti *TextInput) Update(input *InputHandler) bool {
	mx, my := input.MousePosition()
	ti.hovered = mx >= ti.X && mx < ti.X+ti.W && my >= ti.Y && my < ti.Y+ti.H

	// Handle click to focus
	if input.IsLeftJustPressed() {
		ti.focused = ti.hovered
	}

	if !ti.focused {
		return false
	}

	ti.cursorBlink++
	if ti.cursorBlink > 60 {
		ti.cursorBlink = 0
	}

	// Handle text input
	chars := ebiten.AppendInputChars(nil)
	for _, c := range chars {
		if ti.MaxLength == 0 || utf8.RuneCountInString(ti.Value) < ti.MaxLength {
			ti.Value += string(c)
		}
	}

	// Handle backspace
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if len(ti.Value) > 0 {
			_, size := utf8.DecodeLastRuneInString(ti.Value)
			ti.Value = ti.Value[:len(ti.Value)-size]
		}
	}

	// Handle escape to unfocus
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		ti.focused = false
	}

	return true
}

// Draw renders the text input.
func (ti *TextInput) Draw(screen *ebiten.Image) {
	// Background - slightly lighter on hover
	bgColor := widgetBg
	if ti.hovered && !ti.focused {
		bgColor = color.RGBA{52, 56, 62, 255}
	}
	vector.DrawFilledRect(screen, float32(ti.X), float32(ti.Y), float32(ti.W), float32(ti.H), bgColor, false)

	// Border - accent on hover/focus
	borderColor := widgetBorder
	if ti.focused {
		borderColor = widgetFocusBorder
	} else if ti.hovered {
		borderColor = accentColor
	}
	vector.StrokeRect(screen, float32(ti.X), float32(ti.Y), float32(ti.W), float32(ti.H), 2, borderColor, false)

	// Text or placeholder
	face := GetRegularFace()
	if face == nil {
		return
	}

	textX := ti.X + 10
	textY := ti.Y + ti.H/2

	if ti.Value != "" {
		op := &text.DrawOptions{}
		_, h := MeasureText(ti.Value, face)
		op.GeoM.Translate(float64(textX), float64(textY)-h/2)
		op.ColorScale.ScaleWithColor(inputTextColor)
		text.Draw(screen, ti.Value, face, op)

		// Cursor
		if ti.focused && ti.cursorBlink < 30 {
			w, _ := MeasureText(ti.Value, face)
			cursorX := float32(textX) + float32(w) + 2
			vector.DrawFilledRect(screen, cursorX, float32(ti.Y+8), 2, float32(ti.H-16), inputTextColor, false)
		}
	} else if ti.Placeholder != "" {
		op := &text.DrawOptions{}
		_, h := MeasureText(ti.Placeholder, face)
		op.GeoM.Translate(float64(textX), float64(textY)-h/2)
		op.ColorScale.ScaleWithColor(inputPlaceholder)
		text.Draw(screen, ti.Placeholder, face, op)

		// Cursor when focused and empty
		if ti.focused && ti.cursorBlink < 30 {
			vector.DrawFilledRect(screen, float32(textX), float32(ti.Y+8), 2, float32(ti.H-16), inputTextColor, false)
		}
	}
}

// IsFocused returns true if the input is focused.
func (ti *TextInput) IsFocused() bool {
	return ti.focused
}

// SetFocused sets the focus state.
func (ti *TextInput) SetFocused(focused bool) {
	ti.focused = focused
}

// RadioOption represents a single radio button option.
type RadioOption struct {
	Label string
	Value int
}

// RadioGroup is a group of mutually exclusive radio buttons.
type RadioGroup struct {
	X, Y      int
	Options   []RadioOption
	Selected  int
	ItemH     int
	hovered   int
}

// NewRadioGroup creates a new radio group.
func NewRadioGroup(x, y int, options []RadioOption, selected int) *RadioGroup {
	return &RadioGroup{
		X:        x,
		Y:        y,
		Options:  options,
		Selected: selected,
		ItemH:    30,
		hovered:  -1,
	}
}

// Update handles radio group input.
func (rg *RadioGroup) Update(input *InputHandler) bool {
	mx, my := input.MousePosition()
	rg.hovered = -1

	for i := range rg.Options {
		itemY := rg.Y + i*rg.ItemH
		if mx >= rg.X && mx < rg.X+200 && my >= itemY && my < itemY+rg.ItemH {
			rg.hovered = i
			if input.IsLeftJustPressed() {
				rg.Selected = i
				return true
			}
		}
	}
	return false
}

// Draw renders the radio group.
func (rg *RadioGroup) Draw(screen *ebiten.Image) {
	face := GetRegularFace()
	if face == nil {
		return
	}

	for i, opt := range rg.Options {
		itemY := rg.Y + i*rg.ItemH
		isSelected := i == rg.Selected
		isHovered := i == rg.hovered

		// Draw hover background
		if isHovered && !isSelected {
			hoverBg := color.RGBA{55, 60, 68, 255}
			vector.DrawFilledRect(screen, float32(rg.X-4), float32(itemY), 200, float32(rg.ItemH), hoverBg, false)
		}

		// Radio circle
		cx := float32(rg.X + 10)
		cy := float32(itemY + rg.ItemH/2)
		radius := float32(8)

		// Outer circle
		circleColor := radioInactive
		if isSelected {
			circleColor = radioActive
		} else if isHovered {
			circleColor = accentColor
		}
		vector.DrawFilledCircle(screen, cx, cy, radius, circleColor, false)

		// Inner circle for selected
		if isSelected {
			vector.DrawFilledCircle(screen, cx, cy, radius-4, inputTextColor, false)
		}

		// Label
		textX := rg.X + 30
		op := &text.DrawOptions{}
		_, h := MeasureText(opt.Label, face)
		op.GeoM.Translate(float64(textX), float64(itemY+rg.ItemH/2)-h/2)
		textColor := textSecondary
		if isSelected {
			textColor = textPrimary
		} else if isHovered {
			textColor = inputTextColor
		}
		op.ColorScale.ScaleWithColor(textColor)
		text.Draw(screen, opt.Label, face, op)
	}
}

// Checkbox is a toggleable checkbox widget.
type Checkbox struct {
	X, Y    int
	Label   string
	Checked bool
	hovered bool
}

// NewCheckbox creates a new checkbox.
func NewCheckbox(x, y int, label string, checked bool) *Checkbox {
	return &Checkbox{
		X:       x,
		Y:       y,
		Label:   label,
		Checked: checked,
	}
}

// Update handles checkbox input.
func (cb *Checkbox) Update(input *InputHandler) bool {
	mx, my := input.MousePosition()
	cb.hovered = mx >= cb.X && mx < cb.X+200 && my >= cb.Y && my < cb.Y+24

	if input.IsLeftJustPressed() && cb.hovered {
		cb.Checked = !cb.Checked
		return true
	}
	return false
}

// Draw renders the checkbox.
func (cb *Checkbox) Draw(screen *ebiten.Image) {
	face := GetRegularFace()
	if face == nil {
		return
	}

	// Checkbox box
	boxX := float32(cb.X)
	boxY := float32(cb.Y)
	boxSize := float32(20)

	bgColor := widgetBg
	if cb.hovered {
		bgColor = widgetHoverBg
	}
	vector.DrawFilledRect(screen, boxX, boxY, boxSize, boxSize, bgColor, false)

	// Border - accent on hover
	borderC := widgetBorder
	if cb.hovered {
		borderC = accentColor
	} else if cb.Checked {
		borderC = checkboxCheck
	}
	vector.StrokeRect(screen, boxX, boxY, boxSize, boxSize, 2, borderC, false)

	// Checkmark
	if cb.Checked {
		// Draw a simple checkmark using lines
		vector.StrokeLine(screen, boxX+4, boxY+10, boxX+8, boxY+14, 2, checkboxCheck, false)
		vector.StrokeLine(screen, boxX+8, boxY+14, boxX+16, boxY+6, 2, checkboxCheck, false)
	}

	// Label
	textX := cb.X + 30
	op := &text.DrawOptions{}
	_, h := MeasureText(cb.Label, face)
	op.GeoM.Translate(float64(textX), float64(cb.Y+10)-h/2)
	textColor := textSecondary
	if cb.Checked {
		textColor = textPrimary
	} else if cb.hovered {
		textColor = inputTextColor
	}
	op.ColorScale.ScaleWithColor(textColor)
	text.Draw(screen, cb.Label, face, op)
}

// ButtonGroup is a horizontal group of toggle buttons.
type ButtonGroup struct {
	X, Y     int
	Options  []string
	Selected int
	ButtonW  int
	ButtonH  int
	hovered  int
	pressed  int
}

// NewButtonGroup creates a new button group.
func NewButtonGroup(x, y int, options []string, selected int, buttonW, buttonH int) *ButtonGroup {
	return &ButtonGroup{
		X:        x,
		Y:        y,
		Options:  options,
		Selected: selected,
		ButtonW:  buttonW,
		ButtonH:  buttonH,
		hovered:  -1,
		pressed:  -1,
	}
}

// Update handles button group input.
func (bg *ButtonGroup) Update(input *InputHandler) bool {
	mx, my := input.MousePosition()
	bg.hovered = -1
	bg.pressed = -1

	for i := range bg.Options {
		btnX := bg.X + i*bg.ButtonW
		if mx >= btnX && mx < btnX+bg.ButtonW && my >= bg.Y && my < bg.Y+bg.ButtonH {
			bg.hovered = i
			if input.IsLeftPressed() {
				bg.pressed = i
			}
			if input.IsLeftJustPressed() {
				bg.Selected = i
				return true
			}
		}
	}
	return false
}

// Draw renders the button group.
func (bg *ButtonGroup) Draw(screen *ebiten.Image) {
	face := GetRegularFace()
	if face == nil {
		return
	}

	// Tab colors - keep in sync with panel.go
	tabActive := color.RGBA{76, 132, 96, 255}
	tabInactive := color.RGBA{50, 54, 60, 255}
	tabHover := color.RGBA{65, 70, 78, 255}
	tabPressed := color.RGBA{40, 44, 50, 255}
	borderColor := color.RGBA{70, 75, 82, 255}

	for i, label := range bg.Options {
		btnX := bg.X + i*bg.ButtonW
		isSelected := i == bg.Selected
		isHovered := i == bg.hovered
		isPressed := i == bg.pressed

		// Button background
		bgColor := tabInactive
		if isSelected {
			bgColor = tabActive
		} else if isPressed {
			bgColor = tabPressed
		} else if isHovered {
			bgColor = tabHover
		}
		vector.DrawFilledRect(screen, float32(btnX), float32(bg.Y), float32(bg.ButtonW), float32(bg.ButtonH), bgColor, false)

		// Border - accent on hover, match bg on selected
		bordC := borderColor
		if isSelected {
			bordC = tabActive
		} else if isHovered {
			bordC = accentColor
		}
		vector.StrokeRect(screen, float32(btnX), float32(bg.Y), float32(bg.ButtonW), float32(bg.ButtonH), 1, bordC, false)

		// Label
		w, h := MeasureText(label, face)
		centerX := float64(btnX) + float64(bg.ButtonW)/2 - w/2
		centerY := float64(bg.Y) + float64(bg.ButtonH)/2 - h/2
		op := &text.DrawOptions{}
		op.GeoM.Translate(centerX, centerY)
		textColor := textSecondary
		if isSelected {
			textColor = textPrimary
		}
		op.ColorScale.ScaleWithColor(textColor)
		text.Draw(screen, label, face, op)
	}
}

// ModalButton is a button for modal dialogs.
type ModalButton struct {
	X, Y, W, H int
	Label      string
	Primary    bool
	OnClick    func()
	hovered    bool
	pressed    bool
}

// IsHovered returns true if the button is hovered.
func (mb *ModalButton) IsHovered() bool {
	return mb.hovered
}

// NewModalButton creates a new modal button.
func NewModalButton(x, y, w, h int, label string, primary bool, onClick func()) *ModalButton {
	return &ModalButton{
		X: x, Y: y, W: w, H: h,
		Label:   label,
		Primary: primary,
		OnClick: onClick,
	}
}

// Update handles modal button input.
func (mb *ModalButton) Update(input *InputHandler) bool {
	mx, my := input.MousePosition()
	mb.hovered = mx >= mb.X && mx < mb.X+mb.W && my >= mb.Y && my < mb.Y+mb.H
	mb.pressed = input.IsLeftPressed() && mb.hovered

	if input.IsLeftJustPressed() && mb.hovered && mb.OnClick != nil {
		mb.OnClick()
		return true
	}
	return false
}

// Draw renders the modal button.
func (mb *ModalButton) Draw(screen *ebiten.Image) {
	face := GetRegularFace()
	if face == nil {
		return
	}

	// Button colors
	var bgColor color.RGBA
	var borderC color.RGBA

	if mb.Primary {
		bgColor = accentColor
		borderC = color.RGBA{56, 155, 100, 255}
		if mb.pressed {
			bgColor = color.RGBA{56, 155, 100, 255}
		} else if mb.hovered {
			bgColor = color.RGBA{96, 195, 140, 255}
			borderC = color.RGBA{116, 215, 160, 255}
		}
	} else {
		bgColor = buttonBg
		borderC = widgetBorder
		if mb.pressed {
			bgColor = color.RGBA{40, 44, 50, 255}
		} else if mb.hovered {
			bgColor = buttonHoverBg
			borderC = accentColor
		}
	}

	// Draw background
	vector.DrawFilledRect(screen, float32(mb.X), float32(mb.Y), float32(mb.W), float32(mb.H), bgColor, false)

	// Draw border
	vector.StrokeRect(screen, float32(mb.X), float32(mb.Y), float32(mb.W), float32(mb.H), 1, borderC, false)

	// Label
	w, h := MeasureText(mb.Label, face)
	centerX := float64(mb.X) + float64(mb.W)/2 - w/2
	centerY := float64(mb.Y) + float64(mb.H)/2 - h/2
	op := &text.DrawOptions{}
	op.GeoM.Translate(centerX, centerY)
	op.ColorScale.ScaleWithColor(textPrimary)
	text.Draw(screen, mb.Label, face, op)
}

// Divider draws a horizontal divider line.
func DrawDivider(screen *ebiten.Image, x, y, w int) {
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), 1, dividerColor, false)
}

// SectionHeader draws a section header with label.
func DrawSectionHeader(screen *ebiten.Image, label string, x, y int) {
	face := GetRegularFace()
	if face == nil {
		return
	}
	op := &text.DrawOptions{}
	_, h := MeasureText(label, face)
	op.GeoM.Translate(float64(x), float64(y)-h/2)
	op.ColorScale.ScaleWithColor(textMuted)
	text.Draw(screen, label, face, op)
}
