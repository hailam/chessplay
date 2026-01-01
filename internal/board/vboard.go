package board

// VBoard is a lightweight board for move simulation.
// Unlike Position, it only contains data needed for attack detection.
// Size: ~130 bytes, stack-allocated, no GC pressure.
type VBoard struct {
	Pieces      [2][6]Bitboard
	Occupied    [2]Bitboard
	AllOccupied Bitboard
	KingSquare  [2]Square
}

// NewVBoard creates a VBoard from a Position.
func NewVBoard(p *Position) VBoard {
	return VBoard{
		Pieces:      p.Pieces,
		Occupied:    p.Occupied,
		AllOccupied: p.AllOccupied,
		KingSquare:  p.KingSquare,
	}
}

// ApplyMove applies a move to the VBoard (no validation, no hash update).
func (v *VBoard) ApplyMove(m Move, us Color) {
	them := us.Other()
	from, to := m.From(), m.To()
	fromBB, toBB := SquareBB(from), SquareBB(to)

	// Find moving piece type
	var pt PieceType
	for t := Pawn; t <= King; t++ {
		if v.Pieces[us][t]&fromBB != 0 {
			pt = t
			break
		}
	}

	// Handle capture - remove enemy piece at destination
	for t := Pawn; t <= King; t++ {
		if v.Pieces[them][t]&toBB != 0 {
			v.Pieces[them][t] &^= toBB
			v.Occupied[them] &^= toBB
			break
		}
	}

	// En passant capture
	if m.IsEnPassant() {
		var capSq Square
		if us == White {
			capSq = to - 8
		} else {
			capSq = to + 8
		}
		capBB := SquareBB(capSq)
		v.Pieces[them][Pawn] &^= capBB
		v.Occupied[them] &^= capBB
	}

	// Move the piece
	moveBB := fromBB | toBB
	v.Pieces[us][pt] ^= moveBB
	v.Occupied[us] ^= moveBB
	v.AllOccupied = v.Occupied[White] | v.Occupied[Black]

	// Update king position if king moved
	if pt == King {
		v.KingSquare[us] = to
	}

	// Handle promotion
	if m.IsPromotion() {
		v.Pieces[us][Pawn] &^= toBB
		v.Pieces[us][m.Promotion()] |= toBB
	}

	// Handle castling rook movement
	if m.IsCastling() {
		var rookFrom, rookTo Square
		if to > from { // Kingside
			rookFrom = from + 3
			rookTo = from + 1
		} else { // Queenside
			rookFrom = from - 4
			rookTo = from - 1
		}
		rookBB := SquareBB(rookFrom) | SquareBB(rookTo)
		v.Pieces[us][Rook] ^= rookBB
		v.Occupied[us] ^= rookBB
		v.AllOccupied = v.Occupied[White] | v.Occupied[Black]
	}
}

// IsKingAttacked checks if the king on kingSq is attacked by byColor.
func (v *VBoard) IsKingAttacked(kingSq Square, byColor Color) bool {
	us := byColor.Other()

	// Check pawn attacks
	if pawnAttacks[us][kingSq]&v.Pieces[byColor][Pawn] != 0 {
		return true
	}

	// Check knight attacks
	if KnightAttacks(kingSq)&v.Pieces[byColor][Knight] != 0 {
		return true
	}

	// Check king attacks (adjacent kings)
	if KingAttacks(kingSq)&v.Pieces[byColor][King] != 0 {
		return true
	}

	// Check bishop/queen attacks (diagonals)
	bishopsQueens := v.Pieces[byColor][Bishop] | v.Pieces[byColor][Queen]
	if BishopAttacks(kingSq, v.AllOccupied)&bishopsQueens != 0 {
		return true
	}

	// Check rook/queen attacks (ranks/files)
	rooksQueens := v.Pieces[byColor][Rook] | v.Pieces[byColor][Queen]
	if RookAttacks(kingSq, v.AllOccupied)&rooksQueens != 0 {
		return true
	}

	return false
}
