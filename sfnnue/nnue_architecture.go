// NNUE network architecture definition.
// Ported from Stockfish src/nnue/nnue_architecture.h

package sfnnue

import (
	"io"

	"github.com/hailam/chessplay/sfnnue/features"
	"github.com/hailam/chessplay/sfnnue/layers"
)

// Network architecture constants (nnue_architecture.h:43-52)
const (
	// Big network dimensions
	TransformedFeatureDimensionsBig = 1024
	L2Big                           = 15
	L3Big                           = 32

	// Small network dimensions
	TransformedFeatureDimensionsSmall = 128
	L2Small                           = 15
	L3Small                           = 32

	// Common parameters
	PSQTBuckets = 8
	LayerStacks = 8
)

// Feature set dimensions
const (
	// HalfKAv2_hm dimensions
	PSQInputDimensions = features.Dimensions // 22528

	// FullThreats dimensions (for big network)
	ThreatInputDimensions = features.ThreatDimensions // 79856
)

// ForwardBuffers holds pre-allocated buffers for the forward pass.
// Avoids allocation per Propagate call.
type ForwardBuffers struct {
	FC0Out    [32]int32 // CeilToMultiple(FC0Outputs, 32)
	AcSqr0Out [64]uint8 // CeilToMultiple(FC0Outputs*2, 32) - holds both sqr and regular
	Ac0Out    [32]uint8 // CeilToMultiple(FC0Outputs, 32)
	FC1Out    [32]int32 // CeilToMultiple(FC1Outputs, 32)
	Ac1Out    [32]uint8 // CeilToMultiple(FC1Outputs, 32)
	FC2Out    [32]int32 // CeilToMultiple(1, 32)
}

// NetworkArchitecture represents the neural network structure.
// Ported from nnue_architecture.h:60-153
type NetworkArchitecture struct {
	TransformedFeatureDimensions int
	FC0Outputs                   int // L2 + 1
	FC1Outputs                   int // L3

	// Layers
	FC0    *layers.AffineTransformSparseInput // TransformedFeatureDimensions*2 -> FC0Outputs
	AcSqr0 *layers.SqrClippedReLU             // FC0Outputs
	Ac0    *layers.ClippedReLU                // FC0Outputs
	FC1    *layers.AffineTransform            // FC0Outputs*2 -> FC1Outputs
	Ac1    *layers.ClippedReLU                // FC1Outputs
	FC2    *layers.AffineTransform            // FC1Outputs -> 1

	// Pre-allocated buffers for forward pass (avoids allocation per Propagate)
	buffers ForwardBuffers
}

// NewBigNetworkArchitecture creates the big network architecture
// Ported from nnue_architecture.h:66-71
func NewBigNetworkArchitecture() *NetworkArchitecture {
	fc0Out := L2Big + 1 // 16
	return &NetworkArchitecture{
		TransformedFeatureDimensions: TransformedFeatureDimensionsBig,
		FC0Outputs:                   fc0Out,
		FC1Outputs:                   L3Big,
		// FC0 input is TransformedFeatureDimensions (NOT *2)
		// The feature transformer outputs HalfDimensions via pairwise multiplication
		FC0:    layers.NewAffineTransformSparseInput(TransformedFeatureDimensionsBig, fc0Out),
		AcSqr0: layers.NewSqrClippedReLU(fc0Out),
		Ac0:    layers.NewClippedReLU(fc0Out),
		FC1:    layers.NewAffineTransform(fc0Out*2, L3Big),
		Ac1:    layers.NewClippedReLU(L3Big),
		FC2:    layers.NewAffineTransform(L3Big, 1),
	}
}

// NewSmallNetworkArchitecture creates the small network architecture
// Ported from nnue_architecture.h:66-71
func NewSmallNetworkArchitecture() *NetworkArchitecture {
	fc0Out := L2Small + 1 // 16
	return &NetworkArchitecture{
		TransformedFeatureDimensions: TransformedFeatureDimensionsSmall,
		FC0Outputs:                   fc0Out,
		FC1Outputs:                   L3Small,
		// FC0 input is TransformedFeatureDimensions (NOT *2)
		FC0:    layers.NewAffineTransformSparseInput(TransformedFeatureDimensionsSmall, fc0Out),
		AcSqr0: layers.NewSqrClippedReLU(fc0Out),
		Ac0:    layers.NewClippedReLU(fc0Out),
		FC1:    layers.NewAffineTransform(fc0Out*2, L3Small),
		Ac1:    layers.NewClippedReLU(L3Small),
		FC2:    layers.NewAffineTransform(L3Small, 1),
	}
}

