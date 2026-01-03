// NNUE Network loading and evaluation.
// Ported from Stockfish src/nnue/network.h and network.cpp

package sfnnue

import (
	"fmt"
	"io"
	"os"
)

// Network represents a complete NNUE network (big or small).
// Ported from network.h:57-118
type Network struct {
	// Feature transformer
	FeatureTransformer *FeatureTransformer

	// Layer stacks (one per bucket)
	LayerStacks [LayerStacks]*NetworkArchitecture

	// Network type
	IsBig bool

	// File info
	CurrentFile    string
	NetDescription string

	// Initialization status
	Initialized bool

	// Expected hash
	Hash uint32
}

// NewBigNetwork creates a new big network
func NewBigNetwork() *Network {
	net := &Network{
		FeatureTransformer: NewBigFeatureTransformer(),
		IsBig:              true,
	}

	// Create layer stacks
	for i := 0; i < LayerStacks; i++ {
		net.LayerStacks[i] = NewBigNetworkArchitecture()
	}

	// Calculate expected hash
	net.Hash = net.calculateHash()

	return net
}

// NewSmallNetwork creates a new small network
func NewSmallNetwork() *Network {
	net := &Network{
		FeatureTransformer: NewSmallFeatureTransformer(),
		IsBig:              false,
	}

	// Create layer stacks
	for i := 0; i < LayerStacks; i++ {
		net.LayerStacks[i] = NewSmallNetworkArchitecture()
	}

	// Calculate expected hash
	net.Hash = net.calculateHash()

	return net
}

// calculateHash calculates the expected hash for this network.
// Ported from network.h:114
func (n *Network) calculateHash() uint32 {
	return n.FeatureTransformer.GetHashValue() ^ n.LayerStacks[0].GetHashValue()
}

// Load loads network parameters from a file.
// Ported from network.cpp:111-137
func (n *Network) Load(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	return n.LoadFromReader(f)
}

// LoadFromReader loads network parameters from a reader.
func (n *Network) LoadFromReader(r io.Reader) error {
	n.Initialized = true

	// Read and validate header
	hashValue, description, err := n.readHeader(r)
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	if hashValue != n.Hash {
		return fmt.Errorf("hash mismatch: expected %08x, got %08x", n.Hash, hashValue)
	}

	n.NetDescription = description

	// Read parameters
	if err := n.readParameters(r); err != nil {
		return fmt.Errorf("failed to read parameters: %w", err)
	}

	return nil
}

// readHeader reads and validates the network file header.
// Ported from network.cpp:344-358
func (n *Network) readHeader(r io.Reader) (uint32, string, error) {
	// Read version
	version, err := ReadLittleEndian[uint32](r)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read version: %w", err)
	}
	if version != Version {
		return 0, "", fmt.Errorf("version mismatch: expected %08x, got %08x", Version, version)
	}

	// Read hash
	hashValue, err := ReadLittleEndian[uint32](r)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read hash: %w", err)
	}

	// Read description size
	descSize, err := ReadLittleEndian[uint32](r)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read description size: %w", err)
	}

	// Read description
	descBytes := make([]byte, descSize)
	if _, err := io.ReadFull(r, descBytes); err != nil {
		return 0, "", fmt.Errorf("failed to read description: %w", err)
	}

	return hashValue, string(descBytes), nil
}

// readParameters reads all network parameters.
// Ported from network.cpp:374-390
func (n *Network) readParameters(r io.Reader) error {
	// Read feature transformer (with hash check)
	transformerHash, err := ReadLittleEndian[uint32](r)
	if err != nil {
		return fmt.Errorf("failed to read transformer hash: %w", err)
	}
	expectedTransformerHash := n.FeatureTransformer.GetHashValue()
	if transformerHash != expectedTransformerHash {
		return fmt.Errorf("transformer hash mismatch: expected %08x, got %08x",
			expectedTransformerHash, transformerHash)
	}

	if err := n.FeatureTransformer.ReadParameters(r); err != nil {
		return fmt.Errorf("failed to read transformer parameters: %w", err)
	}

	// Read layer stacks
	for i := 0; i < LayerStacks; i++ {
		// Read layer stack hash
		stackHash, err := ReadLittleEndian[uint32](r)
		if err != nil {
			return fmt.Errorf("failed to read layer stack %d hash: %w", i, err)
		}
		expectedStackHash := n.LayerStacks[i].GetHashValue()
		if stackHash != expectedStackHash {
			return fmt.Errorf("layer stack %d hash mismatch: expected %08x, got %08x",
				i, expectedStackHash, stackHash)
		}

		if err := n.LayerStacks[i].ReadParameters(r); err != nil {
			return fmt.Errorf("failed to read layer stack %d: %w", i, err)
		}
	}

	return nil
}

