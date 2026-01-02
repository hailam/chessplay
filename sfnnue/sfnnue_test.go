package sfnnue

import (
	"encoding/binary"
	"os"
	"testing"
)

const (
	weightsDir  = "/Users/maix/apps/go/chessplay/weights"
	bigNetFile  = weightsDir + "/nn-c288c895ea92.nnue"
	smallNetFile = weightsDir + "/nn-37f18f62d772.nnue"
)

func TestInspectNetworkHeader(t *testing.T) {
	files := []string{bigNetFile, smallNetFile}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			t.Logf("Skipping %s: %v", file, err)
			continue
		}
		defer f.Close()

		var version, hash, descSize uint32
		binary.Read(f, binary.LittleEndian, &version)
		binary.Read(f, binary.LittleEndian, &hash)
		binary.Read(f, binary.LittleEndian, &descSize)

		desc := make([]byte, descSize)
		f.Read(desc)

		t.Logf("File: %s", file)
		t.Logf("  Version: %08x (expected: %08x)", version, Version)
		t.Logf("  Hash: %08x", hash)
		t.Logf("  Description: %s", string(desc))
	}
}

func TestLoadBigNetwork(t *testing.T) {
	net := NewBigNetwork()
	t.Logf("Big network expected hash: %08x", net.Hash)

	f, err := os.Open(bigNetFile)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}
	defer f.Close()

	err = net.LoadFromReader(f)
	if err != nil {
		t.Errorf("Failed to load big network: %v", err)
		return
	}

	t.Logf("Loaded big network: %s", net.NetDescription)
}

func TestLoadSmallNetwork(t *testing.T) {
	net := NewSmallNetwork()
	t.Logf("Small network expected hash: %08x", net.Hash)

	f, err := os.Open(smallNetFile)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}
	defer f.Close()

	err = net.LoadFromReader(f)
	if err != nil {
		t.Errorf("Failed to load small network: %v", err)
		return
	}

	t.Logf("Loaded small network: %s", net.NetDescription)
}

// TestForwardIncrementalUpdate verifies that incremental update produces same result as full refresh
func TestForwardIncrementalUpdate(t *testing.T) {
	// Create a small feature transformer for testing
	halfDims := 128
	inputDims := 1000 // Use smaller dimensions for testing
	ft := &FeatureTransformer{
		HalfDimensions:  halfDims,
		InputDimensions: inputDims,
		UseThreats:      false,
		Biases:          make([]int16, halfDims),
		Weights:         make([]int16, halfDims*inputDims),
		PSQTWeights:     make([]int32, inputDims*PSQTBuckets),
	}

	// Initialize with some test values
	for i := range ft.Biases {
		ft.Biases[i] = int16(i % 100)
	}
	for i := range ft.Weights {
		ft.Weights[i] = int16((i * 7) % 200)
	}
	for i := range ft.PSQTWeights {
		ft.PSQTWeights[i] = int32((i * 3) % 500)
	}

	// Create two accumulators
	prevAcc := NewAccumulator(halfDims)
	currAccIncremental := NewAccumulator(halfDims)
	currAccFull := NewAccumulator(halfDims)

	// Initial features
	initialFeatures := []int{10, 50, 100, 200, 500}

	// Compute full accumulator for initial state
	ft.ComputeAccumulator(initialFeatures, prevAcc.Accumulation[0], prevAcc.PSQTAccumulation[0])
	prevAcc.Computed[0] = true
	prevAcc.KingSq[0] = 4 // e1

	// Simulate a move: remove feature 50, add feature 300
	removed := []int{50}
	added := []int{300}

	// Method 1: Incremental update
	ft.ForwardUpdateIncremental(prevAcc, currAccIncremental, removed, added, 0)

	// Method 2: Full refresh with new feature set
	newFeatures := []int{10, 100, 200, 300, 500} // 50 removed, 300 added
	ft.ComputeAccumulator(newFeatures, currAccFull.Accumulation[0], currAccFull.PSQTAccumulation[0])

	// Compare results
	for i := 0; i < halfDims; i++ {
		if currAccIncremental.Accumulation[0][i] != currAccFull.Accumulation[0][i] {
			t.Errorf("Mismatch at accumulation[%d]: incremental=%d, full=%d",
				i, currAccIncremental.Accumulation[0][i], currAccFull.Accumulation[0][i])
		}
	}

	for i := 0; i < PSQTBuckets; i++ {
		if currAccIncremental.PSQTAccumulation[0][i] != currAccFull.PSQTAccumulation[0][i] {
			t.Errorf("Mismatch at PSQT[%d]: incremental=%d, full=%d",
				i, currAccIncremental.PSQTAccumulation[0][i], currAccFull.PSQTAccumulation[0][i])
		}
	}

	t.Log("Forward incremental update matches full refresh")
}

