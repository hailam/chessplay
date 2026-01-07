package engine

import (
	"github.com/hailam/chessplay/internal/board"
	"github.com/hailam/chessplay/sfnnue"
	"github.com/hailam/chessplay/sfnnue/features"
)

// DirtyPiece tracks a piece change for incremental accumulator updates.
// FromSq = -1 means piece was added (not moved from anywhere).
// ToSq = -1 means piece was removed (captured).
type DirtyPiece struct {
	Piece  int // sfnnue piece encoding (1-14)
	FromSq int // source square (-1 if added)
	ToSq   int // destination square (-1 if removed)
}

// MaxDirtyPieces is the maximum number of dirty pieces per move.
// Normal move: 1, capture: 2, en passant: 2, promotion+capture: 3
const MaxDirtyPieces = 3

// DirtyState tracks piece changes for incremental NNUE updates.
type DirtyState struct {
	Pieces     [MaxDirtyPieces]DirtyPiece
	Count      int
	KingMoved  [2]bool // Whether king moved for each perspective
	KingSq     [2]int  // King squares after move
	Computed   bool    // Whether dirty state has been computed
}

// sfnnuePieceTable maps [color][pieceType] to sfnnue piece encoding.
// board types: Pawn=0, Knight=1, Bishop=2, Rook=3, Queen=4, King=5
// sfnnue types: W_PAWN=1, W_KNIGHT=2, ..., B_PAWN=9, B_KNIGHT=10, ...
var sfnnuePieceTable = [2][6]int{
	{1, 2, 3, 4, 5, 6},    // White: W_PAWN=1, W_KNIGHT=2, etc.
	{9, 10, 11, 12, 13, 14}, // Black: B_PAWN=9, B_KNIGHT=10, etc.
}

// appendActiveIndicesDirect computes active feature indices directly from board.Position,
// avoiding interface dispatch and adapter allocation.
// This is the hot path optimization for P3.
func appendActiveIndicesDirect(perspective int, pos *board.Position, active *features.IndexList) {
	ksq := int(pos.KingSquare[perspective])

	// Iterate through all piece types and colors directly via bitboards.
	// This avoids:
	// 1. Interface dispatch overhead
	// 2. The O(6) search in PieceAt() for each square
	// 3. Adapter allocation
	for c := 0; c < 2; c++ {
		for pt := board.Pawn; pt <= board.King; pt++ {
			sfPiece := sfnnuePieceTable[c][pt]
			bb := uint64(pos.Pieces[c][pt])

			// Process all squares with this piece type
			for bb != 0 {
				// Pop LSB
				sq := trailingZeros64(bb)
				bb &= bb - 1

				// Compute feature index and add to list
				active.Push(features.MakeIndex(perspective, sq, sfPiece, ksq))
			}
		}
	}
}

// trailingZeros64 returns the number of trailing zero bits in x.
// This is a hot path function, optimized for common cases.
func trailingZeros64(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	if x&0xFFFFFFFF == 0 {
		n += 32
		x >>= 32
	}
	if x&0xFFFF == 0 {
		n += 16
		x >>= 16
	}
	if x&0xFF == 0 {
		n += 8
		x >>= 8
	}
	if x&0xF == 0 {
		n += 4
		x >>= 4
	}
	if x&0x3 == 0 {
		n += 2
		x >>= 2
	}
	if x&0x1 == 0 {
		n++
	}
	return n
}

// countPieces returns the total number of pieces on the board.
func countPieces(pos *board.Position) int {
	count := 0
	bb := pos.AllOccupied
	for bb != 0 {
		bb &= bb - 1
		count++
	}
	return count
}

