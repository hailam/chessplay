//go:build goexperiment.simd && amd64
// +build goexperiment.simd,amd64

// SIMD-accelerated operations for NNUE evaluation.
// Requires Go 1.26+ with GOEXPERIMENT=simd on AMD64 architecture.
// ARM64 support is not yet available in Go's experimental SIMD package.

package sfnnue

import (
	"simd/archsimd"
)

// SIMD constants
const (
	// Number of int16 values processed per SIMD iteration (256-bit AVX2)
	simdInt16Width = 16

	// Number of int32 values processed per SIMD iteration (256-bit AVX2)
	simdInt32Width = 8
)

// SIMDAddInt16 adds weights to accumulator using SIMD.
// dst[i] += src[i] for all i in range
func SIMDAddInt16(dst, src []int16) {
	n := len(dst)
	if n != len(src) {
		panic("SIMDAddInt16: slice length mismatch")
	}

	// Process 16 int16 values at a time (256-bit)
	i := 0
	for ; i+simdInt16Width <= n; i += simdInt16Width {
		d := archsimd.LoadInt16x16(dst[i:])
		s := archsimd.LoadInt16x16(src[i:])
		archsimd.StoreInt16x16(dst[i:], d.Add(s))
	}

	// Handle remaining elements
	for ; i < n; i++ {
		dst[i] += src[i]
	}
}

// SIMDSubInt16 subtracts weights from accumulator using SIMD.
// dst[i] -= src[i] for all i in range
func SIMDSubInt16(dst, src []int16) {
	n := len(dst)
	if n != len(src) {
		panic("SIMDSubInt16: slice length mismatch")
	}

	// Process 16 int16 values at a time (256-bit)
	i := 0
	for ; i+simdInt16Width <= n; i += simdInt16Width {
		d := archsimd.LoadInt16x16(dst[i:])
		s := archsimd.LoadInt16x16(src[i:])
		archsimd.StoreInt16x16(dst[i:], d.Sub(s))
	}

	// Handle remaining elements
	for ; i < n; i++ {
		dst[i] -= src[i]
	}
}

// SIMDAddInt32 adds weights to PSQT accumulator using SIMD.
// dst[i] += src[i] for all i in range
func SIMDAddInt32(dst, src []int32) {
	n := len(dst)
	if n != len(src) {
		panic("SIMDAddInt32: slice length mismatch")
	}

	// Process 8 int32 values at a time (256-bit)
	i := 0
	for ; i+simdInt32Width <= n; i += simdInt32Width {
		d := archsimd.LoadInt32x8(dst[i:])
		s := archsimd.LoadInt32x8(src[i:])
		archsimd.StoreInt32x8(dst[i:], d.Add(s))
	}

	// Handle remaining elements
	for ; i < n; i++ {
		dst[i] += src[i]
	}
}

// SIMDSubInt32 subtracts weights from PSQT accumulator using SIMD.
// dst[i] -= src[i] for all i in range
func SIMDSubInt32(dst, src []int32) {
	n := len(dst)
	if n != len(src) {
		panic("SIMDSubInt32: slice length mismatch")
	}

	// Process 8 int32 values at a time (256-bit)
	i := 0
	for ; i+simdInt32Width <= n; i += simdInt32Width {
		d := archsimd.LoadInt32x8(dst[i:])
		s := archsimd.LoadInt32x8(src[i:])
		archsimd.StoreInt32x8(dst[i:], d.Sub(s))
	}

	// Handle remaining elements
	for ; i < n; i++ {
		dst[i] -= src[i]
	}
}

// SIMDCopyInt16 copies src to dst using SIMD.
func SIMDCopyInt16(dst, src []int16) {
	n := len(dst)
	if n > len(src) {
		n = len(src)
	}

	// Process 16 int16 values at a time
	i := 0
	for ; i+simdInt16Width <= n; i += simdInt16Width {
		v := archsimd.LoadInt16x16(src[i:])
		archsimd.StoreInt16x16(dst[i:], v)
	}

	// Handle remaining elements
	for ; i < n; i++ {
		dst[i] = src[i]
	}
}

