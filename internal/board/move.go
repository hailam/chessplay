package board

import "fmt"

// Move encodes a chess move in 16 bits:
// bits 0-5:   from square (0-63)
// bits 6-11:  to square (0-63)
// bits 12-13: promotion piece (0=Knight, 1=Bishop, 2=Rook, 3=Queen)
// bits 14-15: flags (0=normal, 1=promotion, 2=en passant, 3=castling)
type Move uint16

// Move flags
const (
	FlagNormal    uint16 = 0 << 14
	FlagPromotion uint16 = 1 << 14
	FlagEnPassant uint16 = 2 << 14
	FlagCastling  uint16 = 3 << 14
)

// NoMove represents an invalid or null move.
const NoMove Move = 0

// NewMove creates a normal move.
func NewMove(from, to Square) Move {
	return Move(from) | Move(to)<<6
}

// NewPromotion creates a promotion move.
func NewPromotion(from, to Square, promo PieceType) Move {
	// promo: Knight=0, Bishop=1, Rook=2, Queen=3
	promoIdx := promo - Knight
	return Move(from) | Move(to)<<6 | Move(promoIdx)<<12 | Move(FlagPromotion)
}

// NewEnPassant creates an en passant capture move.
func NewEnPassant(from, to Square) Move {
	return Move(from) | Move(to)<<6 | Move(FlagEnPassant)
}

// NewCastling creates a castling move (king's movement).
func NewCastling(from, to Square) Move {
	return Move(from) | Move(to)<<6 | Move(FlagCastling)
}

// From returns the origin square.
func (m Move) From() Square {
	return Square(m & 0x3F)
}

// To returns the destination square.
func (m Move) To() Square {
	return Square((m >> 6) & 0x3F)
}

// Flag returns the move flag.
func (m Move) Flag() uint16 {
	return uint16(m) & 0xC000
}

// Promotion returns the promotion piece type (only valid if IsPromotion() is true).
func (m Move) Promotion() PieceType {
	return PieceType((m>>12)&3) + Knight
}

// IsPromotion returns true if this is a promotion move.
func (m Move) IsPromotion() bool {
	return m.Flag() == FlagPromotion
}

// IsCastling returns true if this is a castling move.
func (m Move) IsCastling() bool {
	return m.Flag() == FlagCastling
}

// IsEnPassant returns true if this is an en passant capture.
func (m Move) IsEnPassant() bool {
	return m.Flag() == FlagEnPassant
}

// IsCapture returns true if this move captures a piece.
func (m Move) IsCapture(pos *Position) bool {
	if m.IsEnPassant() {
		return true
	}
	return !pos.IsEmpty(m.To())
}

// IsQuiet returns true if this is not a capture or promotion.
func (m Move) IsQuiet(pos *Position) bool {
	return !m.IsCapture(pos) && !m.IsPromotion()
}

// String returns the UCI format of the move (e.g., "e2e4", "e7e8q").
func (m Move) String() string {
	if m == NoMove {
		return "0000"
	}

	s := m.From().String() + m.To().String()

	if m.IsPromotion() {
		promoChars := []byte{'n', 'b', 'r', 'q'}
		s += string(promoChars[m.Promotion()-Knight])
	}

	return s
}

// ParseMove parses a UCI format move string.
func ParseMove(s string, pos *Position) (Move, error) {
	if len(s) < 4 {
		return NoMove, fmt.Errorf("invalid move string: %s", s)
	}

	from, err := ParseSquare(s[0:2])
	if err != nil {
		return NoMove, err
	}

	to, err := ParseSquare(s[2:4])
	if err != nil {
		return NoMove, err
	}

	// Check for promotion
	if len(s) == 5 {
		var promo PieceType
		switch s[4] {
		case 'n':
			promo = Knight
		case 'b':
			promo = Bishop
		case 'r':
			promo = Rook
		case 'q':
			promo = Queen
		default:
			return NoMove, fmt.Errorf("invalid promotion piece: %c", s[4])
		}
		return NewPromotion(from, to, promo), nil
	}

	// Detect special moves
	piece := pos.PieceAt(from)
	if piece == NoPiece {
		return NoMove, fmt.Errorf("no piece at %s", from)
	}

	pt := piece.Type()

	// Castling
	if pt == King && abs(int(to)-int(from)) == 2 {
		return NewCastling(from, to), nil
	}

	// En passant
	if pt == Pawn && to == pos.EnPassant {
		return NewEnPassant(from, to), nil
	}

	return NewMove(from, to), nil
}

// MoveList is a fixed-size list of moves to avoid allocations.
type MoveList struct {
	moves [256]Move
	count int
}

// NewMoveList creates an empty move list.
func NewMoveList() *MoveList {
	return &MoveList{}
}

// Add adds a move to the list.
func (ml *MoveList) Add(m Move) {
	ml.moves[ml.count] = m
	ml.count++
}

// Len returns the number of moves in the list.
func (ml *MoveList) Len() int {
	return ml.count
}

// Get returns the move at index i.
func (ml *MoveList) Get(i int) Move {
	return ml.moves[i]
}

// Set sets the move at index i.
func (ml *MoveList) Set(i int, m Move) {
	ml.moves[i] = m
}

// Swap swaps two moves in the list.
func (ml *MoveList) Swap(i, j int) {
	ml.moves[i], ml.moves[j] = ml.moves[j], ml.moves[i]
}

// Clear clears the list.
func (ml *MoveList) Clear() {
	ml.count = 0
}

// Contains returns true if the list contains the move.
func (ml *MoveList) Contains(m Move) bool {
	for i := 0; i < ml.count; i++ {
		if ml.moves[i] == m {
			return true
		}
	}
	return false
}

// Slice returns the moves as a slice.
func (ml *MoveList) Slice() []Move {
	return ml.moves[:ml.count]
}

// UndoInfo stores information needed to undo a move.
type UndoInfo struct {
	CapturedPiece  Piece
	CastlingRights CastlingRights
	EnPassant      Square
	HalfMoveClock  int
	Hash           uint64
	PawnKey        uint64
	Checkers       Bitboard
	KingSquare     [2]Square     // King positions before move
	Pieces         [2][6]Bitboard // Full piece bitboards for reliable restoration
	Occupied       [2]Bitboard   // Occupancy bitboards
	AllOccupied    Bitboard      // All pieces
	Valid          bool          // True if move was actually applied
}
