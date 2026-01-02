// NNUE Feature Transformer.
// Ported from Stockfish src/nnue/nnue_feature_transformer.h

package sfnnue

import (
	"fmt"
	"io"

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
func (ft *FeatureTransformer) permuteWeights() {
	// The permutation depends on SIMD width
	// For now, we use identity permutation (non-SIMD path)
	// TODO: Implement proper permutation for AVX2/AVX512
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

	// Apply pairwise multiplication with clipping
	halfDims := ft.HalfDimensions
	for p := 0; p < 2; p++ {
		offset := (halfDims / 2) * p
		acc := accumulation[perspectives[p]]

		if ft.UseThreats {
			maxVal := int16(255)
			for j := 0; j < halfDims/2; j++ {
				sum0 := acc[j]
				sum1 := acc[j+halfDims/2]

				// Clamp to [0, 255]
				if sum0 < 0 {
					sum0 = 0
				} else if sum0 > maxVal {
					sum0 = maxVal
				}
				if sum1 < 0 {
					sum1 = 0
				} else if sum1 > maxVal {
					sum1 = maxVal
				}

				// Pairwise multiply and divide by 512
				output[offset+j] = uint8((int(sum0) * int(sum1)) / 512)
			}
		} else {
			maxVal := int16(127 * 2)
			for j := 0; j < halfDims/2; j++ {
				sum0 := acc[j]
				sum1 := acc[j+halfDims/2]

				// Clamp to [0, 254]
				if sum0 < 0 {
					sum0 = 0
				} else if sum0 > maxVal {
					sum0 = maxVal
				}
				if sum1 < 0 {
					sum1 = 0
				} else if sum1 > maxVal {
					sum1 = maxVal
				}

				// Pairwise multiply and divide by 512
				output[offset+j] = uint8((int(sum0) * int(sum1)) / 512)
			}
		}
	}

	return psqt
}

// ComputeAccumulator computes the full accumulator from scratch.
func (ft *FeatureTransformer) ComputeAccumulator(
	activeIndices []int,
	accumulation []int16,
	psqtAccumulation []int32,
) {
	// Start with biases
	copy(accumulation, ft.Biases)

	// Initialize PSQT to zero
	for i := range psqtAccumulation {
		psqtAccumulation[i] = 0
	}

	// Add weights for active features
	for _, idx := range activeIndices {
		if idx >= 0 && idx < ft.InputDimensions {
			// Add feature weights
			offset := idx * ft.HalfDimensions
			for i := 0; i < ft.HalfDimensions; i++ {
				accumulation[i] += ft.Weights[offset+i]
			}

			// Add PSQT weights
			psqtOffset := idx * PSQTBuckets
			for b := 0; b < PSQTBuckets; b++ {
				psqtAccumulation[b] += ft.PSQTWeights[psqtOffset+b]
			}
		}
	}
}

// UpdateAccumulator incrementally updates the accumulator (in-place).
func (ft *FeatureTransformer) UpdateAccumulator(
	removedIndices, addedIndices []int,
	accumulation []int16,
	psqtAccumulation []int32,
) {
	// Remove old features
	for _, idx := range removedIndices {
		if idx >= 0 && idx < ft.InputDimensions {
			offset := idx * ft.HalfDimensions
			for i := 0; i < ft.HalfDimensions; i++ {
				accumulation[i] -= ft.Weights[offset+i]
			}

			psqtOffset := idx * PSQTBuckets
			for b := 0; b < PSQTBuckets; b++ {
				psqtAccumulation[b] -= ft.PSQTWeights[psqtOffset+b]
			}
		}
	}

	// Add new features
	for _, idx := range addedIndices {
		if idx >= 0 && idx < ft.InputDimensions {
			offset := idx * ft.HalfDimensions
			for i := 0; i < ft.HalfDimensions; i++ {
				accumulation[i] += ft.Weights[offset+i]
			}

			psqtOffset := idx * PSQTBuckets
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
	// Copy previous accumulation to current
	copy(currAcc.Accumulation[perspective], prevAcc.Accumulation[perspective])
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
	// Copy later accumulation to current
	copy(currAcc.Accumulation[perspective], laterAcc.Accumulation[perspective])
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
	// Combine both sets of changes
	allRemoved := make([]int, 0, len(removed1)+len(removed2))
	allRemoved = append(allRemoved, removed1...)
	allRemoved = append(allRemoved, removed2...)

	allAdded := make([]int, 0, len(added1)+len(added2))
	allAdded = append(allAdded, added1...)
	allAdded = append(allAdded, added2...)

	// Apply as single batch update
	ft.ForwardUpdateIncremental(prevAcc, currAcc, allRemoved, allAdded, perspective)
}
