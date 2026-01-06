// NNUE Feature Transformer.
// Ported from Stockfish src/nnue/nnue_feature_transformer.h

package sfnnue

import (
	"fmt"
	"io"
	"unsafe"

	"github.com/hailam/chessplay/sfnnue/features"
)

// FeatureTransformer converts input features to hidden layer values.
// Ported from nnue_feature_transformer.h:81-435
type FeatureTransformer struct {
	HalfDimensions      int
	InputDimensions     int  // PSQ feature dimensions
	UseThreats          bool // True for big network

	// Biases for the accumulator (int16)
	Biases []int16

	// Weights for PSQ features (int16)
	Weights []int16

	// Threat weights (int8, only for big network)
	ThreatWeights []int8

	// PSQT weights (int32)
	PSQTWeights []int32

	// Threat PSQT weights (int32, only for big network)
	ThreatPSQTWeights []int32
}

// NewBigFeatureTransformer creates a feature transformer for the big network
func NewBigFeatureTransformer() *FeatureTransformer {
	halfDims := TransformedFeatureDimensionsBig
	return &FeatureTransformer{
		HalfDimensions:    halfDims,
		InputDimensions:   features.Dimensions,
		UseThreats:        true,
		Biases:            make([]int16, halfDims),
		Weights:           make([]int16, halfDims*features.Dimensions),
		ThreatWeights:     make([]int8, halfDims*features.ThreatDimensions),
		PSQTWeights:       make([]int32, features.Dimensions*PSQTBuckets),
		ThreatPSQTWeights: make([]int32, features.ThreatDimensions*PSQTBuckets),
	}
}

// NewSmallFeatureTransformer creates a feature transformer for the small network
func NewSmallFeatureTransformer() *FeatureTransformer {
	halfDims := TransformedFeatureDimensionsSmall
	return &FeatureTransformer{
		HalfDimensions:  halfDims,
		InputDimensions: features.Dimensions,
		UseThreats:      false,
		Biases:          make([]int16, halfDims),
		Weights:         make([]int16, halfDims*features.Dimensions),
		PSQTWeights:     make([]int32, features.Dimensions*PSQTBuckets),
	}
}

// GetHashValue returns the hash value for this transformer.
// Ported from nnue_feature_transformer.h:126-129
func (ft *FeatureTransformer) GetHashValue() uint32 {
	if ft.UseThreats {
		return features.ThreatHashValue ^ uint32(ft.HalfDimensions*2)
	}
	return features.HashValue ^ uint32(ft.HalfDimensions*2)
}

// ReadParameters reads transformer parameters from a stream.
// Ported from nnue_feature_transformer.h:157-192
func (ft *FeatureTransformer) ReadParameters(r io.Reader) error {
	// Read biases with LEB128 compression
	if err := ReadLEB128(r, ft.Biases); err != nil {
		return fmt.Errorf("failed to read biases: %w", err)
	}

	if ft.UseThreats {
		// Read threat weights (little endian, not LEB128)
		if err := ReadLittleEndianSlice(r, ft.ThreatWeights); err != nil {
			return fmt.Errorf("failed to read threat weights: %w", err)
		}

		// Read PSQ weights with LEB128
		if err := ReadLEB128(r, ft.Weights); err != nil {
			return fmt.Errorf("failed to read weights: %w", err)
		}

		// Read combined PSQT weights
		totalPSQT := (features.ThreatDimensions + features.Dimensions) * PSQTBuckets
		combinedPSQT := make([]int32, totalPSQT)
		if err := ReadLEB128(r, combinedPSQT); err != nil {
			return fmt.Errorf("failed to read PSQT weights: %w", err)
		}

		// Split into threat and regular PSQT weights
		threatPSQTSize := features.ThreatDimensions * PSQTBuckets
		copy(ft.ThreatPSQTWeights, combinedPSQT[:threatPSQTSize])
		copy(ft.PSQTWeights, combinedPSQT[threatPSQTSize:])
	} else {
		// Small network: no threat weights
		if err := ReadLEB128(r, ft.Weights); err != nil {
			return fmt.Errorf("failed to read weights: %w", err)
		}
		if err := ReadLEB128(r, ft.PSQTWeights); err != nil {
			return fmt.Errorf("failed to read PSQT weights: %w", err)
		}
	}

	// Permute weights for SIMD (nnue_feature_transformer.h:186)
	ft.permuteWeights()

	// Scale weights for non-threat network (nnue_feature_transformer.h:188-189)
	if !ft.UseThreats {
		ft.scaleWeights(true)
	}

	return nil
}

// permuteWeights reorders weights for SIMD optimization.
// Ported from nnue_feature_transformer.h:131-137
// For NEON (128-bit): reorders 8-element int16 blocks so that
// consecutive SIMD loads align with pack/unzip instructions.
func (ft *FeatureTransformer) permuteWeights() {
	// NEON permutation order for 8 int16 values (128-bit)
	// This aligns with uzp1/uzp2 operations for efficient packing
	order := []int{0, 2, 1, 3, 4, 6, 5, 7}

	// Permute the main weights
	ft.permuteInt16Slice(ft.Weights, order)

	// Permute the biases
	ft.permuteInt16Slice(ft.Biases, order)
}

