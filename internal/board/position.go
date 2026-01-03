package board

import "fmt"

// CastlingRights represents the available castling options.
type CastlingRights uint8

const (
	WhiteKingSideCastle  CastlingRights = 1 << iota // K
	WhiteQueenSideCastle                            // Q
	BlackKingSideCastle                             // k
	BlackQueenSideCastle                            // q
	NoCastling           CastlingRights = 0
	AllCastling          CastlingRights = WhiteKingSideCastle | WhiteQueenSideCastle | BlackKingSideCastle | BlackQueenSideCastle
)

// String returns the FEN castling rights string.
func (cr CastlingRights) String() string {
	if cr == NoCastling {
		return "-"
	}
	s := ""
	if cr&WhiteKingSideCastle != 0 {
		s += "K"
	}
	if cr&WhiteQueenSideCastle != 0 {
		s += "Q"
	}
	if cr&BlackKingSideCastle != 0 {
		s += "k"
	}
	if cr&BlackQueenSideCastle != 0 {
		s += "q"
	}
	return s
}

// CanCastle returns true if the given side can castle in the given direction.
func (cr CastlingRights) CanCastle(c Color, kingSide bool) bool {
	if c == White {
		if kingSide {
			return cr&WhiteKingSideCastle != 0
		}
		return cr&WhiteQueenSideCastle != 0
	}
	if kingSide {
		return cr&BlackKingSideCastle != 0
	}
	return cr&BlackQueenSideCastle != 0
}

// Position represents a complete chess position.
type Position struct {
	// Piece bitboards: [Color][PieceType]
	Pieces [2][6]Bitboard

	// Occupancy bitboards (cached for efficiency)
	Occupied    [2]Bitboard // All pieces of each color
	AllOccupied Bitboard    // All pieces on the board

	// Game state
	SideToMove     Color
	CastlingRights CastlingRights
	EnPassant      Square // Target square for en passant, NoSquare if none
	HalfMoveClock  int    // Moves since last pawn move or capture (for 50-move rule)
	FullMoveNumber int    // Full move counter, starts at 1

	// Zobrist hash for transposition table
	Hash uint64

	// Pawn hash key for pawn structure caching
	PawnKey uint64

	// King positions (cached for check detection)
	KingSquare [2]Square

	// Checkers bitboard (pieces giving check)
	Checkers Bitboard
}

// NewPosition creates the starting position.
func NewPosition() *Position {
	pos, _ := ParseFEN(StartFEN)
	return pos
}

// Copy creates a deep copy of the position.
func (p *Position) Copy() *Position {
	newPos := *p
	return &newPos
}

// PieceAt returns the piece at the given square, or NoPiece if empty.
func (p *Position) PieceAt(sq Square) Piece {
	bb := SquareBB(sq)

	// Check if square is occupied
	if p.AllOccupied&bb == 0 {
		return NoPiece
	}

	// Find the color
	var c Color
	if p.Occupied[White]&bb != 0 {
		c = White
	} else {
		c = Black
	}

	// Find the piece type
	for pt := Pawn; pt <= King; pt++ {
		if p.Pieces[c][pt]&bb != 0 {
			return NewPiece(pt, c)
		}
	}

	return NoPiece
}

// IsEmpty returns true if the square is empty.
func (p *Position) IsEmpty(sq Square) bool {
	return p.AllOccupied&SquareBB(sq) == 0
}

// setPiece places a piece on a square (does not update hash).
func (p *Position) setPiece(piece Piece, sq Square) {
	if piece == NoPiece {
		return
	}
	c := piece.Color()
	pt := piece.Type()
	bb := SquareBB(sq)

	p.Pieces[c][pt] |= bb
	p.Occupied[c] |= bb
	p.AllOccupied |= bb

	if pt == King {
		p.KingSquare[c] = sq
	}
}

// removePiece removes a piece from a square (does not update hash).
func (p *Position) removePiece(sq Square) Piece {
	piece := p.PieceAt(sq)
	if piece == NoPiece {
		return NoPiece
	}

	c := piece.Color()
	pt := piece.Type()
	bb := SquareBB(sq)

	p.Pieces[c][pt] &^= bb
	p.Occupied[c] &^= bb
	p.AllOccupied &^= bb

	return piece
}

// movePiece moves a piece from one square to another (does not update hash).
func (p *Position) movePiece(from, to Square) {
	piece := p.PieceAt(from)
	if piece == NoPiece {
		return
	}

	c := piece.Color()
	pt := piece.Type()
	fromBB := SquareBB(from)
	toBB := SquareBB(to)
	moveBB := fromBB | toBB

	p.Pieces[c][pt] ^= moveBB
	p.Occupied[c] ^= moveBB
	p.AllOccupied ^= moveBB

	if pt == King {
		p.KingSquare[c] = to
	}
}

// updateOccupied recalculates occupancy bitboards from piece bitboards.
func (p *Position) updateOccupied() {
	p.Occupied[White] = Empty
	p.Occupied[Black] = Empty

	for pt := Pawn; pt <= King; pt++ {
		p.Occupied[White] |= p.Pieces[White][pt]
		p.Occupied[Black] |= p.Pieces[Black][pt]
	}

	p.AllOccupied = p.Occupied[White] | p.Occupied[Black]
}

// findKings locates and caches the king positions.
func (p *Position) findKings() {
	p.KingSquare[White] = p.Pieces[White][King].LSB()
	p.KingSquare[Black] = p.Pieces[Black][King].LSB()
}