// TestBackwardIncrementalUpdate verifies backward update reverses changes correctly
func TestBackwardIncrementalUpdate(t *testing.T) {
	halfDims := 128
	inputDims := 1000
	ft := &FeatureTransformer{
		HalfDimensions:  halfDims,
		InputDimensions: inputDims,
		UseThreats:      false,
		Biases:          make([]int16, halfDims),
		Weights:         make([]int16, halfDims*inputDims),
		PSQTWeights:     make([]int32, inputDims*PSQTBuckets),
	}

	for i := range ft.Biases {
		ft.Biases[i] = int16(i % 100)
	}
	for i := range ft.Weights {
		ft.Weights[i] = int16((i * 7) % 200)
	}
	for i := range ft.PSQTWeights {
		ft.PSQTWeights[i] = int32((i * 3) % 500)
	}

	// Create accumulators
	originalAcc := NewAccumulator(halfDims)
	laterAcc := NewAccumulator(halfDims)
	recoveredAcc := NewAccumulator(halfDims)

	// Compute original state
	originalFeatures := []int{10, 50, 100, 200, 500}
	ft.ComputeAccumulator(originalFeatures, originalAcc.Accumulation[0], originalAcc.PSQTAccumulation[0])
	originalAcc.Computed[0] = true

	// Forward update to later state
	removed := []int{50}
	added := []int{300}
	ft.ForwardUpdateIncremental(originalAcc, laterAcc, removed, added, 0)

	// Backward update to recover original
	ft.BackwardUpdateIncremental(laterAcc, recoveredAcc, removed, added, 0)

	// Compare recovered with original
	for i := 0; i < halfDims; i++ {
		if recoveredAcc.Accumulation[0][i] != originalAcc.Accumulation[0][i] {
			t.Errorf("Mismatch at accumulation[%d]: recovered=%d, original=%d",
				i, recoveredAcc.Accumulation[0][i], originalAcc.Accumulation[0][i])
		}
	}

	for i := 0; i < PSQTBuckets; i++ {
		if recoveredAcc.PSQTAccumulation[0][i] != originalAcc.PSQTAccumulation[0][i] {
			t.Errorf("Mismatch at PSQT[%d]: recovered=%d, original=%d",
				i, recoveredAcc.PSQTAccumulation[0][i], originalAcc.PSQTAccumulation[0][i])
		}
	}

	t.Log("Backward incremental update correctly reverses changes")
}

// TestDoubleUpdateOptimization verifies double update equals two separate updates
func TestDoubleUpdateOptimization(t *testing.T) {
	halfDims := 128
	inputDims := 1000
	ft := &FeatureTransformer{
		HalfDimensions:  halfDims,
		InputDimensions: inputDims,
		UseThreats:      false,
		Biases:          make([]int16, halfDims),
		Weights:         make([]int16, halfDims*inputDims),
		PSQTWeights:     make([]int32, inputDims*PSQTBuckets),
	}

	for i := range ft.Biases {
		ft.Biases[i] = int16(i % 100)
	}
	for i := range ft.Weights {
		ft.Weights[i] = int16((i * 7) % 200)
	}
	for i := range ft.PSQTWeights {
		ft.PSQTWeights[i] = int32((i * 3) % 500)
	}

	// Create accumulators
	originalAcc := NewAccumulator(halfDims)
	singleUpdateAcc := NewAccumulator(halfDims)
	doubleUpdateAcc := NewAccumulator(halfDims)

	// Compute original state
	originalFeatures := []int{10, 50, 100, 200, 500}
	ft.ComputeAccumulator(originalFeatures, originalAcc.Accumulation[0], originalAcc.PSQTAccumulation[0])
	originalAcc.Computed[0] = true

	// Two separate moves
	removed1, added1 := []int{50}, []int{300}
	removed2, added2 := []int{100}, []int{400}

	// Method 1: Two separate updates
	intermediateAcc := NewAccumulator(halfDims)
	ft.ForwardUpdateIncremental(originalAcc, intermediateAcc, removed1, added1, 0)
	ft.ForwardUpdateIncremental(intermediateAcc, singleUpdateAcc, removed2, added2, 0)

	// Method 2: Double update
	ft.DoubleUpdateIncremental(originalAcc, doubleUpdateAcc, removed1, added1, removed2, added2, 0)

	// Compare results
	for i := 0; i < halfDims; i++ {
		if doubleUpdateAcc.Accumulation[0][i] != singleUpdateAcc.Accumulation[0][i] {
			t.Errorf("Mismatch at accumulation[%d]: double=%d, single=%d",
				i, doubleUpdateAcc.Accumulation[0][i], singleUpdateAcc.Accumulation[0][i])
		}
	}

	for i := 0; i < PSQTBuckets; i++ {
		if doubleUpdateAcc.PSQTAccumulation[0][i] != singleUpdateAcc.PSQTAccumulation[0][i] {
			t.Errorf("Mismatch at PSQT[%d]: double=%d, single=%d",
				i, doubleUpdateAcc.PSQTAccumulation[0][i], singleUpdateAcc.PSQTAccumulation[0][i])
		}
	}

	t.Log("Double update optimization equals two separate updates")
}

// TestAccumulatorStack verifies stack operations
func TestAccumulatorStack(t *testing.T) {
	stack := NewAccumulatorStack()

	if stack.Size != 1 {
		t.Errorf("Initial size should be 1, got %d", stack.Size)
	}

	// Push and verify size increases
	stack.Push()
	if stack.Size != 2 {
		t.Errorf("After push, size should be 2, got %d", stack.Size)
	}

	// Verify Previous returns correct accumulator
	prev := stack.PreviousBig()
	if prev == nil {
		t.Error("PreviousBig should not be nil after push")
	}

	// Pop and verify size decreases
	stack.Pop()
	if stack.Size != 1 {
		t.Errorf("After pop, size should be 1, got %d", stack.Size)
	}

	// Previous should be nil when at bottom of stack
	prev = stack.PreviousBig()
	if prev != nil {
		t.Error("PreviousBig should be nil when at bottom of stack")
	}

	t.Log("Accumulator stack operations work correctly")
}
