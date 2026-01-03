//go:build arm64 && !goexperiment.simd

// ARM64 NEON SIMD operations for NNUE evaluation.
// Uses pure Go assembly with ARM64 NEON instructions.
// No CGO required.

package sfnnue

import "unsafe"

// Assembly function declarations (implemented in simd_arm64.s)

//go:noescape
func neonAddInt16(dst, src unsafe.Pointer, n int)

//go:noescape
func neonSubInt16(dst, src unsafe.Pointer, n int)

//go:noescape
func neonCopyInt16(dst, src unsafe.Pointer, n int)

//go:noescape
func neonAddInt16Offset(dst, src unsafe.Pointer, offset, count int)

//go:noescape
func neonSubInt16Offset(dst, src unsafe.Pointer, offset, count int)

// SIMDAddInt16 adds src to dst using ARM64 NEON.
// dst[i] += src[i] for all i in range
func SIMDAddInt16(dst, src []int16) {
	n := len(dst)
	if n == 0 || n != len(src) {
		return
	}

	// Process 8 elements at a time (128-bit NEON)
	simdCount := n &^ 7 // Round down to multiple of 8
	if simdCount > 0 {
		neonAddInt16(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), simdCount)
	}

	// Handle remainder
	for i := simdCount; i < n; i++ {
		dst[i] += src[i]
	}
}

// SIMDSubInt16 subtracts src from dst using ARM64 NEON.
// dst[i] -= src[i] for all i in range
func SIMDSubInt16(dst, src []int16) {
	n := len(dst)
	if n == 0 || n != len(src) {
		return
	}

	simdCount := n &^ 7
	if simdCount > 0 {
		neonSubInt16(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), simdCount)
	}

	for i := simdCount; i < n; i++ {
		dst[i] -= src[i]
	}
}

// SIMDAddInt32 adds src to dst (scalar - NEON int32 not implemented yet).
func SIMDAddInt32(dst, src []int32) {
	for i := range dst {
		dst[i] += src[i]
	}
}

// SIMDSubInt32 subtracts src from dst (scalar - NEON int32 not implemented yet).
func SIMDSubInt32(dst, src []int32) {
	for i := range dst {
		dst[i] -= src[i]
	}
}

// SIMDCopyInt16 copies src to dst using ARM64 NEON.
func SIMDCopyInt16(dst, src []int16) {
	n := len(dst)
	if n > len(src) {
		n = len(src)
	}
	if n == 0 {
		return
	}

	simdCount := n &^ 7
	if simdCount > 0 {
		neonCopyInt16(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), simdCount)
	}

	for i := simdCount; i < n; i++ {
		dst[i] = src[i]
	}
}

// SIMDCopyInt32 copies src to dst (uses built-in copy).
func SIMDCopyInt32(dst, src []int32) {
	copy(dst, src)
}

// SIMDAddInt16Offset adds src[offset:offset+count] to dst[0:count] using NEON.
func SIMDAddInt16Offset(dst []int16, src []int16, offset, count int) {
	if count == 0 || offset+count > len(src) || count > len(dst) {
		return
	}

	simdCount := count &^ 7
	if simdCount > 0 {
		neonAddInt16Offset(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), offset, simdCount)
	}

	for i := simdCount; i < count; i++ {
		dst[i] += src[offset+i]
	}
}

// SIMDSubInt16Offset subtracts src[offset:offset+count] from dst[0:count] using NEON.
func SIMDSubInt16Offset(dst []int16, src []int16, offset, count int) {
	if count == 0 || offset+count > len(src) || count > len(dst) {
		return
	}

	simdCount := count &^ 7
	if simdCount > 0 {
		neonSubInt16Offset(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), offset, simdCount)
	}

	for i := simdCount; i < count; i++ {
		dst[i] -= src[offset+i]
	}
}

// SIMDDotProductInt8Uint8 computes dot product (scalar fallback).
func SIMDDotProductInt8Uint8(weights []int8, inputs []uint8, count int) int32 {
	var sum int32
	// Unroll by 4 for better performance
	i := 0
	for ; i+4 <= count; i += 4 {
		sum += int32(weights[i]) * int32(inputs[i])
		sum += int32(weights[i+1]) * int32(inputs[i+1])
		sum += int32(weights[i+2]) * int32(inputs[i+2])
		sum += int32(weights[i+3]) * int32(inputs[i+3])
	}
	for ; i < count; i++ {
		sum += int32(weights[i]) * int32(inputs[i])
	}
	return sum
}

// SIMDClippedReLU applies ClippedReLU activation (scalar fallback).
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
