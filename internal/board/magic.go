package board

// Magic bitboard implementation for sliding piece attacks.
// Uses pre-computed magic numbers for fast lookup.

// Magic holds the magic bitboard data for a single square.
type Magic struct {
	Mask   Bitboard  // Relevant occupancy mask (excludes edges)
	Magic  uint64    // Magic multiplier
	Shift  uint8     // Bits to shift right
	Offset uint32    // Index into attack table
}

var (
	bishopMagics [64]Magic
	rookMagics   [64]Magic

	// Attack tables (fancy magic bitboards)
	bishopTable [5248]Bitboard  // Total bishop attack table entries
	rookTable   [102400]Bitboard // Total rook attack table entries
)

// Pre-computed magic numbers (found through trial and error)
var bishopMagicNumbers = [64]uint64{
	0x0002020202020200, 0x0002020202020000, 0x0004010202000000, 0x0004040080000000,
	0x0001104000000000, 0x0000821040000000, 0x0000410410400000, 0x0000104104104000,
	0x0000040404040400, 0x0000020202020200, 0x0000040102020000, 0x0000040400800000,
	0x0000011040000000, 0x0000008210400000, 0x0000004104104000, 0x0000002082082000,
	0x0004000808080800, 0x0002000404040400, 0x0001000202020200, 0x0000800802004000,
	0x0000800400A00000, 0x0000200100884000, 0x0000400082082000, 0x0000200041041000,
	0x0002080010101000, 0x0001040008080800, 0x0000208004010400, 0x0000404004010200,
	0x0000840000802000, 0x0000404002011000, 0x0000808001041000, 0x0000404000820800,
	0x0001041000202000, 0x0000820800101000, 0x0000104400080800, 0x0000020080080080,
	0x0000404040040100, 0x0000808100020100, 0x0001010100020800, 0x0000808080010400,
	0x0000820820004000, 0x0000410410002000, 0x0000082088001000, 0x0000002011000800,
	0x0000080100400400, 0x0001010101000200, 0x0002020202000400, 0x0001010101000200,
	0x0000410410400000, 0x0000208208200000, 0x0000002084100000, 0x0000000020880000,
	0x0000001002020000, 0x0000040408020000, 0x0004040404040000, 0x0002020202020000,
	0x0000104104104000, 0x0000002082082000, 0x0000000020841000, 0x0000000000208800,
	0x0000000010020200, 0x0000000404080200, 0x0000040404040400, 0x0002020202020200,
}

var rookMagicNumbers = [64]uint64{
	0x0080001020400080, 0x0040001000200040, 0x0080081000200080, 0x0080040800100080,
	0x0080020400080080, 0x0080010200040080, 0x0080008001000200, 0x0080002040800100,
	0x0000800020400080, 0x0000400020005000, 0x0000801000200080, 0x0000800800100080,
	0x0000800400080080, 0x0000800200040080, 0x0000800100020080, 0x0000800040800100,
	0x0000208000400080, 0x0000404000201000, 0x0000808010002000, 0x0000808008001000,
	0x0000808004000800, 0x0000808002000400, 0x0000010100020004, 0x0000020000408104,
	0x0000208080004000, 0x0000200040005000, 0x0000100080200080, 0x0000080080100080,
	0x0000040080080080, 0x0000020080040080, 0x0000010080800200, 0x0000800080004100,
	0x0000204000800080, 0x0000200040401000, 0x0000100080802000, 0x0000080080801000,
	0x0000040080800800, 0x0000020080800400, 0x0000020001010004, 0x0000800040800100,
	0x0000204000808000, 0x0000200040008080, 0x0000100020008080, 0x0000080010008080,
	0x0000040008008080, 0x0000020004008080, 0x0000010002008080, 0x0000004081020004,
	0x0000204000800080, 0x0000200040008080, 0x0000100020008080, 0x0000080010008080,
	0x0000040008008080, 0x0000020004008080, 0x0000800100020080, 0x0000800041000080,
	0x00FFFCDDFCED714A, 0x007FFCDDFCED714A, 0x003FFFCDFFD88096, 0x0000040810002101,
	0x0001000204080011, 0x0001000204000801, 0x0001000082000401, 0x0001FFFAABFAD1A2,
}

func initMagics() {
	initBishopMagics()
	initRookMagics()
}

func initBishopMagics() {
	var offset uint32 = 0
	for sq := A1; sq <= H8; sq++ {
		mask := bishopMask(sq)
		bits := mask.PopCount()

		bishopMagics[sq] = Magic{
			Mask:   mask,
			Magic:  bishopMagicNumbers[sq],
			Shift:  uint8(64 - bits),
			Offset: offset,
		}

		// Generate all possible occupancies and compute attacks
		numEntries := 1 << bits
		for i := 0; i < numEntries; i++ {
			occ := indexToOccupancy(i, bits, mask)
			idx := (uint64(occ) * bishopMagicNumbers[sq]) >> (64 - bits)
			bishopTable[offset+uint32(idx)] = bishopAttacksSlow(sq, occ)
		}
		offset += uint32(numEntries)
	}
}

