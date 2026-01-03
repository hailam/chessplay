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
