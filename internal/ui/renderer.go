package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/hailam/chessplay/internal/board"
)

// Theme defines the color scheme for the board.
type Theme struct {
	LightSquare    color.RGBA
	DarkSquare     color.RGBA
	SelectedSquare color.RGBA
	LegalMoveColor color.RGBA
	LastMoveColor  color.RGBA
	CheckColor     color.RGBA
	Background     color.RGBA
	TextColor      color.RGBA
	ButtonColor    color.RGBA
	ButtonHover    color.RGBA
}

// DefaultTheme returns the default color theme.
func DefaultTheme() *Theme {
	return &Theme{
		LightSquare:    color.RGBA{240, 217, 181, 255}, // Tan
		DarkSquare:     color.RGBA{181, 136, 99, 255},  // Brown
		SelectedSquare: color.RGBA{247, 247, 105, 180}, // Yellow highlight
		LegalMoveColor: color.RGBA{130, 151, 105, 200}, // Green dots
		LastMoveColor:  color.RGBA{180, 190, 100, 90},  // Softer yellow-green (reduced alpha)
		CheckColor:     color.RGBA{255, 100, 100, 180}, // Red
		Background:     color.RGBA{40, 44, 52, 255},    // Dark gray
		TextColor:      color.RGBA{220, 220, 220, 255}, // Light gray
		ButtonColor:    color.RGBA{60, 64, 72, 255},    // Medium gray
		ButtonHover:    color.RGBA{80, 84, 92, 255},    // Lighter gray
	}
}

// Renderer handles all drawing operations.
type Renderer struct {
	sprites    *SpriteManager
	theme      *Theme
	boardSize  int
	squareSize int
	scale      float64 // HiDPI scale factor
}

// NewRenderer creates a new renderer.
func NewRenderer(boardSize, squareSize int) *Renderer {
	return &Renderer{
		sprites:    NewSpriteManager(squareSize),
		theme:      DefaultTheme(),
		boardSize:  boardSize,
		squareSize: squareSize,
		scale:      1.0,
	}
}

// SetScale sets the HiDPI scale factor for rendering.
func (r *Renderer) SetScale(scale float64) {
	r.scale = scale
	r.sprites.SetScale(scale)
}

// s returns the scaled value for rendering.
func (r *Renderer) s(v int) float32 {
	return float32(float64(v) * r.scale)
}

// sf returns the scaled float value for rendering.
func (r *Renderer) sf(v float32) float32 {
	return v * float32(r.scale)
}

// DrawBoard draws the chess board squares.
func (r *Renderer) DrawBoard(screen *ebiten.Image) {
	for rank := 0; rank < 8; rank++ {
		for file := 0; file < 8; file++ {
			x := r.s(file * r.squareSize)
			y := r.s((7 - rank) * r.squareSize) // Flip so rank 1 is at bottom

			var c color.RGBA
			if (rank+file)%2 == 0 {
				c = r.theme.DarkSquare
			} else {
				c = r.theme.LightSquare
			}

			vector.DrawFilledRect(screen, x, y, r.s(r.squareSize), r.s(r.squareSize), c, false)
		}
	}

	// Draw coordinates
	r.drawCoordinates(screen)
}

// drawCoordinates draws file letters and rank numbers on the board.
func (r *Renderer) drawCoordinates(screen *ebiten.Image) {
	// This is a simplified version - for full text rendering we'd use a font
	// For now, we'll skip the coordinate labels as they require font loading
}

// DrawHighlights draws selection and legal move highlights.
func (r *Renderer) DrawHighlights(screen *ebiten.Image, selected board.Square, legalMoves *board.MoveList, lastMove board.Move) {
	// Highlight last move
	if lastMove != board.NoMove {
		r.highlightSquare(screen, lastMove.From(), r.theme.LastMoveColor)
		r.highlightSquare(screen, lastMove.To(), r.theme.LastMoveColor)
	}

	// Highlight selected square
	if selected != board.NoSquare {
		r.highlightSquare(screen, selected, r.theme.SelectedSquare)
	}

	// Draw legal move indicators
	if legalMoves != nil {
		for i := 0; i < legalMoves.Len(); i++ {
			move := legalMoves.Get(i)
			r.drawLegalMoveIndicator(screen, move.To())
		}
	}
}

