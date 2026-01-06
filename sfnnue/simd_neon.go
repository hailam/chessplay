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

//go:noescape
func neonAddInt16x2(dst, src unsafe.Pointer, n int)

//go:noescape
func neonSubInt16x2(dst, src unsafe.Pointer, n int)

//go:noescape
func neonCopyInt16x2(dst, src unsafe.Pointer, n int)

//go:noescape
func neonAddInt16OffsetX2(dst, src unsafe.Pointer, offset, count int)

//go:noescape
func neonSubInt16OffsetX2(dst, src unsafe.Pointer, offset, count int)

//go:noescape
func neonAddInt32(dst, src unsafe.Pointer, n int)

//go:noescape
func neonSubInt32(dst, src unsafe.Pointer, n int)

//go:noescape
func neonCopyInt32(dst, src unsafe.Pointer, n int)

//go:noescape
func prefetchL1(addr unsafe.Pointer)

//go:noescape
func prefetchL2(addr unsafe.Pointer)

//go:noescape
func prefetchLine(addr unsafe.Pointer, count int)

//go:noescape
func neonDotProductInt8Uint8(weights, inputs unsafe.Pointer, count int) int32

//go:noescape
func neonClippedReLU(input, output unsafe.Pointer, count, shift int)

//go:noescape
func neonTransformClampMul(acc0, acc1, output unsafe.Pointer, count, maxVal int)

// PrefetchL1 prefetches memory at addr into L1 cache.
func PrefetchL1(addr unsafe.Pointer) {
	prefetchL1(addr)
}

// PrefetchL2 prefetches memory at addr into L2 cache.
func PrefetchL2(addr unsafe.Pointer) {
	prefetchL2(addr)
}

// PrefetchLines prefetches count cache lines (64 bytes each) starting at addr.
func PrefetchLines(addr unsafe.Pointer, count int) {
	prefetchLine(addr, count)
}

// SIMDAddInt16 adds src to dst using ARM64 NEON.
// dst[i] += src[i] for all i in range
// Uses dual-vector (16 elements/iter) when possible for better throughput.
func SIMDAddInt16(dst, src []int16) {
	n := len(dst)
	if n == 0 || n != len(src) {
		return
	}

	processed := 0

	// Process 16 elements at a time (dual 128-bit NEON)
	simd2Count := n &^ 15 // Round down to multiple of 16
	if simd2Count > 0 {
		neonAddInt16x2(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), simd2Count)
		processed = simd2Count
	}

	// Process remaining 8-15 elements with single-vector
	simdCount := (n - processed) &^ 7
	if simdCount > 0 {
		neonAddInt16(unsafe.Pointer(&dst[processed]), unsafe.Pointer(&src[processed]), simdCount)
		processed += simdCount
	}

	// Handle remainder (0-7 elements)
	for i := processed; i < n; i++ {
		dst[i] += src[i]
	}
}

// SIMDSubInt16 subtracts src from dst using ARM64 NEON.
// dst[i] -= src[i] for all i in range
// Uses dual-vector (16 elements/iter) when possible for better throughput.
func SIMDSubInt16(dst, src []int16) {
	n := len(dst)
	if n == 0 || n != len(src) {
		return
	}

	processed := 0

	// Process 16 elements at a time (dual 128-bit NEON)
	simd2Count := n &^ 15
	if simd2Count > 0 {
		neonSubInt16x2(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), simd2Count)
		processed = simd2Count
	}

	// Process remaining 8-15 elements with single-vector
	simdCount := (n - processed) &^ 7
	if simdCount > 0 {
		neonSubInt16(unsafe.Pointer(&dst[processed]), unsafe.Pointer(&src[processed]), simdCount)
		processed += simdCount
	}

	// Handle remainder
	for i := processed; i < n; i++ {
		dst[i] -= src[i]
	}
}

// SIMDAddInt32 adds src to dst using ARM64 NEON.
// dst[i] += src[i] for all i in range
func SIMDAddInt32(dst, src []int32) {
	n := len(dst)
	if n == 0 || n != len(src) {
		return
	}

	// Process 4 elements at a time (128-bit NEON with 32-bit ints)
	simdCount := n &^ 3 // Round down to multiple of 4
	if simdCount > 0 {
		neonAddInt32(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), simdCount)
	}

	// Handle remainder
	for i := simdCount; i < n; i++ {
		dst[i] += src[i]
	}
}

// SIMDSubInt32 subtracts src from dst using ARM64 NEON.
// dst[i] -= src[i] for all i in range
func SIMDSubInt32(dst, src []int32) {
	n := len(dst)
	if n == 0 || n != len(src) {
		return
	}

	simdCount := n &^ 3
	if simdCount > 0 {
		neonSubInt32(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), simdCount)
	}

	for i := simdCount; i < n; i++ {
		dst[i] -= src[i]
	}
}

