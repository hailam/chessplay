package ui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hailam/chessplay/internal/board"
	"github.com/hailam/chessplay/internal/engine"
)

// UI Constants
const (
	ScreenWidth  = 960
	ScreenHeight = 640 // Match board height to eliminate unused space
	BoardSize    = 640
	SquareSize   = BoardSize / 8
	PanelWidth   = ScreenWidth - BoardSize
)

// GameMode represents the current game mode.
type GameMode int

const (
	ModeHumanVsHuman GameMode = iota
	ModeHumanVsComputer
)

// Difficulty represents AI difficulty levels.
type Difficulty int

const (
	DifficultyEasy Difficulty = iota
	DifficultyMedium
	DifficultyHard
)

// Game implements ebiten.Game interface.
type Game struct {
	// Core game state
	position    *board.Position
	moveHistory []board.Move
	sanHistory  []string

	// UI state
	selectedSquare board.Square
	legalMoves     *board.MoveList
	dragging       bool
	dragPiece      board.Piece
	dragSquare     board.Square
	lastMove       board.Move

	// Game settings
	mode       GameMode
	difficulty Difficulty

	// Components
	renderer *Renderer
	input    *InputHandler
	panel    *Panel
	feedback *FeedbackManager

	// AI Engine
	engine     *engine.Engine
	aiThinking bool
	aiMove     chan board.Move

	// Game state
	gameOver   bool
	gameResult string
}

// NewGame creates a new chess game.
func NewGame() *Game {
	g := &Game{
		position:       board.NewPosition(),
		selectedSquare: board.NoSquare,
		mode:           ModeHumanVsComputer,
		difficulty:     DifficultyMedium,
		renderer:       NewRenderer(BoardSize, SquareSize),
		input:          NewInputHandler(),
		engine:         engine.NewEngine(64), // 64MB hash table
		aiMove:         make(chan board.Move, 1),
	}

	// Set initial engine difficulty
	g.engine.SetDifficulty(engine.Medium)

	g.panel = NewPanel(g)
	g.feedback = NewFeedbackManager()
	g.position.UpdateCheckers()

	return g
}

// Update handles game logic updates.
func (g *Game) Update() error {
	// Update input
	g.input.Update()

	// Update feedback animations
	g.feedback.Update()

	// Handle panel interactions
	if g.panel.HandleInput(g.input) {
		return nil // Panel handled the input
	}

	// Handle board interactions
	g.handleBoardInput()

	// Check for AI move
	g.checkAIMove()

	return nil
}

// Draw renders the game.
func (g *Game) Draw(screen *ebiten.Image) {
	// Clear background
	screen.Fill(g.renderer.Theme().Background)

	// Draw board
	g.renderer.DrawBoard(screen)

	// Draw highlights for check
	if g.position.InCheck() {
		g.renderer.DrawCheck(screen, g.position.KingSquare[g.position.SideToMove])
	}

	// Draw highlights (last move, selection, legal moves)
	g.renderer.DrawHighlights(screen, g.selectedSquare, g.legalMoves, g.lastMove)

	// Draw pieces with shake animations
	g.renderer.DrawPiecesWithAnimations(screen, g.position, g.dragging, g.dragSquare, g.feedback.Animations())

	// Draw dragged piece
	if g.dragging {
		mx, my := g.input.MousePosition()
		g.renderer.DrawDraggedPiece(screen, g.dragPiece, mx, my)
	}

	// Draw feedback overlays (animations, toasts)
	g.feedback.Draw(screen, g.renderer)

	// Draw panel
	g.panel.Draw(screen, g.renderer)
}

// Layout returns the game's screen dimensions.
// Width is dynamic based on panel collapsed state.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if g.panel != nil && g.panel.Collapsed() {
		return BoardSize + CollapsedWidth, ScreenHeight
	}
	return ScreenWidth, ScreenHeight
}

// handleBoardInput processes mouse interactions with the board.
func (g *Game) handleBoardInput() {
	if g.gameOver {
		return
	}

	// Don't allow moves while AI is thinking
	if g.aiThinking {
		return
	}

	// Only allow moves for current player in human vs computer mode
	if g.mode == ModeHumanVsComputer && g.position.SideToMove == board.Black {
		return
	}

	mx, my := g.input.MousePosition()

	// Check if mouse is on the board
	if mx >= BoardSize || my >= BoardSize {
		return
	}

	// Handle mouse press
	if g.input.IsLeftJustPressed() {
		sq := g.renderer.ScreenToSquare(mx, my)
		if sq == board.NoSquare {
			return
		}

		piece := g.position.PieceAt(sq)

		// If clicking on our own piece, select it
		if piece != board.NoPiece && piece.Color() == g.position.SideToMove {
			g.selectSquare(sq)
			g.startDrag(sq)
			return
		}

		// If we have a selection and clicking on a legal move target, make the move
		if g.selectedSquare != board.NoSquare && g.legalMoves != nil {
			move := g.findMove(g.selectedSquare, sq)
			if move != board.NoMove {
				g.makeMove(move)
				return
			}
		}

		// Clear selection
		g.clearSelection()
	}

	// Handle dragging
	if g.dragging {
		// Update drag position (handled in Draw)

		if g.input.IsLeftJustReleased() {
			g.handleDragRelease(mx, my)
		}
	}
}

