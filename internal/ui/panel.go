package ui

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Button represents a clickable button.
type Button struct {
	X, Y, W, H int
	Label      string
	OnClick    func()
	hovered    bool
	active     bool // For toggle buttons
}

// Panel represents the side panel with controls and move history.
type Panel struct {
	game        *Game
	buttons     []*Button
	diffButtons []*Button
	scrollY     int
}

// NewPanel creates a new panel for the given game.
func NewPanel(g *Game) *Panel {
	p := &Panel{
		game: g,
	}

	// New Game button
	p.buttons = append(p.buttons, &Button{
		X: BoardSize + 20, Y: 20, W: 130, H: 40,
		Label:   "New Game",
		OnClick: g.NewGameAction,
	})

	// Mode toggle button
	p.buttons = append(p.buttons, &Button{
		X: BoardSize + 160, Y: 20, W: 130, H: 40,
		Label:   "vs Computer",
		OnClick: g.ToggleModeAction,
	})

	// Difficulty buttons
	diffY := 75
	p.diffButtons = append(p.diffButtons, &Button{
		X: BoardSize + 20, Y: diffY, W: 85, H: 30,
		Label:   "Easy",
		OnClick: func() { g.SetDifficulty(DifficultyEasy) },
	})
	p.diffButtons = append(p.diffButtons, &Button{
		X: BoardSize + 115, Y: diffY, W: 85, H: 30,
		Label:   "Medium",
		OnClick: func() { g.SetDifficulty(DifficultyMedium) },
	})
	p.diffButtons = append(p.diffButtons, &Button{
		X: BoardSize + 210, Y: diffY, W: 85, H: 30,
		Label:   "Hard",
		OnClick: func() { g.SetDifficulty(DifficultyHard) },
	})

	return p
}

// HandleInput processes input for the panel. Returns true if input was handled.
func (p *Panel) HandleInput(input *InputHandler) bool {
	mx, my := input.MousePosition()

	// Update button hover states
	for _, btn := range p.buttons {
		btn.hovered = mx >= btn.X && mx < btn.X+btn.W && my >= btn.Y && my < btn.Y+btn.H
	}
	for _, btn := range p.diffButtons {
		btn.hovered = mx >= btn.X && mx < btn.X+btn.W && my >= btn.Y && my < btn.Y+btn.H
	}

	// Handle clicks
	if input.IsLeftJustPressed() {
		for _, btn := range p.buttons {
			if btn.hovered && btn.OnClick != nil {
				btn.OnClick()
				return true
			}
		}
		for _, btn := range p.diffButtons {
			if btn.hovered && btn.OnClick != nil {
				btn.OnClick()
				return true
			}
		}
	}

	return false
}

// Draw renders the panel.
func (p *Panel) Draw(screen *ebiten.Image, r *Renderer) {
	theme := r.Theme()

	// Draw panel background
	vector.DrawFilledRect(screen, float32(BoardSize), 0, float32(PanelWidth), float32(ScreenHeight), theme.Background, false)

	// Draw buttons
	for _, btn := range p.buttons {
		p.drawButton(screen, btn, theme)
	}

	// Update mode button label
	if p.game.GameMode() == ModeHumanVsHuman {
		p.buttons[1].Label = "vs Human"
	} else {
		p.buttons[1].Label = "vs Computer"
	}

	// Draw difficulty buttons (only show in vs Computer mode)
	if p.game.GameMode() == ModeHumanVsComputer {
		for i, btn := range p.diffButtons {
			btn.active = Difficulty(i) == p.game.Difficulty()
			p.drawButton(screen, btn, theme)
		}
	}

	// Draw move history title
	titleY := 130
	p.drawText(screen, "Move History", BoardSize+20, titleY, theme.TextColor)

	// Draw moves
	p.drawMoveHistory(screen, r, titleY+30)

	// Draw game status
	if p.game.GameOver() {
		p.drawText(screen, p.game.GameResult(), BoardSize+20, ScreenHeight-60, color.RGBA{255, 200, 0, 255})
	} else if p.game.IsAIThinking() {
		p.drawText(screen, "AI thinking...", BoardSize+20, ScreenHeight-60, color.RGBA{150, 200, 255, 255})
	} else {
		turn := "White to move"
		if p.game.Position().SideToMove == 1 {
			turn = "Black to move"
		}
		p.drawText(screen, turn, BoardSize+20, ScreenHeight-60, theme.TextColor)
	}
}

// drawButton draws a button.
func (p *Panel) drawButton(screen *ebiten.Image, btn *Button, theme *Theme) {
	var bgColor color.RGBA
	if btn.active {
		bgColor = color.RGBA{100, 150, 100, 255} // Green for active
	} else if btn.hovered {
		bgColor = theme.ButtonHover
	} else {
		bgColor = theme.ButtonColor
	}

	// Draw background
	vector.DrawFilledRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), bgColor, false)

	// Draw border
	vector.StrokeRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), 2, theme.TextColor, false)

	// Draw label (centered)
	centerX := btn.X + btn.W/2
	centerY := btn.Y + btn.H/2
	p.drawTextCentered(screen, btn.Label, centerX, centerY, theme.TextColor)
}

// drawMoveHistory draws the move history list.
func (p *Panel) drawMoveHistory(screen *ebiten.Image, r *Renderer, startY int) {
	theme := r.Theme()
	moves := p.game.SANHistory()

	y := startY
	maxY := ScreenHeight - 100

	for i := 0; i < len(moves); i += 2 {
		if y > maxY {
			break
		}

		moveNum := (i / 2) + 1
		line := fmt.Sprintf("%d. %s", moveNum, moves[i])
		if i+1 < len(moves) {
			line += fmt.Sprintf("  %s", moves[i+1])
		}

		p.drawText(screen, line, BoardSize+20, y, theme.TextColor)
		y += 20
	}
}

// drawText draws text at the given position using proper font rendering.
func (p *Panel) drawText(screen *ebiten.Image, s string, x, y int, c color.Color) {
	face := GetRegularFace()
	if face == nil {
		return
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(c)
	text.Draw(screen, s, face, op)
}

// drawTextCentered draws text centered at the given position.
func (p *Panel) drawTextCentered(screen *ebiten.Image, s string, centerX, centerY int, c color.Color) {
	face := GetRegularFace()
	if face == nil {
		return
	}
	w, h := MeasureText(s, face)
	x := float64(centerX) - w/2
	y := float64(centerY) - h/2
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(screen, s, face, op)
}
