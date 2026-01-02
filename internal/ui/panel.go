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
	CollapsedWidth  = 20
	CollapseButtonW = 16
	CollapseButtonH = 48
	SectionLabelH   = 20
)

// Panel colors
var (
	panelBg         = color.RGBA{38, 40, 45, 255}    // Dark background
	sectionBg       = color.RGBA{48, 52, 58, 255}    // Slightly lighter section
	tabActiveBg     = color.RGBA{76, 132, 96, 255}   // Green for active tab
	tabInactiveBg   = color.RGBA{50, 54, 60, 255}    // Darker gray for inactive
	tabHoverBg      = color.RGBA{65, 70, 78, 255}    // Visible hover state
	buttonBg        = color.RGBA{50, 54, 60, 255}    // Button background (darker)
	buttonHoverBg   = color.RGBA{65, 70, 78, 255}    // Button hover (brighter)
	buttonPressedBg = color.RGBA{40, 44, 50, 255}    // Button pressed (darker)
	buttonBorder    = color.RGBA{70, 75, 82, 255}    // Subtle button border
	buttonActiveBg  = color.RGBA{76, 132, 96, 255}   // Active button (green)
	accentColor     = color.RGBA{76, 175, 120, 255}  // Green accent
	accentHover     = color.RGBA{96, 195, 140, 255}  // Lighter green on hover
	accentPressed   = color.RGBA{56, 155, 100, 255}  // Darker green on press
	textPrimary     = color.RGBA{240, 240, 245, 255} // Primary text
	textSecondary   = color.RGBA{160, 165, 175, 255} // Secondary text
	textMuted       = color.RGBA{120, 125, 135, 255} // Muted text
	dividerColor    = color.RGBA{60, 65, 72, 255}    // Divider line
	moveRowAlt      = color.RGBA{44, 48, 54, 255}    // Alternating row
	statusThinking  = color.RGBA{100, 180, 255, 255} // Blue for thinking
	statusGameOver  = color.RGBA{255, 200, 80, 255}  // Yellow for game over
	collapseButtonC = color.RGBA{60, 65, 72, 255}    // Collapse button (lighter for visibility)
)

// Button represents a clickable UI element.
type Button struct {
	X, Y, W, H int
	Label      string
	OnClick    func()
	hovered    bool
	pressed    bool
	active     bool
}

