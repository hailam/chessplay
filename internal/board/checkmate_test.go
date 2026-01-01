package board

import (
	"testing"
)

func TestCheckmate(t *testing.T) {
	// Test position: Back rank mate - already checkmate
	// White: Ka1, Ra8
	// Black: Kh8, pawns on g7 and h7 blocking escape
	// Black is already in checkmate (Black to move)
	pos, err := ParseFEN("R6k/6pp/8/8/8/8/8/K7 b - - 0 1")
	if err != nil {
		t.Fatal("Error parsing FEN:", err)
	}

	t.Log("Checkmate position:")
	t.Log(pos)

	pos.UpdateCheckers()

	t.Log("Checkers bitboard:", pos.Checkers)
	t.Log("InCheck:", pos.InCheck())

	// List all legal moves for black
	blackMoves := pos.GenerateLegalMoves()
	t.Log("Black legal moves:", blackMoves.Len())
	for i := 0; i < blackMoves.Len(); i++ {
		t.Log("  Move:", blackMoves.Get(i))
	}

	t.Log("HasLegalMoves:", pos.HasLegalMoves())
	t.Log("IsCheckmate:", pos.IsCheckmate())
	t.Log("IsStalemate:", pos.IsStalemate())

	if !pos.IsCheckmate() {
		t.Error("Expected checkmate but got false")
	}
}

func TestNotCheckmate(t *testing.T) {
	// Test position: King CAN escape - not checkmate
	// Black king on h8, rook on g8 but king can take it
	pos, err := ParseFEN("6Rk/8/8/8/8/8/8/K7 b - - 0 1")
	if err != nil {
		t.Fatal("Error parsing FEN:", err)
	}

	t.Log("Not checkmate position (king can capture rook):")
	t.Log(pos)

	pos.UpdateCheckers()

	t.Log("Checkers bitboard:", pos.Checkers)
	t.Log("InCheck:", pos.InCheck())

	blackMoves := pos.GenerateLegalMoves()
	t.Log("Black legal moves:", blackMoves.Len())
	for i := 0; i < blackMoves.Len(); i++ {
		t.Log("  Move:", blackMoves.Get(i))
	}

	t.Log("IsCheckmate:", pos.IsCheckmate())

	if pos.IsCheckmate() {
		t.Error("Expected NOT checkmate but got true")
	}
}
