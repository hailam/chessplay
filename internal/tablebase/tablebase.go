package tablebase

import (
	"github.com/hailam/chessplay/internal/board"
)

// WDL represents Win/Draw/Loss result.
type WDL int

const (
	WDLLoss        WDL = -2
	WDLBlessedLoss WDL = -1 // Cursed win (win but 50-move rule may interfere)
	WDLDraw        WDL = 0
	WDLCursedWin   WDL = 1 // Blessed loss (loss but 50-move rule may save)
	WDLWin         WDL = 2
)

// ProbeResult contains the result of a tablebase probe.
type ProbeResult struct {
	Found bool
	WDL   WDL
	DTZ   int // Distance to zeroing move (pawn move or capture)
}

// RootResult contains the best move from tablebase at root position.
type RootResult struct {
	Found bool
	Move  board.Move
	WDL   WDL
	DTZ   int
}

// Prober is the interface for tablebase probing.
type Prober interface {
	// Probe looks up a position in the tablebase.
	// Returns win/draw/loss information if the position is in the tablebase.
	Probe(pos *board.Position) ProbeResult

	// ProbeRoot finds the best move from the tablebase at the root position.
	// This is more expensive as it needs to evaluate all legal moves.
	ProbeRoot(pos *board.Position) RootResult

	// MaxPieces returns the maximum number of pieces supported.
	MaxPieces() int

	// Available returns true if tablebases are loaded and available.
	Available() bool
}

// WDLToScore converts a WDL result to a search score.
// Uses the convention: positive = winning, negative = losing.
func WDLToScore(wdl WDL, ply int) int {
	const mateScore = 30000

	switch wdl {
	case WDLWin:
		return mateScore - ply // Win gets high score, closer ply = higher
	case WDLCursedWin:
		return mateScore - 100 - ply // Cursed win is slightly worse
	case WDLDraw:
		return 0
	case WDLBlessedLoss:
		return -mateScore + 100 + ply // Blessed loss is slightly better than loss
	case WDLLoss:
		return -mateScore + ply // Loss gets negative score
	default:
		return 0
	}
}

// NoopProber is a prober that always returns "not found".
// Use this as a placeholder when tablebases are not available.
type NoopProber struct{}

func (NoopProber) Probe(pos *board.Position) ProbeResult {
	return ProbeResult{Found: false}
}

func (NoopProber) ProbeRoot(pos *board.Position) RootResult {
	return RootResult{Found: false}
}

func (NoopProber) MaxPieces() int {
	return 0
}

func (NoopProber) Available() bool {
	return false
}

// CountPieces returns the total number of pieces on the board.
func CountPieces(pos *board.Position) int {
	return pos.AllOccupied.PopCount()
}
