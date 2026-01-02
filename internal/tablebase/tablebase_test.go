package tablebase

import (
	"testing"

	"github.com/hailam/chessplay/internal/board"
)

func TestNoopProber(t *testing.T) {
	prober := NoopProber{}

	if prober.Available() {
		t.Error("NoopProber should not be available")
	}

	if prober.MaxPieces() != 0 {
		t.Errorf("NoopProber MaxPieces should be 0, got %d", prober.MaxPieces())
	}

	pos := board.NewPosition()
	result := prober.Probe(pos)
	if result.Found {
		t.Error("NoopProber should not find anything")
	}

	rootResult := prober.ProbeRoot(pos)
	if rootResult.Found {
		t.Error("NoopProber ProbeRoot should not find anything")
	}
}

func TestCountPieces(t *testing.T) {
	pos := board.NewPosition()
	count := CountPieces(pos)

	// Starting position has 32 pieces
	if count != 32 {
		t.Errorf("Starting position should have 32 pieces, got %d", count)
	}
}

func TestWDLToScore(t *testing.T) {
	tests := []struct {
		wdl      WDL
		ply      int
		positive bool // Should score be positive (winning)?
	}{
		{WDLWin, 0, true},
		{WDLWin, 10, true},
		{WDLCursedWin, 0, true},
		{WDLDraw, 0, false},
		{WDLBlessedLoss, 0, false},
		{WDLLoss, 0, false},
	}

	for _, tc := range tests {
		score := WDLToScore(tc.wdl, tc.ply)
		isPositive := score > 0

		if tc.positive && !isPositive {
			t.Errorf("WDL %d at ply %d should give positive score, got %d", tc.wdl, tc.ply, score)
		}
		if !tc.positive && tc.wdl != WDLDraw && isPositive {
			t.Errorf("WDL %d at ply %d should give non-positive score, got %d", tc.wdl, tc.ply, score)
		}
	}
}
