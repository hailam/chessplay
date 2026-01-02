package nnue

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Weight file format constants
const (
	MagicNumber = 0x46524B53 // "FRKS" - Feature-based RKISS Stockfish-like format
	Version     = 1
)

// FileHeader is the header of the weight file.
type FileHeader struct {
	Magic   uint32
	Version uint32
	L1Size  uint32
	L2Size  uint32
}

// LoadWeights loads network weights from a binary file.
// File format:
//   - Header: Magic (4 bytes), Version (4 bytes), L1Size (4 bytes), L2Size (4 bytes)
//   - L1Weights: HalfKPSize * L1Size * int16
//   - L1Bias: L1Size * int16
//   - L2Weights: L1Size*2 * L2Size * int8
//   - L2Bias: L2Size * int32
//   - OutputWeights: L2Size * int8
//   - OutputBias: int32
func (n *Network) LoadWeights(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open weights file: %w", err)
	}
	defer f.Close()

	// Read header
	var header FileHeader
	if err := binary.Read(f, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Validate header
	if header.Magic != MagicNumber {
		return fmt.Errorf("invalid magic number: expected %x, got %x", MagicNumber, header.Magic)
	}
	if header.Version != Version {
		return fmt.Errorf("unsupported version: expected %d, got %d", Version, header.Version)
	}
	if header.L1Size != L1Size {
		return fmt.Errorf("L1 size mismatch: expected %d, got %d", L1Size, header.L1Size)
	}
	if header.L2Size != L2Size {
		return fmt.Errorf("L2 size mismatch: expected %d, got %d", L2Size, header.L2Size)
	}

	// Read L1 weights
	for i := 0; i < HalfKPSize; i++ {
		if err := binary.Read(f, binary.LittleEndian, &n.L1Weights[i]); err != nil {
			return fmt.Errorf("failed to read L1 weights at %d: %w", i, err)
		}
	}

	// Read L1 bias
	if err := binary.Read(f, binary.LittleEndian, &n.L1Bias); err != nil {
		return fmt.Errorf("failed to read L1 bias: %w", err)
	}

	// Read L2 weights
	for i := 0; i < L1Size*2; i++ {
		if err := binary.Read(f, binary.LittleEndian, &n.L2Weights[i]); err != nil {
			return fmt.Errorf("failed to read L2 weights at %d: %w", i, err)
		}
	}

	// Read L2 bias
	if err := binary.Read(f, binary.LittleEndian, &n.L2Bias); err != nil {
		return fmt.Errorf("failed to read L2 bias: %w", err)
	}

	// Read output weights
	if err := binary.Read(f, binary.LittleEndian, &n.OutputWeights); err != nil {
		return fmt.Errorf("failed to read output weights: %w", err)
	}

	// Read output bias
	if err := binary.Read(f, binary.LittleEndian, &n.OutputBias); err != nil {
		return fmt.Errorf("failed to read output bias: %w", err)
	}

	return nil
}

// SaveWeights saves network weights to a binary file.
func (n *Network) SaveWeights(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create weights file: %w", err)
	}
	defer f.Close()

	// Write header
	header := FileHeader{
		Magic:   MagicNumber,
		Version: Version,
		L1Size:  L1Size,
		L2Size:  L2Size,
	}
	if err := binary.Write(f, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write L1 weights
	for i := 0; i < HalfKPSize; i++ {
		if err := binary.Write(f, binary.LittleEndian, &n.L1Weights[i]); err != nil {
			return fmt.Errorf("failed to write L1 weights at %d: %w", i, err)
		}
	}

	// Write L1 bias
	if err := binary.Write(f, binary.LittleEndian, &n.L1Bias); err != nil {
		return fmt.Errorf("failed to write L1 bias: %w", err)
	}

	// Write L2 weights
	for i := 0; i < L1Size*2; i++ {
		if err := binary.Write(f, binary.LittleEndian, &n.L2Weights[i]); err != nil {
			return fmt.Errorf("failed to write L2 weights at %d: %w", i, err)
		}
	}

	// Write L2 bias
	if err := binary.Write(f, binary.LittleEndian, &n.L2Bias); err != nil {
		return fmt.Errorf("failed to write L2 bias: %w", err)
	}

	// Write output weights
	if err := binary.Write(f, binary.LittleEndian, &n.OutputWeights); err != nil {
		return fmt.Errorf("failed to write output weights: %w", err)
	}

	// Write output bias
	if err := binary.Write(f, binary.LittleEndian, &n.OutputBias); err != nil {
		return fmt.Errorf("failed to write output bias: %w", err)
	}

	return nil
}

// LoadWeightsFromReader loads network weights from an io.Reader.
func (n *Network) LoadWeightsFromReader(r io.Reader) error {
	// Read header
	var header FileHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Validate header
	if header.Magic != MagicNumber {
		return fmt.Errorf("invalid magic number: expected %x, got %x", MagicNumber, header.Magic)
	}
	if header.Version != Version {
		return fmt.Errorf("unsupported version: expected %d, got %d", Version, header.Version)
	}

	// Read L1 weights
	for i := 0; i < HalfKPSize; i++ {
		if err := binary.Read(r, binary.LittleEndian, &n.L1Weights[i]); err != nil {
			return fmt.Errorf("failed to read L1 weights at %d: %w", i, err)
		}
	}

	// Read L1 bias
	if err := binary.Read(r, binary.LittleEndian, &n.L1Bias); err != nil {
		return fmt.Errorf("failed to read L1 bias: %w", err)
	}

	// Read L2 weights
	for i := 0; i < L1Size*2; i++ {
		if err := binary.Read(r, binary.LittleEndian, &n.L2Weights[i]); err != nil {
			return fmt.Errorf("failed to read L2 weights at %d: %w", i, err)
		}
	}

	// Read L2 bias
	if err := binary.Read(r, binary.LittleEndian, &n.L2Bias); err != nil {
		return fmt.Errorf("failed to read L2 bias: %w", err)
	}

	// Read output weights
	if err := binary.Read(r, binary.LittleEndian, &n.OutputWeights); err != nil {
		return fmt.Errorf("failed to read output weights: %w", err)
	}

	// Read output bias
	if err := binary.Read(r, binary.LittleEndian, &n.OutputBias); err != nil {
		return fmt.Errorf("failed to read output bias: %w", err)
	}

	return nil
}