// selectSquare selects a square and generates legal moves from it.
func (g *Game) selectSquare(sq board.Square) {
	g.selectedSquare = sq
	g.legalMoves = g.getLegalMovesFrom(sq)
}

// clearSelection clears the current selection.
func (g *Game) clearSelection() {
	g.selectedSquare = board.NoSquare
	g.legalMoves = nil
	g.dragging = false
	g.dragPiece = board.NoPiece
	g.dragSquare = board.NoSquare
}

// startDrag begins dragging a piece.
func (g *Game) startDrag(sq board.Square) {
	g.dragging = true
	g.dragPiece = g.position.PieceAt(sq)
	g.dragSquare = sq
}

// handleDragRelease handles releasing a dragged piece.
func (g *Game) handleDragRelease(mx, my int) {
	targetSq := g.renderer.ScreenToSquare(mx, my)

	if targetSq != board.NoSquare && g.legalMoves != nil {
		move := g.findMove(g.dragSquare, targetSq)
		if move != board.NoMove {
			g.makeMove(move)
			return
		}

		// Move was attempted but not valid - determine why and show feedback
		if g.dragSquare != targetSq {
			reason := g.determineInvalidMoveReason(g.dragSquare, targetSq)
			g.feedback.OnInvalidMove(g.dragSquare, targetSq, reason)
		}
	}

	// Invalid drop - clear selection
	g.clearSelection()
}

// determineInvalidMoveReason analyzes why a move from src to dst is invalid.
func (g *Game) determineInvalidMoveReason(src, dst board.Square) InvalidMoveReason {
	piece := g.position.PieceAt(src)
	if piece == board.NoPiece {
		return ReasonUnknown
	}

	// Check if destination has own piece
	destPiece := g.position.PieceAt(dst)
	if destPiece != board.NoPiece && destPiece.Color() == piece.Color() {
		return ReasonBlockedByOwnPiece
	}

	// Check if move exists in pseudo-legal moves (would leave king in check)
	pseudoMoves := g.position.GeneratePseudoLegalMoves()
	for i := 0; i < pseudoMoves.Len(); i++ {
		m := pseudoMoves.Get(i)
		if m.From() == src && m.To() == dst {
			// Move was generated but filtered as illegal - leaves king in check
			return ReasonWouldLeaveKingInCheck
		}
	}

	// Move wasn't even generated - invalid piece movement
	return ReasonInvalidPieceMovement
}

// getLegalMovesFrom returns all legal moves from the given square.
func (g *Game) getLegalMovesFrom(sq board.Square) *board.MoveList {
	fmt.Printf("DEBUG: Getting legal moves from square %v\n", sq)
	fmt.Printf("DEBUG: Piece at square: %v\n", g.position.PieceAt(sq))
	fmt.Printf("DEBUG: Board state:\n%s\n", g.position.String())

	allMoves := g.position.GenerateLegalMoves()
	fmt.Printf("DEBUG: Total legal moves for position: %d\n", allMoves.Len())

	filtered := board.NewMoveList()

	for i := 0; i < allMoves.Len(); i++ {
		move := allMoves.Get(i)
		if move.From() == sq {
			filtered.Add(move)
			fmt.Printf("DEBUG: Found move: %v\n", move)
		}
	}

	fmt.Printf("DEBUG: Filtered moves from %v: %d\n", sq, filtered.Len())
	return filtered
}

// findMove finds a legal move from src to dst.
func (g *Game) findMove(src, dst board.Square) board.Move {
	if g.legalMoves == nil {
		return board.NoMove
	}

	for i := 0; i < g.legalMoves.Len(); i++ {
		move := g.legalMoves.Get(i)
		if move.From() == src && move.To() == dst {
			// TODO: Handle promotion - for now just promote to queen
			if move.IsPromotion() {
				// Find queen promotion
				for j := 0; j < g.legalMoves.Len(); j++ {
					m := g.legalMoves.Get(j)
					if m.From() == src && m.To() == dst && m.Promotion() == board.Queen {
						return m
					}
				}
			}
			return move
		}
	}

	return board.NoMove
}

