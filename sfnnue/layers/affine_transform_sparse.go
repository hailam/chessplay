// AffineTransformSparseInput layer for sparse input (first layer after transformer).
// Ported from Stockfish src/nnue/layers/affine_transform_sparse_input.h

package layers

import (
	"fmt"
	"io"

	"github.com/hailam/chessplay/sfnnue/common"
)

// AffineTransformSparseInput represents a fully connected layer optimized for sparse input.
// Used as the first layer after the feature transformer, where many inputs are zero.
// Ported from affine_transform_sparse_input.h:174-371
type AffineTransformSparseInput struct {
	InputDimensions       int
	OutputDimensions      int
	PaddedInputDimensions int

	// Biases are stored as int32
	Biases []int32

	// Weights are stored as int8
	Weights []int8
}

// NewAffineTransformSparseInput creates a new sparse input affine layer
func NewAffineTransformSparseInput(inputDims, outputDims int) *AffineTransformSparseInput {
	paddedInput := common.CeilToMultiple(inputDims, common.MaxSimdWidth)

	return &AffineTransformSparseInput{
		InputDimensions:       inputDims,
		OutputDimensions:      outputDims,
		PaddedInputDimensions: paddedInput,
		Biases:                make([]int32, outputDims),
		Weights:               make([]int8, outputDims*paddedInput),
	}
}

// GetHashValue returns the hash for this layer (same as regular AffineTransform)
// Ported from affine_transform_sparse_input.h:201-208
func (a *AffineTransformSparseInput) GetHashValue(prevHash uint32) uint32 {
	return AffineTransformHashValue(prevHash, a.OutputDimensions)
}

// ReadParameters reads layer parameters from a stream.
// Ported from affine_transform_sparse_input.h:223-230
func (a *AffineTransformSparseInput) ReadParameters(r io.Reader) error {
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

// getWeightIndex returns the scrambled weight index for chunk processing.
// Ported from affine_transform_sparse_input.h:210-221
// ChunkSize is 4 for SSSE3/NEON, 1 otherwise
func (a *AffineTransformSparseInput) getWeightIndex(i int) int {
	const chunkSize = 4 // Assuming SSSE3/NEON
	return (i/chunkSize)%(a.PaddedInputDimensions/chunkSize)*a.OutputDimensions*chunkSize +
		i/a.PaddedInputDimensions*chunkSize + i%chunkSize
}

// Propagate performs the forward pass with sparse input optimization.
// Input: uint8 slice, Output: int32 slice
// Ported from affine_transform_sparse_input.h:250-363
func (a *AffineTransformSparseInput) Propagate(input []uint8, output []int32) {
	// Copy biases to output
	copy(output, a.Biases)

	// Find non-zero input chunks (4 bytes at a time)
	const chunkSize = 4
	numChunks := common.CeilToMultiple(a.InputDimensions, 8) / chunkSize

	// Reinterpret input as int32 for chunk processing
	input32 := make([]int32, (len(input)+3)/4)
	for i := 0; i < len(input); i++ {
		input32[i/4] |= int32(input[i]) << (8 * (i % 4))
	}

	// Find non-zero chunks
	nnzIndices := make([]int, 0, numChunks)
	for i := 0; i < numChunks; i++ {
		if input32[i] != 0 {
			nnzIndices = append(nnzIndices, i)
		}
	}

	// Process only non-zero chunks
	for _, idx := range nnzIndices {
		in := input32[idx]
		// Unpack the 4 bytes
		b0 := uint8(in)
		b1 := uint8(in >> 8)
		b2 := uint8(in >> 16)
		b3 := uint8(in >> 24)

		colOffset := idx * a.OutputDimensions * chunkSize

		for k := 0; k < a.OutputDimensions; k++ {
			weightOffset := colOffset + k*chunkSize
			output[k] += int32(a.Weights[weightOffset+0]) * int32(b0)
			output[k] += int32(a.Weights[weightOffset+1]) * int32(b1)
			output[k] += int32(a.Weights[weightOffset+2]) * int32(b2)
			output[k] += int32(a.Weights[weightOffset+3]) * int32(b3)
		}
	}
}
