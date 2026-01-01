package board

import (
	"fmt"
	"math/bits"
)

// Bitboard represents a 64-bit board where each bit corresponds to a square.
// Bit 0 = A1, Bit 7 = H1, Bit 56 = A8, Bit 63 = H8 (Little-Endian Rank-File Mapping).
type Bitboard uint64

// File masks
const (
	FileA Bitboard = 0x0101010101010101
	FileB Bitboard = 0x0202020202020202
	FileC Bitboard = 0x0404040404040404
	FileD Bitboard = 0x0808080808080808
	FileE Bitboard = 0x1010101010101010
	FileF Bitboard = 0x2020202020202020
	FileG Bitboard = 0x4040404040404040
	FileH Bitboard = 0x8080808080808080
)

// Rank masks
const (
	Rank1 Bitboard = 0x00000000000000FF
	Rank2 Bitboard = 0x000000000000FF00
	Rank3 Bitboard = 0x0000000000FF0000
	Rank4 Bitboard = 0x00000000FF000000
	Rank5 Bitboard = 0x000000FF00000000
	Rank6 Bitboard = 0x0000FF0000000000
	Rank7 Bitboard = 0x00FF000000000000
	Rank8 Bitboard = 0xFF00000000000000
)

// Special masks
const (
	Empty    Bitboard = 0
	Universe Bitboard = 0xFFFFFFFFFFFFFFFF

	// Edges
	NotFileA Bitboard = ^FileA
	NotFileH Bitboard = ^FileH
	NotFileAB Bitboard = ^(FileA | FileB)
	NotFileGH Bitboard = ^(FileG | FileH)

	// Center squares
	Center     Bitboard = (FileD | FileE) & (Rank4 | Rank5)
	BigCenter  Bitboard = (FileC | FileD | FileE | FileF) & (Rank3 | Rank4 | Rank5 | Rank6)

	// King safety zones
	WhiteKingSide  Bitboard = (FileF | FileG | FileH) & (Rank1 | Rank2)
	WhiteQueenSide Bitboard = (FileA | FileB | FileC) & (Rank1 | Rank2)
	BlackKingSide  Bitboard = (FileF | FileG | FileH) & (Rank7 | Rank8)
	BlackQueenSide Bitboard = (FileA | FileB | FileC) & (Rank7 | Rank8)
)

// FileMask returns the file mask for a given file (0-7).
var FileMask = [8]Bitboard{FileA, FileB, FileC, FileD, FileE, FileF, FileG, FileH}

// RankMask returns the rank mask for a given rank (0-7).
var RankMask = [8]Bitboard{Rank1, Rank2, Rank3, Rank4, Rank5, Rank6, Rank7, Rank8}

// SquareBB returns a bitboard with only the given square set.
func SquareBB(sq Square) Bitboard {
	return 1 << sq
}

// Set sets a bit at the given square.
func (b Bitboard) Set(sq Square) Bitboard {
	return b | (1 << sq)
}

// Clear clears a bit at the given square.
func (b Bitboard) Clear(sq Square) Bitboard {
	return b &^ (1 << sq)
}

// IsSet returns true if the bit at the given square is set.
func (b Bitboard) IsSet(sq Square) bool {
	return b&(1<<sq) != 0
}

// Toggle flips the bit at the given square.
func (b Bitboard) Toggle(sq Square) Bitboard {
	return b ^ (1 << sq)
}

// PopCount returns the number of set bits (population count).
func (b Bitboard) PopCount() int {
	return bits.OnesCount64(uint64(b))
}

// LSB returns the least significant bit (lowest square index).
func (b Bitboard) LSB() Square {
	if b == 0 {
		return NoSquare
	}
	return Square(bits.TrailingZeros64(uint64(b)))
}

// MSB returns the most significant bit (highest square index).
func (b Bitboard) MSB() Square {
	if b == 0 {
		return NoSquare
	}
	return Square(63 - bits.LeadingZeros64(uint64(b)))
}

// PopLSB removes and returns the least significant bit.
func (b *Bitboard) PopLSB() Square {
	sq := b.LSB()
	*b &= *b - 1 // Clear the LSB
	return sq
}

// More returns true if there are any bits set.
func (b Bitboard) More() bool {
	return b != 0
}

// Empty returns true if no bits are set.
func (b Bitboard) Empty() bool {
	return b == 0
}

// Shift operations for move generation

// North shifts the bitboard one rank up (toward rank 8).
func (b Bitboard) North() Bitboard {
	return b << 8
}

// South shifts the bitboard one rank down (toward rank 1).
func (b Bitboard) South() Bitboard {
	return b >> 8
}

// East shifts the bitboard one file right (toward file h).
func (b Bitboard) East() Bitboard {
	return (b << 1) & NotFileA
}

// West shifts the bitboard one file left (toward file a).
func (b Bitboard) West() Bitboard {
	return (b >> 1) & NotFileH
}

// NorthEast shifts the bitboard one square toward a8 corner.
func (b Bitboard) NorthEast() Bitboard {
	return (b << 9) & NotFileA
}

// NorthWest shifts the bitboard one square toward h8 corner.
func (b Bitboard) NorthWest() Bitboard {
	return (b << 7) & NotFileH
}

// SouthEast shifts the bitboard one square toward h1 corner.
func (b Bitboard) SouthEast() Bitboard {
	return (b >> 7) & NotFileA
}

// SouthWest shifts the bitboard one square toward a1 corner.
func (b Bitboard) SouthWest() Bitboard {
	return (b >> 9) & NotFileH
}

// Fill operations for sliding pieces

// NorthFill fills all squares north of the set bits.
func (b Bitboard) NorthFill() Bitboard {
	b |= b << 8
	b |= b << 16
	b |= b << 32
	return b
}

// SouthFill fills all squares south of the set bits.
func (b Bitboard) SouthFill() Bitboard {
	b |= b >> 8
	b |= b >> 16
	b |= b >> 32
	return b
}

// FileFill fills the entire file(s) containing any set bit.
func (b Bitboard) FileFill() Bitboard {
	return b.NorthFill() | b.SouthFill()
}

// String returns a visual representation of the bitboard.
func (b Bitboard) String() string {
	s := ""
	for rank := 7; rank >= 0; rank-- {
		s += fmt.Sprintf("%d ", rank+1)
		for file := 0; file < 8; file++ {
			sq := NewSquare(file, rank)
			if b.IsSet(sq) {
				s += "1 "
			} else {
				s += ". "
			}
		}
		s += "\n"
	}
	s += "  a b c d e f g h\n"
	return s
}

// ForEach calls the function for each set square.
func (b Bitboard) ForEach(f func(Square)) {
	for b != 0 {
		sq := b.PopLSB()
		f(sq)
	}
}

// Squares returns a slice of all squares that are set.
func (b Bitboard) Squares() []Square {
	squares := make([]Square, 0, b.PopCount())
	for b != 0 {
		squares = append(squares, b.PopLSB())
	}
	return squares
}
