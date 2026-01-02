package book

import (
	"encoding/binary"
	"io"
	"math/rand"
	"os"
	"sort"

	"github.com/hailam/chessplay/internal/board"
)

// BookEntry represents a single book entry.
type BookEntry struct {
	Move   board.Move
	Weight uint16
}

// Book represents an opening book.
type Book struct {
	entries map[uint64][]BookEntry
}

// New creates an empty book.
func New() *Book {
	return &Book{
		entries: make(map[uint64][]BookEntry),
	}
}

// LoadPolyglot loads a Polyglot format opening book from a file.
func LoadPolyglot(filename string) (*Book, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return LoadPolyglotReader(file)
}

// LoadPolyglotReader loads a Polyglot format book from a reader.
func LoadPolyglotReader(r io.Reader) (*Book, error) {
	book := New()

	// Polyglot entry format:
	// 8 bytes: position key (big-endian)
	// 2 bytes: move (big-endian)
	// 2 bytes: weight (big-endian)
	// 4 bytes: learn data (ignored)
	var entry [16]byte

	for {
		_, err := io.ReadFull(r, entry[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		key := binary.BigEndian.Uint64(entry[0:8])
		moveData := binary.BigEndian.Uint16(entry[8:10])
		weight := binary.BigEndian.Uint16(entry[10:12])

		move := decodePolyglotMove(moveData)
		if move != board.NoMove {
			book.entries[key] = append(book.entries[key], BookEntry{
				Move:   move,
				Weight: weight,
			})
		}
	}

	return book, nil
}

// decodePolyglotMove converts a Polyglot move encoding to our Move type.
// Polyglot move format (bits):
// 0-5: to square
// 6-11: from square
// 12-14: promotion piece (0=none, 1=knight, 2=bishop, 3=rook, 4=queen)
func decodePolyglotMove(data uint16) board.Move {
	toFile := data & 7
	toRank := (data >> 3) & 7
	fromFile := (data >> 6) & 7
	fromRank := (data >> 9) & 7
	promo := (data >> 12) & 7

	from := board.NewSquare(int(fromFile), int(fromRank))
	to := board.NewSquare(int(toFile), int(toRank))

	// Handle castling: Polyglot uses king-captures-rook encoding
	// We need to convert to our e1-g1/e1-c1 encoding
	if from == board.E1 && to == board.H1 {
		to = board.G1 // White kingside
	} else if from == board.E1 && to == board.A1 {
		to = board.C1 // White queenside
	} else if from == board.E8 && to == board.H8 {
		to = board.G8 // Black kingside
	} else if from == board.E8 && to == board.A8 {
		to = board.C8 // Black queenside
	}

	if promo > 0 {
		// Promotion pieces: 1=knight, 2=bishop, 3=rook, 4=queen
		promoTypes := []board.PieceType{0, board.Knight, board.Bishop, board.Rook, board.Queen}
		return board.NewPromotion(from, to, promoTypes[promo])
	}

	return board.NewMove(from, to)
}

// Probe looks up a position in the book and returns a move using weighted random selection.
func (b *Book) Probe(pos *board.Position) (board.Move, bool) {
	if b == nil {
		return board.NoMove, false
	}

	key := pos.PolyglotHash()
	entries, ok := b.entries[key]
	if !ok || len(entries) == 0 {
		return board.NoMove, false
	}

	// Sort by weight (highest first) for deterministic ordering
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Weight > entries[j].Weight
	})

	// Weighted random selection
	totalWeight := uint32(0)
	for _, e := range entries {
		totalWeight += uint32(e.Weight)
	}

	if totalWeight == 0 {
		// All weights are 0, just pick the first
		return verifyAndConvert(pos, entries[0].Move), true
	}

	r := rand.Uint32() % totalWeight
	cumulative := uint32(0)
	for _, e := range entries {
		cumulative += uint32(e.Weight)
		if r < cumulative {
			return verifyAndConvert(pos, e.Move), true
		}
	}

	// Fallback to first entry
	return verifyAndConvert(pos, entries[0].Move), true
}

// ProbeAll returns all book moves for the position, sorted by weight.
func (b *Book) ProbeAll(pos *board.Position) []BookEntry {
	if b == nil {
		return nil
	}

	key := pos.PolyglotHash()
	entries, ok := b.entries[key]
	if !ok {
		return nil
	}

	// Sort by weight (highest first)
	result := make([]BookEntry, len(entries))
	copy(result, entries)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Weight > result[j].Weight
	})

	return result
}

// verifyAndConvert ensures the move is legal and adjusts flags if needed.
func verifyAndConvert(pos *board.Position, move board.Move) board.Move {
	// Find the matching legal move to get correct flags (castling, en passant, etc.)
	legalMoves := pos.GenerateLegalMoves()
	from := move.From()
	to := move.To()

	for i := 0; i < legalMoves.Len(); i++ {
		lm := legalMoves.Get(i)
		if lm.From() == from && lm.To() == to {
			// For promotions, match the promotion piece
			if move.IsPromotion() && lm.IsPromotion() {
				if move.Promotion() == lm.Promotion() {
					return lm
				}
			} else if !move.IsPromotion() && !lm.IsPromotion() {
				return lm
			}
		}
	}

	return board.NoMove
}

// Size returns the number of unique positions in the book.
func (b *Book) Size() int {
	if b == nil {
		return 0
	}
	return len(b.entries)
}
