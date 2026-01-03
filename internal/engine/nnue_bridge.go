package engine

import (
	"github.com/hailam/chessplay/internal/board"
	"github.com/hailam/chessplay/sfnnue"
	"github.com/hailam/chessplay/sfnnue/features"
)

// positionAdapter wraps board.Position to implement sfnnue's features.Position interface.
type positionAdapter struct {
	pos *board.Position
}

// KingSquare returns the king square for the given color.
func (p *positionAdapter) KingSquare(color int) int {
	return int(p.pos.KingSquare[color])
}

// PieceOn returns the piece at the given square in sfnnue format.
// sfnnue piece format: W_PAWN=1, W_KNIGHT=2, ..., B_PAWN=9, B_KNIGHT=10, ...
func (p *positionAdapter) PieceOn(sq int) int {
	piece := p.pos.PieceAt(board.Square(sq))
	if piece == board.NoPiece {
		return features.NO_PIECE
	}

	// board.Piece encoding: color in upper bits, type in lower bits
	pieceType := piece.Type()
	pieceColor := piece.Color()

	// Convert to sfnnue format: color * 8 + type
	// board types: Pawn=0, Knight=1, Bishop=2, Rook=3, Queen=4, King=5
	// sfnnue types: PAWN=1, KNIGHT=2, BISHOP=3, ROOK=4, QUEEN=5, KING=6
	sfType := int(pieceType) + 1

	if pieceColor == board.White {
		return sfType // W_PAWN=1, W_KNIGHT=2, etc.
	}
	return sfType + 8 // B_PAWN=9, B_KNIGHT=10, etc.
}

// Pieces returns a bitboard of all pieces on the board.
func (p *positionAdapter) Pieces() uint64 {
	return uint64(p.pos.AllOccupied)
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

	adapter := &positionAdapter{pos: w.pos}
	pieceCount := countPieces(w.pos)

	// Get current accumulators
	bigAcc := w.nnueAcc.CurrentBig()
	smallAcc := w.nnueAcc.CurrentSmall()

	// Recompute accumulators if needed
	for perspective := 0; perspective < 2; perspective++ {
		if !bigAcc.Computed[perspective] {
			computeAccumulator(w.nnueNet.Big, adapter, bigAcc, perspective)
		}
		if !smallAcc.Computed[perspective] {
			computeAccumulator(w.nnueNet.Small, adapter, smallAcc, perspective)
		}
	}

	// Evaluate using both networks
	sideToMove := 0
	if w.pos.SideToMove == board.Black {
		sideToMove = 1
	}

	// Big network evaluation
	bigPsqt, bigPositional := w.nnueNet.Big.Evaluate(
		bigAcc.Accumulation,
		bigAcc.PSQTAccumulation,
		sideToMove,
		pieceCount,
	)

	// Small network evaluation (PSQT only)
	smallPsqt, _ := w.nnueNet.Small.Evaluate(
		smallAcc.Accumulation,
		smallAcc.PSQTAccumulation,
		sideToMove,
		pieceCount,
	)

	// Combine: use big network's positional + small network's PSQT
	// This matches Stockfish's approach
	score := int(bigPositional) + int(smallPsqt+bigPsqt)/2

	return score
}

// computeAccumulator computes the accumulator from scratch for a perspective.
func computeAccumulator(net *sfnnue.Network, adapter *positionAdapter, acc *sfnnue.Accumulator, perspective int) {
	// Get active feature indices
	var activeList features.IndexList
	features.AppendActiveIndices(perspective, adapter, &activeList)

	// Convert to slice
	activeIndices := make([]int, activeList.Size)
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
	acc.KingSq[perspective] = adapter.KingSquare(perspective)
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
