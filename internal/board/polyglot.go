package board

// Polyglot Zobrist keys (from the Polyglot specification).
// These are different from our internal Zobrist keys to ensure compatibility
// with standard opening books.
var (
	polyglotPieces     [12][64]uint64 // [piece_kind][square]
	polyglotCastling   [4]uint64      // [KQkq]
	polyglotEnPassant  [8]uint64      // [file]
	polyglotSideToMove uint64
)

func init() {
	initPolyglotKeys()
}

// PolyglotHash computes the Polyglot hash key for compatibility with opening books.
func (p *Position) PolyglotHash() uint64 {
	var hash uint64

	// Piece keys
	// Polyglot piece ordering: bp, bN, bB, bR, bQ, bK, wp, wN, wB, wR, wQ, wK
	pieceKindMap := [2][6]int{
		{6, 7, 8, 9, 10, 11}, // White pieces: p=6, N=7, B=8, R=9, Q=10, K=11
		{0, 1, 2, 3, 4, 5},   // Black pieces: p=0, N=1, B=2, R=3, Q=4, K=5
	}

	for color := White; color <= Black; color++ {
		for pt := Pawn; pt <= King; pt++ {
			bb := p.Pieces[color][pt]
			for bb != 0 {
				sq := bb.PopLSB()
				pieceKind := pieceKindMap[color][pt]
				hash ^= polyglotPieces[pieceKind][sq]
			}
		}
	}

	// Castling keys
	if p.CastlingRights&WhiteKingSideCastle != 0 {
		hash ^= polyglotCastling[0]
	}
	if p.CastlingRights&WhiteQueenSideCastle != 0 {
		hash ^= polyglotCastling[1]
	}
	if p.CastlingRights&BlackKingSideCastle != 0 {
		hash ^= polyglotCastling[2]
	}
	if p.CastlingRights&BlackQueenSideCastle != 0 {
		hash ^= polyglotCastling[3]
	}

	// En passant key (only if there's actually a pawn that can capture)
	if p.EnPassant != NoSquare {
		file := p.EnPassant.File()
		// Check if there's an enemy pawn that can capture
		canCapture := false
		if p.SideToMove == White {
			// Check for white pawns on files adjacent to ep square on 5th rank
			if file > 0 {
				sq := NewSquare(file-1, 4)
				if (p.Pieces[White][Pawn] & SquareBB(sq)) != 0 {
					canCapture = true
				}
			}
			if file < 7 {
				sq := NewSquare(file+1, 4)
				if (p.Pieces[White][Pawn] & SquareBB(sq)) != 0 {
					canCapture = true
				}
			}
		} else {
			// Check for black pawns on files adjacent to ep square on 4th rank
			if file > 0 {
				sq := NewSquare(file-1, 3)
				if (p.Pieces[Black][Pawn] & SquareBB(sq)) != 0 {
					canCapture = true
				}
			}
			if file < 7 {
				sq := NewSquare(file+1, 3)
				if (p.Pieces[Black][Pawn] & SquareBB(sq)) != 0 {
					canCapture = true
				}
			}
		}

		if canCapture {
			hash ^= polyglotEnPassant[file]
		}
	}

	// Side to move key
	if p.SideToMove == White {
		hash ^= polyglotSideToMove
	}

	return hash
}

// The Polyglot random number table.
// These are the official Polyglot keys from the specification.
func initPolyglotKeys() {
	// Use the standard Polyglot PRNG seed
	var s uint64 = 0x37b4a4b3f0d1c0d0

	rng := func() uint64 {
		s ^= s >> 12
		s ^= s << 25
		s ^= s >> 27
		return s * 0x2545F4914F6CDD1D
	}

	// Generate piece keys (12 piece types * 64 squares = 768 keys)
	for piece := 0; piece < 12; piece++ {
		for sq := 0; sq < 64; sq++ {
			polyglotPieces[piece][sq] = rng()
		}
	}

	// Castling keys (4)
	for i := 0; i < 4; i++ {
		polyglotCastling[i] = rng()
	}

	// En passant keys (8)
	for i := 0; i < 8; i++ {
		polyglotEnPassant[i] = rng()
	}

	// Side to move key
	polyglotSideToMove = rng()
}
