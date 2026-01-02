package book

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/hailam/chessplay/internal/board"
)

func TestPolyglotHash(t *testing.T) {
	// Test that PolyglotHash returns consistent values
	pos := board.NewPosition()
	hash1 := pos.PolyglotHash()
	hash2 := pos.PolyglotHash()

	if hash1 != hash2 {
		t.Errorf("PolyglotHash not consistent: %x != %x", hash1, hash2)
	}

	// Make a move and check hash changes
	undo := pos.MakeMove(board.NewMove(board.E2, board.E4))
	hash3 := pos.PolyglotHash()

	if hash1 == hash3 {
		t.Error("PolyglotHash should change after move")
	}

	// Unmake and check hash is restored
	pos.UnmakeMove(board.NewMove(board.E2, board.E4), undo)
	hash4 := pos.PolyglotHash()

	if hash1 != hash4 {
		t.Errorf("PolyglotHash not restored after unmake: %x != %x", hash1, hash4)
	}

	t.Logf("Starting position PolyglotHash: %016x", hash1)
}

func TestBookLoadAndProbe(t *testing.T) {
	// Create a simple test book in memory
	// Entry format: 8 bytes key + 2 bytes move + 2 bytes weight + 4 bytes learn
	pos := board.NewPosition()
	key := pos.PolyglotHash()

	// Encode e2e4 in Polyglot format:
	// from = e2 = (4, 1) = 4 + 1*8 = 12 -> file=4, rank=1
	// to = e4 = (4, 3) = 4 + 3*8 = 28 -> file=4, rank=3
	// move = to_file | (to_rank << 3) | (from_file << 6) | (from_rank << 9)
	// e2e4 = 4 | (3 << 3) | (4 << 6) | (1 << 9) = 4 | 24 | 256 | 512 = 796
	e2e4Encoded := uint16(4 | (3 << 3) | (4 << 6) | (1 << 9))

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, key)
	binary.Write(&buf, binary.BigEndian, e2e4Encoded)
	binary.Write(&buf, binary.BigEndian, uint16(100)) // weight
	binary.Write(&buf, binary.BigEndian, uint32(0))   // learn

	book, err := LoadPolyglotReader(&buf)
	if err != nil {
		t.Fatalf("Failed to load book: %v", err)
	}

	if book.Size() != 1 {
		t.Errorf("Expected book size 1, got %d", book.Size())
	}

	// Probe the book
	move, found := book.Probe(pos)
	if !found {
		t.Fatal("Expected to find move in book")
	}

	if move.From() != board.E2 || move.To() != board.E4 {
		t.Errorf("Expected e2e4, got %s", move.String())
	}

	t.Logf("Book move: %s", move.String())
}

func TestBookMiss(t *testing.T) {
	book := New()
	pos := board.NewPosition()

	move, found := book.Probe(pos)
	if found {
		t.Error("Expected book miss on empty book")
	}
	if move != board.NoMove {
		t.Errorf("Expected NoMove on miss, got %s", move.String())
	}
}

func TestDecodePolyglotMove(t *testing.T) {
	// Test e2e4 decoding
	// e2 = file 4, rank 1; e4 = file 4, rank 3
	e2e4 := uint16(4 | (3 << 3) | (4 << 6) | (1 << 9))
	move := decodePolyglotMove(e2e4)

	if move.From() != board.E2 {
		t.Errorf("Expected from=e2, got %s", move.From().String())
	}
	if move.To() != board.E4 {
		t.Errorf("Expected to=e4, got %s", move.To().String())
	}

	// Test d7d5 decoding
	// d7 = file 3, rank 6; d5 = file 3, rank 4
	d7d5 := uint16(3 | (4 << 3) | (3 << 6) | (6 << 9))
	move = decodePolyglotMove(d7d5)

	if move.From() != board.D7 {
		t.Errorf("Expected from=d7, got %s", move.From().String())
	}
	if move.To() != board.D5 {
		t.Errorf("Expected to=d5, got %s", move.To().String())
	}
}
