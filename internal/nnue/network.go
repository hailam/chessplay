package nnue

import "github.com/hailam/chessplay/internal/board"

// Network holds the NNUE weights.
type Network struct {
	// Layer 1: HalfKPSize -> L1Size (per perspective)
	// Weights are quantized as int16
	L1Weights [HalfKPSize][L1Size]int16
	L1Bias    [L1Size]int16

	// Layer 2: L1Size*2 (both perspectives) -> L2Size
	L2Weights [L1Size * 2][L2Size]int8
	L2Bias    [L2Size]int32

	// Output layer: L2Size -> 1
	OutputWeights [L2Size]int8
	OutputBias    int32
}

// NewNetwork creates a network with zero weights (must load weights or init random).
func NewNetwork() *Network {
	return &Network{}
}

// Forward computes the network output given an accumulator.
// Returns evaluation in centipawns from the perspective of the side to move.
func (n *Network) Forward(acc *Accumulator, sideToMove board.Color) int {
	// Select perspective ordering - side to move comes first
	var stmAcc, nstmAcc *[L1Size]int16
	if sideToMove == board.White {
		stmAcc = &acc.White
		nstmAcc = &acc.Black
	} else {
		stmAcc = &acc.Black
		nstmAcc = &acc.White
	}

	// Layer 1 output: apply clipped ReLU to accumulated values
	// First half is side to move, second half is opponent
	var l1Out [L1Size * 2]int8
	for i := 0; i < L1Size; i++ {
		l1Out[i] = ClampedReLU(stmAcc[i])
		l1Out[L1Size+i] = ClampedReLU(nstmAcc[i])
	}

	// Layer 2: matrix multiply + bias + clipped ReLU
	var l2Out [L2Size]int8
	for i := 0; i < L2Size; i++ {
		sum := n.L2Bias[i]
		for j := 0; j < L1Size*2; j++ {
			sum += int32(l1Out[j]) * int32(n.L2Weights[j][i])
		}
		// Scale down and apply clipped ReLU
		scaled := int16(sum >> L1QuantShift)
		l2Out[i] = ClampedReLU(scaled)
	}

	// Output layer
	var output int32 = n.OutputBias
	for i := 0; i < L2Size; i++ {
		output += int32(l2Out[i]) * int32(n.OutputWeights[i])
	}

	// Scale to centipawns
	return int(output * OutputScale >> (L2QuantShift + 8))
}

// InitRandom initializes weights with small random values (for testing only).
func (n *Network) InitRandom(seed int64) {
	// Use a simple LCG for reproducibility
	state := uint64(seed)
	next := func() int16 {
		state = state*6364136223846793005 + 1442695040888963407
		return int16((state >> 48) & 0xFF) - 128 // Small random values -128 to 127
	}

	// L1 weights (very small to avoid overflow)
	for i := 0; i < HalfKPSize; i++ {
		for j := 0; j < L1Size; j++ {
			n.L1Weights[i][j] = next() >> 5 // Very small: -4 to 3
		}
	}

	// L1 bias
	for i := 0; i < L1Size; i++ {
		n.L1Bias[i] = next() >> 3 // Small: -16 to 15
	}

	// L2 weights
	for i := 0; i < L1Size*2; i++ {
		for j := 0; j < L2Size; j++ {
			val := next() >> 6 // Very small
			if val > 127 {
				val = 127
			} else if val < -128 {
				val = -128
			}
			n.L2Weights[i][j] = int8(val)
		}
	}

	// L2 bias
	for i := 0; i < L2Size; i++ {
		n.L2Bias[i] = int32(next())
	}

	// Output weights
	for i := 0; i < L2Size; i++ {
		val := next() >> 6
		if val > 127 {
			val = 127
		} else if val < -128 {
			val = -128
		}
		n.OutputWeights[i] = int8(val)
	}

	// Output bias
	n.OutputBias = int32(next()) * 100 // Centered around zero
}
