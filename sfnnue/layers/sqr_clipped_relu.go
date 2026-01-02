// SqrClippedReLU (Squared Clipped ReLU) activation layer.
// Ported from Stockfish src/nnue/layers/sqr_clipped_relu.h

package layers

// SqrClippedReLU applies squared clipped ReLU activation.
// Output = min(127, (input² >> (2 * WeightScaleBits + 7)))
// Ported from sqr_clipped_relu.h:33-105

// SqrClippedReLUHashValue returns the hash value (same as ClippedReLU)
// Ported from sqr_clipped_relu.h:49-53
func SqrClippedReLUHashValue(prevHash uint32) uint32 {
	return 0x538D24C7 + prevHash
}

// SqrClippedReLU represents a squared clipped ReLU activation layer
type SqrClippedReLU struct {
	InputDimensions  int
	OutputDimensions int
}

// NewSqrClippedReLU creates a new SqrClippedReLU layer
func NewSqrClippedReLU(dims int) *SqrClippedReLU {
	return &SqrClippedReLU{
		InputDimensions:  dims,
		OutputDimensions: dims,
	}
}

// GetHashValue returns the hash for this layer type
func (s *SqrClippedReLU) GetHashValue(prevHash uint32) uint32 {
	return SqrClippedReLUHashValue(prevHash)
}

// ReadParameters reads layer parameters (none for SqrClippedReLU)
func (s *SqrClippedReLU) ReadParameters() error {
	return nil
}

// Propagate applies the squared clipped ReLU activation.
// Input: int32 slice, Output: uint8 slice
// Ported from sqr_clipped_relu.h:68-104
func (s *SqrClippedReLU) Propagate(input []int32, output []uint8) {
	// Shift amount: 2 * WeightScaleBits + 7 = 2 * 6 + 7 = 19
	const shift = 2*WeightScaleBits + 7

	for i := 0; i < s.InputDimensions; i++ {
		// Square the input value
		val := int64(input[i]) * int64(input[i])
		// Shift right and clamp to 127
		result := val >> shift
		if result > 127 {
			result = 127
		}
		output[i] = uint8(result)
	}
}