// permuteInt16Slice reorders an int16 slice in 8-element chunks according to order.
func (ft *FeatureTransformer) permuteInt16Slice(data []int16, order []int) {
	blockSize := len(order)
	temp := make([]int16, blockSize)

	// Process in blocks of 8
	for start := 0; start+blockSize <= len(data); start += blockSize {
		// Copy reordered elements to temp
		for i, o := range order {
			temp[i] = data[start+o]
		}
		// Copy back
		copy(data[start:start+blockSize], temp)
	}
}

// scaleWeights scales weights by 2 for proper clipping behavior.
// Ported from nnue_feature_transformer.h:147-152
func (ft *FeatureTransformer) scaleWeights(read bool) {
	if read {
		for i := range ft.Weights {
			ft.Weights[i] *= 2
		}
		for i := range ft.Biases {
			ft.Biases[i] *= 2
		}
	} else {
		for i := range ft.Weights {
			ft.Weights[i] /= 2
		}
		for i := range ft.Biases {
			ft.Biases[i] /= 2
		}
	}
}

// Transform converts accumulated features to transformer output.
// Ported from nnue_feature_transformer.h:243-424
func (ft *FeatureTransformer) Transform(
	accumulation [2][]int16, // [color][HalfDimensions]
	psqtAccumulation [2][]int32, // [color][PSQTBuckets]
	perspectives [2]int, // [0]=stm, [1]=nstm
	bucket int,
	output []uint8,
) int32 {
	// Calculate PSQT score
	psqt := psqtAccumulation[perspectives[0]][bucket] - psqtAccumulation[perspectives[1]][bucket]
	if ft.UseThreats {
		psqt /= 2
	} else {
		psqt /= 2
	}

	// Apply pairwise multiplication with clipping using fused SIMD operation
	halfDims := ft.HalfDimensions
	halfHalfDims := halfDims / 2

	// Determine max value based on network type
	maxVal := 255 // Big network with threats
	if !ft.UseThreats {
		maxVal = 254 // Small network (127 * 2)
	}

	for p := 0; p < 2; p++ {
		offset := halfHalfDims * p
		acc := accumulation[perspectives[p]]

		// Use fused SIMD operation: clamp + multiply + shift in one pass
		// acc0 = acc[0:halfHalfDims], acc1 = acc[halfHalfDims:halfDims]
		SIMDTransformClampMul(
			acc[:halfHalfDims],
			acc[halfHalfDims:halfDims],
			output[offset:offset+halfHalfDims],
			maxVal,
		)
	}

	return psqt
}

// ComputeAccumulator computes the full accumulator from scratch.
func (ft *FeatureTransformer) ComputeAccumulator(
	activeIndices []int,
	accumulation []int16,
	psqtAccumulation []int32,
) {
	// Start with biases (SIMD accelerated)
	SIMDCopyInt16(accumulation, ft.Biases)

	// Initialize PSQT to zero
	for i := range psqtAccumulation {
		psqtAccumulation[i] = 0
	}

	// BCE hint for PSQT accumulation array
	if len(psqtAccumulation) >= PSQTBuckets {
		_ = psqtAccumulation[PSQTBuckets-1]
	}

	// Add weights for active features (SIMD accelerated)
	for _, idx := range activeIndices {
		if idx >= 0 && idx < ft.InputDimensions {
			// Add feature weights using SIMD
			offset := idx * ft.HalfDimensions
			SIMDAddInt16Offset(accumulation, ft.Weights, offset, ft.HalfDimensions)

			// Add PSQT weights (small loop, not worth SIMD)
			psqtOffset := idx * PSQTBuckets
			// BCE hint for PSQT weights access
			_ = ft.PSQTWeights[psqtOffset+PSQTBuckets-1]
			for b := 0; b < PSQTBuckets; b++ {
				psqtAccumulation[b] += ft.PSQTWeights[psqtOffset+b]
			}
		}
	}
}

