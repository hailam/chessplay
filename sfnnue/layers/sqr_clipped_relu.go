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
// Optimized with loop unrolling for better performance.
func (s *SqrClippedReLU) Propagate(input []int32, output []uint8) {
	// Shift amount: 2 * WeightScaleBits + 7 = 2 * 6 + 7 = 19
	const shift = 2*WeightScaleBits + 7
	n := s.InputDimensions

	// Unroll by 4 for better performance
	i := 0
	for ; i+4 <= n; i += 4 {
		// Square and shift
		v0 := int64(input[i]) * int64(input[i]) >> shift
		v1 := int64(input[i+1]) * int64(input[i+1]) >> shift
		v2 := int64(input[i+2]) * int64(input[i+2]) >> shift
		v3 := int64(input[i+3]) * int64(input[i+3]) >> shift

		// Clamp to 127
		if v0 > 127 {
			v0 = 127
		}
		if v1 > 127 {
			v1 = 127
		}
		if v2 > 127 {
			v2 = 127
		}
		if v3 > 127 {
			v3 = 127
		}

		output[i] = uint8(v0)
		output[i+1] = uint8(v1)
		output[i+2] = uint8(v2)
		output[i+3] = uint8(v3)
	}

	// Handle remaining elements
	for ; i < n; i++ {
		val := int64(input[i]) * int64(input[i]) >> shift
		if val > 127 {
			val = 127
		}
		output[i] = uint8(val)
	}
}
