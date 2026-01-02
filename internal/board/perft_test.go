package board

import "testing"

// Perft counts the number of leaf nodes at the given depth.
// This is the standard way to verify move generation correctness.
func perft(p *Position, depth int) int64 {
	if depth == 0 {
		return 1
	}

	moves := p.GenerateLegalMoves()
	if depth == 1 {
		return int64(moves.Len())
	}

	var nodes int64
	for i := 0; i < moves.Len(); i++ {
		m := moves.Get(i)
		undo := p.MakeMove(m)
		nodes += perft(p, depth-1)
		p.UnmakeMove(m, undo)
	}
	return nodes
}

// TestPerftStartingPosition tests move generation from the starting position.
func TestPerftStartingPosition(t *testing.T) {
	pos := NewPosition()

	tests := []struct {
		depth    int
		expected int64
	}{
		{1, 20},
		{2, 400},
		{3, 8902},
		{4, 197281},
		// Depth 5 takes longer, enable for thorough testing:
		// {5, 4865609},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			got := perft(pos, tc.depth)
			if got != tc.expected {
				t.Errorf("perft(%d) = %d, want %d", tc.depth, got, tc.expected)
			}
		})
	}
}

// TestPerftKiwipete tests the famous Kiwipete position with many edge cases.
// FEN: r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq -
func TestPerftKiwipete(t *testing.T) {
	pos, err := ParseFEN("r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq -")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	tests := []struct {
		depth    int
		expected int64
	}{
		{1, 48},
		{2, 2039},
		{3, 97862},
		// {4, 4085603}, // Takes ~1s, enable for thorough testing
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			got := perft(pos, tc.depth)
			if got != tc.expected {
				t.Errorf("perft(%d) = %d, want %d", tc.depth, got, tc.expected)
			}
		})
	}
}

// TestPerftPosition3 tests en passant edge cases.
// FEN: 8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - -
func TestPerftPosition3(t *testing.T) {
	pos, err := ParseFEN("8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - -")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	tests := []struct {
		depth    int
		expected int64
	}{
		{1, 14},
		{2, 191},
		{3, 2812},
		{4, 43238},
		// {5, 674624}, // Enable for thorough testing
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			got := perft(pos, tc.depth)
			if got != tc.expected {
				t.Errorf("perft(%d) = %d, want %d", tc.depth, got, tc.expected)
			}
		})
	}
}

// TestPerftEnPassantPin tests the specific en passant horizontal pin edge case.
// FEN: 8/8/8/8/k2Pp2R/8/8/4K3 b - d3 0 1
// Black pawn on e4 can capture en passant d3, but this would expose the black king
// on a4 to the white rook on h4.
func TestPerftEnPassantPin(t *testing.T) {
	pos, err := ParseFEN("8/8/8/8/k2Pp2R/8/8/4K3 b - d3 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// The en passant capture should be illegal
	moves := pos.GenerateLegalMoves()
	for i := 0; i < moves.Len(); i++ {
		m := moves.Get(i)
		if m.IsEnPassant() {
			t.Errorf("En passant move %v should be illegal (horizontal pin)", m)
		}
	}

	// Verify perft
	// Depth 1: Ka3, Ka5, Kb3, Kb4, Kb5, e3 = 6 moves
	// Depth 2: After e4e3 (14), after king moves (16 each x5) = 14 + 80 = 94
	tests := []struct {
		depth    int
		expected int64
	}{
		{1, 6},
		{2, 94},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			got := perft(pos, tc.depth)
			if got != tc.expected {
				t.Errorf("perft(%d) = %d, want %d", tc.depth, got, tc.expected)
			}
		})
	}
}