func initRookMagics() {
	var offset uint32 = 0
	for sq := A1; sq <= H8; sq++ {
		mask := rookMask(sq)
		bits := mask.PopCount()

		rookMagics[sq] = Magic{
			Mask:   mask,
			Magic:  rookMagicNumbers[sq],
			Shift:  uint8(64 - bits),
			Offset: offset,
		}

		// Generate all possible occupancies and compute attacks
		numEntries := 1 << bits
		for i := 0; i < numEntries; i++ {
			occ := indexToOccupancy(i, bits, mask)
			idx := (uint64(occ) * rookMagicNumbers[sq]) >> (64 - bits)
			rookTable[offset+uint32(idx)] = rookAttacksSlow(sq, occ)
		}
		offset += uint32(numEntries)
	}
}

// bishopMask returns the relevant occupancy mask for bishop at square.
// Excludes edge squares since they don't affect the result.
func bishopMask(sq Square) Bitboard {
	return bishopAttacksSlow(sq, 0) & ^(Rank1 | Rank8 | FileA | FileH)
}

// rookMask returns the relevant occupancy mask for rook at square.
func rookMask(sq Square) Bitboard {
	file := sq.File()
	rank := sq.Rank()

	var mask Bitboard

	// Horizontal (exclude edges unless rook is on edge)
	for f := 1; f < 7; f++ {
		if f != file {
			mask |= SquareBB(NewSquare(f, rank))
		}
	}

	// Vertical (exclude edges unless rook is on edge)
	for r := 1; r < 7; r++ {
		if r != rank {
			mask |= SquareBB(NewSquare(file, r))
		}
	}

	return mask
}

// indexToOccupancy converts an index to an occupancy bitboard.
func indexToOccupancy(index, bits int, mask Bitboard) Bitboard {
	var occ Bitboard
	for i := 0; i < bits; i++ {
		sq := mask.LSB()
		mask &= mask - 1
		if index&(1<<i) != 0 {
			occ |= SquareBB(sq)
		}
	}
	return occ
}

// bishopAttacksSlow computes bishop attacks by ray casting (used during initialization).
func bishopAttacksSlow(sq Square, occupied Bitboard) Bitboard {
	var attacks Bitboard
	file, rank := sq.File(), sq.Rank()

	// Northeast
	for f, r := file+1, rank+1; f <= 7 && r <= 7; f, r = f+1, r+1 {
		s := NewSquare(f, r)
		attacks |= SquareBB(s)
		if occupied&SquareBB(s) != 0 {
			break
		}
	}

	// Northwest
	for f, r := file-1, rank+1; f >= 0 && r <= 7; f, r = f-1, r+1 {
		s := NewSquare(f, r)
		attacks |= SquareBB(s)
		if occupied&SquareBB(s) != 0 {
			break
		}
	}

	// Southeast
	for f, r := file+1, rank-1; f <= 7 && r >= 0; f, r = f+1, r-1 {
		s := NewSquare(f, r)
		attacks |= SquareBB(s)
		if occupied&SquareBB(s) != 0 {
			break
		}
	}

	// Southwest
	for f, r := file-1, rank-1; f >= 0 && r >= 0; f, r = f-1, r-1 {
		s := NewSquare(f, r)
		attacks |= SquareBB(s)
		if occupied&SquareBB(s) != 0 {
			break
		}
	}

	return attacks
}

// rookAttacksSlow computes rook attacks by ray casting (used during initialization).
func rookAttacksSlow(sq Square, occupied Bitboard) Bitboard {
	var attacks Bitboard
	file, rank := sq.File(), sq.Rank()

	// North
	for r := rank + 1; r <= 7; r++ {
		s := NewSquare(file, r)
		attacks |= SquareBB(s)
		if occupied&SquareBB(s) != 0 {
			break
		}
	}

	// South
	for r := rank - 1; r >= 0; r-- {
		s := NewSquare(file, r)
		attacks |= SquareBB(s)
		if occupied&SquareBB(s) != 0 {
			break
		}
	}

	// East
	for f := file + 1; f <= 7; f++ {
		s := NewSquare(f, rank)
		attacks |= SquareBB(s)
		if occupied&SquareBB(s) != 0 {
			break
		}
	}

	// West
	for f := file - 1; f >= 0; f-- {
		s := NewSquare(f, rank)
		attacks |= SquareBB(s)
		if occupied&SquareBB(s) != 0 {
			break
		}
	}

	return attacks
}

// getBishopAttacks returns bishop attacks using magic bitboards.
func getBishopAttacks(sq Square, occupied Bitboard) Bitboard {
	m := &bishopMagics[sq]
	idx := ((uint64(occupied) & uint64(m.Mask)) * m.Magic) >> m.Shift
	return bishopTable[m.Offset+uint32(idx)]
}

// getRookAttacks returns rook attacks using magic bitboards.
func getRookAttacks(sq Square, occupied Bitboard) Bitboard {
	m := &rookMagics[sq]
	idx := ((uint64(occupied) & uint64(m.Mask)) * m.Magic) >> m.Shift
	return rookTable[m.Offset+uint32(idx)]
}
