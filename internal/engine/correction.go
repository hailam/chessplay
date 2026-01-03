package engine

import (
	"github.com/hailam/chessplay/internal/board"
)

// CorrectionHistory adjusts static evaluation based on search results.
// When the search discovers the static eval was wrong, we record the error
// and apply corrections to similar positions in the future.
// Based on Stockfish's correction history.
type CorrectionHistory struct {
	// Position-based correction indexed by hash
	// Uses 16-bit entries to save memory
	positionCorr [65536]int16
}

// NewCorrectionHistory creates a new correction history table.
func NewCorrectionHistory() *CorrectionHistory {
	return &CorrectionHistory{}
}

// Get returns the correction value for a position.
// The correction should be added to the static evaluation.
func (ch *CorrectionHistory) Get(pos *board.Position) int {
	idx := pos.Hash & 0xFFFF
	return int(ch.positionCorr[idx])
}

// Update records a correction based on the difference between
// the static evaluation and the search result.
// Uses gravity update: new = old + (target - old) / 16
func (ch *CorrectionHistory) Update(pos *board.Position, searchScore, staticEval, depth int) {
	// Only update if we have meaningful data
	if depth < 1 {
		return
	}

	// Calculate the error
	diff := searchScore - staticEval

	// Scale bonus by depth (deeper searches are more reliable)
	bonus := diff * depth / 8

	// Clamp the bonus to prevent extreme updates
	if bonus > 256 {
		bonus = 256
	} else if bonus < -256 {
		bonus = -256
	}

	idx := pos.Hash & 0xFFFF
	old := int(ch.positionCorr[idx])

	// Gravity update: gradually move toward the target
	newVal := old + (bonus-old)/16

	// Clamp to int16 range but with reasonable limits
	if newVal > 16000 {
		newVal = 16000
	} else if newVal < -16000 {
		newVal = -16000
	}

	ch.positionCorr[idx] = int16(newVal)
}

// Clear resets all correction values.
func (ch *CorrectionHistory) Clear() {
	for i := range ch.positionCorr {
		ch.positionCorr[i] = 0
	}
}

// Age scales down all correction values (called between games/positions).
func (ch *CorrectionHistory) Age() {
	for i := range ch.positionCorr {
		ch.positionCorr[i] /= 2
	}
}
