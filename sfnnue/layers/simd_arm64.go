//go:build arm64

// ARM64 NEON SIMD operations for affine transform layers.
// Uses pure Go assembly with ARM64 NEON instructions.
// No CGO required.

package layers

import "unsafe"

// Assembly function declaration (implemented in simd_arm64.s)

//go:noescape
func neonDotProductInt8Uint8(weights, inputs unsafe.Pointer, count int) int32

//go:noescape
func neonSparseChunkMulAcc(output, weights unsafe.Pointer, outLen int, inputChunk uint32)

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

// SIMDSparseChunkMulAcc processes one non-zero input chunk across all outputs.
// Uses ARM64 NEON for vectorized computation with horizontal sums.
// output: int32 output array
// weights: int8 weights at colOffset (contiguous for all outputs)
// outLen: number of outputs (must be multiple of 4 for SIMD, typically 16)
// inputChunk: packed input bytes as uint32
func SIMDSparseChunkMulAcc(output []int32, weights []int8, outLen int, inputChunk uint32) {
	if outLen == 0 || inputChunk == 0 {
		return
	}
	neonSparseChunkMulAcc(
		unsafe.Pointer(&output[0]),
		unsafe.Pointer(&weights[0]),
		outLen,
		inputChunk,
	)
}
