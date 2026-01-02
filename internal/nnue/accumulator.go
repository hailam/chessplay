package nnue

import "github.com/hailam/chessplay/internal/board"

// Accumulator stores the accumulated hidden layer values for incremental updates.
// Each side has its own accumulator from its perspective.
type Accumulator struct {
	// Hidden layer values for white and black perspectives
	// Stored as int16 for quantized arithmetic
	White [L1Size]int16
	Black [L1Size]int16

	// Track if accumulator is computed
	Computed bool
}

// AccumulatorStack manages accumulators during search.
type AccumulatorStack struct {
	stack [128]Accumulator // One per ply
	top   int
}

// NewAccumulatorStack creates a new accumulator stack.
func NewAccumulatorStack() *AccumulatorStack {
	return &AccumulatorStack{}
}

// Push saves current accumulator state.
func (s *AccumulatorStack) Push() {
	if s.top < 127 {
		s.stack[s.top+1] = s.stack[s.top]
		s.top++
	}
}

// Pop restores previous accumulator state.
func (s *AccumulatorStack) Pop() {
	if s.top > 0 {
		s.top--
	}
}

// Current returns the current accumulator.
func (s *AccumulatorStack) Current() *Accumulator {
	return &s.stack[s.top]
}

// Reset resets the stack to initial state.
func (s *AccumulatorStack) Reset() {
	s.top = 0
	s.stack[0].Computed = false
}

// ComputeFull computes the accumulator from scratch for a position.
func (acc *Accumulator) ComputeFull(pos *board.Position, net *Network) {
	// Get active features
	whiteFeatures, blackFeatures := GetActiveFeatures(pos)

	// Start with bias
	copy(acc.White[:], net.L1Bias[:])
	copy(acc.Black[:], net.L1Bias[:])

	// Add active feature weights
	for _, idx := range whiteFeatures {
		if idx >= 0 && idx < HalfKPSize {
			for i := 0; i < L1Size; i++ {
				acc.White[i] += net.L1Weights[idx][i]
			}
		}
	}

	for _, idx := range blackFeatures {
		if idx >= 0 && idx < HalfKPSize {
			for i := 0; i < L1Size; i++ {
				acc.Black[i] += net.L1Weights[idx][i]
			}
		}
	}

	acc.Computed = true
}

// UpdateIncremental updates the accumulator incrementally for a move.
// This is the key efficiency optimization - O(changed pieces) instead of O(all pieces).
// Should be called AFTER the move has been made on the position.
func (acc *Accumulator) UpdateIncremental(pos *board.Position, m board.Move, captured board.Piece, net *Network) {
	if !acc.Computed {
		acc.ComputeFull(pos, net)
		return
	}

	movedPiece := pos.PieceAt(m.To())
	if movedPiece == board.NoPiece {
		// Invalid state, recompute
		acc.Computed = false
		return
	}

	// King moves require full recomputation (king square changed)
	if movedPiece.Type() == board.King {
		acc.ComputeFull(pos, net)
		return
	}

	// Get changed features
	whiteAdd, whiteRem, blackAdd, blackRem := GetChangedFeatures(pos, m, captured)

	// Apply removals
	for _, idx := range whiteRem {
		if idx >= 0 && idx < HalfKPSize {
			for i := 0; i < L1Size; i++ {
				acc.White[i] -= net.L1Weights[idx][i]
			}
		}
	}
	for _, idx := range blackRem {
		if idx >= 0 && idx < HalfKPSize {
			for i := 0; i < L1Size; i++ {
				acc.Black[i] -= net.L1Weights[idx][i]
			}
		}
	}

	// Apply additions
	for _, idx := range whiteAdd {
		if idx >= 0 && idx < HalfKPSize {
			for i := 0; i < L1Size; i++ {
				acc.White[i] += net.L1Weights[idx][i]
			}
		}
	}
	for _, idx := range blackAdd {
		if idx >= 0 && idx < HalfKPSize {
			for i := 0; i < L1Size; i++ {
				acc.Black[i] += net.L1Weights[idx][i]
			}
		}
	}
}