// makeMove applies a move to the game.
func (g *Game) makeMove(m board.Move) {
	// Determine move properties before making the move
	isCapture := m.IsCapture(g.position)
	isCastling := m.IsCastling()

	// Record SAN before making move
	san := g.moveToSAN(m)
	g.sanHistory = append(g.sanHistory, san)

	// Make the move
	g.position.MakeMove(m)
	g.moveHistory = append(g.moveHistory, m)
	g.lastMove = m

	// Clear selection
	g.clearSelection()

	// Update checkers
	g.position.UpdateCheckers()

	// Play move sound (before checking game end, which may play its own sound)
	g.feedback.OnMoveMade(isCapture, isCastling)

	// Check for game end
	g.checkGameEnd()

	// Start AI thinking if it's computer's turn
	if !g.gameOver && g.mode == ModeHumanVsComputer && g.position.SideToMove == board.Black {
		g.startAIThinking()
	}
}

// moveToSAN converts a move to SAN notation.
func (g *Game) moveToSAN(m board.Move) string {
	return m.ToSAN(g.position)
}

// checkGameEnd checks if the game is over.
func (g *Game) checkGameEnd() {
	if g.position.IsCheckmate() {
		g.gameOver = true
		if g.position.SideToMove == board.White {
			g.gameResult = "Black wins by checkmate!"
			g.feedback.OnCheckmate(board.Black)
		} else {
			g.gameResult = "White wins by checkmate!"
			g.feedback.OnCheckmate(board.White)
		}
	} else if g.position.IsStalemate() {
		g.gameOver = true
		g.gameResult = "Draw by stalemate"
		g.feedback.OnStalemate()
	} else if g.position.HalfMoveClock >= 100 {
		g.gameOver = true
		g.gameResult = "Draw by 50-move rule"
		g.feedback.OnDraw("50-move rule")
	} else if g.position.InCheck() {
		// Show check notification (not game over)
		g.feedback.OnCheck()
	}
}

// startAIThinking starts the AI search in a goroutine.
func (g *Game) startAIThinking() {
	g.aiThinking = true

	// Copy position for the search
	pos := g.position.Copy()

	go func() {
		move := g.engine.Search(pos)
		g.aiMove <- move // Always send, even if NoMove (game over)
	}()
}

// checkAIMove checks if the AI has made a move.
func (g *Game) checkAIMove() {
	if !g.aiThinking {
		return
	}

	select {
	case move := <-g.aiMove:
		g.aiThinking = false
		if move == board.NoMove {
			// AI has no valid move - game should be over (checkmate/stalemate)
			g.checkGameEnd()
			return
		}
		g.makeMove(move)
	default:
		// Still thinking
	}
}

// NewGameAction resets the game to starting position.
func (g *Game) NewGameAction() {
	g.position = board.NewPosition()
	g.moveHistory = nil
	g.sanHistory = nil
	g.lastMove = board.NoMove
	g.clearSelection()
	g.gameOver = false
	g.gameResult = ""
	g.aiThinking = false
	g.position.UpdateCheckers()

	// Clear AI channel
	select {
	case <-g.aiMove:
	default:
	}
}

// ToggleModeAction toggles between Human vs Human and Human vs Computer.
func (g *Game) ToggleModeAction() {
	if g.mode == ModeHumanVsHuman {
		g.mode = ModeHumanVsComputer
	} else {
		g.mode = ModeHumanVsHuman
	}
}

// SetDifficulty sets the AI difficulty.
func (g *Game) SetDifficulty(d Difficulty) {
	g.difficulty = d
	// Map UI difficulty to engine difficulty
	switch d {
	case DifficultyEasy:
		g.engine.SetDifficulty(engine.Easy)
	case DifficultyMedium:
		g.engine.SetDifficulty(engine.Medium)
	case DifficultyHard:
		g.engine.SetDifficulty(engine.Hard)
	}
}

// Position returns the current position.
func (g *Game) Position() *board.Position {
	return g.position
}

// MoveHistory returns the move history.
func (g *Game) MoveHistory() []board.Move {
	return g.moveHistory
}

// SANHistory returns the SAN move history.
func (g *Game) SANHistory() []string {
	return g.sanHistory
}

// GameMode returns the current game mode.
func (g *Game) GameMode() GameMode {
	return g.mode
}

// Difficulty returns the current AI difficulty.
func (g *Game) Difficulty() Difficulty {
	return g.difficulty
}

// GameOver returns true if the game is over.
func (g *Game) GameOver() bool {
	return g.gameOver
}

// GameResult returns the game result string.
func (g *Game) GameResult() string {
	return g.gameResult
}

// IsAIThinking returns true if the AI is currently thinking.
func (g *Game) IsAIThinking() bool {
	return g.aiThinking
}