// Panel represents the side panel with controls and move history.
type Panel struct {
	game      *Game
	collapsed bool

	// UI elements
	collapseBtn *Button
	newGameBtn  *Button
	settingsBtn *Button
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
	// Collapse/expand button - integrated tab at panel edge
	tabY := (ScreenHeight - CollapseButtonH) / 2 // Vertically centered
	if p.collapsed {
		p.collapseBtn = &Button{
			X: BoardSize + 2,
			Y: tabY,
			W: CollapseButtonW, H: CollapseButtonH,
			OnClick: func() { p.toggleCollapse() },
		}
	} else {
		p.collapseBtn = &Button{
			X: BoardSize, // Flush with panel left edge
			Y: tabY,
			W: CollapseButtonW, H: CollapseButtonH,
			OnClick: func() { p.toggleCollapse() },
		}
	}

	// Content area - full width, collapse button doesn't take space
	contentX := BoardSize + PanelPadding
	contentW := PanelWidth - PanelPadding*2

	// New Game button (full width, prominent)
	newGameY := PanelPadding + 8
	p.newGameBtn = &Button{
		X: contentX, Y: newGameY,
		W: contentW, H: ButtonHeight,
		Label:   "New Game",
		OnClick: p.game.NewGameAction,
	}

	// Settings button (below New Game)
	settingsY := newGameY + ButtonHeight + 8
	p.settingsBtn = &Button{
		X: contentX, Y: settingsY,
		W: contentW, H: ButtonHeight - 6,
		Label:   "Settings",
		OnClick: p.game.ShowSettings,
	}

	// Mode section: label + tabs
	modeLabelY := settingsY + ButtonHeight - 6 + SectionSpacing - 8
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
	p.collapseBtn.pressed = input.IsLeftPressed() && p.collapseBtn.hovered

	if input.IsLeftJustPressed() && p.collapseBtn.hovered {
		p.collapseBtn.OnClick() // toggleCollapse handles button recreation and window resize
		return true
	}

	if p.collapsed {
		return false
	}

	// Handle scroll wheel for move history
	_, wheelY := ebiten.Wheel()
	if wheelY != 0 {
		historyY := p.getHistoryStartY()
		// Check if mouse is in move history area
		if mx >= BoardSize && my >= historyY && my < ScreenHeight-70 {
			p.scrollY -= int(wheelY * 30) // 30px per scroll tick
			if p.scrollY < 0 {
				p.scrollY = 0
			}
			if p.scrollY > p.maxScrollY {
				p.scrollY = p.maxScrollY
			}
		}
	}

	// Check other buttons for hover
	p.newGameBtn.hovered = p.isInside(mx, my, p.newGameBtn)
	p.settingsBtn.hovered = p.isInside(mx, my, p.settingsBtn)
	for _, btn := range p.modeTabs {
		btn.hovered = p.isInside(mx, my, btn)
	}
	for _, btn := range p.diffTabs {
		btn.hovered = p.isInside(mx, my, btn)
	}

	// Track pressed state (mouse down on button)
	if input.IsLeftPressed() {
		p.newGameBtn.pressed = p.newGameBtn.hovered
		p.settingsBtn.pressed = p.settingsBtn.hovered
		for _, btn := range p.modeTabs {
			btn.pressed = btn.hovered
		}
		for _, btn := range p.diffTabs {
			btn.pressed = btn.hovered
		}
	} else {
		// Clear pressed state when mouse released
		p.newGameBtn.pressed = false
		p.settingsBtn.pressed = false
		for _, btn := range p.modeTabs {
			btn.pressed = false
		}
		for _, btn := range p.diffTabs {
			btn.pressed = false
		}
	}

	// Handle clicks
	if input.IsLeftJustPressed() {
		if p.newGameBtn.hovered {
			p.newGameBtn.OnClick()
			return true
		}
		if p.settingsBtn.hovered {
			p.settingsBtn.OnClick()
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

// AnyButtonHovered returns true if any button in the panel is hovered.
func (p *Panel) AnyButtonHovered() bool {
	if p.collapseBtn.hovered {
		return true
	}
	if p.collapsed {
		return false
	}
	if p.newGameBtn.hovered || p.settingsBtn.hovered {
		return true
	}
	for _, btn := range p.modeTabs {
		if btn.hovered {
			return true
		}
	}
	for _, btn := range p.diffTabs {
		if btn.hovered {
			return true
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

	// Draw Settings button
	p.drawSecondaryButton(screen, p.settingsBtn)

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

	// Use panel background color to blend in as integrated tab
	bgColor := panelBg
	if btn.hovered {
		bgColor = sectionBg // Subtle highlight on hover
	}

	// Draw as integrated tab (no border, blends with panel)
	vector.DrawFilledRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), bgColor, false)

	// Draw arrow - muted by default, bright on hover
	arrow := "‹"
	if expand {
		arrow = "›"
	}
	textC := textMuted
	if btn.hovered {
		textC = textPrimary
	}
	p.drawTextCentered(screen, arrow, btn.X+btn.W/2, btn.Y+btn.H/2, textC)
}

func (p *Panel) drawPrimaryButton(screen *ebiten.Image, btn *Button) {
	bgColor := accentColor
	if btn.pressed {
		bgColor = accentPressed
	} else if btn.hovered {
		bgColor = accentHover
	}

	// Draw button background
	vector.DrawFilledRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), bgColor, false)

	// Draw border for depth
	borderC := color.RGBA{56, 155, 100, 255}
	if btn.hovered {
		borderC = color.RGBA{116, 215, 160, 255} // Lighter border on hover
	}
	vector.StrokeRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), 1, borderC, false)

	// Draw label
	p.drawTextCentered(screen, btn.Label, btn.X+btn.W/2, btn.Y+btn.H/2, textPrimary)
}

func (p *Panel) drawSecondaryButton(screen *ebiten.Image, btn *Button) {
	bgColor := buttonBg
	if btn.pressed {
		bgColor = buttonPressedBg
	} else if btn.hovered {
		bgColor = buttonHoverBg
	}

	// Draw button background
	vector.DrawFilledRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), bgColor, false)

	// Draw border
	borderC := buttonBorder
	if btn.hovered {
		borderC = accentColor // Green border on hover
	}
	vector.StrokeRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), 1, borderC, false)

	p.drawTextCentered(screen, btn.Label, btn.X+btn.W/2, btn.Y+btn.H/2, textSecondary)
}