// SIMDCopyInt32 copies src to dst using SIMD.
func SIMDCopyInt32(dst, src []int32) {
	n := len(dst)
	if n > len(src) {
		n = len(src)
	}

	// Process 8 int32 values at a time
	i := 0
	for ; i+simdInt32Width <= n; i += simdInt32Width {
		v := archsimd.LoadInt32x8(src[i:])
		archsimd.StoreInt32x8(dst[i:], v)
	}

	// Handle remaining elements
	for ; i < n; i++ {
		dst[i] = src[i]
	}
}

// SIMDAddInt16Offset adds weights to accumulator with offset using SIMD.
// dst[i] += src[offset+i] for i in [0, count)
func SIMDAddInt16Offset(dst []int16, src []int16, offset, count int) {
	// Process 16 int16 values at a time
	i := 0
	for ; i+simdInt16Width <= count; i += simdInt16Width {
		d := archsimd.LoadInt16x16(dst[i:])
		s := archsimd.LoadInt16x16(src[offset+i:])
		archsimd.StoreInt16x16(dst[i:], d.Add(s))
	}

	// Handle remaining elements
	for ; i < count; i++ {
		dst[i] += src[offset+i]
	}
}

// SIMDSubInt16Offset subtracts weights from accumulator with offset using SIMD.
// dst[i] -= src[offset+i] for i in [0, count)
func SIMDSubInt16Offset(dst []int16, src []int16, offset, count int) {
	// Process 16 int16 values at a time
	i := 0
	for ; i+simdInt16Width <= count; i += simdInt16Width {
		d := archsimd.LoadInt16x16(dst[i:])
		s := archsimd.LoadInt16x16(src[offset+i:])
		archsimd.StoreInt16x16(dst[i:], d.Sub(s))
	}

	// Handle remaining elements
	for ; i < count; i++ {
		dst[i] -= src[offset+i]
	}
}

// SIMDDotProductInt8Uint8 computes dot product of int8 weights and uint8 inputs.
// Used for affine transform propagation: sum(weights[i] * inputs[i])
// Returns the accumulated int32 result.
func SIMDDotProductInt8Uint8(weights []int8, inputs []uint8, count int) int32 {
	var sum int32

	// Process 32 elements at a time using Int8x32
	i := 0
	for ; i+32 <= count; i += 32 {
		// Load weights as int8 and inputs as uint8
		// Note: Go 1.26 simd doesn't have direct int8*uint8 VPMADDUBSW
		// We'll use int16 intermediate for now
		for j := 0; j < 32; j++ {
			sum += int32(weights[i+j]) * int32(inputs[i+j])
		}
	}

	// Handle remaining elements
	for ; i < count; i++ {
		sum += int32(weights[i]) * int32(inputs[i])
	}

	return sum
}

// SIMDClippedReLU applies ClippedReLU activation: clamp(x >> shift, 0, 127)
// Input: int32 slice, Output: uint8 slice
func SIMDClippedReLU(input []int32, output []uint8, shift int) {
	n := len(input)

	// Process 8 int32 values at a time
	i := 0
	for ; i+simdInt32Width <= n; i += simdInt32Width {
		v := archsimd.LoadInt32x8(input[i:])

		// Shift right
		v = v.ShiftRight(shift)

		// Clamp to [0, 127] using Max/Min
		zero := archsimd.Int32x8{}
		maxVal := archsimd.BroadcastInt32x8(127)
		v = v.Max(zero).Min(maxVal)

		// Store to output (pack to uint8)
		for j := 0; j < 8; j++ {
			output[i+j] = uint8(v.Get(j))
		}
	}

	// Handle remaining elements
	for ; i < n; i++ {
		val := input[i] >> shift
		if val < 0 {
			val = 0
		} else if val > 127 {
			val = 127
		}
		output[i] = uint8(val)
	}
}
