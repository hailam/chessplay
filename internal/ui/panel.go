package ui

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Panel dimensions
const (
	PanelPadding    = 20
	SectionSpacing  = 28
	ButtonHeight    = 40
	TabHeight       = 34
	CollapsedWidth  = 36
	CollapseButtonW = 28
	CollapseButtonH = 80
	SectionLabelH   = 20
)

// Panel colors
var (
	panelBg         = color.RGBA{38, 40, 45, 255}    // Dark background
	sectionBg       = color.RGBA{48, 52, 58, 255}    // Slightly lighter section
	tabActiveBg     = color.RGBA{76, 132, 96, 255}   // Green for active tab
	tabInactiveBg   = color.RGBA{58, 62, 68, 255}    // Gray for inactive
	tabHoverBg      = color.RGBA{68, 72, 78, 255}    // Hover state
	buttonBg        = color.RGBA{58, 62, 68, 255}    // Button background
	buttonHoverBg   = color.RGBA{78, 82, 88, 255}    // Button hover
	buttonActiveBg  = color.RGBA{76, 132, 96, 255}   // Active button (green)
	accentColor     = color.RGBA{76, 175, 120, 255}  // Green accent
	textPrimary     = color.RGBA{240, 240, 245, 255} // Primary text
	textSecondary   = color.RGBA{160, 165, 175, 255} // Secondary text
	textMuted       = color.RGBA{120, 125, 135, 255} // Muted text
	dividerColor    = color.RGBA{60, 65, 72, 255}    // Divider line
	moveRowAlt      = color.RGBA{44, 48, 54, 255}    // Alternating row
	statusThinking  = color.RGBA{100, 180, 255, 255} // Blue for thinking
	statusGameOver  = color.RGBA{255, 200, 80, 255}  // Yellow for game over
	collapseButtonC = color.RGBA{58, 62, 68, 255}    // Collapse button
)

// Button represents a clickable UI element.
type Button struct {
	X, Y, W, H int
	Label      string
	OnClick    func()
	hovered    bool
	active     bool
}

// Panel represents the side panel with controls and move history.
type Panel struct {
	game      *Game
	collapsed bool

	// UI elements
	collapseBtn *Button
	newGameBtn  *Button
	modeTabs    []*Button // [0] = vs Human, [1] = vs Computer
	diffTabs    []*Button // [0] = Easy, [1] = Medium, [2] = Hard

	// Move history scroll
	scrollY    int
	maxScrollY int
}

// NewPanel creates a new panel for the given game.
func NewPanel(g *Game) *Panel {
	p := &Panel{
		game:      g,
		collapsed: false,
	}

	p.createButtons()
	return p
}

// createButtons initializes all panel buttons.
func (p *Panel) createButtons() {
	// Collapse/expand button (on right edge of panel, or centered when collapsed)
	if p.collapsed {
		p.collapseBtn = &Button{
			X: BoardSize + (CollapsedWidth-CollapseButtonW)/2,
			Y: (ScreenHeight - CollapseButtonH) / 2,
			W: CollapseButtonW, H: CollapseButtonH,
			OnClick: func() { p.toggleCollapse() },
		}
	} else {
		p.collapseBtn = &Button{
			X: BoardSize + PanelWidth - CollapseButtonW - 4,
			Y: (ScreenHeight - CollapseButtonH) / 2,
			W: CollapseButtonW, H: CollapseButtonH,
			OnClick: func() { p.toggleCollapse() },
		}
	}

	// Content area
	contentX := BoardSize + PanelPadding
	contentW := PanelWidth - PanelPadding*2 - CollapseButtonW

	// New Game button (full width, prominent)
	newGameY := PanelPadding + 8
	p.newGameBtn = &Button{
		X: contentX, Y: newGameY,
		W: contentW, H: ButtonHeight,
		Label:   "New Game",
		OnClick: p.game.NewGameAction,
	}

	// Mode section: label + tabs
	modeLabelY := newGameY + ButtonHeight + SectionSpacing
	modeTabY := modeLabelY + SectionLabelH
	tabW := contentW / 2
	p.modeTabs = []*Button{
		{X: contentX, Y: modeTabY, W: tabW, H: TabHeight, Label: "vs Human",
			OnClick: func() {
				if p.game.GameMode() != ModeHumanVsHuman {
					p.game.ToggleModeAction()
				}
			}},
		{X: contentX + tabW, Y: modeTabY, W: tabW, H: TabHeight, Label: "vs Computer",
			OnClick: func() {
				if p.game.GameMode() != ModeHumanVsComputer {
					p.game.ToggleModeAction()
				}
			}},
	}

	// Difficulty section: label + tabs (only visible in vs Computer mode)
	diffLabelY := modeTabY + TabHeight + SectionSpacing
	diffTabY := diffLabelY + SectionLabelH
	diffTabW := contentW / 3
	p.diffTabs = []*Button{
		{X: contentX, Y: diffTabY, W: diffTabW, H: TabHeight - 2, Label: "Easy",
			OnClick: func() { p.game.SetDifficulty(DifficultyEasy) }},
		{X: contentX + diffTabW, Y: diffTabY, W: diffTabW, H: TabHeight - 2, Label: "Medium",
			OnClick: func() { p.game.SetDifficulty(DifficultyMedium) }},
		{X: contentX + diffTabW*2, Y: diffTabY, W: diffTabW, H: TabHeight - 2, Label: "Hard",
			OnClick: func() { p.game.SetDifficulty(DifficultyHard) }},
	}
}

