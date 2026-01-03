//go:build arm64

#include "textflag.h"

// ARM64 NEON SIMD operations for affine transform layers
// Uses raw WORD opcodes since Go assembler doesn't support NEON mnemonics

// func neonDotProductInt8Uint8(weights, inputs unsafe.Pointer, count int) int32
// Computes: sum(weights[i] * inputs[i]) for i in [0, count)
// weights: int8 slice, inputs: uint8 slice
// Returns: int32 sum
TEXT ·neonDotProductInt8Uint8(SB), NOSPLIT, $0-28
    MOVD  weights+0(FP), R0     // weights pointer (int8)
    MOVD  inputs+8(FP), R1      // inputs pointer (uint8)
    MOVD  count+16(FP), R2      // count
    MOVW  $0, R3                // Initialize scalar accumulator to zero

    // Check if we have at least 8 elements for SIMD
    CMP   $8, R2
    BLT   scalar_only

    // Initialize vector accumulators to zero using EOR
    WORD  $0x6E301E10           // eor v16.16b, v16.16b, v16.16b
    WORD  $0x6E311E31           // eor v17.16b, v17.16b, v17.16b

    // Main loop: process 16 elements at a time
loop16:
    CMP   $16, R2
    BLT   finish_simd

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
    B     loop16

finish_simd:
    // Combine vector accumulators: add v16.4s, v16.4s, v17.4s
    WORD  $0x4EB18610           // add v16.4s, v16.4s, v17.4s

    // Horizontal sum: addv s0, v16.4s
    WORD  $0x4EB1BA00           // addv s0, v16.4s

    // Move to general register: fmov w3, s0
    WORD  $0x1E260003           // fmov w3, s0

scalar_only:
    // Handle remaining elements with scalar loop
    CBZ   R2, done

scalar_loop:
    MOVB  (R0), R4              // load weight (int8) - sign extends
    MOVBU (R1), R5              // load input (uint8) - zero extends
    MULW  R4, R5, R4            // multiply (32-bit)
    ADDW  R4, R3, R3            // accumulate (32-bit)
    ADD   $1, R0
    ADD   $1, R1
    SUB   $1, R2, R2
    CBNZ  R2, scalar_loop

done:
    MOVW  R3, ret+24(FP)        // return result
    RET
