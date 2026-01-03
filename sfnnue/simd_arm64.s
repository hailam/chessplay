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
