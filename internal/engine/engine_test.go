package engine

import (
	"testing"
	"time"

	"github.com/hailam/chessplay/internal/board"
)

func TestMultiPV(t *testing.T) {
	pos := board.NewPosition()
	eng := NewEngine(16)

	limits := SearchLimits{
		Depth:    4,
		MoveTime: 2 * time.Second,
		MultiPV:  3,
	}

	results := eng.SearchMultiPV(pos, limits)

	if len(results) < 2 {
		t.Fatalf("Expected at least 2 PVs, got %d", len(results))
	}

	// Verify different moves are returned
	if results[0].Move == results[1].Move {
		t.Errorf("First two PVs have same move: %s", results[0].Move.String())
	}

	// Verify scores are in descending order (best first)
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("PV %d has higher score than PV %d (%d > %d)",
				i+1, i, results[i].Score, results[i-1].Score)
		}
	}

	t.Logf("Multi-PV results:")
	for i, r := range results {
		t.Logf("  PV %d: %s (score: %d, depth: %d)", i+1, r.Move.String(), r.Score, r.Depth)
	}
}

func TestSearchBasic(t *testing.T) {
	pos := board.NewPosition()
	eng := NewEngine(16)
	eng.SetDifficulty(Easy)

	move := eng.Search(pos)
	if move == board.NoMove {
		t.Error("Search returned NoMove for starting position")
	}
	t.Logf("Best move: %s", move.String())
}

func TestPawnHashTable(t *testing.T) {
	pt := NewPawnTable(1) // 1MB

	pos := board.NewPosition()

	// First probe should miss
	_, _, found := pt.Probe(pos.PawnKey)
	if found {
		t.Error("Expected cache miss on first probe")
	}

	// Store and retrieve
	pt.Store(pos.PawnKey, -15, -20)

	mg, eg, found := pt.Probe(pos.PawnKey)
	if !found {
		t.Error("Expected cache hit after store")
	}
	if mg != -15 || eg != -20 {
		t.Errorf("Wrong values: got mg=%d, eg=%d, want -15, -20", mg, eg)
	}

	// Verify PawnKey changes when pawns move
	oldKey := pos.PawnKey
	move := board.NewMove(board.E2, board.E4)
	undo := pos.MakeMove(move)
	if pos.PawnKey == oldKey {
		t.Error("PawnKey should change when pawn moves")
	}

	// Verify PawnKey is restored on unmake
	pos.UnmakeMove(move, undo)
	if pos.PawnKey != oldKey {
		t.Error("PawnKey should be restored on unmake")
	}

	t.Logf("PawnKey: %016x", pos.PawnKey)
}
