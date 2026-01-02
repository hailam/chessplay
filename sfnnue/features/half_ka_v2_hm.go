// HalfKAv2_hm feature set for NNUE evaluation.
// Ported from Stockfish src/nnue/features/half_ka_v2_hm.h and .cpp
//
// Feature HalfKAv2_hm: Combination of the position of own king and the
// position of pieces. Position mirrored such that king is always on e..h files.

package features

// Square constants
const (
	SQ_A1 = 0
	SQ_H1 = 7
	SQ_A8 = 56
	SQ_H8 = 63

	SQUARE_NB = 64
)

// Color constants
const (
	White = 0
	Black = 1

	COLOR_NB = 2
)

// Piece type constants
const (
	NO_PIECE_TYPE = 0
	PAWN          = 1
	KNIGHT        = 2
	BISHOP        = 3
	ROOK          = 4
	QUEEN         = 5
	KING          = 6

	PIECE_TYPE_NB = 8
)

// Piece constants (color + type encoded)
const (
	NO_PIECE = 0

	W_PAWN   = 1
	W_KNIGHT = 2
	W_BISHOP = 3
	W_ROOK   = 4
	W_QUEEN  = 5
	W_KING   = 6

	B_PAWN   = 9
	B_KNIGHT = 10
	B_BISHOP = 11
	B_ROOK   = 12
	B_QUEEN  = 13
	B_KING   = 14

	PIECE_NB = 16
)

// Unique number for each piece type on each square (half_ka_v2_hm.h:41-55)
const (
	PS_NONE     = 0
	PS_W_PAWN   = 0
	PS_B_PAWN   = 1 * SQUARE_NB
	PS_W_KNIGHT = 2 * SQUARE_NB
	PS_B_KNIGHT = 3 * SQUARE_NB
	PS_W_BISHOP = 4 * SQUARE_NB
	PS_B_BISHOP = 5 * SQUARE_NB
	PS_W_ROOK   = 6 * SQUARE_NB
	PS_B_ROOK   = 7 * SQUARE_NB
	PS_W_QUEEN  = 8 * SQUARE_NB
	PS_B_QUEEN  = 9 * SQUARE_NB
	PS_KING     = 10 * SQUARE_NB
	PS_NB       = 11 * SQUARE_NB
)

// Feature name (half_ka_v2_hm.h:67)
const Name = "HalfKAv2_hm(Friend)"

// Hash value embedded in the evaluation file (half_ka_v2_hm.h:70)
const HashValue uint32 = 0x7f234cb8

// Number of feature dimensions (half_ka_v2_hm.h:73-74)
const Dimensions = SQUARE_NB * PS_NB / 2 // = 22528

// Maximum number of simultaneously active features (half_ka_v2_hm.h:105)
const MaxActiveDimensions = 32

// PieceSquareIndex maps piece to piece-square index for each perspective (half_ka_v2_hm.h:57-63)
// Convention: W - us, B - them. Viewed from other side, W and B are reversed.
var PieceSquareIndex = [COLOR_NB][PIECE_NB]int{
	// White perspective
	{PS_NONE, PS_W_PAWN, PS_W_KNIGHT, PS_W_BISHOP, PS_W_ROOK, PS_W_QUEEN, PS_KING, PS_NONE,
		PS_NONE, PS_B_PAWN, PS_B_KNIGHT, PS_B_BISHOP, PS_B_ROOK, PS_B_QUEEN, PS_KING, PS_NONE},
	// Black perspective
	{PS_NONE, PS_B_PAWN, PS_B_KNIGHT, PS_B_BISHOP, PS_B_ROOK, PS_B_QUEEN, PS_KING, PS_NONE,
		PS_NONE, PS_W_PAWN, PS_W_KNIGHT, PS_W_BISHOP, PS_W_ROOK, PS_W_QUEEN, PS_KING, PS_NONE},
}

