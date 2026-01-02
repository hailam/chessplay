package engine

import (
	"github.com/hailam/chessplay/internal/board"
)

// Move ordering priorities
const (
	TTMoveScore     = 10000000 // TT move gets highest priority
	GoodCaptureBase = 1000000  // Base score for good captures
	KillerScore1    = 900000   // First killer move
	KillerScore2    = 800000   // Second killer move
	BadCaptureBase  = -100000  // Losing captures
)

// MVV-LVA (Most Valuable Victim - Least Valuable Attacker) scores
// Higher score = search first
// Score = victimValue * 10 - attackerValue
var mvvLva = [6][6]int{
	//       P    N    B    R    Q    K  (attacker)
	/* P */ {15, 14, 14, 13, 12, 11}, // Pawn victim
	/* N */ {25, 24, 24, 23, 22, 21}, // Knight victim
	/* B */ {35, 34, 34, 33, 32, 31}, // Bishop victim
	/* R */ {45, 44, 44, 43, 42, 41}, // Rook victim
	/* Q */ {55, 54, 54, 53, 52, 51}, // Queen victim
	/* K */ {0, 0, 0, 0, 0, 0},       // King can't be captured
}

// MoveOrderer handles move ordering for the search.
type MoveOrderer struct {
	// Killer moves (quiet moves that caused beta cutoffs)
	killers [MaxPly][2]board.Move

	// History heuristic (indexed by [from][to])
	history [64][64]int

	// Counter move heuristic (indexed by [piece][to])
	counterMoves [12][64]board.Move

	// Capture history (indexed by [attackerPiece][toSquare][capturedPieceType])
	captureHistory [12][64][6]int

	// Countermove history (indexed by [prevPiece][prevTo][movePiece][moveTo])
	countermoveHistory [12][64][12][64]int
}

// NewMoveOrderer creates a new move orderer.
func NewMoveOrderer() *MoveOrderer {
	return &MoveOrderer{}
}

// Clear resets the move orderer for a new search.
func (mo *MoveOrderer) Clear() {
	// Clear killers
	for i := range mo.killers {
		mo.killers[i][0] = board.NoMove
		mo.killers[i][1] = board.NoMove
	}

	// Age history scores (divide by 2 to prevent overflow)
	for i := range mo.history {
		for j := range mo.history[i] {
			mo.history[i][j] /= 2
		}
	}

	// Clear counter moves
	for i := range mo.counterMoves {
		for j := range mo.counterMoves[i] {
			mo.counterMoves[i][j] = board.NoMove
		}
	}

	// Age capture history
	for i := range mo.captureHistory {
		for j := range mo.captureHistory[i] {
			for k := range mo.captureHistory[i][j] {
				mo.captureHistory[i][j][k] /= 2
			}
		}
	}

	// Age countermove history
	for i := range mo.countermoveHistory {
		for j := range mo.countermoveHistory[i] {
			for k := range mo.countermoveHistory[i][j] {
				for l := range mo.countermoveHistory[i][j][k] {
					mo.countermoveHistory[i][j][k][l] /= 2
				}
			}
		}
	}
}

// ScoreMoves assigns scores to moves for ordering.
func (mo *MoveOrderer) ScoreMoves(pos *board.Position, moves *board.MoveList, ply int, ttMove board.Move) []int {
	scores := make([]int, moves.Len())

	for i := 0; i < moves.Len(); i++ {
		move := moves.Get(i)
		scores[i] = mo.scoreMove(pos, move, ply, ttMove)
	}

	return scores
}

// ScoreMovesWithCounter assigns scores including counter-move and CMH bonus.
func (mo *MoveOrderer) ScoreMovesWithCounter(pos *board.Position, moves *board.MoveList, ply int, ttMove, prevMove board.Move) []int {
	scores := make([]int, moves.Len())
	counterMove := mo.GetCounterMove(prevMove, pos)

	// Get previous piece for CMH lookup
	var prevPiece board.Piece = board.NoPiece
	if prevMove != board.NoMove {
		prevPiece = pos.PieceAt(prevMove.To())
	}

	for i := 0; i < moves.Len(); i++ {
		move := moves.Get(i)
		scores[i] = mo.scoreMove(pos, move, ply, ttMove)

		// Counter-move bonus (after killers, before history)
		if move == counterMove && scores[i] < KillerScore2 {
			scores[i] = KillerScore2 - 10000 // Just below second killer
		}

		// Add countermove history bonus for quiet moves
		if !move.IsCapture(pos) && !move.IsPromotion() && move != ttMove {
			movePiece := pos.PieceAt(move.From())
			cmhScore := mo.GetCountermoveHistoryScore(prevMove, prevPiece, movePiece, move.To())
			scores[i] += cmhScore / 2 // Scale down to not dominate
		}
	}

	return scores
}

