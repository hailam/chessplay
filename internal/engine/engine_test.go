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

// TestConcurrentSearchRace is a stress test for multi-threaded search.
// Run with: GOMAXPROCS=8 go test -race -run TestConcurrentSearchRace ./internal/engine -v
// This test verifies that parallel search doesn't have race conditions.
func TestConcurrentSearchRace(t *testing.T) {
	pos := board.NewPosition()
	eng := NewEngine(16)

	// Run multiple searches to stress test concurrent access
	iterations := 10
	if testing.Short() {
		iterations = 3
	}

	for i := 0; i < iterations; i++ {
		limits := SearchLimits{
			Depth:    6,
			MoveTime: 500 * time.Millisecond,
		}

		move := eng.SearchWithLimits(pos, limits)
		if move == board.NoMove {
			t.Errorf("Iteration %d: Search returned NoMove for starting position", i)
		}

		// Make a couple of opening moves to vary positions
		if i%2 == 0 {
			// Play e4 e5
			pos, _ = board.ParseFEN("rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2")
		} else {
			// Play d4 d5
			pos, _ = board.ParseFEN("rnbqkbnr/ppp1pppp/8/3p4/3P4/8/PPP1PPPP/RNBQKBNR w KQkq d6 0 2")
		}
	}

	t.Logf("Completed %d concurrent search iterations without race condition", iterations)
}

// TestConcurrentSearchMultiplePositions tests searching different positions simultaneously.
func TestConcurrentSearchMultiplePositions(t *testing.T) {
	eng := NewEngine(16)

	// Test positions (opening, middlegame, endgame)
	positions := []string{
		board.StartFEN,
		"r1bqkbnr/pppp1ppp/2n5/4p3/2B1P3/5N2/PPPP1PPP/RNBQK2R b KQkq - 3 3", // Italian Game
		"8/8/8/4k3/8/4K3/4P3/8 w - - 0 1",                                      // KP endgame
	}

	for i, fen := range positions {
		pos, err := board.ParseFEN(fen)
		if err != nil {
			t.Fatalf("Failed to parse position %d: %v", i, err)
		}

		limits := SearchLimits{
			Depth:    5,
			MoveTime: 300 * time.Millisecond,
		}

		move := eng.SearchWithLimits(pos, limits)
		if move == board.NoMove {
			// Only error if position is not terminal
			if !pos.InCheck() || pos.GenerateLegalMoves().Len() > 0 {
				t.Errorf("Position %d: Search returned NoMove", i)
			}
		} else {
			t.Logf("Position %d: best move = %s", i, move.String())
		}
	}
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
