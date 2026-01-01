package board

// Zobrist hash keys for position hashing.
// Uses PRNG with fixed seed for reproducibility.
var (
	zobristPiece      [2][7][64]uint64 // [Color][PieceType][Square] - 7 to handle NoPieceType safely
	zobristEnPassant  [8]uint64        // One per file
	zobristCastling   [16]uint64       // All 16 castling combinations
	zobristSideToMove uint64           // XOR when black to move
)

func init() {
	initZobrist()
}

// Simple PRNG for reproducible Zobrist keys
type prng struct {
	state uint64
}

func newPRNG(seed uint64) *prng {
	return &prng{state: seed}
}

// xorshift64* algorithm
func (p *prng) next() uint64 {
	p.state ^= p.state >> 12
	p.state ^= p.state << 25
	p.state ^= p.state >> 27
	return p.state * 0x2545F4914F6CDD1D
}

func initZobrist() {
	rng := newPRNG(0x98F107A2BEEF1234) // Fixed seed

	// Piece keys
	for c := White; c <= Black; c++ {
		for pt := Pawn; pt <= King; pt++ {
			for sq := A1; sq <= H8; sq++ {
				zobristPiece[c][pt][sq] = rng.next()
			}
		}
	}

	// En passant keys (one per file)
	for file := 0; file < 8; file++ {
		zobristEnPassant[file] = rng.next()
	}

	// Castling keys (all 16 combinations)
	for i := 0; i < 16; i++ {
		zobristCastling[i] = rng.next()
	}

	// Side to move key
	zobristSideToMove = rng.next()
}

// ZobristPiece returns the Zobrist key for a piece on a square.
func ZobristPiece(c Color, pt PieceType, sq Square) uint64 {
	return zobristPiece[c][pt][sq]
}

// ZobristEnPassant returns the Zobrist key for an en passant file.
func ZobristEnPassant(file int) uint64 {
	return zobristEnPassant[file]
}

// ZobristCastling returns the Zobrist key for castling rights.
func ZobristCastling(cr CastlingRights) uint64 {
	return zobristCastling[cr]
}

// ZobristSideToMove returns the Zobrist key for side to move.
func ZobristSideToMove() uint64 {
	return zobristSideToMove
}
