//go:build arm64

// ARM64 NEON SIMD operations for affine transform layers.
// Uses pure Go assembly with ARM64 NEON instructions.
// No CGO required.

package layers

import "unsafe"

// Assembly function declaration (implemented in simd_arm64.s)

//go:noescape
func neonDotProductInt8Uint8(weights, inputs unsafe.Pointer, count int) int32

// SIMDDotProductInt8Uint8 computes dot product of int8 weights and uint8 inputs.
// Uses ARM64 NEON for vectorized computation.
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

	return neonDotProductInt8Uint8(
		unsafe.Pointer(&weights[0]),
		unsafe.Pointer(&inputs[0]),
		count,
	)
}