// String returns a visual representation of the position.
func (p *Position) String() string {
	s := "\n"
	for rank := 7; rank >= 0; rank-- {
		s += fmt.Sprintf("%d  ", rank+1)
		for file := 0; file < 8; file++ {
			sq := NewSquare(file, rank)
			piece := p.PieceAt(sq)
			if piece == NoPiece {
				s += ". "
			} else {
				s += piece.String() + " "
			}
		}
		s += "\n"
	}
	s += "\n   a b c d e f g h\n\n"
	s += fmt.Sprintf("Side to move: %s\n", p.SideToMove)
	s += fmt.Sprintf("Castling: %s\n", p.CastlingRights)
	s += fmt.Sprintf("En passant: %s\n", p.EnPassant)
	s += fmt.Sprintf("Half-move clock: %d\n", p.HalfMoveClock)
	s += fmt.Sprintf("Full move: %d\n", p.FullMoveNumber)
	s += fmt.Sprintf("Hash: %016x\n", p.Hash)
	return s
}

// Clear resets the position to an empty board.
func (p *Position) Clear() {
	*p = Position{
		EnPassant:      NoSquare,
		FullMoveNumber: 1,
	}
	p.KingSquare[White] = NoSquare
	p.KingSquare[Black] = NoSquare
}

// Validate checks if the position is valid.
func (p *Position) Validate() error {
	// Check that each side has exactly one king
	if p.Pieces[White][King].PopCount() != 1 {
		return fmt.Errorf("white must have exactly one king")
	}
	if p.Pieces[Black][King].PopCount() != 1 {
		return fmt.Errorf("black must have exactly one king")
	}

	// Check that pawns are not on rank 1 or 8
	if (p.Pieces[White][Pawn]|p.Pieces[Black][Pawn])&(Rank1|Rank8) != 0 {
		return fmt.Errorf("pawns cannot be on rank 1 or 8")
	}

	// Check that opponent's king is not in check (would be illegal position)
	// This will be implemented after attack generation

	return nil
}

// GameOver returns true if the game is over (checkmate, stalemate, or draw).
// This will be implemented after move generation.
func (p *Position) GameOver() bool {
	return false
}

// InCheck returns true if the side to move is in check.
// This will be implemented after attack generation.
func (p *Position) InCheck() bool {
	return p.Checkers != 0
}

// Material returns the material balance (positive favors white).
func (p *Position) Material() int {
	score := 0
	for pt := Pawn; pt < King; pt++ {
		score += p.Pieces[White][pt].PopCount() * PieceValue[pt]
		score -= p.Pieces[Black][pt].PopCount() * PieceValue[pt]
	}
	return score
}

// ComputePinned computes pieces pinned to the king for the side to move.
// Uses Stockfish-style x-ray attack detection.
func (p *Position) ComputePinned() Bitboard {
	us := p.SideToMove
	them := us.Other()
	ksq := p.KingSquare[us]
	pinned := Bitboard(0)

	// Rook/Queen x-ray attacks (horizontal and vertical)
	snipers := RookAttacks(ksq, 0) & (p.Pieces[them][Rook] | p.Pieces[them][Queen])
	for snipers != 0 {
		sq := snipers.PopLSB()
		blockers := Between(sq, ksq) & p.AllOccupied
		if blockers.PopCount() == 1 && blockers&p.Occupied[us] != 0 {
			pinned |= blockers
		}
	}

	// Bishop/Queen x-ray attacks (diagonals)
	snipers = BishopAttacks(ksq, 0) & (p.Pieces[them][Bishop] | p.Pieces[them][Queen])
	for snipers != 0 {
		sq := snipers.PopLSB()
		blockers := Between(sq, ksq) & p.AllOccupied
		if blockers.PopCount() == 1 && blockers&p.Occupied[us] != 0 {
			pinned |= blockers
		}
	}

	return pinned
}

// NullMoveUndo stores state for unmake of null move.
// Returned by MakeNullMove and passed to UnmakeNullMove.
type NullMoveUndo struct {
	EnPassant Square
	Hash      uint64
}

// MakeNullMove makes a null move (passes the turn without moving).
// Used for null move pruning in search.
// Returns undo info that must be passed to UnmakeNullMove.
func (p *Position) MakeNullMove() NullMoveUndo {
	// Save state for unmake
	undo := NullMoveUndo{
		EnPassant: p.EnPassant,
		Hash:      p.Hash,
	}

	// Update hash for en passant removal
	if p.EnPassant != NoSquare {
		p.Hash ^= zobristEnPassant[p.EnPassant.File()]
	}

	// Clear en passant
	p.EnPassant = NoSquare

	// Switch side
	p.SideToMove = p.SideToMove.Other()
	p.Hash ^= zobristSideToMove

	// Update checkers for new side
	p.UpdateCheckers()

	return undo
}

// UnmakeNullMove undoes a null move.
func (p *Position) UnmakeNullMove(undo NullMoveUndo) {
	// Restore state
	p.EnPassant = undo.EnPassant
	p.Hash = undo.Hash
	p.SideToMove = p.SideToMove.Other()

	// Update checkers for restored side
	p.UpdateCheckers()
}

// HasNonPawnMaterial returns true if the side to move has non-pawn material.
// Used for null move pruning (avoid in pure pawn endgames due to zugzwang).
func (p *Position) HasNonPawnMaterial() bool {
	us := p.SideToMove
	return p.Pieces[us][Knight]|p.Pieces[us][Bishop]|p.Pieces[us][Rook]|p.Pieces[us][Queen] != 0
}