// GetHashValue returns the hash value for this architecture.
// Ported from nnue_architecture.h:74-86
func (n *NetworkArchitecture) GetHashValue() uint32 {
	// Input slice hash
	hashValue := uint32(0xEC42E90D)
	hashValue ^= uint32(n.TransformedFeatureDimensions * 2)

	// Chain hash through layers
	hashValue = n.FC0.GetHashValue(hashValue)
	hashValue = n.Ac0.GetHashValue(hashValue) // Note: uses Ac0 hash, not AcSqr0
	hashValue = n.FC1.GetHashValue(hashValue)
	hashValue = n.Ac1.GetHashValue(hashValue)
	hashValue = n.FC2.GetHashValue(hashValue)

	return hashValue
}

// ReadParameters reads all layer parameters from a stream.
// Ported from nnue_architecture.h:89-93
func (n *NetworkArchitecture) ReadParameters(r io.Reader) error {
	if err := n.FC0.ReadParameters(r); err != nil {
		return err
	}
	// Ac0 and AcSqr0 have no parameters
	if err := n.FC1.ReadParameters(r); err != nil {
		return err
	}
	// Ac1 has no parameters
	if err := n.FC2.ReadParameters(r); err != nil {
		return err
	}
	return nil
}

// Propagate performs the forward pass through all layers.
// Uses pre-allocated buffers to avoid allocation per call.
// Ported from nnue_architecture.h:102-139
func (n *NetworkArchitecture) Propagate(transformedFeatures []uint8) int32 {
	// Use pre-allocated buffers (sliced to required size)
	fc0Out := n.buffers.FC0Out[:CeilToMultiple(n.FC0Outputs, 32)]
	acSqr0Out := n.buffers.AcSqr0Out[:CeilToMultiple(n.FC0Outputs*2, 32)]
	ac0Out := n.buffers.Ac0Out[:CeilToMultiple(n.FC0Outputs, 32)]
	fc1Out := n.buffers.FC1Out[:CeilToMultiple(n.FC1Outputs, 32)]
	ac1Out := n.buffers.Ac1Out[:CeilToMultiple(n.FC1Outputs, 32)]
	fc2Out := n.buffers.FC2Out[:CeilToMultiple(1, 32)]

	// Forward pass
	n.FC0.Propagate(transformedFeatures, fc0Out)
	n.AcSqr0.Propagate(fc0Out, acSqr0Out[:n.FC0Outputs])
	// Use SIMD ClippedReLU for performance (WeightScaleBits = 6)
	SIMDClippedReLU(fc0Out, ac0Out, 6)

	// Concatenate sqr and regular relu outputs (nnue_architecture.h:127-128)
	copy(acSqr0Out[n.FC0Outputs:], ac0Out[:n.FC0Outputs])

	n.FC1.Propagate(acSqr0Out, fc1Out)
	// Use SIMD ClippedReLU for performance (WeightScaleBits = 6)
	SIMDClippedReLU(fc1Out, ac1Out, 6)
	n.FC2.Propagate(ac1Out, fc2Out)

	// Add forward output from fc0_out[FC_0_OUTPUTS] (nnue_architecture.h:133-137)
	// This is a skip connection scaled appropriately
	fwdOut := (fc0Out[n.FC0Outputs-1]) * (600 * OutputScale) / (127 * (1 << WeightScaleBits))
	outputValue := fc2Out[0] + fwdOut

	return outputValue
}

// BigNetworkHash returns the expected hash for the big network
func BigNetworkHash() uint32 {
	arch := NewBigNetworkArchitecture()
	return arch.GetHashValue()
}

// SmallNetworkHash returns the expected hash for the small network
func SmallNetworkHash() uint32 {
	arch := NewSmallNetworkArchitecture()
	return arch.GetHashValue()
}
