// AffineTransform (fully connected) layer.
// Ported from Stockfish src/nnue/layers/affine_transform.h

package layers

import (
	"fmt"
	"io"

	"github.com/hailam/chessplay/sfnnue/common"
)

// AffineTransformHashValue returns the hash value for AffineTransform layer.
// Ported from affine_transform.h:145-151
func AffineTransformHashValue(prevHash uint32, outputDims int) uint32 {
	hashValue := uint32(0xCC03DAE4)
	hashValue += uint32(outputDims)
	hashValue ^= prevHash >> 1
	hashValue ^= prevHash << 31
	return hashValue
}

// AffineTransform represents a fully connected (affine) layer.
// Ported from affine_transform.h:126-307
type AffineTransform struct {
	InputDimensions       int
	OutputDimensions      int
	PaddedInputDimensions int

	// Biases are stored as int32 (BiasType = OutputType = int32)
	Biases []int32

	// Weights are stored as int8
	Weights []int8
}

// NewAffineTransform creates a new affine transform layer
func NewAffineTransform(inputDims, outputDims int) *AffineTransform {
	paddedInput := common.CeilToMultiple(inputDims, common.MaxSimdWidth)

	return &AffineTransform{
		InputDimensions:       inputDims,
		OutputDimensions:      outputDims,
		PaddedInputDimensions: paddedInput,
		Biases:                make([]int32, outputDims),
		Weights:               make([]int8, outputDims*paddedInput),
	}
}

// GetHashValue returns the hash for this layer
func (a *AffineTransform) GetHashValue(prevHash uint32) uint32 {
	return AffineTransformHashValue(prevHash, a.OutputDimensions)
}

// ReadParameters reads layer parameters from a stream.
// Ported from affine_transform.h:167-173
func (a *AffineTransform) ReadParameters(r io.Reader) error {
	// Read biases as int32 (BiasType = OutputType = int32)
	if err := common.ReadLittleEndianSlice(r, a.Biases); err != nil {
		return fmt.Errorf("failed to read biases: %w", err)
	}

	// Read weights as int8 (WeightType = int8)
	weightData := make([]int8, a.OutputDimensions*a.PaddedInputDimensions)
	if err := common.ReadLittleEndianSlice(r, weightData); err != nil {
		return fmt.Errorf("failed to read weights: %w", err)
	}

	// Apply scrambled indexing for SIMD optimization
	for i, w := range weightData {
		idx := a.getWeightIndex(i)
		a.Weights[idx] = w
	}

	return nil
}

// getWeightIndex returns the scrambled weight index for SIMD optimization.
// Ported from affine_transform.h:153-164
func (a *AffineTransform) getWeightIndex(i int) int {
	// Scrambled layout for SIMD: process in chunks of 4
	return (i/4)%(a.PaddedInputDimensions/4)*a.OutputDimensions*4 +
		i/a.PaddedInputDimensions*4 + i%4
}

// Propagate performs the forward pass: output = weights * input + bias
// Input: uint8 slice, Output: int32 slice
// Ported from affine_transform.h:194-299
// Uses SIMD-accelerated dot product when available.
func (a *AffineTransform) Propagate(input []uint8, output []int32) {
	// Matrix multiplication with SIMD-accelerated dot product
	for i := 0; i < a.OutputDimensions; i++ {
		offset := i * a.PaddedInputDimensions
		output[i] = a.Biases[i] + SIMDDotProductInt8Uint8(a.Weights[offset:], input, a.InputDimensions)
	}
}
