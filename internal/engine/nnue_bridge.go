package engine

import (
	"github.com/hailam/chessplay/internal/board"
	"github.com/hailam/chessplay/sfnnue"
	"github.com/hailam/chessplay/sfnnue/features"
)

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

// nnueEvaluate performs NNUE evaluation for the worker's position.
func (w *Worker) nnueEvaluate() int {
	if w.nnueNet == nil || w.nnueAcc == nil {
		return EvaluateWithPawnTable(w.pos, w.pawnTable)
	}

	pieceCount := countPieces(w.pos)

	// Get current accumulators
	bigAcc := w.nnueAcc.CurrentBig()
	smallAcc := w.nnueAcc.CurrentSmall()

	// Recompute accumulators if needed (use pre-allocated buffer for active indices)
	for perspective := 0; perspective < 2; perspective++ {
		if !bigAcc.Computed[perspective] {
			computeAccumulator(w.nnueNet.Big, w.pos, bigAcc, perspective, w.activeIndicesBuffer[:])
		}
		if !smallAcc.Computed[perspective] {
			computeAccumulator(w.nnueNet.Small, w.pos, smallAcc, perspective, w.activeIndicesBuffer[:])
		}
	}

	// Evaluate using both networks
	sideToMove := 0
	if w.pos.SideToMove == board.Black {
		sideToMove = 1
	}

	// Big network evaluation (use pre-allocated transform buffer from accumulator stack)
	bigPsqt, bigPositional := w.nnueNet.Big.Evaluate(
		bigAcc.Accumulation,
		bigAcc.PSQTAccumulation,
		sideToMove,
		pieceCount,
		w.nnueAcc.TransformBuffer[:],
	)

	// Small network evaluation (PSQT only, reuses same transform buffer)
	smallPsqt, _ := w.nnueNet.Small.Evaluate(
		smallAcc.Accumulation,
		smallAcc.PSQTAccumulation,
		sideToMove,
		pieceCount,
		w.nnueAcc.TransformBuffer[:],
	)

	// Combine: use big network's positional + small network's PSQT
	// This matches Stockfish's approach
	score := int(bigPositional) + int(smallPsqt+bigPsqt)/2

	return score
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
func (w *Worker) nnuePush() {
	if w.useNNUE && w.nnueAcc != nil {
		w.nnueAcc.Push()
		// Mark new level as needing recomputation
		bigAcc := w.nnueAcc.CurrentBig()
		smallAcc := w.nnueAcc.CurrentSmall()
		bigAcc.Computed[0] = false
		bigAcc.Computed[1] = false
		smallAcc.Computed[0] = false
		smallAcc.Computed[1] = false
	}
}

// nnuePop restores accumulator state after unmaking a move.
func (w *Worker) nnuePop() {
	if w.useNNUE && w.nnueAcc != nil {
		w.nnueAcc.Pop()
	}
}
