// Package board implements chess board representation using bitboards.
package board

import "fmt"

// Square represents a square on the chess board (0-63).
// Uses Little-Endian Rank-File Mapping: A1=0, H1=7, A8=56, H8=63.
type Square uint8

// Square constants for all 64 squares.
const (
	A1 Square = iota
	B1
	C1
	D1
	E1
	F1
	G1
	H1
	A2
	B2
	C2
	D2
	E2
	F2
	G2
	H2
	A3
	B3
	C3
	D3
	E3
	F3
	G3
	H3
	A4
	B4
	C4
	D4
	E4
	F4
	G4
	H4
	A5
	B5
	C5
	D5
	E5
	F5
	G5
	H5
	A6
	B6
	C6
	D6
	E6
	F6
	G6
	H6
	A7
	B7
	C7
	D7
	E7
	F7
	G7
	H7
	A8
	B8
	C8
	D8
	E8
	F8
	G8
	H8
	NoSquare Square = 64
)

// File returns the file (column) of the square (0-7, where 0=a, 7=h).
func (sq Square) File() int {
	return int(sq) & 7
}

// Rank returns the rank (row) of the square (0-7, where 0=1, 7=8).
func (sq Square) Rank() int {
	return int(sq) >> 3
}

// String returns the algebraic notation for the square (e.g., "e4").
func (sq Square) String() string {
	if sq >= NoSquare {
		return "-"
	}
	return fmt.Sprintf("%c%c", 'a'+sq.File(), '1'+sq.Rank())
}

// NewSquare creates a square from file and rank (0-indexed).
func NewSquare(file, rank int) Square {
	return Square(rank*8 + file)
}

// ParseSquare parses algebraic notation (e.g., "e4") into a Square.
func ParseSquare(s string) (Square, error) {
	if len(s) != 2 {
		return NoSquare, fmt.Errorf("invalid square: %s", s)
	}

	file := int(s[0] - 'a')
	rank := int(s[1] - '1')

	if file < 0 || file > 7 || rank < 0 || rank > 7 {
		return NoSquare, fmt.Errorf("invalid square: %s", s)
	}

	return NewSquare(file, rank), nil
}

// IsValid returns true if the square is a valid board square (0-63).
func (sq Square) IsValid() bool {
	return sq < NoSquare
}

// Mirror returns the square mirrored vertically (for black's perspective).
func (sq Square) Mirror() Square {
	return sq ^ 56
}

// RelativeRank returns the rank from a given color's perspective.
// For White, rank 0 is the 1st rank; for Black, rank 0 is the 8th rank.
func (sq Square) RelativeRank(c Color) int {
	if c == White {
		return sq.Rank()
	}
	return 7 - sq.Rank()
}
