package board

// Pre-computed attack tables for non-sliding pieces
var (
	knightAttacks [64]Bitboard
	kingAttacks   [64]Bitboard
	pawnAttacks   [2][64]Bitboard // [Color][Square]
	pawnPushes    [2][64]Bitboard // [Color][Square] - single push targets

	// Between and Line bitboards for pins/checks
	betweenBB [64][64]Bitboard // Squares strictly between two squares
	lineBB    [64][64]Bitboard // Full line through two squares (including endpoints)
)

func init() {
	initKnightAttacks()
	initKingAttacks()
	initPawnAttacks()
	initBetweenBB()
	initLineBB()
	initMagics() // From magic.go
}

func initKnightAttacks() {
	for sq := A1; sq <= H8; sq++ {
		bb := SquareBB(sq)

		// Knight moves: 2+1 or 1+2 in any direction
		attacks := Empty

		// Up 2, left/right 1
		attacks |= (bb << 17) & NotFileA  // NNE
		attacks |= (bb << 15) & NotFileH  // NNW
		attacks |= (bb >> 17) & NotFileH  // SSW
		attacks |= (bb >> 15) & NotFileA  // SSE

		// Up 1, left/right 2
		attacks |= (bb << 10) & NotFileAB // ENE
		attacks |= (bb << 6) & NotFileGH  // WNW
		attacks |= (bb >> 10) & NotFileGH // WSW
		attacks |= (bb >> 6) & NotFileAB  // ESE

		knightAttacks[sq] = attacks
	}
}

func initKingAttacks() {
	for sq := A1; sq <= H8; sq++ {
		bb := SquareBB(sq)

		// King moves: 1 square in any direction
		attacks := bb.North() | bb.South()
		attacks |= bb.East() | bb.West()
		attacks |= bb.NorthEast() | bb.NorthWest()
		attacks |= bb.SouthEast() | bb.SouthWest()

		kingAttacks[sq] = attacks
	}
}

func initPawnAttacks() {
	for sq := A1; sq <= H8; sq++ {
		bb := SquareBB(sq)

		// White pawn attacks (diagonal captures going up)
		pawnAttacks[White][sq] = bb.NorthEast() | bb.NorthWest()

		// Black pawn attacks (diagonal captures going down)
		pawnAttacks[Black][sq] = bb.SouthEast() | bb.SouthWest()

		// Pawn pushes (single push targets)
		pawnPushes[White][sq] = bb.North()
		pawnPushes[Black][sq] = bb.South()
	}
}

func initBetweenBB() {
	// For each pair of squares, compute the squares strictly between them
	for sq1 := A1; sq1 <= H8; sq1++ {
		for sq2 := A1; sq2 <= H8; sq2++ {
			if sq1 == sq2 {
				continue
			}

			f1, r1 := sq1.File(), sq1.Rank()
			f2, r2 := sq2.File(), sq2.Rank()

			df := sign(f2 - f1)
			dr := sign(r2 - r1)

			// Only compute for aligned squares (rook or bishop attacks)
			if df != 0 && dr != 0 && abs(f2-f1) != abs(r2-r1) {
				continue // Not on a diagonal
			}

			if df == 0 && dr == 0 {
				continue // Same square
			}

			var between Bitboard
			f, r := f1+df, r1+dr
			for f != f2 || r != r2 {
				if f < 0 || f > 7 || r < 0 || r > 7 {
					break
				}
				between |= SquareBB(NewSquare(f, r))
				f += df
				r += dr
			}

			betweenBB[sq1][sq2] = between
		}
	}
}

func initLineBB() {
	// For each pair of squares, compute the full line through them
	for sq1 := A1; sq1 <= H8; sq1++ {
		for sq2 := A1; sq2 <= H8; sq2++ {
			if sq1 == sq2 {
				continue
			}

			f1, r1 := sq1.File(), sq1.Rank()
			f2, r2 := sq2.File(), sq2.Rank()

			df := sign(f2 - f1)
			dr := sign(r2 - r1)

			// Only compute for aligned squares
			if df != 0 && dr != 0 && abs(f2-f1) != abs(r2-r1) {
				continue
			}

			if df == 0 && dr == 0 {
				continue
			}

			var line Bitboard

			// Extend in negative direction
			f, r := f1, r1
			for f >= 0 && f <= 7 && r >= 0 && r <= 7 {
				line |= SquareBB(NewSquare(f, r))
				f -= df
				r -= dr
			}

			// Extend in positive direction
			f, r = f1+df, r1+dr
			for f >= 0 && f <= 7 && r >= 0 && r <= 7 {
				line |= SquareBB(NewSquare(f, r))
				f += df
				r += dr
			}

			lineBB[sq1][sq2] = line
		}
	}
}