// scoreMove returns the ordering score for a single move.
func (mo *MoveOrderer) scoreMove(pos *board.Position, m board.Move, ply int, ttMove board.Move) int {
	// TT move gets highest priority
	if m == ttMove {
		return TTMoveScore
	}

	from := m.From()
	to := m.To()

	// Captures: MVV-LVA
	if m.IsCapture(pos) {
		attackerPiece := pos.PieceAt(from)
		if attackerPiece == board.NoPiece {
			return GoodCaptureBase // Safety check
		}
		attacker := attackerPiece.Type()

		var victim board.PieceType
		if m.IsEnPassant() {
			victim = board.Pawn
		} else {
			capturedPiece := pos.PieceAt(to)
			if capturedPiece == board.NoPiece {
				// Safety check - shouldn't happen but prevents panic
				return GoodCaptureBase
			}
			victim = capturedPiece.Type()
		}

		// Bounds check for safety (victim should be < King for captures)
		if victim >= board.King || attacker > board.King {
			return GoodCaptureBase
		}

		// Check if it's a winning capture using MVV-LVA
		score := GoodCaptureBase + mvvLva[victim][attacker]*1000

		// Add capture history bonus
		captureHistScore := mo.GetCaptureHistoryScore(attackerPiece, to, victim)
		score += captureHistScore / 4 // Scale appropriately

		// Bonus for capturing with a less valuable piece
		if pieceValues[attacker] < pieceValues[victim] {
			score += 10000 // Clearly winning capture
		}

		return score
	}

	// Promotions (non-capture)
	if m.IsPromotion() {
		return GoodCaptureBase - 1000 + int(m.Promotion())*100
	}

	// Killer moves
	if m == mo.killers[ply][0] {
		return KillerScore1
	}
	if m == mo.killers[ply][1] {
		return KillerScore2
	}

	// History heuristic for quiet moves
	return mo.history[from][to]
}

// SortMoves sorts moves by their scores (descending).
func SortMoves(moves *board.MoveList, scores []int) {
	// Simple selection sort (sufficient for ~40 moves)
	n := moves.Len()
	for i := 0; i < n-1; i++ {
		best := i
		for j := i + 1; j < n; j++ {
			if scores[j] > scores[best] {
				best = j
			}
		}
		if best != i {
			// Swap moves
			moves.Swap(i, best)
			// Swap scores
			scores[i], scores[best] = scores[best], scores[i]
		}
	}
}

// PickMove selects the best remaining move and moves it to position index.
// This allows lazy move sorting (only sort as much as needed).
func PickMove(moves *board.MoveList, scores []int, index int) {
	best := index
	for j := index + 1; j < moves.Len(); j++ {
		if scores[j] > scores[best] {
			best = j
		}
	}
	if best != index {
		moves.Swap(index, best)
		scores[index], scores[best] = scores[best], scores[index]
	}
}

// UpdateKillers adds a killer move at the given ply.
func (mo *MoveOrderer) UpdateKillers(m board.Move, ply int) {
	// Don't store captures as killers
	if ply >= MaxPly {
		return
	}

	// Don't store if it's already the first killer
	if mo.killers[ply][0] == m {
		return
	}

	// Shift killers
	mo.killers[ply][1] = mo.killers[ply][0]
	mo.killers[ply][0] = m
}

// UpdateHistory updates the history score for a move.
func (mo *MoveOrderer) UpdateHistory(m board.Move, depth int, isGood bool) {
	from := m.From()
	to := m.To()

	bonus := depth * depth
	if isGood {
		mo.history[from][to] += bonus
		// Prevent overflow
		if mo.history[from][to] > 400000 {
			// Scale down all history scores
			for i := range mo.history {
				for j := range mo.history[i] {
					mo.history[i][j] /= 2
				}
			}
		}
	} else {
		mo.history[from][to] -= bonus
		if mo.history[from][to] < -400000 {
			mo.history[from][to] = -400000
		}
	}
}