func (p *Panel) drawModeTabs(screen *ebiten.Image) {
	for i, btn := range p.modeTabs {
		isActive := (i == 0 && p.game.GameMode() == ModeHumanVsHuman) ||
			(i == 1 && p.game.GameMode() == ModeHumanVsComputer)

		bgColor := tabInactiveBg
		if isActive {
			bgColor = tabActiveBg
		} else if btn.pressed {
			bgColor = buttonPressedBg
		} else if btn.hovered {
			bgColor = tabHoverBg
		}

		// Draw background
		vector.DrawFilledRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), bgColor, false)

		// Draw border - highlight on hover, green on active
		borderC := buttonBorder
		if isActive {
			borderC = tabActiveBg
		} else if btn.hovered {
			borderC = accentColor
		}
		vector.StrokeRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), 1, borderC, false)

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
		} else if btn.pressed {
			bgColor = buttonPressedBg
		} else if btn.hovered {
			bgColor = tabHoverBg
		}

		// Draw background
		vector.DrawFilledRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), bgColor, false)

		// Draw border - highlight on hover, green on active
		borderC := buttonBorder
		if isActive {
			borderC = tabActiveBg
		} else if btn.hovered {
			borderC = accentColor
		}
		vector.StrokeRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), 1, borderC, false)

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
	rowHeight := 22
	maxY := ScreenHeight - 70 // Leave room for status bar
	visibleHeight := maxY - startY

	// Calculate total content height and max scroll
	totalRows := (len(moves) + 1) / 2
	contentHeight := totalRows * rowHeight
	p.maxScrollY = contentHeight - visibleHeight
	if p.maxScrollY < 0 {
		p.maxScrollY = 0
	}

	// Clamp scroll position
	if p.scrollY > p.maxScrollY {
		p.scrollY = p.maxScrollY
	}

	// Calculate starting row based on scroll
	startRow := p.scrollY / rowHeight
	startMoveIdx := startRow * 2

	// Y position adjusted for partial scroll
	y := startY - (p.scrollY % rowHeight)

	for i := startMoveIdx; i < len(moves); i += 2 {
		// Skip if above visible area
		if y < startY-rowHeight {
			y += rowHeight
			continue
		}
		// Stop if below visible area
		if y > maxY {
			break
		}

		// Alternating row background (only if visible)
		if y >= startY-rowHeight && (i/2)%2 == 1 {
			bgY := y - 2
			if bgY < startY {
				bgY = startY
			}
			vector.DrawFilledRect(screen, float32(BoardSize+PanelPadding-4), float32(bgY),
				float32(PanelWidth-PanelPadding*2+8), float32(rowHeight), moveRowAlt, false)
		}

		// Only draw text if within visible bounds
		if y >= startY {
			moveNum := (i / 2) + 1
			numStr := fmt.Sprintf("%d.", moveNum)
			p.drawText(screen, numStr, x, y, textMuted)
			p.drawText(screen, moves[i], x+30, y, textPrimary)
			if i+1 < len(moves) {
				p.drawText(screen, moves[i+1], x+100, y, textPrimary)
			}
		}

		y += rowHeight
	}

	// Show scroll indicator if there's more content
	if p.maxScrollY > 0 {
		// Draw a small scroll indicator on the right
		scrollPct := float32(p.scrollY) / float32(p.maxScrollY)
		indicatorH := float32(visibleHeight) * float32(visibleHeight) / float32(contentHeight)
		if indicatorH < 20 {
			indicatorH = 20
		}
		indicatorY := float32(startY) + scrollPct*(float32(visibleHeight)-indicatorH)
		indicatorX := float32(BoardSize + PanelWidth - 8)
		vector.DrawFilledRect(screen, indicatorX, indicatorY, 4, indicatorH, textMuted, false)
	}
}

func (p *Panel) drawStatusBar(screen *ebiten.Image) {
	statusY := ScreenHeight - 70
	x := BoardSize + PanelPadding

	// Draw divider
	vector.DrawFilledRect(screen, float32(BoardSize+PanelPadding), float32(statusY-10),
		float32(PanelWidth-PanelPadding*2), 1, dividerColor, false)

	// Draw player name and eval mode
	username := p.game.Username()
	if len(username) > 12 {
		username = username[:12] + "..."
	}
	p.drawText(screen, username, x, statusY, textPrimary)

	// Eval mode badge
	evalMode := "Classical"
	evalColor := textSecondary
	if p.game.EvalMode() == EvalNNUE {
		evalMode = "NNUE"
		evalColor = accentColor
	}
	p.drawText(screen, evalMode, x+130, statusY, evalColor)

	// Game status
	var statusText string
	var statusColor color.RGBA

	if p.game.GameOver() {
		statusText = p.game.GameResult()
		statusColor = statusGameOver
	} else if p.game.IsAIThinking() {
		statusText = "AI thinking..."
		statusColor = statusThinking
	} else {
		if p.game.Position().SideToMove == 0 {
			statusText = "White to move"
		} else {
			statusText = "Black to move"
		}
		statusColor = textPrimary
	}

	p.drawText(screen, statusText, x, statusY+22, statusColor)
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