// Evaluate evaluates a position using the network.
// Ported from network.cpp:172-189
func (n *Network) Evaluate(
	accumulation [2][]int16,
	psqtAccumulation [2][]int32,
	sideToMove int,
	pieceCount int,
) (psqt int32, positional int32) {
	// Select bucket based on piece count
	bucket := (pieceCount - 1) / 4
	if bucket < 0 {
		bucket = 0
	} else if bucket >= LayerStacks {
		bucket = LayerStacks - 1
	}

	// Determine perspectives
	perspectives := [2]int{sideToMove, 1 - sideToMove}

	// Transform features
	halfDims := n.FeatureTransformer.HalfDimensions
	transformedFeatures := make([]uint8, halfDims)

	psqt = n.FeatureTransformer.Transform(
		accumulation,
		psqtAccumulation,
		perspectives,
		bucket,
		transformedFeatures,
	)

	// Propagate through layer stack
	positional = n.LayerStacks[bucket].Propagate(transformedFeatures)

	// Scale outputs
	return psqt / int32(OutputScale), positional / int32(OutputScale)
}

// Networks holds both big and small networks.
// Ported from network.h:132-139
type Networks struct {
	Big   *Network
	Small *Network
}

// NewNetworks creates both networks
func NewNetworks() *Networks {
	return &Networks{
		Big:   NewBigNetwork(),
		Small: NewSmallNetwork(),
	}
}

// LoadNetworks loads both networks from files
func LoadNetworks(bigFile, smallFile string) (*Networks, error) {
	nets := NewNetworks()

	if err := nets.Big.Load(bigFile); err != nil {
		return nil, fmt.Errorf("failed to load big network: %w", err)
	}

	if err := nets.Small.Load(smallFile); err != nil {
		return nil, fmt.Errorf("failed to load small network: %w", err)
	}

	return nets, nil
}

// Evaluator provides a high-level interface for NNUE evaluation.
type Evaluator struct {
	Networks *Networks

	// Unified dual accumulator stack (manages both big and small together)
	DualStack *DualAccumulatorStack

	// Unified dual accumulator cache
	DualCache *DualAccumulatorCache

	// Legacy compatibility - these point into DualStack
	AccStack   *AccumulatorStack // Deprecated: use DualStack
	BigCache   *AccumulatorCache
	SmallCache *AccumulatorCache
}

// NewEvaluator creates a new evaluator from network files
func NewEvaluator(bigFile, smallFile string) (*Evaluator, error) {
	networks, err := LoadNetworks(bigFile, smallFile)
	if err != nil {
		return nil, err
	}

	dualCache := NewDualAccumulatorCache(
		networks.Big.FeatureTransformer.Biases,
		networks.Small.FeatureTransformer.Biases,
	)

	return &Evaluator{
		Networks:   networks,
		DualStack:  NewDualAccumulatorStack(),
		DualCache:  dualCache,
		AccStack:   NewAccumulatorStack(), // Legacy compatibility
		BigCache:   dualCache.Big,
		SmallCache: dualCache.Small,
	}, nil
}

// Push saves accumulator state before a move
func (e *Evaluator) Push() {
	e.DualStack.Push()
	e.AccStack.Push() // Legacy
}

// Pop restores accumulator state after unmaking a move
func (e *Evaluator) Pop() {
	e.DualStack.Pop()
	e.AccStack.Pop() // Legacy
}

// Reset resets the accumulator stack
func (e *Evaluator) Reset() {
	e.DualStack.Reset()
	e.AccStack.Reset() // Legacy
}

// Refresh forces a full recomputation of accumulators
func (e *Evaluator) Refresh() {
	e.DualStack.Current().Reset()
	e.AccStack.CurrentBig().Reset()   // Legacy
	e.AccStack.CurrentSmall().Reset() // Legacy
}

// CurrentDual returns the current dual accumulator
func (e *Evaluator) CurrentDual() *DualAccumulator {
	return e.DualStack.Current()
}

// PreviousDual returns the previous dual accumulator
func (e *Evaluator) PreviousDual() *DualAccumulator {
	return e.DualStack.Previous()
}

// RecordPieceChange records a piece movement for incremental updates
func (e *Evaluator) RecordPieceChange(piece, fromSq, toSq int) {
	e.DualStack.Current().AddDirtyPiece(piece, fromSq, toSq)
}

// RecordKingMove records a king movement (triggers full refresh for that perspective)
func (e *Evaluator) RecordKingMove(perspective, newKingSq int) {
	e.DualStack.Current().SetKingMoved(perspective, newKingSq)
}
