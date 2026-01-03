package engine

import (
	"github.com/hailam/chessplay/internal/board"
)

// CorrectionHistorySize is the number of entries (256k = 4x reduction in collisions)
const CorrectionHistorySize = 262144 // 2^18
const CorrectionHistoryMask = CorrectionHistorySize - 1

// CorrectionHistory adjusts static evaluation based on search results.
// When the search discovers the static eval was wrong, we record the error
// and apply corrections to similar positions in the future.
// Based on Stockfish's correction history.
type CorrectionHistory struct {
	// Position-based correction indexed by hash
	// Uses 16-bit entries to save memory (512KB total)
	positionCorr [CorrectionHistorySize]int16
}

// NewCorrectionHistory creates a new correction history table.
func NewCorrectionHistory() *CorrectionHistory {
	return &CorrectionHistory{}
}

// hashIndex computes a better distributed hash index.
// XORs high bits with low bits for better distribution.
func (ch *CorrectionHistory) hashIndex(hash uint64) int {
	// Mix high and low bits for better distribution
	return int((hash ^ (hash >> 18)) & CorrectionHistoryMask)
}

// Get returns the correction value for a position.
// The correction should be added to the static evaluation.
func (ch *CorrectionHistory) Get(pos *board.Position) int {
	idx := ch.hashIndex(pos.Hash)
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

	idx := ch.hashIndex(pos.Hash)
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
