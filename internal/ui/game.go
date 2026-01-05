package ui

import (
	"fmt"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hailam/chessplay/internal/board"
	"github.com/hailam/chessplay/internal/engine"
	"github.com/hailam/chessplay/internal/storage"
)

// UI Constants
const (
	ScreenWidth  = 960
	ScreenHeight = 640 // Match board height to eliminate unused space
	BoardSize    = 640
	SquareSize   = BoardSize / 8
	PanelWidth   = ScreenWidth - BoardSize
)

// UIScale is the global HiDPI scale factor for all UI drawing.
// Set by Game.Layout() and used by widgets and modals.
var UIScale float64 = 1.0

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

// EvalMode represents the evaluation engine mode.
type EvalMode int

const (
	EvalClassical EvalMode = iota
	EvalNNUE
)

// AssistResult holds the analysis result for Easy mode hints.
type AssistResult struct {
	Evaluation int        // Centipawn score
	BestMove   board.Move // Suggested move
}

// Game implements ebiten.Game interface.
type Game struct {
	// Core game state
	position       *board.Position
	moveHistory    []board.Move
	sanHistory     []string
	positionHashes []uint64 // History of position hashes for repetition detection

	// UI state
	selectedSquare board.Square
	legalMoves     *board.MoveList
	dragging       bool
	dragPiece      board.Piece
	dragSquare     board.Square
	lastMove       board.Move

	// Game settings
	mode        GameMode
	difficulty  Difficulty
	evalMode    EvalMode
	username    string
	playerColor board.Color // Which color the human plays (default: White)

	// Storage
	storage *storage.Storage
	prefs   *storage.UserPreferences

	// Components
	renderer *Renderer
	input    *InputHandler
	panel    *Panel
	feedback *FeedbackManager

	// Modals
	settingsModal *SettingsModal
	welcomeScreen *WelcomeScreen
	downloader    *Downloader

	// Visual effects
	glass *GlassEffect

	// AI Engine
	engine     *engine.Engine
	aiThinking bool
	aiMove     chan board.Move

	// Easy mode assistance
	assistResult  *AssistResult
	assistRunning bool
	assistCh      chan *AssistResult
	showHints     bool // Toggle for hint visibility

	// Game state
	gameOver   bool
	gameResult string

	// HiDPI scaling
	scale float64
}

// NewGame creates a new chess game.
func NewGame() *Game {
	g := &Game{
		position:       board.NewPosition(),
		selectedSquare: board.NoSquare,
		mode:           ModeHumanVsComputer,
		difficulty:     DifficultyMedium,
		evalMode:       EvalClassical,
		username:       "Player",
		playerColor:    board.White, // Human plays White by default
		renderer:       NewRenderer(BoardSize, SquareSize),
		input:          NewInputHandler(),
		engine:         engine.NewEngine(64), // 64MB hash table
		aiMove:         make(chan board.Move, 1),
		assistCh:       make(chan *AssistResult, 1),
		showHints:      true, // Enable hints by default in Easy mode
	}

	// Initialize storage
	var err error
	g.storage, err = storage.NewStorage()
	if err != nil {
		log.Printf("Warning: Failed to initialize storage: %v", err)
	}

	// Load preferences
	g.loadPreferences()

	// Set initial engine difficulty
	g.engine.SetDifficulty(engine.Medium)

	g.panel = NewPanel(g)
	g.feedback = NewFeedbackManager()
	g.glass = NewGlassEffect()

	// Initialize modals
	g.settingsModal = NewSettingsModal()
	g.welcomeScreen = NewWelcomeScreen()
	g.downloader = NewDownloader()

	g.position.UpdateCheckers()

	// Initialize position hash history with starting position
	g.positionHashes = []uint64{g.position.Hash}

	// Check for first launch
	g.checkFirstLaunch()

	return g
}

