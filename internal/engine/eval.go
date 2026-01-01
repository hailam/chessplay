// Package engine implements the chess AI search engine.
package engine

import (
	"github.com/hailam/chessplay/internal/board"
)

// Evaluation constants
const (
	PawnValue   = 100
	KnightValue = 320
	BishopValue = 330
	RookValue   = 500
	QueenValue  = 900
	KingValue   = 20000
)

// Piece values array for quick lookup
var pieceValues = [7]int{PawnValue, KnightValue, BishopValue, RookValue, QueenValue, KingValue, 0}

// Piece-Square Tables (PST) for positional evaluation
// Values are from White's perspective; mirrored for Black

// Pawn PST - encourages central control and advancement
var pawnPST = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	50, 50, 50, 50, 50, 50, 50, 50,
	10, 10, 20, 30, 30, 20, 10, 10,
	5, 5, 10, 25, 25, 10, 5, 5,
	0, 0, 0, 20, 20, 0, 0, 0,
	5, -5, -10, 0, 0, -10, -5, 5,
	5, 10, 10, -20, -20, 10, 10, 5,
	0, 0, 0, 0, 0, 0, 0, 0,
}

// Knight PST - encourages central positioning
var knightPST = [64]int{
	-50, -40, -30, -30, -30, -30, -40, -50,
	-40, -20, 0, 0, 0, 0, -20, -40,
	-30, 0, 10, 15, 15, 10, 0, -30,
	-30, 5, 15, 20, 20, 15, 5, -30,
	-30, 0, 15, 20, 20, 15, 0, -30,
	-30, 5, 10, 15, 15, 10, 5, -30,
	-40, -20, 0, 5, 5, 0, -20, -40,
	-50, -40, -30, -30, -30, -30, -40, -50,
}

// Bishop PST - encourages central diagonals
var bishopPST = [64]int{
	-20, -10, -10, -10, -10, -10, -10, -20,
	-10, 0, 0, 0, 0, 0, 0, -10,
	-10, 0, 5, 10, 10, 5, 0, -10,
	-10, 5, 5, 10, 10, 5, 5, -10,
	-10, 0, 10, 10, 10, 10, 0, -10,
	-10, 10, 10, 10, 10, 10, 10, -10,
	-10, 5, 0, 0, 0, 0, 5, -10,
	-20, -10, -10, -10, -10, -10, -10, -20,
}

// Rook PST - encourages 7th rank and open files
var rookPST = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	5, 10, 10, 10, 10, 10, 10, 5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	0, 0, 0, 5, 5, 0, 0, 0,
}

// Queen PST - slight central preference
var queenPST = [64]int{
	-20, -10, -10, -5, -5, -10, -10, -20,
	-10, 0, 0, 0, 0, 0, 0, -10,
	-10, 0, 5, 5, 5, 5, 0, -10,
	-5, 0, 5, 5, 5, 5, 0, -5,
	0, 0, 5, 5, 5, 5, 0, -5,
	-10, 5, 5, 5, 5, 5, 0, -10,
	-10, 0, 5, 0, 0, 0, 0, -10,
	-20, -10, -10, -5, -5, -10, -10, -20,
}

// King PST (middlegame) - encourages castling
var kingMidgamePST = [64]int{
	-30, -40, -40, -50, -50, -40, -40, -30,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-20, -30, -30, -40, -40, -30, -30, -20,
	-10, -20, -20, -20, -20, -20, -20, -10,
	20, 20, 0, 0, 0, 0, 20, 20,
	20, 30, 10, 0, 0, 10, 30, 20,
}

// King PST (endgame) - king should be active
var kingEndgamePST = [64]int{
	-50, -40, -30, -20, -20, -30, -40, -50,
	-30, -20, -10, 0, 0, -10, -20, -30,
	-30, -10, 20, 30, 30, 20, -10, -30,
	-30, -10, 30, 40, 40, 30, -10, -30,
	-30, -10, 30, 40, 40, 30, -10, -30,
	-30, -10, 20, 30, 30, 20, -10, -30,
	-30, -30, 0, 0, 0, 0, -30, -30,
	-50, -30, -30, -30, -30, -30, -30, -50,
}

