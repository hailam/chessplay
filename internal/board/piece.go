package board

// Color represents the color of a piece or player.
type Color uint8

const (
	White Color = iota
	Black
	NoColor Color = 2
)

// Other returns the opposite color.
func (c Color) Other() Color {
	return c ^ 1
}

// String returns the color name.
func (c Color) String() string {
	switch c {
	case White:
		return "White"
	case Black:
		return "Black"
	default:
		return "NoColor"
	}
}

// PieceType represents the type of a chess piece.
type PieceType uint8

const (
	Pawn PieceType = iota
	Knight
	Bishop
	Rook
	Queen
	King
	NoPieceType PieceType = 6
)

// String returns the piece type name.
func (pt PieceType) String() string {
	switch pt {
	case Pawn:
		return "Pawn"
	case Knight:
		return "Knight"
	case Bishop:
		return "Bishop"
	case Rook:
		return "Rook"
	case Queen:
		return "Queen"
	case King:
		return "King"
	default:
		return "None"
	}
}

// Char returns the FEN character for the piece type (lowercase).
func (pt PieceType) Char() byte {
	chars := []byte{'p', 'n', 'b', 'r', 'q', 'k', ' '}
	if pt > NoPieceType {
		return ' '
	}
	return chars[pt]
}

// PieceValue returns the material value of the piece type in centipawns.
var PieceValue = [7]int{100, 320, 330, 500, 900, 20000, 0}

// Piece combines PieceType and Color into a single value.
// Encoded as: pieceType + color*6
type Piece uint8

const (
	WhitePawn   Piece = Piece(Pawn) + Piece(White)*6
	WhiteKnight Piece = Piece(Knight) + Piece(White)*6
	WhiteBishop Piece = Piece(Bishop) + Piece(White)*6
	WhiteRook   Piece = Piece(Rook) + Piece(White)*6
	WhiteQueen  Piece = Piece(Queen) + Piece(White)*6
	WhiteKing   Piece = Piece(King) + Piece(White)*6
	BlackPawn   Piece = Piece(Pawn) + Piece(Black)*6
	BlackKnight Piece = Piece(Knight) + Piece(Black)*6
	BlackBishop Piece = Piece(Bishop) + Piece(Black)*6
	BlackRook   Piece = Piece(Rook) + Piece(Black)*6
	BlackQueen  Piece = Piece(Queen) + Piece(Black)*6
	BlackKing   Piece = Piece(King) + Piece(Black)*6
	NoPiece     Piece = 12
)

// NewPiece creates a Piece from PieceType and Color.
func NewPiece(pt PieceType, c Color) Piece {
	if pt >= NoPieceType || c >= NoColor {
		return NoPiece
	}
	return Piece(pt) + Piece(c)*6
}

// Type returns the PieceType of the piece.
func (p Piece) Type() PieceType {
	if p >= NoPiece {
		return NoPieceType
	}
	return PieceType(p % 6)
}

// Color returns the Color of the piece.
func (p Piece) Color() Color {
	if p >= NoPiece {
		return NoColor
	}
	return Color(p / 6)
}

// String returns the FEN character for the piece.
// Uppercase for white, lowercase for black.
func (p Piece) String() string {
	if p >= NoPiece {
		return " "
	}
	chars := "PNBRQKpnbrqk"
	return string(chars[p])
}

// PieceFromChar converts a FEN character to a Piece.
func PieceFromChar(c byte) Piece {
	switch c {
	case 'P':
		return WhitePawn
	case 'N':
		return WhiteKnight
	case 'B':
		return WhiteBishop
	case 'R':
		return WhiteRook
	case 'Q':
		return WhiteQueen
	case 'K':
		return WhiteKing
	case 'p':
		return BlackPawn
	case 'n':
		return BlackKnight
	case 'b':
		return BlackBishop
	case 'r':
		return BlackRook
	case 'q':
		return BlackQueen
	case 'k':
		return BlackKing
	default:
		return NoPiece
	}
}

// Value returns the material value of the piece in centipawns.
func (p Piece) Value() int {
	return PieceValue[p.Type()]
}
