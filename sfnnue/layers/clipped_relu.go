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
func (c *ClippedReLU) Propagate(input []int32, output []uint8) {
	for i := 0; i < c.InputDimensions; i++ {
		// Shift right by WeightScaleBits and clamp to [0, 127]
		val := input[i] >> WeightScaleBits
		if val < 0 {
			val = 0
		} else if val > 127 {
			val = 127
		}
		output[i] = uint8(val)
	}
}