// loadPreferences loads user preferences from storage.
func (g *Game) loadPreferences() {
	if g.storage == nil {
		g.prefs = storage.DefaultPreferences()
		return
	}

	var err error
	g.prefs, err = g.storage.LoadPreferences()
	if err != nil {
		log.Printf("Warning: Failed to load preferences: %v", err)
		g.prefs = storage.DefaultPreferences()
	}

	// Apply preferences
	g.username = g.prefs.Username
	g.difficulty = Difficulty(g.prefs.Difficulty)
	g.evalMode = EvalMode(g.prefs.EvalMode)
	g.mode = GameMode(g.prefs.GameMode)

	// Apply player color (convert from storage.PlayerColor to board.Color)
	if g.prefs.PlayerColor == storage.ColorBlack {
		g.playerColor = board.Black
		g.renderer.SetFlipped(true)
	} else {
		g.playerColor = board.White
		g.renderer.SetFlipped(false)
	}

	// Update engine difficulty
	switch g.difficulty {
	case DifficultyEasy:
		g.engine.SetDifficulty(engine.Easy)
	case DifficultyMedium:
		g.engine.SetDifficulty(engine.Medium)
	case DifficultyHard:
		g.engine.SetDifficulty(engine.Hard)
	}

	// Load NNUE networks if eval mode is NNUE and networks exist
	if g.evalMode == EvalNNUE {
		smallExists, bigExists, _ := CheckNNUENetworks()
		if smallExists && bigExists {
			g.loadNNUENetworks()
		}
	}
}

// savePreferences saves current preferences to storage.
func (g *Game) savePreferences() {
	if g.storage == nil {
		return
	}

	g.prefs.Username = g.username
	g.prefs.Difficulty = storage.Difficulty(g.difficulty)
	g.prefs.EvalMode = storage.EvalMode(g.evalMode)
	g.prefs.GameMode = storage.GameMode(g.mode)

	// Convert board.Color to storage.PlayerColor
	if g.playerColor == board.Black {
		g.prefs.PlayerColor = storage.ColorBlack
	} else {
		g.prefs.PlayerColor = storage.ColorWhite
	}

	if err := g.storage.SavePreferences(g.prefs); err != nil {
		log.Printf("Warning: Failed to save preferences: %v", err)
	}
}

// checkFirstLaunch shows welcome screen on first launch.
func (g *Game) checkFirstLaunch() {
	if g.storage == nil {
		return
	}

	isFirst, err := g.storage.IsFirstLaunch()
	if err != nil {
		log.Printf("Warning: Failed to check first launch: %v", err)
		return
	}

	if isFirst {
		g.welcomeScreen.Show(func(name string, evalMode storage.EvalMode) {
			g.username = name
			g.prefs.Username = name
			g.prefs.EvalMode = evalMode

			if err := g.storage.MarkFirstLaunchComplete(); err != nil {
				log.Printf("Warning: Failed to mark first launch complete: %v", err)
			}

			// If NNUE selected, check if we need to download
			if evalMode == storage.EvalNNUE {
				smallExists, bigExists, err := CheckNNUENetworks()
				if err != nil || !smallExists || !bigExists {
					g.savePreferences()
					g.showNNUEDownload()
					return
				}
			}

			g.setEvalMode(EvalMode(evalMode))
			g.savePreferences()
		})
	}
}

// Update handles game logic updates.
func (g *Game) Update() error {
	// Update input
	g.input.Update()

	// Update feedback animations
	g.feedback.Update()

	// Update glass effect animation
	g.glass.Update()

	// Handle welcome screen first (blocks other input)
	if g.welcomeScreen.IsVisible() {
		g.welcomeScreen.Update(g.input)
		g.updateCursor()
		return nil
	}

	// Handle downloader (blocks other input)
	if g.downloader.IsVisible() {
		g.downloader.Update(g.input)
		g.updateCursor()
		return nil
	}

	// Handle settings modal (blocks other input)
	if g.settingsModal.IsVisible() {
		g.settingsModal.Update(g.input)
		g.updateCursor()
		return nil
	}

	// Handle panel interactions
	if g.panel.HandleInput(g.input) {
		g.updateCursor()
		return nil // Panel handled the input
	}

	// Handle board interactions
	g.handleBoardInput()

	// Check for AI move
	g.checkAIMove()

	// Check for assist analysis result (Easy mode)
	g.checkAssistResult()

	// Start assist analysis if it's user's turn in Easy mode
	g.startAssistAnalysis()

	// Update cursor based on hover state
	g.updateCursor()

	return nil
}

// updateCursor sets the cursor shape based on what's being hovered.
func (g *Game) updateCursor() {
	anyHovered := false

	// Check all interactive elements
	if g.welcomeScreen.IsVisible() {
		anyHovered = g.welcomeScreen.AnyButtonHovered()
	} else if g.settingsModal.IsVisible() {
		anyHovered = g.settingsModal.AnyButtonHovered()
	} else {
		anyHovered = g.panel.AnyButtonHovered()
	}

	if anyHovered {
		ebiten.SetCursorShape(ebiten.CursorShapePointer)
	} else {
		ebiten.SetCursorShape(ebiten.CursorShapeDefault)
	}
}

