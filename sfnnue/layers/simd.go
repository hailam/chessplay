//go:build !arm64

// Scalar fallback for layer SIMD operations.
// Used on non-ARM64 platforms.
// ARM64 uses simd_arm64.go with NEON assembly.

package layers

// SIMDDotProductInt8Uint8 computes dot product of int8 weights and uint8 inputs.
// Scalar implementation - the SIMD version is in the parent sfnnue package.
// This is kept simple for now; full SIMD will be used at the sfnnue level.
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

// SIMDSparseChunkMulAcc processes one non-zero input chunk across all outputs.
// Scalar fallback implementation.
func SIMDSparseChunkMulAcc(output []int32, weights []int8, outLen int, inputChunk uint32) {
	if outLen == 0 || inputChunk == 0 {
		return
	}

	// Unpack input bytes
	b0 := uint8(inputChunk)
	b1 := uint8(inputChunk >> 8)
	b2 := uint8(inputChunk >> 16)
	b3 := uint8(inputChunk >> 24)

	// Process each output
	for k := 0; k < outLen; k++ {
		weightOffset := k * 4
		output[k] += int32(weights[weightOffset+0]) * int32(b0)
		output[k] += int32(weights[weightOffset+1]) * int32(b1)
		output[k] += int32(weights[weightOffset+2]) * int32(b2)
		output[k] += int32(weights[weightOffset+3]) * int32(b3)
	}
}
