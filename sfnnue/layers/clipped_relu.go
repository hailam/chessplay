// ClippedReLU activation layer.
// Ported from Stockfish src/nnue/layers/clipped_relu.h

package layers

// ClippedReLU applies clipped ReLU activation: clamp(x >> WeightScaleBits, 0, 127)
// Ported from clipped_relu.h:33-166

const (
	// WeightScaleBits is the number of bits to shift (clipped_relu.h uses nnue_common.h:62)
	WeightScaleBits = 6
)

// ClippedReLUHashValue returns the hash value for ClippedReLU layer (clipped_relu.h:49-53)
func ClippedReLUHashValue(prevHash uint32) uint32 {
	return 0x538D24C7 + prevHash
}

// ClippedReLU represents a ClippedReLU activation layer
type ClippedReLU struct {
	InputDimensions  int
	OutputDimensions int
}

// NewClippedReLU creates a new ClippedReLU layer
func NewClippedReLU(dims int) *ClippedReLU {
	return &ClippedReLU{
		InputDimensions:  dims,
		OutputDimensions: dims,
	}
}

// GetHashValue returns the hash for this layer type
func (c *ClippedReLU) GetHashValue(prevHash uint32) uint32 {
	return ClippedReLUHashValue(prevHash)
}

// ReadParameters reads layer parameters (none for ClippedReLU)
func (c *ClippedReLU) ReadParameters() error {
	return nil
}

// Propagate applies the ClippedReLU activation.
// Input: int32 slice, Output: uint8 slice
// Ported from clipped_relu.h:68-165
// Optimized with loop unrolling for better performance.
func (c *ClippedReLU) Propagate(input []int32, output []uint8) {
	n := c.InputDimensions

	// Unroll by 4 for better performance
	i := 0
	for ; i+4 <= n; i += 4 {
		// Process 4 elements at a time
		v0 := input[i] >> WeightScaleBits
		v1 := input[i+1] >> WeightScaleBits
		v2 := input[i+2] >> WeightScaleBits
		v3 := input[i+3] >> WeightScaleBits

		// Clamp to [0, 127]
		if v0 < 0 {
			v0 = 0
		} else if v0 > 127 {
			v0 = 127
		}
		if v1 < 0 {
			v1 = 0
		} else if v1 > 127 {
			v1 = 127
		}
		if v2 < 0 {
			v2 = 0
		} else if v2 > 127 {
			v2 = 127
		}
		if v3 < 0 {
			v3 = 0
		} else if v3 > 127 {
			v3 = 127
		}

		output[i] = uint8(v0)
		output[i+1] = uint8(v1)
		output[i+2] = uint8(v2)
		output[i+3] = uint8(v3)
	}

	// Handle remaining elements
	for ; i < n; i++ {
		val := input[i] >> WeightScaleBits
		if val < 0 {
			val = 0
		} else if val > 127 {
			val = 127
		}
		output[i] = uint8(val)
	}
}