// KingBuckets maps each king square to a bucket index (half_ka_v2_hm.h:78-87)
// The value is pre-multiplied by PS_NB for efficiency.
var KingBuckets = [SQUARE_NB]int{
	28 * PS_NB, 29 * PS_NB, 30 * PS_NB, 31 * PS_NB, 31 * PS_NB, 30 * PS_NB, 29 * PS_NB, 28 * PS_NB,
	24 * PS_NB, 25 * PS_NB, 26 * PS_NB, 27 * PS_NB, 27 * PS_NB, 26 * PS_NB, 25 * PS_NB, 24 * PS_NB,
	20 * PS_NB, 21 * PS_NB, 22 * PS_NB, 23 * PS_NB, 23 * PS_NB, 22 * PS_NB, 21 * PS_NB, 20 * PS_NB,
	16 * PS_NB, 17 * PS_NB, 18 * PS_NB, 19 * PS_NB, 19 * PS_NB, 18 * PS_NB, 17 * PS_NB, 16 * PS_NB,
	12 * PS_NB, 13 * PS_NB, 14 * PS_NB, 15 * PS_NB, 15 * PS_NB, 14 * PS_NB, 13 * PS_NB, 12 * PS_NB,
	8 * PS_NB, 9 * PS_NB, 10 * PS_NB, 11 * PS_NB, 11 * PS_NB, 10 * PS_NB, 9 * PS_NB, 8 * PS_NB,
	4 * PS_NB, 5 * PS_NB, 6 * PS_NB, 7 * PS_NB, 7 * PS_NB, 6 * PS_NB, 5 * PS_NB, 4 * PS_NB,
	0 * PS_NB, 1 * PS_NB, 2 * PS_NB, 3 * PS_NB, 3 * PS_NB, 2 * PS_NB, 1 * PS_NB, 0 * PS_NB,
}

// OrientTBL orients a square according to perspective (half_ka_v2_hm.h:91-101)
// SQ_H1 means no flip needed, SQ_A1 means flip horizontally.
var OrientTBL = [SQUARE_NB]int{
	SQ_H1, SQ_H1, SQ_H1, SQ_H1, SQ_A1, SQ_A1, SQ_A1, SQ_A1,
	SQ_H1, SQ_H1, SQ_H1, SQ_H1, SQ_A1, SQ_A1, SQ_A1, SQ_A1,
	SQ_H1, SQ_H1, SQ_H1, SQ_H1, SQ_A1, SQ_A1, SQ_A1, SQ_A1,
	SQ_H1, SQ_H1, SQ_H1, SQ_H1, SQ_A1, SQ_A1, SQ_A1, SQ_A1,
	SQ_H1, SQ_H1, SQ_H1, SQ_H1, SQ_A1, SQ_A1, SQ_A1, SQ_A1,
	SQ_H1, SQ_H1, SQ_H1, SQ_H1, SQ_A1, SQ_A1, SQ_A1, SQ_A1,
	SQ_H1, SQ_H1, SQ_H1, SQ_H1, SQ_A1, SQ_A1, SQ_A1, SQ_A1,
	SQ_H1, SQ_H1, SQ_H1, SQ_H1, SQ_A1, SQ_A1, SQ_A1, SQ_A1,
}

// MakeIndex computes the feature index for a piece from a perspective.
// Ported from half_ka_v2_hm.cpp:32-36
func MakeIndex(perspective int, sq int, pc int, ksq int) int {
	flip := 56 * perspective
	return (sq ^ OrientTBL[ksq] ^ flip) + PieceSquareIndex[perspective][pc] + KingBuckets[ksq^flip]
}

// DirtyPiece represents a changed piece for incremental updates.
type DirtyPiece struct {
	From     int // Source square (or SQ_NONE)
	To       int // Destination square (or SQ_NONE if captured)
	Pc       int // The piece that moved
	RemoveSq int // Additional removed piece square (for captures)
	RemovePc int // Additional removed piece (captured piece)
	AddSq    int // Additional added piece square (for promotions/castling)
	AddPc    int // Additional added piece
}

// SQ_NONE represents no square
const SQ_NONE = 64

// RequiresRefresh returns whether the change means a full accumulator refresh is required.
// Ported from half_ka_v2_hm.cpp:65-67
func RequiresRefresh(diff *DirtyPiece, perspective int) bool {
	// King moves require refresh
	pieceType := diff.Pc & 7 // Extract piece type
	pieceColor := diff.Pc >> 3
	return pieceType == KING && pieceColor == perspective
}

// IndexList is a list of feature indices
type IndexList struct {
	Values [MaxActiveDimensions]int
	Size   int
}

// Push adds an index to the list
func (l *IndexList) Push(idx int) {
	if l.Size < MaxActiveDimensions {
		l.Values[l.Size] = idx
		l.Size++
	}
}

// Clear resets the list
func (l *IndexList) Clear() {
	l.Size = 0
}

// Position interface for getting piece information
type Position interface {
	KingSquare(color int) int
	PieceOn(sq int) int
	Pieces() uint64
}