// UpdateAccumulator incrementally updates the accumulator (in-place).
// Uses SIMD for the hot int16 loops and prefetching for better cache performance.
func (ft *FeatureTransformer) UpdateAccumulator(
	removedIndices, addedIndices []int,
	accumulation []int16,
	psqtAccumulation []int32,
) {
	// Calculate number of cache lines per feature weight access
	// HalfDimensions * 2 bytes (int16) / 64 bytes per cache line
	linesPerFeature := (ft.HalfDimensions * 2) / 64
	if linesPerFeature < 1 {
		linesPerFeature = 1
	}

	// BCE hint for PSQT accumulation array
	if len(psqtAccumulation) >= PSQTBuckets {
		_ = psqtAccumulation[PSQTBuckets-1]
	}

	// Remove old features (SIMD accelerated)
	for i, idx := range removedIndices {
		if idx >= 0 && idx < ft.InputDimensions {
			offset := idx * ft.HalfDimensions

			// Prefetch next feature's weights
			if i+1 < len(removedIndices) {
				nextIdx := removedIndices[i+1]
				if nextIdx >= 0 && nextIdx < ft.InputDimensions {
					nextOffset := nextIdx * ft.HalfDimensions
					PrefetchLines(unsafe.Pointer(&ft.Weights[nextOffset]), linesPerFeature)
				}
			}

			SIMDSubInt16Offset(accumulation, ft.Weights, offset, ft.HalfDimensions)

			// PSQT is only 8 elements, not worth SIMD
			psqtOffset := idx * PSQTBuckets
			// BCE hint for PSQT weights access
			_ = ft.PSQTWeights[psqtOffset+PSQTBuckets-1]
			for b := 0; b < PSQTBuckets; b++ {
				psqtAccumulation[b] -= ft.PSQTWeights[psqtOffset+b]
			}
		}
	}

	// Add new features (SIMD accelerated)
	for i, idx := range addedIndices {
		if idx >= 0 && idx < ft.InputDimensions {
			offset := idx * ft.HalfDimensions

			// Prefetch next feature's weights
			if i+1 < len(addedIndices) {
				nextIdx := addedIndices[i+1]
				if nextIdx >= 0 && nextIdx < ft.InputDimensions {
					nextOffset := nextIdx * ft.HalfDimensions
					PrefetchLines(unsafe.Pointer(&ft.Weights[nextOffset]), linesPerFeature)
				}
			}

			SIMDAddInt16Offset(accumulation, ft.Weights, offset, ft.HalfDimensions)

			// PSQT is only 8 elements, not worth SIMD
			psqtOffset := idx * PSQTBuckets
			// BCE hint for PSQT weights access
			_ = ft.PSQTWeights[psqtOffset+PSQTBuckets-1]
			for b := 0; b < PSQTBuckets; b++ {
				psqtAccumulation[b] += ft.PSQTWeights[psqtOffset+b]
			}
		}
	}
}

// ForwardUpdateIncremental performs a forward incremental update from a previous accumulator.
// Copies the previous accumulator state and applies changes.
// Ported from Stockfish nnue_accumulator.cpp:204-257
func (ft *FeatureTransformer) ForwardUpdateIncremental(
	prevAcc *Accumulator,
	currAcc *Accumulator,
	removedIndices, addedIndices []int,
	perspective int,
) {
	// Copy previous accumulation to current (SIMD accelerated)
	SIMDCopyInt16(currAcc.Accumulation[perspective], prevAcc.Accumulation[perspective])
	copy(currAcc.PSQTAccumulation[perspective], prevAcc.PSQTAccumulation[perspective])

	// Apply changes
	ft.UpdateAccumulator(
		removedIndices, addedIndices,
		currAcc.Accumulation[perspective],
		currAcc.PSQTAccumulation[perspective],
	)

	// Mark as computed and copy king square
	currAcc.Computed[perspective] = true
	currAcc.KingSq[perspective] = prevAcc.KingSq[perspective]
}

// BackwardUpdateIncremental performs a backward incremental update.
// Used when we have a computed state later in the tree and want to derive an earlier one.
// Reverses the changes: adds what was removed, removes what was added.
func (ft *FeatureTransformer) BackwardUpdateIncremental(
	laterAcc *Accumulator,
	currAcc *Accumulator,
	removedIndices, addedIndices []int,
	perspective int,
) {
	// Copy later accumulation to current (SIMD accelerated)
	SIMDCopyInt16(currAcc.Accumulation[perspective], laterAcc.Accumulation[perspective])
	copy(currAcc.PSQTAccumulation[perspective], laterAcc.PSQTAccumulation[perspective])

	// Reverse the changes: what was removed gets added back, what was added gets removed
	ft.UpdateAccumulator(
		addedIndices, removedIndices, // Swapped!
		currAcc.Accumulation[perspective],
		currAcc.PSQTAccumulation[perspective],
	)

	// Mark as computed and copy king square
	currAcc.Computed[perspective] = true
	currAcc.KingSq[perspective] = laterAcc.KingSq[perspective]
}

// DoubleUpdateIncremental performs a fused update for two consecutive moves.
// This is more efficient than two separate updates.
func (ft *FeatureTransformer) DoubleUpdateIncremental(
	prevAcc *Accumulator,
	currAcc *Accumulator,
	removed1, added1, removed2, added2 []int,
	perspective int,
) {
	// Combine both sets of changes using stack-allocated arrays (no heap allocation)
	var allRemovedBuf [16]int
	var allAddedBuf [16]int

	removedLen := len(removed1) + len(removed2)
	addedLen := len(added1) + len(added2)

	// Copy to buffers
	copy(allRemovedBuf[:len(removed1)], removed1)
	copy(allRemovedBuf[len(removed1):removedLen], removed2)
	copy(allAddedBuf[:len(added1)], added1)
	copy(allAddedBuf[len(added1):addedLen], added2)

	// Apply as single batch update
	ft.ForwardUpdateIncremental(prevAcc, currAcc, allRemovedBuf[:removedLen], allAddedBuf[:addedLen], perspective)
}