// computeDirtyPieces computes NNUE feature changes for a move.
// Must be called BEFORE MakeMove while position still has original state.
// Returns true if incremental update is possible (no king moves for either perspective).
func (w *Worker) computeDirtyPieces(m board.Move) bool {
	if !w.useNNUE || w.nnueAcc == nil {
		return false
	}

	// Reset dirty state
	w.dirtyState.Count = 0
	w.dirtyState.KingMoved[0] = false
	w.dirtyState.KingMoved[1] = false
	w.dirtyState.Computed = false

	pos := w.pos
	from := m.From()
	to := m.To()
	movingPiece := pos.PieceAt(from)

	if movingPiece == board.NoPiece {
		return false
	}

	us := int(movingPiece.Color())
	pt := movingPiece.Type()
	sfPiece := sfnnuePieceTable[us][pt]

	// Store current king squares (before the move)
	w.dirtyState.KingSq[0] = int(pos.KingSquare[board.White])
	w.dirtyState.KingSq[1] = int(pos.KingSquare[board.Black])

	// Check for king move - requires full refresh for that perspective
	if pt == board.King {
		w.dirtyState.KingMoved[us] = true
		// Update king square for after the move
		w.dirtyState.KingSq[us] = int(to)
		w.dirtyState.Computed = true
		return false // Can't do incremental for king moves
	}

	// Handle castling - king is moving
	if m.IsCastling() {
		w.dirtyState.KingMoved[us] = true
		w.dirtyState.KingSq[us] = int(to)
		w.dirtyState.Computed = true
		return false
	}

	// Record the moving piece: removed from 'from', added to 'to'
	w.dirtyState.Pieces[w.dirtyState.Count] = DirtyPiece{
		Piece:  sfPiece,
		FromSq: int(from),
		ToSq:   int(to),
	}
	w.dirtyState.Count++

	// Handle captures
	if m.IsEnPassant() {
		// En passant: captured pawn is on a different square
		var capturedSq board.Square
		if us == int(board.White) {
			capturedSq = to - 8
		} else {
			capturedSq = to + 8
		}
		capturedColor := 1 - us
		capturedSfPiece := sfnnuePieceTable[capturedColor][board.Pawn]
		w.dirtyState.Pieces[w.dirtyState.Count] = DirtyPiece{
			Piece:  capturedSfPiece,
			FromSq: int(capturedSq),
			ToSq:   -1, // Removed
		}
		w.dirtyState.Count++
	} else {
		// Regular capture
		capturedPiece := pos.PieceAt(to)
		if capturedPiece != board.NoPiece {
			capturedColor := int(capturedPiece.Color())
			capturedPt := capturedPiece.Type()
			capturedSfPiece := sfnnuePieceTable[capturedColor][capturedPt]
			w.dirtyState.Pieces[w.dirtyState.Count] = DirtyPiece{
				Piece:  capturedSfPiece,
				FromSq: int(to),
				ToSq:   -1, // Removed
			}
			w.dirtyState.Count++
		}
	}

	// Handle promotions
	if m.IsPromotion() {
		promoPt := m.Promotion()
		promoSfPiece := sfnnuePieceTable[us][promoPt]

		// The pawn move was already recorded, but we need to fix it:
		// Pawn is removed from 'from', promoted piece appears at 'to'
		// So change the first dirty piece to be pawn removed, add promoted piece added
		w.dirtyState.Pieces[0] = DirtyPiece{
			Piece:  sfPiece, // Pawn
			FromSq: int(from),
			ToSq:   -1, // Removed
		}
		// Add promoted piece
		w.dirtyState.Pieces[w.dirtyState.Count] = DirtyPiece{
			Piece:  promoSfPiece,
			FromSq: -1, // Added (not moved from anywhere)
			ToSq:   int(to),
		}
		w.dirtyState.Count++
	}

	w.dirtyState.Computed = true
	return true
}

// computeFeatureDeltas computes removed and added feature indices for incremental update.
// Returns slices into pre-allocated buffers.
func (w *Worker) computeFeatureDeltas(perspective, ksq int) (removed, added []int) {
	// Use activeIndicesBuffer split in half: first 32 for removed, second 32 for added
	removedBuf := w.activeIndicesBuffer[0:32]
	addedBuf := w.activeIndicesBuffer[32:64]
	removedCount := 0
	addedCount := 0

	for i := 0; i < w.dirtyState.Count; i++ {
		dp := &w.dirtyState.Pieces[i]

		if dp.FromSq >= 0 {
			// Piece removed from FromSq
			idx := features.MakeIndex(perspective, dp.FromSq, dp.Piece, ksq)
			removedBuf[removedCount] = idx
			removedCount++
		}

		if dp.ToSq >= 0 {
			// Piece added to ToSq
			idx := features.MakeIndex(perspective, dp.ToSq, dp.Piece, ksq)
			addedBuf[addedCount] = idx
			addedCount++
		}
	}

	return removedBuf[:removedCount], addedBuf[:addedCount]
}