// Draw renders the game.
func (g *Game) Draw(screen *ebiten.Image) {
	// Set HiDPI scale factor for all rendering components
	g.renderer.SetScale(g.scale)
	g.panel.SetScale(g.scale)

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

	// Draw hint arrow (Easy mode only)
	if g.difficulty == DifficultyEasy && g.showHints && g.assistResult != nil {
		g.renderer.DrawHintArrow(screen, g.assistResult.BestMove.From(), g.assistResult.BestMove.To())
	}

	// Draw pieces with shake animations
	g.renderer.DrawPiecesWithAnimations(screen, g.position, g.dragging, g.dragSquare, g.feedback.Animations())

	// Draw dragged piece
	if g.dragging {
		mx, my := g.input.MousePosition()
		g.renderer.DrawDraggedPiece(screen, g.dragPiece, mx, my)
	}

	// Draw feedback overlays (animations, toasts)
	g.feedback.Draw(screen, g.renderer, g.glass)

	// Draw panel
	g.panel.Draw(screen, g.renderer, g.glass)

	// Draw modals on top (with glass effect)
	g.settingsModal.Draw(screen, g.glass)
	g.downloader.Draw(screen, g.glass)
	g.welcomeScreen.Draw(screen, g.glass)
}

// Layout returns the game's screen dimensions.
// Width is dynamic based on panel collapsed state.
// Uses device scale factor for crisp rendering on HiDPI displays.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	// Get and store device scale factor (2.0 on Retina, 1.0 on standard displays)
	g.scale = ebiten.Monitor().DeviceScaleFactor()
	if g.scale < 1.0 {
		g.scale = 1.0 // Ensure minimum scale of 1.0
	}

	// Update global scale for widgets and modals
	UIScale = g.scale

	if g.panel != nil && g.panel.Collapsed() {
		return int(float64(BoardSize+CollapsedWidth) * g.scale), int(float64(ScreenHeight) * g.scale)
	}
	return int(float64(ScreenWidth) * g.scale), int(float64(ScreenHeight) * g.scale)
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

	// Only allow moves for human player in human vs computer mode
	if g.mode == ModeHumanVsComputer && g.position.SideToMove != g.playerColor {
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

		// Handle castling: allow dragging King to Rook square
		// Users naturally castle by moving King to Rook, but internal moves use King's destination
		if move.IsCastling() && move.From() == src {
			// Kingside: E1→H1 (White) or E8→H8 (Black) should match E1→G1 / E8→G8
			if (src == board.E1 && dst == board.H1 && move.To() == board.G1) ||
				(src == board.E8 && dst == board.H8 && move.To() == board.G8) {
				return move
			}
			// Queenside: E1→A1 (White) or E8→A8 (Black) should match E1→C1 / E8→C8
			if (src == board.E1 && dst == board.A1 && move.To() == board.C1) ||
				(src == board.E8 && dst == board.A8 && move.To() == board.C8) {
				return move
			}
		}
	}

	return board.NoMove
}

// makeMove applies a move to the game.
func (g *Game) makeMove(m board.Move) {
	// Debug logging - before move
	log.Printf("[MOVE] Before: SideToMove=%v, Move=%v (from=%v to=%v)",
		g.position.SideToMove, m, m.From(), m.To())
	log.Printf("[MOVE] Piece at from=%v: %v", m.From(), g.position.PieceAt(m.From()))

	// Determine move properties before making the move
	isCapture := m.IsCapture(g.position)
	isCastling := m.IsCastling()

	// Record SAN before making move
	san := g.moveToSAN(m)
	g.sanHistory = append(g.sanHistory, san)

	// Make the move
	g.position.MakeMove(m)

	// Debug logging - after move
	log.Printf("[MOVE] After: SideToMove=%v", g.position.SideToMove)
	g.moveHistory = append(g.moveHistory, m)
	g.lastMove = m

	// Record position hash for repetition detection
	g.positionHashes = append(g.positionHashes, g.position.Hash)

	// Clear selection
	g.clearSelection()

	// Clear assist (will re-analyze after AI moves)
	g.clearAssist()

	// Update checkers
	g.position.UpdateCheckers()

	// Play move sound (before checking game end, which may play its own sound)
	g.feedback.OnMoveMade(isCapture, isCastling)

	// Check for game end
	g.checkGameEnd()

	// Start AI thinking if it's computer's turn
	if !g.gameOver && g.mode == ModeHumanVsComputer && g.position.SideToMove != g.playerColor {
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
	} else if g.isThreefoldRepetition() {
		g.gameOver = true
		g.gameResult = "Draw by threefold repetition"
		g.feedback.OnDraw("threefold repetition")
	} else if g.position.HalfMoveClock >= 100 {
		g.gameOver = true
		g.gameResult = "Draw by 50-move rule"
		g.feedback.OnDraw("50-move rule")
	} else if g.position.InCheck() {
		// Show check notification (not game over)
		g.feedback.OnCheck()
	}
}

