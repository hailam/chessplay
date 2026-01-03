//go:build arm64

#include "textflag.h"

// ARM64 NEON SIMD operations for NNUE evaluation
// Uses raw WORD opcodes since Go assembler doesn't support NEON mnemonics

// NEON Opcode Reference:
// ld1 {v0.8h}, [x0]       = 0x4C407000  (load 8 x int16)
// ld1 {v1.8h}, [x1]       = 0x4C407021  (load 8 x int16)
// st1 {v0.8h}, [x0]       = 0x4C007000  (store 8 x int16)
// add v0.8h, v0.8h, v1.8h = 0x4E618400  (add 8 x int16)
// sub v0.8h, v0.8h, v1.8h = 0x6E618400  (sub 8 x int16)

// func neonAddInt16(dst, src unsafe.Pointer, n int)
// Adds n int16 values: dst[i] += src[i]
// Processes 8 elements (128-bit) per iteration
TEXT ·neonAddInt16(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD n+16(FP), R2       // count (must be multiple of 8)

	CBZ  R2, done_add       // if n == 0, return
loop_add:
	WORD $0x4C407000        // ld1 {v0.8h}, [x0]
	WORD $0x4C407021        // ld1 {v1.8h}, [x1]
	WORD $0x4E618400        // add v0.8h, v0.8h, v1.8h
	WORD $0x4C007000        // st1 {v0.8h}, [x0]
	ADD  $16, R0            // advance dst by 16 bytes (8 x int16)
	ADD  $16, R1            // advance src by 16 bytes
	SUBS $8, R2, R2         // n -= 8, set flags
	BNE  loop_add           // loop if n != 0
done_add:
	RET

// func neonSubInt16(dst, src unsafe.Pointer, n int)
// Subtracts n int16 values: dst[i] -= src[i]
// Processes 8 elements (128-bit) per iteration
TEXT ·neonSubInt16(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD n+16(FP), R2       // count (must be multiple of 8)

	CBZ  R2, done_sub       // if n == 0, return
loop_sub:
	WORD $0x4C407000        // ld1 {v0.8h}, [x0]
	WORD $0x4C407021        // ld1 {v1.8h}, [x1]
	WORD $0x6E618400        // sub v0.8h, v0.8h, v1.8h
	WORD $0x4C007000        // st1 {v0.8h}, [x0]
	ADD  $16, R0            // advance dst by 16 bytes
	ADD  $16, R1            // advance src by 16 bytes
	SUBS $8, R2, R2         // n -= 8, set flags
	BNE  loop_sub           // loop if n != 0
done_sub:
	RET

// func neonCopyInt16(dst, src unsafe.Pointer, n int)
// Copies n int16 values: dst[i] = src[i]
// Processes 8 elements (128-bit) per iteration
TEXT ·neonCopyInt16(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD n+16(FP), R2       // count (must be multiple of 8)

	CBZ  R2, done_copy      // if n == 0, return
loop_copy:
	WORD $0x4C407021        // ld1 {v1.8h}, [x1]
	WORD $0x4C007001        // st1 {v1.8h}, [x0]
	ADD  $16, R0            // advance dst by 16 bytes
	ADD  $16, R1            // advance src by 16 bytes
	SUBS $8, R2, R2         // n -= 8, set flags
	BNE  loop_copy          // loop if n != 0
done_copy:
	RET

// func neonAddInt16Offset(dst unsafe.Pointer, src unsafe.Pointer, offset, count int)
// Adds with offset: dst[i] += src[offset + i]
// Processes 8 elements per iteration
TEXT ·neonAddInt16Offset(SB), NOSPLIT, $0-32
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD offset+16(FP), R3  // offset (in elements)
	MOVD count+24(FP), R2   // count (must be multiple of 8)

	// Adjust src pointer by offset (offset * 2 bytes per int16)
	LSL  $1, R3, R3         // offset *= 2
	ADD  R3, R1, R1         // src += offset

	CBZ  R2, done_add_off   // if count == 0, return
loop_add_off:
	WORD $0x4C407000        // ld1 {v0.8h}, [x0]
	WORD $0x4C407021        // ld1 {v1.8h}, [x1]
	WORD $0x4E618400        // add v0.8h, v0.8h, v1.8h
	WORD $0x4C007000        // st1 {v0.8h}, [x0]
	ADD  $16, R0
	ADD  $16, R1
	SUBS $8, R2, R2
	BNE  loop_add_off
done_add_off:
	RET

// func neonSubInt16Offset(dst unsafe.Pointer, src unsafe.Pointer, offset, count int)
// Subtracts with offset: dst[i] -= src[offset + i]
// Processes 8 elements per iteration
TEXT ·neonSubInt16Offset(SB), NOSPLIT, $0-32
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD offset+16(FP), R3  // offset (in elements)
	MOVD count+24(FP), R2   // count (must be multiple of 8)

	// Adjust src pointer by offset (offset * 2 bytes per int16)
	LSL  $1, R3, R3         // offset *= 2
	ADD  R3, R1, R1         // src += offset

	CBZ  R2, done_sub_off   // if count == 0, return
loop_sub_off:
	WORD $0x4C407000        // ld1 {v0.8h}, [x0]
	WORD $0x4C407021        // ld1 {v1.8h}, [x1]
	WORD $0x6E618400        // sub v0.8h, v0.8h, v1.8h
	WORD $0x4C007000        // st1 {v0.8h}, [x0]
	ADD  $16, R0
	ADD  $16, R1
	SUBS $8, R2, R2
	BNE  loop_sub_off
done_sub_off:
	RET

// ============================================================================
// DUAL-VECTOR INT16 OPERATIONS (2x 128-bit = 16 elements per iteration)
// ============================================================================
// Multi-register NEON Opcode Reference:
// ld1 {v0.8h, v1.8h}, [x0]       = 0x4C40A400  (load 16 x int16)
// ld1 {v2.8h, v3.8h}, [x1]       = 0x4C40A422  (load 16 x int16)
// st1 {v0.8h, v1.8h}, [x0]       = 0x4C00A400  (store 16 x int16)
// add v0.8h, v0.8h, v2.8h        = 0x4E628400  (add 8 x int16)
// add v1.8h, v1.8h, v3.8h        = 0x4E638421  (add 8 x int16)
// sub v0.8h, v0.8h, v2.8h        = 0x6E628400  (sub 8 x int16)
// sub v1.8h, v1.8h, v3.8h        = 0x6E638421  (sub 8 x int16)

// func neonAddInt16x2(dst, src unsafe.Pointer, n int)
// Adds n int16 values: dst[i] += src[i]
// Processes 16 elements (2x 128-bit) per iteration
// n must be a multiple of 16
TEXT ·neonAddInt16x2(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD n+16(FP), R2       // count (must be multiple of 16)

	CBZ  R2, done_add2      // if n == 0, return
loop_add2:
	WORD $0x4C40A400        // ld1 {v0.8h, v1.8h}, [x0]
	WORD $0x4C40A422        // ld1 {v2.8h, v3.8h}, [x1]
	WORD $0x4E628400        // add v0.8h, v0.8h, v2.8h
	WORD $0x4E638421        // add v1.8h, v1.8h, v3.8h
	WORD $0x4C00A400        // st1 {v0.8h, v1.8h}, [x0]
	ADD  $32, R0            // advance dst by 32 bytes (16 x int16)
	ADD  $32, R1            // advance src by 32 bytes
	SUBS $16, R2, R2        // n -= 16, set flags
	BNE  loop_add2          // loop if n != 0
done_add2:
	RET

// func neonSubInt16x2(dst, src unsafe.Pointer, n int)
// Subtracts n int16 values: dst[i] -= src[i]
// Processes 16 elements (2x 128-bit) per iteration
// n must be a multiple of 16
TEXT ·neonSubInt16x2(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD n+16(FP), R2       // count (must be multiple of 16)

	CBZ  R2, done_sub2      // if n == 0, return
loop_sub2:
	WORD $0x4C40A400        // ld1 {v0.8h, v1.8h}, [x0]
	WORD $0x4C40A422        // ld1 {v2.8h, v3.8h}, [x1]
	WORD $0x6E628400        // sub v0.8h, v0.8h, v2.8h
	WORD $0x6E638421        // sub v1.8h, v1.8h, v3.8h
	WORD $0x4C00A400        // st1 {v0.8h, v1.8h}, [x0]
	ADD  $32, R0            // advance dst by 32 bytes
	ADD  $32, R1            // advance src by 32 bytes
	SUBS $16, R2, R2        // n -= 16, set flags
	BNE  loop_sub2          // loop if n != 0
done_sub2:
	RET

// func neonCopyInt16x2(dst, src unsafe.Pointer, n int)
// Copies n int16 values: dst[i] = src[i]
// Processes 16 elements (2x 128-bit) per iteration
// n must be a multiple of 16
TEXT ·neonCopyInt16x2(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD n+16(FP), R2       // count (must be multiple of 16)

	CBZ  R2, done_copy2     // if n == 0, return
loop_copy2:
	WORD $0x4C40A422        // ld1 {v2.8h, v3.8h}, [x1]
	WORD $0x4C00A402        // st1 {v2.8h, v3.8h}, [x0]
	ADD  $32, R0            // advance dst by 32 bytes
	ADD  $32, R1            // advance src by 32 bytes
	SUBS $16, R2, R2        // n -= 16, set flags
	BNE  loop_copy2         // loop if n != 0
done_copy2:
	RET

// func neonAddInt16OffsetX2(dst unsafe.Pointer, src unsafe.Pointer, offset, count int)
// Adds with offset: dst[i] += src[offset + i]
// Processes 16 elements per iteration
TEXT ·neonAddInt16OffsetX2(SB), NOSPLIT, $0-32
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD offset+16(FP), R3  // offset (in elements)
	MOVD count+24(FP), R2   // count (must be multiple of 16)

	// Adjust src pointer by offset (offset * 2 bytes per int16)
	LSL  $1, R3, R3         // offset *= 2
	ADD  R3, R1, R1         // src += offset

	CBZ  R2, done_add_off2  // if count == 0, return
loop_add_off2:
	WORD $0x4C40A400        // ld1 {v0.8h, v1.8h}, [x0]
	WORD $0x4C40A422        // ld1 {v2.8h, v3.8h}, [x1]
	WORD $0x4E628400        // add v0.8h, v0.8h, v2.8h
	WORD $0x4E638421        // add v1.8h, v1.8h, v3.8h
	WORD $0x4C00A400        // st1 {v0.8h, v1.8h}, [x0]
	ADD  $32, R0
	ADD  $32, R1
	SUBS $16, R2, R2
	BNE  loop_add_off2
done_add_off2:
	RET

// func neonSubInt16OffsetX2(dst unsafe.Pointer, src unsafe.Pointer, offset, count int)
// Subtracts with offset: dst[i] -= src[offset + i]
// Processes 16 elements per iteration
TEXT ·neonSubInt16OffsetX2(SB), NOSPLIT, $0-32
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD offset+16(FP), R3  // offset (in elements)
	MOVD count+24(FP), R2   // count (must be multiple of 16)

	// Adjust src pointer by offset (offset * 2 bytes per int16)
	LSL  $1, R3, R3         // offset *= 2
	ADD  R3, R1, R1         // src += offset

	CBZ  R2, done_sub_off2  // if count == 0, return
loop_sub_off2:
	WORD $0x4C40A400        // ld1 {v0.8h, v1.8h}, [x0]
	WORD $0x4C40A422        // ld1 {v2.8h, v3.8h}, [x1]
	WORD $0x6E628400        // sub v0.8h, v0.8h, v2.8h
	WORD $0x6E638421        // sub v1.8h, v1.8h, v3.8h
	WORD $0x4C00A400        // st1 {v0.8h, v1.8h}, [x0]
	ADD  $32, R0
	ADD  $32, R1
	SUBS $16, R2, R2
	BNE  loop_sub_off2
done_sub_off2:
	RET

// ============================================================================
// INT32 NEON OPERATIONS (for PSQT accumulation)
// ============================================================================
// NEON int32 Opcode Reference:
// ld1 {v0.4s}, [x0]       = 0x4C407800  (load 4 x int32)
// ld1 {v1.4s}, [x1]       = 0x4C407821  (load 4 x int32)
// st1 {v0.4s}, [x0]       = 0x4C007800  (store 4 x int32)
// add v0.4s, v0.4s, v1.4s = 0x4EA18400  (add 4 x int32)
// sub v0.4s, v0.4s, v1.4s = 0x6EA18400  (sub 4 x int32)

// func neonAddInt32(dst, src unsafe.Pointer, n int)
// Adds n int32 values: dst[i] += src[i]
// Processes 4 elements (128-bit) per iteration
TEXT ·neonAddInt32(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD n+16(FP), R2       // count (must be multiple of 4)

	CBZ  R2, done_add32     // if n == 0, return
loop_add32:
	WORD $0x4C407800        // ld1 {v0.4s}, [x0]
	WORD $0x4C407821        // ld1 {v1.4s}, [x1]
	WORD $0x4EA18400        // add v0.4s, v0.4s, v1.4s
	WORD $0x4C007800        // st1 {v0.4s}, [x0]
	ADD  $16, R0            // advance dst by 16 bytes (4 x int32)
	ADD  $16, R1            // advance src by 16 bytes
	SUBS $4, R2, R2         // n -= 4, set flags
	BNE  loop_add32         // loop if n != 0
done_add32:
	RET

// func neonSubInt32(dst, src unsafe.Pointer, n int)
// Subtracts n int32 values: dst[i] -= src[i]
// Processes 4 elements (128-bit) per iteration
TEXT ·neonSubInt32(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD n+16(FP), R2       // count (must be multiple of 4)

	CBZ  R2, done_sub32     // if n == 0, return
loop_sub32:
	WORD $0x4C407800        // ld1 {v0.4s}, [x0]
	WORD $0x4C407821        // ld1 {v1.4s}, [x1]
	WORD $0x6EA18400        // sub v0.4s, v0.4s, v1.4s
	WORD $0x4C007800        // st1 {v0.4s}, [x0]
	ADD  $16, R0            // advance dst by 16 bytes
	ADD  $16, R1            // advance src by 16 bytes
	SUBS $4, R2, R2         // n -= 4, set flags
	BNE  loop_sub32         // loop if n != 0
done_sub32:
	RET

// func neonCopyInt32(dst, src unsafe.Pointer, n int)
// Copies n int32 values: dst[i] = src[i]
// Processes 4 elements (128-bit) per iteration
TEXT ·neonCopyInt32(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0      // dst pointer
	MOVD src+8(FP), R1      // src pointer
	MOVD n+16(FP), R2       // count (must be multiple of 4)

	CBZ  R2, done_copy32    // if n == 0, return
loop_copy32:
	WORD $0x4C407821        // ld1 {v1.4s}, [x1]
	WORD $0x4C007801        // st1 {v1.4s}, [x0]
	ADD  $16, R0            // advance dst by 16 bytes
	ADD  $16, R1            // advance src by 16 bytes
	SUBS $4, R2, R2         // n -= 4, set flags
	BNE  loop_copy32        // loop if n != 0
done_copy32:
	RET

// ============================================================================
// PREFETCH OPERATIONS
// ============================================================================
// ARM64 prefetch opcodes:
// prfm pldl1keep, [x0]  = 0xF9800000  (prefetch for load, L1 cache, keep)
// prfm pldl2keep, [x0]  = 0xF9800800  (prefetch for load, L2 cache, keep)

// func prefetchL1(addr unsafe.Pointer)
// Prefetches memory at addr into L1 cache
TEXT ·prefetchL1(SB), NOSPLIT, $0-8
	MOVD addr+0(FP), R0
	WORD $0xF9800000        // prfm pldl1keep, [x0]
	RET

// func prefetchL2(addr unsafe.Pointer)
// Prefetches memory at addr into L2 cache
TEXT ·prefetchL2(SB), NOSPLIT, $0-8
	MOVD addr+0(FP), R0
	WORD $0xF9800800        // prfm pldl2keep, [x0]
	RET

// func prefetchLine(addr unsafe.Pointer, count int)
// Prefetches count cache lines starting at addr (64 bytes per line)
TEXT ·prefetchLine(SB), NOSPLIT, $0-16
	MOVD addr+0(FP), R0
	MOVD count+8(FP), R1

	CBZ  R1, done_prefetch
loop_prefetch:
	WORD $0xF9800000        // prfm pldl1keep, [x0]
	ADD  $64, R0            // advance by cache line size
	SUBS $1, R1, R1
	BNE  loop_prefetch
done_prefetch:
	RET

// ============================================================================
// DOT PRODUCT (int8 weights * uint8 inputs)
// ============================================================================
// NEON int8/uint8 operations reference:
// sshll  v2.8h, v0.8b, #0   = 0x0F08A402 (sign-extend low 8 int8 to int16)
// sshll2 v3.8h, v0.16b, #0  = 0x4F08A403 (sign-extend high 8 int8 to int16)
// ushll  v4.8h, v1.8b, #0   = 0x2F08A424 (zero-extend low 8 uint8 to uint16)
// ushll2 v5.8h, v1.16b, #0  = 0x6F08A425 (zero-extend high 8 uint8 to uint16)
// smull  v6.4s, v2.4h, v4.4h = 0x0E64C046 (signed multiply to 32-bit)
// smull2 v7.4s, v2.8h, v4.8h = 0x4E64C047 (signed multiply high to 32-bit)
// addv   s0, v16.4s          = 0x4EB1BA00 (horizontal add)

// func neonDotProductInt8Uint8(weights, inputs unsafe.Pointer, count int) int32
// Computes: sum(weights[i] * inputs[i]) for i in [0, count)
// weights: int8 slice, inputs: uint8 slice
// Returns: int32 sum
TEXT ·neonDotProductInt8Uint8(SB), NOSPLIT, $0-28
	MOVD  weights+0(FP), R0     // weights pointer (int8)
	MOVD  inputs+8(FP), R1      // inputs pointer (uint8)
	MOVD  count+16(FP), R2      // count
	MOVW  $0, R3                // Initialize scalar accumulator to zero

	// Check if we have at least 16 elements for SIMD
	CMP   $16, R2
	BLT   dot_scalar_only

	// Initialize vector accumulators to zero using EOR
	WORD  $0x6E301E10           // eor v16.16b, v16.16b, v16.16b
	WORD  $0x6E311E31           // eor v17.16b, v17.16b, v17.16b

	// Main loop: process 16 elements at a time
dot_loop16:
	CMP   $16, R2
	BLT   dot_finish_simd

	// Load 16 int8 weights: ld1 {v0.16b}, [x0]
	WORD  $0x4C407000           // ld1 {v0.16b}, [x0]
	// Load 16 uint8 inputs: ld1 {v1.16b}, [x1]
	WORD  $0x4C407021           // ld1 {v1.16b}, [x1]

	// Sign-extend low 8 int8 to int16: sshll v2.8h, v0.8b, #0
	WORD  $0x0F08A402           // sshll v2.8h, v0.8b, #0
	// Sign-extend high 8 int8 to int16: sshll2 v3.8h, v0.16b, #0
	WORD  $0x4F08A403           // sshll2 v3.8h, v0.16b, #0

	// Zero-extend low 8 uint8 to uint16: ushll v4.8h, v1.8b, #0
	WORD  $0x2F08A424           // ushll v4.8h, v1.8b, #0
	// Zero-extend high 8 uint8 to uint16: ushll2 v5.8h, v1.16b, #0
	WORD  $0x6F08A425           // ushll2 v5.8h, v1.16b, #0

	// Multiply low half: smull v6.4s, v2.4h, v4.4h (low 4)
	WORD  $0x0E64C046           // smull v6.4s, v2.4h, v4.4h
	// Multiply low half high: smull2 v7.4s, v2.8h, v4.8h (high 4 of low half)
	WORD  $0x4E64C047           // smull2 v7.4s, v2.8h, v4.8h

	// Accumulate: add v16.4s, v16.4s, v6.4s
	WORD  $0x4EA68610           // add v16.4s, v16.4s, v6.4s
	WORD  $0x4EA78631           // add v17.4s, v17.4s, v7.4s

	// Multiply high half: smull v6.4s, v3.4h, v5.4h (low 4 of high half)
	WORD  $0x0E65C066           // smull v6.4s, v3.4h, v5.4h
	// Multiply high half high: smull2 v7.4s, v3.8h, v5.8h (high 4 of high half)
	WORD  $0x4E65C067           // smull2 v7.4s, v3.8h, v5.8h

	// Accumulate
	WORD  $0x4EA68610           // add v16.4s, v16.4s, v6.4s
	WORD  $0x4EA78631           // add v17.4s, v17.4s, v7.4s

	ADD   $16, R0               // advance weights pointer
	ADD   $16, R1               // advance inputs pointer
	SUB   $16, R2, R2           // count -= 16
	B     dot_loop16

dot_finish_simd:
	// Combine vector accumulators: add v16.4s, v16.4s, v17.4s
	WORD  $0x4EB18610           // add v16.4s, v16.4s, v17.4s

	// Horizontal sum: addv s0, v16.4s
	WORD  $0x4EB1BA00           // addv s0, v16.4s

	// Move to general register: fmov w3, s0
	WORD  $0x1E260003           // fmov w3, s0

dot_scalar_only:
	// Handle remaining elements with scalar loop
	CBZ   R2, dot_done

dot_scalar_loop:
	MOVB  (R0), R4              // load weight (int8) - sign extends
	MOVBU (R1), R5              // load input (uint8) - zero extends
	MULW  R4, R5, R4            // multiply (32-bit)
	ADDW  R4, R3, R3            // accumulate (32-bit)
	ADD   $1, R0
	ADD   $1, R1
	SUB   $1, R2, R2
	CBNZ  R2, dot_scalar_loop

dot_done:
	MOVW  R3, ret+24(FP)        // return result
	RET

// ============================================================================
// CLIPPED RELU (int32 input -> uint8 output with shift and clamp)
// ============================================================================
// NEON clamp/shift operations:
// sqshrun v0.8b, v0.8h, #shift = clamp to [0,255] with shift (for 16-bit)
// For 32-bit we need: shift, clamp to [0, 127], pack

// func neonClippedReLU(input unsafe.Pointer, output unsafe.Pointer, count, shift int)
// Applies: output[i] = clamp(input[i] >> shift, 0, 127)
// Processes 4 elements per SIMD iteration
TEXT ·neonClippedReLU(SB), NOSPLIT, $0-32
	MOVD  input+0(FP), R0       // input pointer (int32)
	MOVD  output+8(FP), R1      // output pointer (uint8)
	MOVD  count+16(FP), R2      // count
	MOVD  shift+24(FP), R3      // shift amount

	// Prepare constants
	MOVW  $0, R5                // zero for clamping
	MOVW  $127, R6              // max value for clamping

	// Check if we have at least 4 elements
	CMP   $4, R2
	BLT   crelu_scalar

	// Create vector constants
	// dup v30.4s, w5 (zero) - 0x4E050BC0 doesn't work, use eor instead
	WORD  $0x6E3E1FDE           // eor v30.16b, v30.16b, v30.16b
	// dup v31.4s, w6 (127)
	WORD  $0x4E040CDF           // dup v31.4s, w6

	// Negate shift for right shift using signed shift
	NEG   R3, R4
	// dup v29.4s, w4 (-shift)
	WORD  $0x4E040C9D           // dup v29.4s, w4

crelu_loop4:
	CMP   $4, R2
	BLT   crelu_scalar

	// Load 4 int32: ld1 {v0.4s}, [x0]
	WORD  $0x4C407800           // ld1 {v0.4s}, [x0]

	// Arithmetic right shift: sshl v0.4s, v0.4s, v29.4s (negative shift = right shift)
	WORD  $0x4EBD4400           // sshl v0.4s, v0.4s, v29.4s

	// Clamp to [0, 127]
	// smax v0.4s, v0.4s, v30.4s (clamp lower to 0)
	WORD  $0x4EBE6400           // smax v0.4s, v0.4s, v30.4s
	// smin v0.4s, v0.4s, v31.4s (clamp upper to 127)
	WORD  $0x4EBF6C00           // smin v0.4s, v0.4s, v31.4s

	// Narrow 4x int32 -> 4x int16: xtn v0.4h, v0.4s
	WORD  $0x0EA12800           // xtn v0.4h, v0.4s
	// Narrow 4x int16 -> 4x int8: xtn v0.8b, v0.8h (only lower 4 used)
	WORD  $0x0E212800           // xtn v0.8b, v0.8h

	// Store 4 bytes: st1 {v0.s}[0], [x1]
	WORD  $0x0D008021           // st1 {v0.s}[0], [x1]

	ADD   $16, R0               // advance input by 16 bytes (4 x int32)
	ADD   $4, R1                // advance output by 4 bytes
	SUB   $4, R2, R2
	B     crelu_loop4

crelu_scalar:
	CBZ   R2, crelu_done

crelu_scalar_loop:
	MOVW  (R0), R4              // load int32
	ASR   R3, R4, R4            // arithmetic right shift

	// Clamp to [0, 127]
	CMP   R5, R4
	CSEL  LT, R5, R4, R4        // if R4 < 0, R4 = 0
	CMP   R6, R4
	CSEL  GT, R6, R4, R4        // if R4 > 127, R4 = 127

	MOVB  R4, (R1)              // store uint8
	ADD   $4, R0
	ADD   $1, R1
	SUB   $1, R2, R2
	CBNZ  R2, crelu_scalar_loop

crelu_done:
	RET
