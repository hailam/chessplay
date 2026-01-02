// I/O utilities for NNUE binary format.
// Ported from Stockfish src/nnue/nnue_common.h

package common

import (
	"encoding/binary"
	"io"
)

// MaxSimdWidth is the maximum SIMD register width in bytes.
// Ported from nnue_common.h:66
const MaxSimdWidth = 32

// CeilToMultiple rounds n up to be a multiple of base.
func CeilToMultiple(n, base int) int {
	return (n + base - 1) / base * base
}

// ReadLittleEndian reads a value from a stream in little-endian order.
// Works with any fixed-size integer type.
func ReadLittleEndian[T any](r io.Reader) (T, error) {
	var result T
	err := binary.Read(r, binary.LittleEndian, &result)
	return result, err
}

// ReadLittleEndianSlice reads a slice of values in little-endian order.
func ReadLittleEndianSlice[T any](r io.Reader, out []T) error {
	return binary.Read(r, binary.LittleEndian, out)
}

// WriteLittleEndian writes a value to a stream in little-endian order.
func WriteLittleEndian[T any](w io.Writer, value T) error {
	return binary.Write(w, binary.LittleEndian, value)
}
