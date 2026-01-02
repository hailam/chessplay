// NNUE common types and utilities.
// Ported from Stockfish src/nnue/nnue_common.h

package sfnnue

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Type aliases matching Stockfish
type BiasType = int16
type WeightType = int16
type ThreatWeightType = int8
type PSQTWeightType = int32
type IndexType = uint32
type TransformedFeatureType = uint8

// Version of the evaluation file (nnue_common.h:58)
const Version uint32 = 0x7AF32F20

// Constants used in evaluation value calculation (nnue_common.h:61-62)
const (
	OutputScale     = 16
	WeightScaleBits = 6
)

// Size of cache line in bytes (nnue_common.h:65)
const CacheLineSize = 64

// LEB128 compression magic string (nnue_common.h:67-68)
const Leb128MagicString = "COMPRESSED_LEB128"
const Leb128MagicStringSize = len(Leb128MagicString)

// SIMD width constants (nnue_common.h:71-81)
const (
	SimdWidth    = 32 // AVX2
	MaxSimdWidth = 32
)

// CeilToMultiple rounds n up to be a multiple of base (nnue_common.h:87-90)
func CeilToMultiple[T ~int | ~uint | ~int32 | ~uint32](n, base T) T {
	return (n + base - 1) / base * base
}

// ReadLittleEndian reads an integer from a stream in little-endian order (nnue_common.h:96-115)
func ReadLittleEndian[T int8 | uint8 | int16 | uint16 | int32 | uint32](r io.Reader) (T, error) {
	var result T
	err := binary.Read(r, binary.LittleEndian, &result)
	return result, err
}

// ReadLittleEndianSlice reads integers in bulk from a little-endian stream (nnue_common.h:151-158)
func ReadLittleEndianSlice[T int8 | uint8 | int16 | uint16 | int32 | uint32](r io.Reader, out []T) error {
	return binary.Read(r, binary.LittleEndian, out)
}

// WriteLittleEndian writes an integer to a stream in little-endian order (nnue_common.h:122-146)
func WriteLittleEndian[T int8 | uint8 | int16 | uint16 | int32 | uint32](w io.Writer, value T) error {
	return binary.Write(w, binary.LittleEndian, value)
}

// WriteLittleEndianSlice writes integers in bulk to a little-endian stream (nnue_common.h:163-170)
func WriteLittleEndianSlice[T int8 | uint8 | int16 | uint16 | int32 | uint32](w io.Writer, values []T) error {
	return binary.Write(w, binary.LittleEndian, values)
}

// ReadLEB128 reads N signed integers from a stream compressed using signed LEB128 format.
// See https://en.wikipedia.org/wiki/LEB128 for a description of the compression scheme.
// Ported from nnue_common.h:176-220
func ReadLEB128[T int16 | int32](r io.Reader, out []T) error {
	// Check the presence of our LEB128 magic string
	magic := make([]byte, Leb128MagicStringSize)
	if _, err := io.ReadFull(r, magic); err != nil {
		return fmt.Errorf("failed to read LEB128 magic: %w", err)
	}
	if string(magic) != Leb128MagicString {
		return fmt.Errorf("invalid LEB128 magic: expected %q, got %q", Leb128MagicString, string(magic))
	}

	// Read byte count
	bytesLeft, err := ReadLittleEndian[uint32](r)
	if err != nil {
		return fmt.Errorf("failed to read LEB128 byte count: %w", err)
	}

	const bufSize = 4096
	buf := make([]byte, bufSize)
	bufPos := uint32(bufSize) // Start empty to trigger first read

	for i := range out {
		var result T
		var shift uint

		for {
			if bufPos == bufSize {
				toRead := min(bytesLeft, bufSize)
				if _, err := io.ReadFull(r, buf[:toRead]); err != nil {
					return fmt.Errorf("failed to read LEB128 data: %w", err)
				}
				bufPos = 0
			}

			b := buf[bufPos]
			bufPos++
			bytesLeft--

			result |= T(b&0x7f) << shift
			shift += 7

			if b&0x80 == 0 {
				// Sign extend if needed
				bitSize := uint(8 * unsafe_Sizeof(result))
				if shift < bitSize && (b&0x40) != 0 {
					result |= ^T(0) << shift
				}
				break
			}

			if shift >= uint(8*unsafe_Sizeof(result)) {
				break
			}
		}

		out[i] = result
	}

	if bytesLeft != 0 {
		return fmt.Errorf("LEB128 bytes remaining: %d", bytesLeft)
	}

	return nil
}

// WriteLEB128 writes signed integers to a stream with LEB128 compression.
// Ported from nnue_common.h:227-285
func WriteLEB128[T int16 | int32](w io.Writer, values []T) error {
	// Write our LEB128 magic string
	if _, err := w.Write([]byte(Leb128MagicString)); err != nil {
		return fmt.Errorf("failed to write LEB128 magic: %w", err)
	}

	// First pass: count bytes needed
	var byteCount uint32
	for _, value := range values {
		v := value
		for {
			b := byte(v & 0x7f)
			v >>= 7
			byteCount++
			if (b&0x40 == 0 && v == 0) || (b&0x40 != 0 && v == -1) {
				break
			}
		}
	}

	// Write byte count
	if err := WriteLittleEndian(w, byteCount); err != nil {
		return fmt.Errorf("failed to write LEB128 byte count: %w", err)
	}

	// Second pass: write encoded bytes
	const bufSize = 4096
	buf := make([]byte, 0, bufSize)

	flush := func() error {
		if len(buf) > 0 {
			if _, err := w.Write(buf); err != nil {
				return err
			}
			buf = buf[:0]
		}
		return nil
	}

	for _, value := range values {
		v := value
		for {
			b := byte(v & 0x7f)
			v >>= 7
			if (b&0x40 == 0 && v == 0) || (b&0x40 != 0 && v == -1) {
				buf = append(buf, b)
				if len(buf) == bufSize {
					if err := flush(); err != nil {
						return err
					}
				}
				break
			}
			buf = append(buf, b|0x80)
			if len(buf) == bufSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}

	return flush()
}

// unsafe_Sizeof returns the size in bytes of a value of type T.
// This is a helper to avoid importing unsafe.
func unsafe_Sizeof[T any](v T) int {
	var zero T
	switch any(zero).(type) {
	case int8, uint8:
		return 1
	case int16, uint16:
		return 2
	case int32, uint32:
		return 4
	case int64, uint64:
		return 8
	default:
		return 8 // Default to 8 bytes
	}
}

// min returns the minimum of two values
func min[T ~int | ~uint | ~uint32](a, b T) T {
	if a < b {
		return a
	}
	return b
}