// HandleInput processes input for the panel. Returns true if input was handled.
func (p *Panel) HandleInput(input *InputHandler) bool {
	mx, my := input.MousePosition()

	// Always check collapse button
	p.collapseBtn.hovered = p.isInside(mx, my, p.collapseBtn)
	if input.IsLeftJustPressed() && p.collapseBtn.hovered {
		p.collapseBtn.OnClick() // toggleCollapse handles button recreation and window resize
		return true
	}

	if p.collapsed {
		return false
	}

	// Check other buttons
	p.newGameBtn.hovered = p.isInside(mx, my, p.newGameBtn)
	for _, btn := range p.modeTabs {
		btn.hovered = p.isInside(mx, my, btn)
	}
	for _, btn := range p.diffTabs {
		btn.hovered = p.isInside(mx, my, btn)
	}

	// Handle clicks
	if input.IsLeftJustPressed() {
		if p.newGameBtn.hovered {
			p.newGameBtn.OnClick()
			return true
		}
		for _, btn := range p.modeTabs {
			if btn.hovered {
				btn.OnClick()
				return true
			}
		}
		if p.game.GameMode() == ModeHumanVsComputer {
			for _, btn := range p.diffTabs {
				if btn.hovered {
					btn.OnClick()
					return true
				}
			}
		}
	}

	return false
}

func (p *Panel) isInside(mx, my int, btn *Button) bool {
	return mx >= btn.X && mx < btn.X+btn.W && my >= btn.Y && my < btn.Y+btn.H
}

// Draw renders the panel.
func (p *Panel) Draw(screen *ebiten.Image, r *Renderer) {
	panelX := float32(BoardSize)

	if p.collapsed {
		// Draw collapsed state - just a thin bar with expand button
		vector.DrawFilledRect(screen, panelX, 0, float32(CollapsedWidth), float32(ScreenHeight), panelBg, false)
		p.drawCollapseButton(screen, true)
		return
	}

	// Draw panel background
	vector.DrawFilledRect(screen, panelX, 0, float32(PanelWidth), float32(ScreenHeight), panelBg, false)

	// Draw collapse button
	p.drawCollapseButton(screen, false)

	// Draw New Game button
	p.drawPrimaryButton(screen, p.newGameBtn)

	// Draw mode section
	modeLabelY := p.modeTabs[0].Y - SectionLabelH
	p.drawSectionLabel(screen, "Game Mode", BoardSize+PanelPadding, modeLabelY)
	p.drawModeTabs(screen)

	// Draw difficulty section (only in vs Computer mode)
	if p.game.GameMode() == ModeHumanVsComputer {
		diffLabelY := p.diffTabs[0].Y - SectionLabelH
		p.drawSectionLabel(screen, "Difficulty", BoardSize+PanelPadding, diffLabelY)
		p.drawDifficultyTabs(screen)
	}

	// Draw move history section
	historyY := p.getHistoryStartY()
	p.drawSectionLabel(screen, "Moves", BoardSize+PanelPadding, historyY)
	p.drawMoveHistory(screen, historyY+SectionLabelH+4)

	// Draw status bar at bottom
	p.drawStatusBar(screen)
}

func (p *Panel) getHistoryStartY() int {
	if p.game.GameMode() == ModeHumanVsComputer {
		return p.diffTabs[0].Y + p.diffTabs[0].H + SectionSpacing - 4
	}
	return p.modeTabs[0].Y + p.modeTabs[0].H + SectionSpacing - 4
}

func (p *Panel) drawCollapseButton(screen *ebiten.Image, expand bool) {
	btn := p.collapseBtn
	bgColor := collapseButtonC
	if btn.hovered {
		bgColor = buttonHoverBg
	}

	// Draw rounded rect
	vector.DrawFilledRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), bgColor, false)

	// Draw arrow icon
	arrow := "‹"
	if expand {
		arrow = "›"
	}
	p.drawTextCentered(screen, arrow, btn.X+btn.W/2, btn.Y+btn.H/2, textPrimary)
}

