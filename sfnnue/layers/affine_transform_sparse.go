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

	// Use stack-allocated arrays to avoid heap allocation in hot path
	// Max size: big network has InputDimensions=1024, so max 256 chunks
	var input32Buf [256]int32
	var nnzIndicesBuf [256]int
	input32 := input32Buf[:numChunks]
	nnzCount := 0

	// Reinterpret input as int32 for chunk processing
	// BCE hint: prove bounds safety to compiler
	inputLen := len(input)
	if inputLen > 0 {
		_ = input[inputLen-1]
	}
	for i := 0; i < inputLen; i++ {
		input32[i/4] |= int32(input[i]) << (8 * (i % 4))
	}

	// Find non-zero chunks and collect indices
	for i := 0; i < numChunks; i++ {
		if input32[i] != 0 {
			nnzIndicesBuf[nnzCount] = i
			nnzCount++
		}
	}
	nnzIndices := nnzIndicesBuf[:nnzCount]

	// BCE hints for inner loop
	outLen := a.OutputDimensions
	if outLen > 0 {
		_ = output[outLen-1]
	}

	// Process only non-zero chunks using SIMD
	for _, idx := range nnzIndices {
		in := input32[idx]
		colOffset := idx * outLen * chunkSize

		// Use SIMD for the inner loop: processes all outputs for this chunk
		// The SIMD function handles the multiply-accumulate with horizontal sums
		SIMDSparseChunkMulAcc(output[:outLen], a.Weights[colOffset:], outLen, uint32(in))
	}
}