func sign(x int) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// KnightAttacks returns the knight attack bitboard for a square.
func KnightAttacks(sq Square) Bitboard {
	return knightAttacks[sq]
}

// KingAttacks returns the king attack bitboard for a square.
func KingAttacks(sq Square) Bitboard {
	return kingAttacks[sq]
}

// PawnAttacks returns the pawn attack bitboard for a square and color.
func PawnAttacks(sq Square, c Color) Bitboard {
	return pawnAttacks[c][sq]
}

// PawnPushes returns the pawn push target bitboard for a square and color.
func PawnPushes(sq Square, c Color) Bitboard {
	return pawnPushes[c][sq]
}

// BishopAttacks returns the bishop attack bitboard for a square with given occupancy.
func BishopAttacks(sq Square, occupied Bitboard) Bitboard {
	return getBishopAttacks(sq, occupied)
}

// RookAttacks returns the rook attack bitboard for a square with given occupancy.
func RookAttacks(sq Square, occupied Bitboard) Bitboard {
	return getRookAttacks(sq, occupied)
}

// QueenAttacks returns the queen attack bitboard for a square with given occupancy.
func QueenAttacks(sq Square, occupied Bitboard) Bitboard {
	return BishopAttacks(sq, occupied) | RookAttacks(sq, occupied)
}

// Between returns the bitboard of squares strictly between two squares.
// Returns empty if squares are not aligned (not on same rank, file, or diagonal).
func Between(sq1, sq2 Square) Bitboard {
	return betweenBB[sq1][sq2]
}

// Line returns the bitboard of the full line through two squares.
// Returns empty if squares are not aligned.
func Line(sq1, sq2 Square) Bitboard {
	return lineBB[sq1][sq2]
}

// Aligned returns true if three squares are on the same line.
func Aligned(sq1, sq2, sq3 Square) bool {
	return lineBB[sq1][sq2]&SquareBB(sq3) != 0
}

// AttackersTo returns a bitboard of all pieces attacking a square.
func (p *Position) AttackersTo(sq Square, occupied Bitboard) Bitboard {
	return (pawnAttacks[Black][sq] & p.Pieces[White][Pawn]) |
		(pawnAttacks[White][sq] & p.Pieces[Black][Pawn]) |
		(knightAttacks[sq] & (p.Pieces[White][Knight] | p.Pieces[Black][Knight])) |
		(kingAttacks[sq] & (p.Pieces[White][King] | p.Pieces[Black][King])) |
		(BishopAttacks(sq, occupied) & (p.Pieces[White][Bishop] | p.Pieces[Black][Bishop] | p.Pieces[White][Queen] | p.Pieces[Black][Queen])) |
		(RookAttacks(sq, occupied) & (p.Pieces[White][Rook] | p.Pieces[Black][Rook] | p.Pieces[White][Queen] | p.Pieces[Black][Queen]))
}

// AttackersByColor returns a bitboard of pieces of the given color attacking a square.
func (p *Position) AttackersByColor(sq Square, c Color, occupied Bitboard) Bitboard {
	enemy := c.Other()
	return (pawnAttacks[enemy][sq] & p.Pieces[c][Pawn]) |
		(knightAttacks[sq] & p.Pieces[c][Knight]) |
		(kingAttacks[sq] & p.Pieces[c][King]) |
		(BishopAttacks(sq, occupied) & (p.Pieces[c][Bishop] | p.Pieces[c][Queen])) |
		(RookAttacks(sq, occupied) & (p.Pieces[c][Rook] | p.Pieces[c][Queen]))
}

// IsSquareAttacked returns true if the square is attacked by the given color.
func (p *Position) IsSquareAttacked(sq Square, byColor Color) bool {
	return p.AttackersByColor(sq, byColor, p.AllOccupied) != 0
}

// UpdateCheckers updates the Checkers bitboard for the side to move.
func (p *Position) UpdateCheckers() {
	// Use actual King bitboard for defensive correctness
	us := p.SideToMove
	kingBB := p.Pieces[us][King]
	if kingBB == 0 {
		// No King on board - can't compute checkers, set to 0
		p.Checkers = 0
		return
	}
	kingSq := kingBB.LSB()
	p.Checkers = p.AttackersByColor(kingSq, us.Other(), p.AllOccupied)
}