// UpdateCounterMove updates the counter move table.
func (mo *MoveOrderer) UpdateCounterMove(prevMove, counterMove board.Move, pos *board.Position) {
	if prevMove == board.NoMove {
		return
	}

	piece := pos.PieceAt(prevMove.To())
	if piece == board.NoPiece {
		return
	}

	mo.counterMoves[piece][prevMove.To()] = counterMove
}

// GetCounterMove returns the counter move for a previous move.
func (mo *MoveOrderer) GetCounterMove(prevMove board.Move, pos *board.Position) board.Move {
	if prevMove == board.NoMove {
		return board.NoMove
	}

	piece := pos.PieceAt(prevMove.To())
	if piece == board.NoPiece {
		return board.NoMove
	}

	return mo.counterMoves[piece][prevMove.To()]
}

// GetHistoryScore returns the history score for a move.
// Used for history pruning in search.
func (mo *MoveOrderer) GetHistoryScore(m board.Move) int {
	return mo.history[m.From()][m.To()]
}

// UpdateCaptureHistory updates the capture history for a move.
func (mo *MoveOrderer) UpdateCaptureHistory(attackerPiece board.Piece, toSq board.Square, capturedType board.PieceType, depth int, isGood bool) {
	if attackerPiece == board.NoPiece || capturedType >= board.King {
		return
	}

	bonus := depth * depth
	if isGood {
		mo.captureHistory[attackerPiece][toSq][capturedType] += bonus
		if mo.captureHistory[attackerPiece][toSq][capturedType] > 400000 {
			mo.scaleCaptureHistory()
		}
	} else {
		mo.captureHistory[attackerPiece][toSq][capturedType] -= bonus
		if mo.captureHistory[attackerPiece][toSq][capturedType] < -400000 {
			mo.captureHistory[attackerPiece][toSq][capturedType] = -400000
		}
	}
}

func (mo *MoveOrderer) scaleCaptureHistory() {
	for i := range mo.captureHistory {
		for j := range mo.captureHistory[i] {
			for k := range mo.captureHistory[i][j] {
				mo.captureHistory[i][j][k] /= 2
			}
		}
	}
}

// GetCaptureHistoryScore returns the capture history score for a capture move.
func (mo *MoveOrderer) GetCaptureHistoryScore(attackerPiece board.Piece, toSq board.Square, capturedType board.PieceType) int {
	if attackerPiece == board.NoPiece || capturedType >= board.King {
		return 0
	}
	return mo.captureHistory[attackerPiece][toSq][capturedType]
}

// UpdateCountermoveHistory updates the countermove history for a quiet move.
func (mo *MoveOrderer) UpdateCountermoveHistory(prevMove, goodMove board.Move, prevPiece, movePiece board.Piece, depth int, isGood bool) {
	if prevMove == board.NoMove || prevPiece == board.NoPiece || movePiece == board.NoPiece {
		return
	}

	prevTo := prevMove.To()
	moveTo := goodMove.To()
	bonus := depth * depth

	if isGood {
		mo.countermoveHistory[prevPiece][prevTo][movePiece][moveTo] += bonus
		if mo.countermoveHistory[prevPiece][prevTo][movePiece][moveTo] > 400000 {
			mo.scaleCountermoveHistory()
		}
	} else {
		mo.countermoveHistory[prevPiece][prevTo][movePiece][moveTo] -= bonus
		if mo.countermoveHistory[prevPiece][prevTo][movePiece][moveTo] < -400000 {
			mo.countermoveHistory[prevPiece][prevTo][movePiece][moveTo] = -400000
		}
	}
}

func (mo *MoveOrderer) scaleCountermoveHistory() {
	for i := range mo.countermoveHistory {
		for j := range mo.countermoveHistory[i] {
			for k := range mo.countermoveHistory[i][j] {
				for l := range mo.countermoveHistory[i][j][k] {
					mo.countermoveHistory[i][j][k][l] /= 2
				}
			}
		}
	}
}

// GetCountermoveHistoryScore returns the CMH score for a move given the previous move.
func (mo *MoveOrderer) GetCountermoveHistoryScore(prevMove board.Move, prevPiece, movePiece board.Piece, moveTo board.Square) int {
	if prevMove == board.NoMove || prevPiece == board.NoPiece || movePiece == board.NoPiece {
		return 0
	}
	return mo.countermoveHistory[prevPiece][prevMove.To()][movePiece][moveTo]
}