// All PSTs combined for easy lookup
var psts = [...][64]int{
	pawnPST, knightPST, bishopPST, rookPST, queenPST, kingMidgamePST,
}

// Evaluate returns the static evaluation of the position from White's perspective.
func Evaluate(pos *board.Position) int {
	var mgScore, egScore int // Middlegame and endgame scores
	var phase int             // Game phase (for tapered eval)

	// Evaluate material and positional factors for both sides
	for c := board.White; c <= board.Black; c++ {
		sign := 1
		if c == board.Black {
			sign = -1
		}

		for pt := board.Pawn; pt <= board.King; pt++ {
			bb := pos.Pieces[c][pt]
			for bb != 0 {
				sq := bb.PopLSB()

				// Material
				mgScore += sign * pieceValues[pt]
				egScore += sign * pieceValues[pt]

				// Piece-square tables
				pstSq := sq
				if c == board.Black {
					pstSq = sq.Mirror() // Mirror for black
				}

				if pt == board.King {
					mgScore += sign * kingMidgamePST[pstSq]
					egScore += sign * kingEndgamePST[pstSq]
				} else {
					pstValue := psts[pt][pstSq]
					mgScore += sign * pstValue
					egScore += sign * pstValue
				}

				// Phase calculation (for tapered eval)
				switch pt {
				case board.Knight, board.Bishop:
					phase += 1
				case board.Rook:
					phase += 2
				case board.Queen:
					phase += 4
				}
			}
		}
	}

	// Tapered evaluation (interpolate between middlegame and endgame)
	// Maximum phase = 2*4 + 2*1 + 2*1 + 2*2 = 16 per side = 32 total
	const maxPhase = 24
	if phase > maxPhase {
		phase = maxPhase
	}

	score := (mgScore*phase + egScore*(maxPhase-phase)) / maxPhase

	// Return score from side to move's perspective
	if pos.SideToMove == board.Black {
		return -score
	}
	return score
}

// EvaluateMaterial returns just the material balance (for quick evaluation).
func EvaluateMaterial(pos *board.Position) int {
	score := 0
	for pt := board.Pawn; pt < board.King; pt++ {
		score += pos.Pieces[board.White][pt].PopCount() * pieceValues[pt]
		score -= pos.Pieces[board.Black][pt].PopCount() * pieceValues[pt]
	}
	if pos.SideToMove == board.Black {
		return -score
	}
	return score
}

// IsEndgame returns true if the position is in the endgame phase.
func IsEndgame(pos *board.Position) bool {
	// Simple heuristic: endgame if both sides have no queens
	// or total material (excluding kings) is low
	whiteQueens := pos.Pieces[board.White][board.Queen].PopCount()
	blackQueens := pos.Pieces[board.Black][board.Queen].PopCount()

	if whiteQueens == 0 && blackQueens == 0 {
		return true
	}

	// Count total non-pawn, non-king material
	whitePieces := pos.Pieces[board.White][board.Knight].PopCount() +
		pos.Pieces[board.White][board.Bishop].PopCount() +
		pos.Pieces[board.White][board.Rook].PopCount()
	blackPieces := pos.Pieces[board.Black][board.Knight].PopCount() +
		pos.Pieces[board.Black][board.Bishop].PopCount() +
		pos.Pieces[board.Black][board.Rook].PopCount()

	return whiteQueens+blackQueens <= 1 && whitePieces+blackPieces <= 4
}

// SEE (Static Exchange Evaluation) estimates the result of a capture sequence.
// Returns the estimated material gain/loss.
func SEE(pos *board.Position, m board.Move) int {
	// Simplified SEE - just return the captured piece value minus attacker value
	// A full SEE would simulate the entire capture sequence

	from := m.From()
	to := m.To()

	attacker := pos.PieceAt(from)
	if attacker == board.NoPiece {
		return 0
	}

	victim := pos.PieceAt(to)
	if m.IsEnPassant() {
		return PawnValue - pieceValues[attacker.Type()] + PawnValue // Pawn captures pawn
	}

	if victim == board.NoPiece {
		return 0 // Not a capture
	}

	return pieceValues[victim.Type()] - pieceValues[attacker.Type()] + pieceValues[attacker.Type()]
}