// DrawCheck highlights the king's square if in check.
func (r *Renderer) DrawCheck(screen *ebiten.Image, kingSq board.Square) {
	if kingSq != board.NoSquare {
		r.highlightSquare(screen, kingSq, r.theme.CheckColor)
	}
}

// highlightSquare draws a colored overlay on a square.
func (r *Renderer) highlightSquare(screen *ebiten.Image, sq board.Square, c color.RGBA) {
	if sq == board.NoSquare {
		return
	}
	x, y := r.SquareToScreen(sq)
	vector.DrawFilledRect(screen, r.s(x), r.s(y), r.s(r.squareSize), r.s(r.squareSize), c, false)
}

// drawLegalMoveIndicator draws a circle on legal move squares.
func (r *Renderer) drawLegalMoveIndicator(screen *ebiten.Image, sq board.Square) {
	x, y := r.SquareToScreen(sq)
	cx := r.s(x) + r.s(r.squareSize)/2
	cy := r.s(y) + r.s(r.squareSize)/2
	radius := r.s(r.squareSize) * 0.15

	vector.DrawFilledCircle(screen, cx, cy, radius, r.theme.LegalMoveColor, false)
}

// DrawPieces draws all pieces on the board.
func (r *Renderer) DrawPieces(screen *ebiten.Image, pos *board.Position, dragging bool, dragSquare board.Square) {
	r.DrawPiecesWithAnimations(screen, pos, dragging, dragSquare, nil)
}

// DrawPiecesWithAnimations draws all pieces with optional shake animations.
func (r *Renderer) DrawPiecesWithAnimations(screen *ebiten.Image, pos *board.Position, dragging bool, dragSquare board.Square, anims *AnimationManager) {
	for sq := board.A1; sq <= board.H8; sq++ {
		// Skip the dragged piece
		if dragging && sq == dragSquare {
			continue
		}

		piece := pos.PieceAt(sq)
		if piece == board.NoPiece {
			continue
		}

		x, y := r.SquareToScreen(sq)

		// Apply shake offset if animations are active
		if anims != nil {
			offsetX, offsetY := anims.GetShakeOffset(sq)
			x += int(offsetX)
			y += int(offsetY)
		}

		// Scale coordinates for HiDPI
		r.sprites.DrawPieceAt(screen, piece, int(r.s(x)), int(r.s(y)))
	}
}

// DrawDraggedPiece draws the piece being dragged at the mouse position.
// mouseX, mouseY are in logical coordinates (will be scaled for drawing).
func (r *Renderer) DrawDraggedPiece(screen *ebiten.Image, piece board.Piece, mouseX, mouseY int) {
	if piece == board.NoPiece {
		return
	}

	// Scale mouse position for drawing and center the piece on the cursor
	halfSize := int(r.s(r.squareSize)) / 2
	x := int(r.s(mouseX)) - halfSize
	y := int(r.s(mouseY)) - halfSize

	r.sprites.DrawPieceAt(screen, piece, x, y)
}

// SquareToScreen converts a board square to screen coordinates.
func (r *Renderer) SquareToScreen(sq board.Square) (int, int) {
	file := sq.File()
	rank := sq.Rank()
	x := file * r.squareSize
	y := (7 - rank) * r.squareSize // Flip so rank 1 is at bottom
	return x, y
}

// ScreenToSquare converts screen coordinates to a board square.
func (r *Renderer) ScreenToSquare(x, y int) board.Square {
	if x < 0 || x >= r.boardSize || y < 0 || y >= r.boardSize {
		return board.NoSquare
	}
	file := x / r.squareSize
	rank := 7 - (y / r.squareSize) // Flip so rank 1 is at bottom
	return board.NewSquare(file, rank)
}

// BoardSize returns the board size in pixels.
func (r *Renderer) BoardSize() int {
	return r.boardSize
}

// SquareSize returns the size of one square in pixels.
func (r *Renderer) SquareSize() int {
	return r.squareSize
}

// Theme returns the current theme.
func (r *Renderer) Theme() *Theme {
	return r.theme
}

// Sprites returns the sprite manager.
func (r *Renderer) Sprites() *SpriteManager {
	return r.sprites
}