func (p *Panel) drawPrimaryButton(screen *ebiten.Image, btn *Button) {
	bgColor := accentColor
	if btn.hovered {
		bgColor = color.RGBA{96, 195, 140, 255} // Lighter green on hover
	}

	// Draw button with rounded corners (approximated with filled rect)
	vector.DrawFilledRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), bgColor, false)

	// Draw label
	p.drawTextCentered(screen, btn.Label, btn.X+btn.W/2, btn.Y+btn.H/2, textPrimary)
}

func (p *Panel) drawModeTabs(screen *ebiten.Image) {
	for i, btn := range p.modeTabs {
		isActive := (i == 0 && p.game.GameMode() == ModeHumanVsHuman) ||
			(i == 1 && p.game.GameMode() == ModeHumanVsComputer)

		bgColor := tabInactiveBg
		if isActive {
			bgColor = tabActiveBg
		} else if btn.hovered {
			bgColor = tabHoverBg
		}

		vector.DrawFilledRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), bgColor, false)

		textColor := textSecondary
		if isActive {
			textColor = textPrimary
		}
		p.drawTextCentered(screen, btn.Label, btn.X+btn.W/2, btn.Y+btn.H/2, textColor)
	}
}

func (p *Panel) drawDifficultyTabs(screen *ebiten.Image) {
	for i, btn := range p.diffTabs {
		isActive := Difficulty(i) == p.game.Difficulty()

		bgColor := tabInactiveBg
		if isActive {
			bgColor = tabActiveBg
		} else if btn.hovered {
			bgColor = tabHoverBg
		}

		vector.DrawFilledRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), bgColor, false)

		textColor := textSecondary
		if isActive {
			textColor = textPrimary
		}
		p.drawTextCentered(screen, btn.Label, btn.X+btn.W/2, btn.Y+btn.H/2, textColor)
	}
}

func (p *Panel) drawSectionLabel(screen *ebiten.Image, label string, x, y int) {
	p.drawText(screen, label, x, y, textMuted)
}

func (p *Panel) drawMoveHistory(screen *ebiten.Image, startY int) {
	moves := p.game.SANHistory()
	if len(moves) == 0 {
		p.drawText(screen, "No moves yet", BoardSize+PanelPadding, startY+5, textMuted)
		return
	}

	x := BoardSize + PanelPadding
	y := startY
	rowHeight := 22
	maxY := ScreenHeight - 70 // Leave room for status bar

	for i := 0; i < len(moves); i += 2 {
		if y > maxY {
			// Show "more moves" indicator
			remaining := (len(moves) - i) / 2
			if (len(moves)-i)%2 != 0 {
				remaining++
			}
			p.drawText(screen, fmt.Sprintf("... +%d more", remaining), x, y, textMuted)
			break
		}

		// Alternating row background
		if (i/2)%2 == 1 {
			vector.DrawFilledRect(screen, float32(BoardSize+PanelPadding-4), float32(y-2),
				float32(PanelWidth-PanelPadding*2+8), float32(rowHeight), moveRowAlt, false)
		}

		moveNum := (i / 2) + 1

		// Move number
		numStr := fmt.Sprintf("%d.", moveNum)
		p.drawText(screen, numStr, x, y, textMuted)

		// White's move
		p.drawText(screen, moves[i], x+30, y, textPrimary)

		// Black's move (if exists)
		if i+1 < len(moves) {
			p.drawText(screen, moves[i+1], x+100, y, textPrimary)
		}

		y += rowHeight
	}
}

func (p *Panel) drawStatusBar(screen *ebiten.Image) {
	statusY := ScreenHeight - 50
	x := BoardSize + PanelPadding

	// Draw divider
	vector.DrawFilledRect(screen, float32(BoardSize+PanelPadding), float32(statusY-10),
		float32(PanelWidth-PanelPadding*2), 1, dividerColor, false)

	var statusText string
	var statusColor color.RGBA

	if p.game.GameOver() {
		statusText = p.game.GameResult()
		statusColor = statusGameOver
	} else if p.game.IsAIThinking() {
		statusText = "● AI thinking..."
		statusColor = statusThinking
	} else {
		if p.game.Position().SideToMove == 0 {
			statusText = "○ White to move"
		} else {
			statusText = "● Black to move"
		}
		statusColor = textPrimary
	}

	p.drawText(screen, statusText, x, statusY+5, statusColor)
}

// Text drawing helpers
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

// Collapsed returns whether the panel is collapsed.
func (p *Panel) Collapsed() bool {
	return p.collapsed
}

// toggleCollapse toggles the panel collapsed state and resizes the window.
func (p *Panel) toggleCollapse() {
	p.collapsed = !p.collapsed
	p.createButtons()

	// Resize window to match new layout
	if p.collapsed {
		ebiten.SetWindowSize(BoardSize+CollapsedWidth, ScreenHeight)
	} else {
		ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	}
}