// PopLSB pops and returns the least significant bit position
func PopLSB(bb *uint64) int {
	if *bb == 0 {
		return -1
	}
	sq := TrailingZeros(*bb)
	*bb &= *bb - 1
	return sq
}

// TrailingZeros returns the number of trailing zeros
func TrailingZeros(bb uint64) int {
	if bb == 0 {
		return 64
	}
	n := 0
	if bb&0xFFFFFFFF == 0 {
		n += 32
		bb >>= 32
	}
	if bb&0xFFFF == 0 {
		n += 16
		bb >>= 16
	}
	if bb&0xFF == 0 {
		n += 8
		bb >>= 8
	}
	if bb&0xF == 0 {
		n += 4
		bb >>= 4
	}
	if bb&0x3 == 0 {
		n += 2
		bb >>= 2
	}
	if bb&0x1 == 0 {
		n += 1
	}
	return n
}

// AppendActiveIndices gets a list of indices for active features.
// Ported from half_ka_v2_hm.cpp:40-48
func AppendActiveIndices(perspective int, pos Position, active *IndexList) {
	ksq := pos.KingSquare(perspective)
	bb := pos.Pieces()
	for bb != 0 {
		sq := PopLSB(&bb)
		pc := pos.PieceOn(sq)
		if pc != NO_PIECE {
			active.Push(MakeIndex(perspective, sq, pc, ksq))
		}
	}
}

// AppendChangedIndices gets a list of indices for recently changed features.
// Ported from half_ka_v2_hm.cpp:52-63
func AppendChangedIndices(perspective int, ksq int, diff *DirtyPiece, removed, added *IndexList) {
	removed.Push(MakeIndex(perspective, diff.From, diff.Pc, ksq))
	if diff.To != SQ_NONE {
		added.Push(MakeIndex(perspective, diff.To, diff.Pc, ksq))
	}

	if diff.RemoveSq != SQ_NONE {
		removed.Push(MakeIndex(perspective, diff.RemoveSq, diff.RemovePc, ksq))
	}

	if diff.AddSq != SQ_NONE {
		added.Push(MakeIndex(perspective, diff.AddSq, diff.AddPc, ksq))
	}
}

// GetChangedFeatures computes the removed and added feature indices for a move.
// This is a convenience function for incremental accumulator updates.
// Returns slices of feature indices that were removed and added.
func GetChangedFeatures(
	perspective int,
	ksq int,
	fromSq, toSq int,
	movingPiece int,
	capturedPiece int, // NO_PIECE if not a capture
	promotionPiece int, // NO_PIECE if not a promotion
	isEnPassant bool,
	epCaptureSq int, // Square of captured pawn for en passant
	isCastling bool,
	rookFromSq, rookToSq int, // Rook squares for castling
) (removed, added []int) {
	removed = make([]int, 0, 4)
	added = make([]int, 0, 4)

	// Moving piece removed from source square
	removed = append(removed, MakeIndex(perspective, fromSq, movingPiece, ksq))

	// Handle promotions vs regular moves
	if promotionPiece != NO_PIECE {
		// Promotion: add promoted piece at destination
		added = append(added, MakeIndex(perspective, toSq, promotionPiece, ksq))
	} else {
		// Regular move: add moving piece at destination
		added = append(added, MakeIndex(perspective, toSq, movingPiece, ksq))
	}

	// Handle captures
	if capturedPiece != NO_PIECE {
		if isEnPassant {
			// En passant: captured pawn is on different square
			removed = append(removed, MakeIndex(perspective, epCaptureSq, capturedPiece, ksq))
		} else {
			// Normal capture: captured piece is on destination square
			removed = append(removed, MakeIndex(perspective, toSq, capturedPiece, ksq))
		}
	}

	// Handle castling: rook also moves
	if isCastling {
		// Determine rook piece based on perspective
		rookPiece := W_ROOK
		if perspective == Black {
			rookPiece = B_ROOK
		}
		// Actually, rook piece depends on the color of the moving king
		kingColor := movingPiece >> 3 // Extract color from piece
		if kingColor == 1 {           // Black
			rookPiece = B_ROOK
		} else {
			rookPiece = W_ROOK
		}
		removed = append(removed, MakeIndex(perspective, rookFromSq, rookPiece, ksq))
		added = append(added, MakeIndex(perspective, rookToSq, rookPiece, ksq))
	}

	return removed, added
}

// IsKingMove checks if the piece is a king
func IsKingMove(piece int) bool {
	return (piece & 7) == KING
}
