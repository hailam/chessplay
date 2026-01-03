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