// isThreefoldRepetition checks if the current position has occurred 3 times.
func (g *Game) isThreefoldRepetition() bool {
	if len(g.positionHashes) < 5 {
		// Need at least 5 positions (4 half-moves) for threefold repetition
		return false
	}

	currentHash := g.position.Hash
	count := 0
	matchIndices := []int{}

	for i, h := range g.positionHashes {
		if h == currentHash {
			count++
			matchIndices = append(matchIndices, i)
			if count >= 3 {
				// Debug logging to diagnose false positives
				log.Printf("[THREEFOLD] Detected! Hash=%x Count=%d MatchIndices=%v TotalPositions=%d",
					currentHash, count, matchIndices, len(g.positionHashes))
				log.Printf("[THREEFOLD] Current FEN: %s", g.position.ToFEN())
				return true
			}
		}
	}
	return false
}

// startAIThinking starts the AI search in a goroutine.
func (g *Game) startAIThinking() {
	// Assertion: AI should only think when it's computer's turn
	if g.position.SideToMove == g.playerColor {
		log.Printf("ERROR: startAIThinking called but SideToMove is %v (player's turn)!",
			g.position.SideToMove)
		return
	}

	log.Printf("[AI] Starting AI search - SideToMove=%v", g.position.SideToMove)
	g.aiThinking = true

	// Copy position for the search
	pos := g.position.Copy()

	// Pass position history for repetition detection
	g.engine.SetPositionHistory(g.positionHashes)

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
		log.Printf("[AI] Received move from engine: %v (from=%v to=%v)", move, move.From(), move.To())
		log.Printf("[AI] Current position SideToMove: %v", g.position.SideToMove)
		g.aiThinking = false
		if move == board.NoMove {
			// AI has no valid move - game should be over (checkmate/stalemate)
			log.Printf("[AI] No valid move - checking game end")
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
	g.positionHashes = []uint64{g.position.Hash} // Reset with starting position
	g.lastMove = board.NoMove
	g.clearSelection()
	g.clearAssist()
	g.gameOver = false
	g.gameResult = ""
	g.aiThinking = false
	g.position.UpdateCheckers()

	// Clear AI channel
	select {
	case <-g.aiMove:
	default:
	}

	// If player chose Black, AI (White) moves first
	if g.mode == ModeHumanVsComputer && g.playerColor == board.Black {
		g.startAIThinking()
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

// SetPlayerColor sets which color the human player controls.
// When set to Black, the board will be flipped and AI will move first.
func (g *Game) SetPlayerColor(color board.Color) {
	g.playerColor = color
	// Flip board so player's pieces are at the bottom
	g.renderer.SetFlipped(color == board.Black)
}

// PlayerColor returns the color the human player controls.
func (g *Game) PlayerColor() board.Color {
	return g.playerColor
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

// Username returns the current username.
func (g *Game) Username() string {
	return g.username
}

// EvalMode returns the current evaluation mode.
func (g *Game) EvalMode() EvalMode {
	return g.evalMode
}

// ShowSettings opens the settings modal.
func (g *Game) ShowSettings() {
	g.settingsModal.Show(g.prefs, func(prefs *storage.UserPreferences) {
		// Apply all preferences immediately
		g.username = prefs.Username
		g.SetDifficulty(Difficulty(prefs.Difficulty))
		g.prefs.SoundEnabled = prefs.SoundEnabled
		g.prefs.Username = prefs.Username
		g.prefs.Difficulty = prefs.Difficulty
		g.prefs.EvalMode = prefs.EvalMode
		g.prefs.PlayerColor = prefs.PlayerColor

		// Apply player color (convert from storage.PlayerColor to board.Color)
		if prefs.PlayerColor == storage.ColorBlack {
			g.SetPlayerColor(board.Black)
		} else {
			g.SetPlayerColor(board.White)
		}

		// Handle NNUE mode - check if networks need downloading
		if prefs.EvalMode == storage.EvalNNUE {
			smallExists, bigExists, _ := CheckNNUENetworks()
			if !smallExists || !bigExists {
				// Networks missing - save prefs directly (don't use savePreferences which
				// overwrites EvalMode with g.evalMode), then start download
				if g.storage != nil {
					g.storage.SavePreferences(g.prefs)
				}
				g.showNNUEDownload()
				return
			}
		}

		// Update eval mode (either Classical, or NNUE with files ready)
		g.setEvalMode(EvalMode(prefs.EvalMode))
		g.savePreferences()
	}, nil)
}

// showNNUEDownload shows the NNUE download dialog.
func (g *Game) showNNUEDownload() {
	g.downloader.Show(func() {
		// Download complete - update eval mode and save
		g.setEvalMode(EvalNNUE)
		g.savePreferences()
		log.Printf("NNUE networks downloaded successfully")
	}, func() {
		// Download cancelled - revert to classical
		g.setEvalMode(EvalClassical)
		g.prefs.EvalMode = storage.EvalClassical
		g.savePreferences()
	})
}

// loadNNUENetworks loads NNUE network files into the engine.
func (g *Game) loadNNUENetworks() {
	smallPath, bigPath, err := GetNNUEPaths()
	if err != nil {
		log.Printf("Warning: Failed to get NNUE paths: %v", err)
		return
	}

	if err := g.engine.LoadNNUE(bigPath, smallPath); err != nil {
		log.Printf("Warning: Failed to load NNUE networks: %v", err)
		return
	}

	g.engine.SetUseNNUE(true)
	log.Printf("NNUE networks loaded successfully")
}

// setEvalMode sets the evaluation mode and updates the engine.
func (g *Game) setEvalMode(mode EvalMode) {
	g.evalMode = mode
	if mode == EvalNNUE {
		if g.engine.HasNNUE() {
			g.engine.SetUseNNUE(true)
		} else {
			g.loadNNUENetworks()
		}
	} else {
		g.engine.SetUseNNUE(false)
	}
}

// Close cleans up game resources.
func (g *Game) Close() {
	if g.storage != nil {
		g.storage.Close()
	}
}

// startAssistAnalysis starts background analysis for Easy mode hints.
// Only runs when it's the user's turn and difficulty is Easy.
func (g *Game) startAssistAnalysis() {
	// Only in Easy mode
	if g.difficulty != DifficultyEasy {
		return
	}
	// Only when it's human's turn in HvC mode
	if g.mode == ModeHumanVsComputer && g.position.SideToMove != g.playerColor {
		return
	}
	// Don't run if game is over or AI is thinking
	if g.gameOver || g.aiThinking {
		return
	}
	// Don't run if already have a result (wait until move is made)
	if g.assistResult != nil {
		return
	}
	// Don't run if already analyzing
	if g.assistRunning {
		return
	}

	log.Printf("[Assist] Starting analysis for Easy mode hint")
	g.assistRunning = true

	go func() {
		// Quick search to find best move
		limits := engine.SearchLimits{
			Depth:    5,
			MoveTime: 500 * time.Millisecond,
		}
		pos := g.position.Copy()
		bestMove := g.engine.SearchWithLimits(pos, limits)
		eval := g.engine.Evaluate(pos)

		g.assistCh <- &AssistResult{
			Evaluation: eval,
			BestMove:   bestMove,
		}
	}()
}

// checkAssistResult checks for completed assist analysis.
func (g *Game) checkAssistResult() {
	if !g.assistRunning {
		return
	}

	select {
	case result := <-g.assistCh:
		g.assistRunning = false
		g.assistResult = result
		log.Printf("[Assist] Result received: eval=%d, move=%v", result.Evaluation, result.BestMove)
	default:
		// Still analyzing
	}
}

// clearAssist clears the current assist result.
func (g *Game) clearAssist() {
	g.assistResult = nil
	g.assistRunning = false
	// Drain channel if anything pending
	select {
	case <-g.assistCh:
	default:
	}
}
