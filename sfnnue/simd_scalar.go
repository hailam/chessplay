//go:build !arm64 && (!goexperiment.simd || !amd64)

// Scalar fallback for NNUE operations when SIMD is not available.
// Used when:
// - Building on non-ARM64, non-AMD64 platforms (e.g., 386, riscv64)
// - Building on AMD64 without GOEXPERIMENT=simd
// ARM64 uses simd_neon.go with NEON assembly.
// AMD64 with GOEXPERIMENT=simd uses simd.go.

package sfnnue

// SIMDAddInt16 adds weights to accumulator (scalar fallback).
// dst[i] += src[i] for all i in range
func SIMDAddInt16(dst, src []int16) {
	for i := range dst {
		dst[i] += src[i]
	}
}

// SIMDSubInt16 subtracts weights from accumulator (scalar fallback).
// dst[i] -= src[i] for all i in range
func SIMDSubInt16(dst, src []int16) {
	for i := range dst {
		dst[i] -= src[i]
	}
}

// SIMDAddInt32 adds weights to PSQT accumulator (scalar fallback).
// dst[i] += src[i] for all i in range
func SIMDAddInt32(dst, src []int32) {
	for i := range dst {
		dst[i] += src[i]
	}
}

// SIMDSubInt32 subtracts weights from PSQT accumulator (scalar fallback).
// dst[i] -= src[i] for all i in range
func SIMDSubInt32(dst, src []int32) {
	for i := range dst {
		dst[i] -= src[i]
	}
}

// SIMDCopyInt16 copies src to dst (scalar fallback).
func SIMDCopyInt16(dst, src []int16) {
	copy(dst, src)
}

// SIMDCopyInt32 copies src to dst (scalar fallback).
func SIMDCopyInt32(dst, src []int32) {
	copy(dst, src)
}

// SIMDAddInt16Offset adds weights to accumulator with offset (scalar fallback).
// dst[i] += src[offset+i] for i in [0, count)
func SIMDAddInt16Offset(dst []int16, src []int16, offset, count int) {
	for i := 0; i < count; i++ {
		dst[i] += src[offset+i]
	}
}

// SIMDSubInt16Offset subtracts weights from accumulator with offset (scalar fallback).
// dst[i] -= src[offset+i] for i in [0, count)
func SIMDSubInt16Offset(dst []int16, src []int16, offset, count int) {
	for i := 0; i < count; i++ {
		dst[i] -= src[offset+i]
	}
}

// SIMDDotProductInt8Uint8 computes dot product of int8 weights and uint8 inputs (scalar fallback).
func SIMDDotProductInt8Uint8(weights []int8, inputs []uint8, count int) int32 {
	var sum int32
	for i := 0; i < count; i++ {
		sum += int32(weights[i]) * int32(inputs[i])
	}
	return sum
}

// SIMDClippedReLU applies ClippedReLU activation (scalar fallback).
// clamp(x >> shift, 0, 127)
func SIMDClippedReLU(input []int32, output []uint8, shift int) {
	for i := range input {
		val := input[i] >> shift
		if val < 0 {
			val = 0
		} else if val > 127 {
			val = 127
		}
		output[i] = uint8(val)
	}
}