// SIMDCopyInt16 copies src to dst using ARM64 NEON.
// Uses dual-vector (16 elements/iter) when possible for better throughput.
func SIMDCopyInt16(dst, src []int16) {
	n := len(dst)
	if n > len(src) {
		n = len(src)
	}
	if n == 0 {
		return
	}

	processed := 0

	// Process 16 elements at a time
	simd2Count := n &^ 15
	if simd2Count > 0 {
		neonCopyInt16x2(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), simd2Count)
		processed = simd2Count
	}

	// Process remaining 8-15 elements
	simdCount := (n - processed) &^ 7
	if simdCount > 0 {
		neonCopyInt16(unsafe.Pointer(&dst[processed]), unsafe.Pointer(&src[processed]), simdCount)
		processed += simdCount
	}

	for i := processed; i < n; i++ {
		dst[i] = src[i]
	}
}

// SIMDCopyInt32 copies src to dst using ARM64 NEON.
func SIMDCopyInt32(dst, src []int32) {
	n := len(dst)
	if n > len(src) {
		n = len(src)
	}
	if n == 0 {
		return
	}

	simdCount := n &^ 3
	if simdCount > 0 {
		neonCopyInt32(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), simdCount)
	}

	for i := simdCount; i < n; i++ {
		dst[i] = src[i]
	}
}

// SIMDAddInt16Offset adds src[offset:offset+count] to dst[0:count] using NEON.
// Uses dual-vector when possible.
func SIMDAddInt16Offset(dst []int16, src []int16, offset, count int) {
	if count == 0 || offset+count > len(src) || count > len(dst) {
		return
	}

	processed := 0

	// Process 16 elements at a time
	simd2Count := count &^ 15
	if simd2Count > 0 {
		neonAddInt16OffsetX2(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), offset, simd2Count)
		processed = simd2Count
	}

	// Process remaining 8-15 elements
	simdCount := (count - processed) &^ 7
	if simdCount > 0 {
		neonAddInt16Offset(unsafe.Pointer(&dst[processed]), unsafe.Pointer(&src[0]), offset+processed, simdCount)
		processed += simdCount
	}

	for i := processed; i < count; i++ {
		dst[i] += src[offset+i]
	}
}

// SIMDSubInt16Offset subtracts src[offset:offset+count] from dst[0:count] using NEON.
// Uses dual-vector when possible.
func SIMDSubInt16Offset(dst []int16, src []int16, offset, count int) {
	if count == 0 || offset+count > len(src) || count > len(dst) {
		return
	}

	processed := 0

	// Process 16 elements at a time
	simd2Count := count &^ 15
	if simd2Count > 0 {
		neonSubInt16OffsetX2(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), offset, simd2Count)
		processed = simd2Count
	}

	// Process remaining 8-15 elements
	simdCount := (count - processed) &^ 7
	if simdCount > 0 {
		neonSubInt16Offset(unsafe.Pointer(&dst[processed]), unsafe.Pointer(&src[0]), offset+processed, simdCount)
		processed += simdCount
	}

	for i := processed; i < count; i++ {
		dst[i] -= src[offset+i]
	}
}

// SIMDDotProductInt8Uint8 computes dot product using ARM64 NEON.
// Returns: sum(weights[i] * inputs[i]) for i in [0, count)
func SIMDDotProductInt8Uint8(weights []int8, inputs []uint8, count int) int32 {
	if count == 0 {
		return 0
	}
	if count > len(weights) {
		count = len(weights)
	}
	if count > len(inputs) {
		count = len(inputs)
	}
	return neonDotProductInt8Uint8(unsafe.Pointer(&weights[0]), unsafe.Pointer(&inputs[0]), count)
}

// SIMDClippedReLU applies ClippedReLU activation using ARM64 NEON.
// Applies: output[i] = clamp(input[i] >> shift, 0, 127)
func SIMDClippedReLU(input []int32, output []uint8, shift int) {
	count := len(input)
	if count == 0 {
		return
	}
	if count > len(output) {
		count = len(output)
	}
	neonClippedReLU(unsafe.Pointer(&input[0]), unsafe.Pointer(&output[0]), count, shift)
}

// SIMDTransformClampMul performs the fused Transform inner loop using ARM64 NEON.
// Computes: output[i] = uint8((clamp(acc0[i], 0, maxVal) * clamp(acc1[i], 0, maxVal)) >> 9)
// acc0 and acc1 are the two halves of the accumulation array.
// This fuses clamp + multiply + shift into a single SIMD operation.
func SIMDTransformClampMul(acc0, acc1 []int16, output []uint8, maxVal int) {
	count := len(acc0)
	if count == 0 {
		return
	}
	if count > len(acc1) {
		count = len(acc1)
	}
	if count > len(output) {
		count = len(output)
	}
	neonTransformClampMul(unsafe.Pointer(&acc0[0]), unsafe.Pointer(&acc1[0]), unsafe.Pointer(&output[0]), count, maxVal)
}
