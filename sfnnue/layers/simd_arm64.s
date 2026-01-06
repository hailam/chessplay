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

// ============================================================================
// SPARSE CHUNK MULTIPLY-ACCUMULATE (for AffineTransformSparseInput)
// ============================================================================
// Processes one non-zero input chunk (4 bytes) across all outputs.
// For each group of 4 outputs, computes:
//   output[k:k+4] += dot_products(weights[k*4:(k+4)*4], [b0,b1,b2,b3])
//
// func neonSparseChunkMulAcc(output, weights unsafe.Pointer, outLen int, inputChunk uint32)
// output: pointer to int32 output array
// weights: pointer to int8 weights at colOffset (contiguous for all outputs)
// outLen: number of outputs (typically 16, must be multiple of 4)
// inputChunk: packed input bytes as uint32 [b3<<24 | b2<<16 | b1<<8 | b0]
TEXT ·neonSparseChunkMulAcc(SB), NOSPLIT, $0-28
    MOVD  output+0(FP), R0      // output pointer (int32*)
    MOVD  weights+8(FP), R1     // weights pointer (int8*)
    MOVD  outLen+16(FP), R2     // output length (must be multiple of 4)
    MOVW  inputChunk+24(FP), R3 // packed input bytes

    // Check if we have at least 4 outputs
    CMP   $4, R2
    BLT   sparse_scalar

    // Broadcast input bytes to v1.16b = [b0,b1,b2,b3] x 4
    // First, duplicate the 32-bit chunk to all lanes
    WORD  $0x4E040C61           // dup v1.4s, w3
    // Now v1.16b has [b0,b1,b2,b3,b0,b1,b2,b3,b0,b1,b2,b3,b0,b1,b2,b3]

sparse_loop4:
    CMP   $4, R2
    BLT   sparse_scalar

    // Load 16 int8 weights for 4 outputs: ld1 {v0.16b}, [x1]
    WORD  $0x4C407020           // ld1 {v0.16b}, [x1]

    // Sign-extend weights to int16
    // sshll v2.8h, v0.8b, #0 (low 8 weights)
    WORD  $0x0F08A402           // sshll v2.8h, v0.8b, #0
    // sshll2 v3.8h, v0.16b, #0 (high 8 weights)
    WORD  $0x4F08A403           // sshll2 v3.8h, v0.16b, #0

    // Zero-extend inputs to uint16
    // ushll v4.8h, v1.8b, #0 (low 8 bytes)
    WORD  $0x2F08A424           // ushll v4.8h, v1.8b, #0
    // ushll2 v5.8h, v1.16b, #0 (high 8 bytes)
    WORD  $0x6F08A425           // ushll2 v5.8h, v1.16b, #0

    // Multiply to int32: 16 products split into 4 vectors
    // smull v6.4s, v2.4h, v4.4h  → products 0-3 (for output 0)
    WORD  $0x0E64C046           // smull v6.4s, v2.4h, v4.4h
    // smull2 v7.4s, v2.8h, v4.8h → products 4-7 (for output 1)
    WORD  $0x4E64C047           // smull2 v7.4s, v2.8h, v4.8h
    // smull v8.4s, v3.4h, v5.4h  → products 8-11 (for output 2)
    WORD  $0x0E65C068           // smull v8.4s, v3.4h, v5.4h
    // smull2 v9.4s, v3.8h, v5.8h → products 12-15 (for output 3)
    WORD  $0x4E65C069           // smull2 v9.4s, v3.8h, v5.8h

    // Horizontal sum within each group of 4 using pairwise add
    // Step 1: addp v6.4s, v6.4s, v7.4s → [p0+p1, p2+p3, p4+p5, p6+p7]
    WORD  $0x4EA7BCC6           // addp v6.4s, v6.4s, v7.4s
    // addp v8.4s, v8.4s, v9.4s → [p8+p9, p10+p11, p12+p13, p14+p15]
    WORD  $0x4EA9BD08           // addp v8.4s, v8.4s, v9.4s

    // Step 2: addp v6.4s, v6.4s, v8.4s → [result0, result1, result2, result3]
    WORD  $0x4EA8BCC6           // addp v6.4s, v6.4s, v8.4s

    // Load current output values: ld1 {v10.4s}, [x0]
    WORD  $0x4C40780A           // ld1 {v10.4s}, [x0]

    // Add results to output: add v10.4s, v10.4s, v6.4s
    WORD  $0x4EA6854A           // add v10.4s, v10.4s, v6.4s

    // Store updated output: st1 {v10.4s}, [x0]
    WORD  $0x4C00780A           // st1 {v10.4s}, [x0]

    ADD   $16, R0               // advance output by 16 bytes (4 x int32)
    ADD   $16, R1               // advance weights by 16 bytes (16 x int8)
    SUB   $4, R2, R2            // outLen -= 4
    B     sparse_loop4

sparse_scalar:
    // Handle remaining outputs (0-3) with scalar loop
    CBZ   R2, sparse_done

    // Extract input bytes
    UBFXW $0, R3, $8, R4        // b0 = inputChunk & 0xFF
    UBFXW $8, R3, $8, R5        // b1 = (inputChunk >> 8) & 0xFF
    UBFXW $16, R3, $8, R6       // b2 = (inputChunk >> 16) & 0xFF
    UBFXW $24, R3, $8, R7       // b3 = (inputChunk >> 24) & 0xFF

sparse_scalar_loop:
    // Load 4 int8 weights
    MOVB  (R1), R8
    MOVB  1(R1), R9
    MOVB  2(R1), R10
    MOVB  3(R1), R11

    // Sign-extend to 32-bit
    SXTBW R8, R8
    SXTBW R9, R9
    SXTBW R10, R10
    SXTBW R11, R11

    // Multiply and accumulate
    MULW  R8, R4, R8            // w0 * b0
    MULW  R9, R5, R9            // w1 * b1
    MULW  R10, R6, R10          // w2 * b2
    MULW  R11, R7, R11          // w3 * b3

    ADDW  R8, R9, R8            // (w0*b0) + (w1*b1)
    ADDW  R10, R11, R10         // (w2*b2) + (w3*b3)
    ADDW  R8, R10, R8           // sum all 4

    // Add to output
    MOVW  (R0), R9
    ADDW  R8, R9, R9
    MOVW  R9, (R0)

    ADD   $4, R0                // advance output by 4 bytes (1 x int32)
    ADD   $4, R1                // advance weights by 4 bytes (4 x int8)
    SUB   $1, R2, R2
    CBNZ  R2, sparse_scalar_loop

sparse_done:
    RET
