package layers

import (
	"testing"
)

func TestSIMDDotProductInt8Uint8(t *testing.T) {
	// Test basic functionality
	weights := make([]int8, 256)
	inputs := make([]uint8, 256)

	// Fill with test data
	for i := range weights {
		weights[i] = int8(i % 128)
		inputs[i] = uint8(i % 256)
	}

	// Compute expected result (scalar)
	var expected int32
	for i := 0; i < 256; i++ {
		expected += int32(weights[i]) * int32(inputs[i])
	}

	// Compute using SIMD
	result := SIMDDotProductInt8Uint8(weights, inputs, 256)

	if result != expected {
		t.Errorf("DotProduct mismatch: got %d, expected %d", result, expected)
	}

	t.Logf("DotProduct result: %d (expected %d)", result, expected)
}

func TestSIMDDotProductInt8Uint8_Negative(t *testing.T) {
	// Test with negative weights
	weights := make([]int8, 128)
	inputs := make([]uint8, 128)

	// Fill with negative weights
	for i := range weights {
		weights[i] = int8(-64 + i%128)
		inputs[i] = uint8(i)
	}

	// Compute expected result
	var expected int32
	for i := 0; i < 128; i++ {
		expected += int32(weights[i]) * int32(inputs[i])
	}

	result := SIMDDotProductInt8Uint8(weights, inputs, 128)

	if result != expected {
		t.Errorf("DotProduct with negatives mismatch: got %d, expected %d", result, expected)
	}

	t.Logf("DotProduct with negatives: %d (expected %d)", result, expected)
}

func TestSIMDDotProductInt8Uint8_SmallCount(t *testing.T) {
	// Test with counts that don't align to SIMD width
	for count := 1; count <= 20; count++ {
		weights := make([]int8, count)
		inputs := make([]uint8, count)

		for i := 0; i < count; i++ {
			weights[i] = int8(i + 1)
			inputs[i] = uint8(i + 1)
		}

		var expected int32
		for i := 0; i < count; i++ {
			expected += int32(weights[i]) * int32(inputs[i])
		}

		result := SIMDDotProductInt8Uint8(weights, inputs, count)

		if result != expected {
			t.Errorf("Count %d: got %d, expected %d", count, result, expected)
		}
	}
}

// BenchmarkSIMDDotProductInt8Uint8 benchmarks the dot product operation
func BenchmarkSIMDDotProductInt8Uint8_256(b *testing.B) {
	weights := make([]int8, 256)
	inputs := make([]uint8, 256)

	for i := range weights {
		weights[i] = int8(i % 128)
		inputs[i] = uint8(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SIMDDotProductInt8Uint8(weights, inputs, 256)
	}
}

func BenchmarkSIMDDotProductInt8Uint8_512(b *testing.B) {
	weights := make([]int8, 512)
	inputs := make([]uint8, 512)

	for i := range weights {
		weights[i] = int8(i % 128)
		inputs[i] = uint8(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SIMDDotProductInt8Uint8(weights, inputs, 512)
	}
}

func BenchmarkSIMDDotProductInt8Uint8_1024(b *testing.B) {
	weights := make([]int8, 1024)
	inputs := make([]uint8, 1024)

	for i := range weights {
		weights[i] = int8(i % 128)
		inputs[i] = uint8(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SIMDDotProductInt8Uint8(weights, inputs, 1024)
	}
}
