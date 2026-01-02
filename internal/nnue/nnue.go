// Package nnue implements NNUE (Efficiently Updatable Neural Network) evaluation.
package nnue

import "github.com/hailam/chessplay/internal/board"

// Network architecture constants
const (
	// HalfKP feature dimensions
	NumKingSquares  = 64
	NumPieceTypes   = 10 // P, N, B, R, Q for both colors (excluding kings)
	NumPieceSquares = 64

	// Total input features per perspective
	// King square * (piece_type * piece_square)
	HalfKPSize = NumKingSquares * NumPieceTypes * NumPieceSquares // 40960

	// Network dimensions
	L1Size     = 256 // First hidden layer (per perspective, so 512 total)
	L2Size     = 32  // Second hidden layer
	OutputSize = 1   // Single output value

	// Quantization constants
	InputQuantShift = 6   // Input weights scaled by 2^6 = 64
	L1QuantShift    = 6   // L1 output scaled by 2^6
	L2QuantShift    = 6   // L2 output scaled by 2^6
	OutputScale     = 600 // Final scale to centipawns
)

// ClampedReLU clamps value to [0, 127] for quantized inference.
func ClampedReLU(x int16) int8 {
	if x < 0 {
		return 0
	}
	if x > 127 {
		return 127
	}
	return int8(x)
}

// Evaluator is the main NNUE evaluator interface.
type Evaluator struct {
	net   *Network
	stack *AccumulatorStack
}

// NewEvaluator creates a new NNUE evaluator.
// If weightsFile is empty, uses random weights for testing.
func NewEvaluator(weightsFile string) (*Evaluator, error) {
	net := NewNetwork()

	if weightsFile != "" {
		if err := net.LoadWeights(weightsFile); err != nil {
			return nil, err
		}
	} else {
		net.InitRandom(12345) // For testing only
	}

	return &Evaluator{
		net:   net,
		stack: NewAccumulatorStack(),
	}, nil
}

// Evaluate returns NNUE evaluation for the position.
// Returns score in centipawns from side to move's perspective.
func (e *Evaluator) Evaluate(pos *board.Position) int {
	acc := e.stack.Current()
	if !acc.Computed {
		acc.ComputeFull(pos, e.net)
	}
	return e.net.Forward(acc, pos.SideToMove)
}

// Push saves accumulator state (call before MakeMove).
func (e *Evaluator) Push() {
	e.stack.Push()
}

// Pop restores accumulator state (call after UnmakeMove).
func (e *Evaluator) Pop() {
	e.stack.Pop()
}

// Refresh forces a full recomputation of the accumulator.
func (e *Evaluator) Refresh(pos *board.Position) {
	e.stack.Current().ComputeFull(pos, e.net)
}

// Update updates the accumulator incrementally for a move.
// Should be called after MakeMove.
func (e *Evaluator) Update(pos *board.Position, m board.Move, captured board.Piece) {
	e.stack.Current().UpdateIncremental(pos, m, captured, e.net)
}

// Reset resets the accumulator stack (for new game).
func (e *Evaluator) Reset() {
	e.stack.Reset()
}