// simpleEval returns the absolute material advantage for network selection.
// Stockfish uses this to decide small vs big network (threshold 962).
func (w *Worker) simpleEval() int {
	pos := w.pos
	score := 0
	// Pawn=100, Knight=320, Bishop=330, Rook=500, Queen=900
	pieceValues := [6]int{100, 320, 330, 500, 900, 0}

	for pt := board.Pawn; pt <= board.Queen; pt++ {
		whitePieces := popCount64(uint64(pos.Pieces[board.White][pt]))
		blackPieces := popCount64(uint64(pos.Pieces[board.Black][pt]))
		score += (whitePieces - blackPieces) * pieceValues[pt]
	}

	if pos.SideToMove == board.Black {
		score = -score
	}
	return absInt(score)
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func popCount64(x uint64) int {
	count := 0
	for x != 0 {
		x &= x - 1
		count++
	}
	return count
}

// ensureAccumulatorComputed updates or recomputes the accumulator for the given network.
func (w *Worker) ensureAccumulatorComputed(net *sfnnue.Network, acc *sfnnue.Accumulator, isSmall bool) {
	var prevAcc *sfnnue.Accumulator
	if isSmall {
		prevAcc = w.nnueAcc.PreviousSmall()
	} else {
		prevAcc = w.nnueAcc.PreviousBig()
	}

	for perspective := 0; perspective < 2; perspective++ {
		if acc.Computed[perspective] {
			continue
		}

		// Check if we can do incremental update
		canIncremental := prevAcc != nil &&
			prevAcc.Computed[perspective] &&
			!acc.NeedsRefresh[perspective] &&
			w.dirtyState.Computed && w.dirtyState.Count > 0

		if canIncremental {
			ksq := int(w.pos.KingSquare[perspective])
			removed, added := w.computeFeatureDeltas(perspective, ksq)

			net.FeatureTransformer.UpdateAccumulator(
				removed, added,
				acc.Accumulation[perspective],
				acc.PSQTAccumulation[perspective],
			)
			acc.Computed[perspective] = true
			acc.KingSq[perspective] = ksq
		} else {
			// Full recomputation required
			computeAccumulator(net, w.pos, acc, perspective, w.activeIndicesBuffer[:])
		}
	}
}

// nnueEvaluate performs NNUE evaluation for the worker's position.
// Uses dual-network evaluation for better accuracy (working approach from Jan 5).
// Adds optimism tracking for Stockfish-style score adjustments.
func (w *Worker) nnueEvaluate() int {
	if w.nnueNet == nil || w.nnueAcc == nil {
		return EvaluateWithPawnTable(w.pos, w.pawnTable)
	}

	pieceCount := countPieces(w.pos)
	sideToMove := 0
	if w.pos.SideToMove == board.Black {
		sideToMove = 1
	}

	// Get accumulators for both networks
	bigAcc := w.nnueAcc.CurrentBig()
	smallAcc := w.nnueAcc.CurrentSmall()

	// Ensure accumulators are computed for both networks
	w.ensureAccumulatorComputed(w.nnueNet.Big, bigAcc, false)
	w.ensureAccumulatorComputed(w.nnueNet.Small, smallAcc, true)

	// Big network evaluation
	bigPsqt, bigPositional := w.nnueNet.Big.Evaluate(
		bigAcc.Accumulation,
		bigAcc.PSQTAccumulation,
		sideToMove,
		pieceCount,
		w.nnueAcc.TransformBuffer[:],
	)

	// Small network evaluation (PSQT only - used for averaging)
	smallPsqt, _ := w.nnueNet.Small.Evaluate(
		smallAcc.Accumulation,
		smallAcc.PSQTAccumulation,
		sideToMove,
		pieceCount,
		w.nnueAcc.TransformBuffer[:],
	)

	// Combine: use big network's positional + averaged PSQT from both networks
	// This is the working approach from Jan 5 that beat Stockfish level 3
	score := int(bigPositional) + int(smallPsqt+bigPsqt)/2

	// Get optimism for side to move (Stockfish evaluate.cpp)
	optimism := w.optimism[sideToMove]

	// Material-based score adjustment with optimism (simplified Stockfish formula)
	// This adds a small optimism bonus scaled by material
	pawnCount := popCount64(uint64(w.pos.Pieces[board.White][board.Pawn])) +
		popCount64(uint64(w.pos.Pieces[board.Black][board.Pawn]))
	material := 534*pawnCount + nonPawnMaterial(w.pos)

	// Scale optimism by material (similar to Stockfish but with working base formula)
	// optimism * (7191 + material) / 77871 adds a small optimism-based adjustment
	score += optimism * (7191 + material) / 77871

	// Rule50 dampening
	rule50 := int(w.pos.HalfMoveClock)
	score -= score * rule50 / 199

	return score
}

// nonPawnMaterial calculates the total material value excluding pawns.
// Used for material scaling in NNUE evaluation.
func nonPawnMaterial(pos *board.Position) int {
	// Knight=320, Bishop=330, Rook=500, Queen=900
	pieceValues := [6]int{0, 320, 330, 500, 900, 0}
	total := 0
	for c := 0; c < 2; c++ {
		for pt := board.Knight; pt <= board.Queen; pt++ {
			total += popCount64(uint64(pos.Pieces[c][pt])) * pieceValues[pt]
		}
	}
	return total
}

// computeAccumulator computes the accumulator from scratch for a perspective.
// indexBuffer is a pre-allocated buffer to avoid allocation per call.
// Uses direct bitboard iteration to avoid interface dispatch overhead (P3 optimization).
func computeAccumulator(net *sfnnue.Network, pos *board.Position, acc *sfnnue.Accumulator, perspective int, indexBuffer []int) {
	// Get active feature indices using direct function (avoids interface dispatch)
	var activeList features.IndexList
	appendActiveIndicesDirect(perspective, pos, &activeList)

	// Use pre-allocated buffer (slice to actual size)
	activeIndices := indexBuffer[:activeList.Size]
	for i := 0; i < activeList.Size; i++ {
		activeIndices[i] = activeList.Values[i]
	}

	// Compute accumulator
	net.FeatureTransformer.ComputeAccumulator(
		activeIndices,
		acc.Accumulation[perspective],
		acc.PSQTAccumulation[perspective],
	)

	// Mark as computed
	acc.Computed[perspective] = true
	acc.KingSq[perspective] = int(pos.KingSquare[perspective])
}

// resetNNUEAccumulators marks accumulators as needing recomputation.
func (w *Worker) resetNNUEAccumulators() {
	if w.nnueAcc != nil {
		w.nnueAcc.Reset()
	}
}

// nnuePush saves accumulator state before making a move.
// The dirty pieces should already be computed via computeDirtyPieces().
// Push() copies parent accumulators to current level.
// We only set NeedsRefresh for perspectives where the king moved.
func (w *Worker) nnuePush() {
	if w.useNNUE && w.nnueAcc != nil {
		w.nnueAcc.Push()

		// Get current accumulators (copied from parent by Push)
		bigAcc := w.nnueAcc.CurrentBig()
		smallAcc := w.nnueAcc.CurrentSmall()

		// Set NeedsRefresh based on dirty state
		// If dirty state not computed (null move or edge case), require full refresh
		if !w.dirtyState.Computed {
			bigAcc.NeedsRefresh[0] = true
			bigAcc.NeedsRefresh[1] = true
			smallAcc.NeedsRefresh[0] = true
			smallAcc.NeedsRefresh[1] = true
			// Also mark as not computed to force full recomputation
			bigAcc.Computed[0] = false
			bigAcc.Computed[1] = false
			smallAcc.Computed[0] = false
			smallAcc.Computed[1] = false
		} else {
			// Only set NeedsRefresh for perspectives where king moved
			for p := 0; p < 2; p++ {
				if w.dirtyState.KingMoved[p] {
					bigAcc.NeedsRefresh[p] = true
					smallAcc.NeedsRefresh[p] = true
					// King moved - mark as not computed for this perspective
					bigAcc.Computed[p] = false
					smallAcc.Computed[p] = false
				} else {
					// No king move - can do incremental update
					bigAcc.NeedsRefresh[p] = false
					smallAcc.NeedsRefresh[p] = false
					// Mark as NOT computed - the values are inherited from parent
					// but need incremental update to be valid for this position
					bigAcc.Computed[p] = false
					smallAcc.Computed[p] = false
				}
			}
		}
	}
}

// nnuePop restores accumulator state after unmaking a move.
func (w *Worker) nnuePop() {
	if w.useNNUE && w.nnueAcc != nil {
		w.nnueAcc.Pop()
	}
}
