package engine

import (
	"sync/atomic"

	"github.com/hailam/chessplay/internal/board"
)

// TTFlag indicates the type of bound stored in the transposition table.
type TTFlag uint8

const (
	TTExact      TTFlag = iota // Exact score
	TTLowerBound               // Failed high (beta cutoff)
	TTUpperBound               // Failed low
)

// TTEntry represents an entry in the transposition table.
// This is the logical view - internally stored as packed atomic values.
type TTEntry struct {
	Key      uint64     // Full 64-bit Zobrist hash for verification
	BestMove board.Move // Best move found
	Score    int16      // Score (bounded by flag)
	Depth    int8       // Search depth
	Flag     TTFlag     // Type of bound
	Age      uint8      // Generation for replacement
	IsPV     bool       // True if this entry was on the principal variation
}

// TTEntryPacked is the lock-free atomic storage format.
// Uses XOR verification to detect torn reads/writes.
type TTEntryPacked struct {
	// keyData stores: key XOR moveData (for verification)
	keyData atomic.Uint64

	// moveData packs: move(16) | score(16) | depth(8) | flag(4) | isPV(4) | age(8) | reserved(8)
	moveData atomic.Uint64
}

// packMoveData packs entry fields into a single uint64.
func packMoveData(move board.Move, score int16, depth int8, flag TTFlag, isPV bool, age uint8) uint64 {
	var data uint64
	data |= uint64(move) & 0xFFFF                   // bits 0-15: move
	data |= (uint64(uint16(score)) & 0xFFFF) << 16  // bits 16-31: score
	data |= (uint64(uint8(depth)) & 0xFF) << 32     // bits 32-39: depth
	data |= (uint64(flag) & 0xF) << 40              // bits 40-43: flag
	isPVBit := uint64(0)
	if isPV {
		isPVBit = 1
	}
	data |= isPVBit << 44                           // bit 44: isPV
	data |= (uint64(age) & 0xFF) << 48              // bits 48-55: age
	return data
}

// unpackMoveData unpacks a uint64 into entry fields.
func unpackMoveData(data uint64) (move board.Move, score int16, depth int8, flag TTFlag, isPV bool, age uint8) {
	move = board.Move(data & 0xFFFF)
	score = int16((data >> 16) & 0xFFFF)
	depth = int8((data >> 32) & 0xFF)
	flag = TTFlag((data >> 40) & 0xF)
	isPV = ((data >> 44) & 0x1) != 0
	age = uint8((data >> 48) & 0xFF)
	return
}

// TranspositionTable is a lock-free hash table for storing search results.
// Uses atomic operations with XOR verification for thread-safety.
type TranspositionTable struct {
	entries []TTEntryPacked
	size    uint64
	mask    uint64
	age     atomic.Uint32

	// Statistics (atomic for thread-safety)
	hits   atomic.Uint64
	probes atomic.Uint64
}

// NewTranspositionTable creates a transposition table with the given size in MB.
func NewTranspositionTable(sizeMB int) *TranspositionTable {
	// Calculate number of entries
	entrySize := uint64(16) // Two uint64 values
	numEntries := (uint64(sizeMB) * 1024 * 1024) / entrySize

	// Round down to power of 2 for fast modulo
	numEntries = roundDownToPowerOf2(numEntries)

	return &TranspositionTable{
		entries: make([]TTEntryPacked, numEntries),
		size:    numEntries,
		mask:    numEntries - 1,
	}
}

// roundDownToPowerOf2 rounds n down to the nearest power of 2.
func roundDownToPowerOf2(n uint64) uint64 {
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	return (n + 1) >> 1
}

// Probe looks up a position in the transposition table.
// Returns the entry and true if found, otherwise returns empty entry and false.
// Lock-free: uses atomic loads with XOR verification.
func (tt *TranspositionTable) Probe(hash uint64) (TTEntry, bool) {
	tt.probes.Add(1)

	idx := hash & tt.mask
	entry := &tt.entries[idx]

	// Atomic load of both values
	keyData := entry.keyData.Load()
	moveData := entry.moveData.Load()

	// XOR verification: keyData should equal hash XOR moveData
	// This detects torn reads where only one value was updated
	if keyData != (hash ^ moveData) {
		return TTEntry{}, false
	}

	// Unpack the data
	move, score, depth, flag, isPV, age := unpackMoveData(moveData)

	// Verify we have valid data
	if depth <= 0 {
		return TTEntry{}, false
	}

	tt.hits.Add(1)
	return TTEntry{
		Key:      hash,
		BestMove: move,
		Score:    score,
		Depth:    depth,
		Flag:     flag,
		Age:      age,
		IsPV:     isPV,
	}, true
}

// Store saves a position in the transposition table.
// isPV indicates if this position was on the principal variation.
// Lock-free: uses atomic stores with XOR encoding.
func (tt *TranspositionTable) Store(hash uint64, depth int, score int, flag TTFlag, bestMove board.Move, isPV bool) {
	idx := hash & tt.mask
	entry := &tt.entries[idx]
	currentAge := uint8(tt.age.Load())

	// Read existing entry for replacement decision
	existingKeyData := entry.keyData.Load()
	existingMoveData := entry.moveData.Load()

	// Recover the stored hash using XOR: storedHash = keyData XOR moveData
	existingKey := existingKeyData ^ existingMoveData

	// Unpack existing entry data
	_, _, existingDepth, existingFlag, existingIsPV, existingAge := unpackMoveData(existingMoveData)

	// Check if existing entry is valid (has non-zero depth)
	existingValid := existingDepth > 0

	// Calculate existing entry quality
	var existingQuality int
	if existingValid {
		existingQuality = int(existingDepth) * 4
		if existingAge == currentAge {
			existingQuality += 256 // Current age bonus
		}
		if existingFlag == TTExact {
			existingQuality += 2 // Exact score bonus
		}
		if existingIsPV {
			existingQuality += 4 // PV bonus
		}
	}

	// Calculate new entry quality
	newQuality := depth * 4
	newQuality += 256 // New entry is always current age
	if flag == TTExact {
		newQuality += 2
	}
	if isPV {
		newQuality += 4
	}

	// Replace if:
	// 1. Same position (update with new info), OR
	// 2. New entry has higher or equal quality, OR
	// 3. Existing entry is invalid/empty
	if existingKey == hash || newQuality >= existingQuality || !existingValid {
		// Pack and store atomically
		newMoveData := packMoveData(bestMove, int16(score), int8(depth), flag, isPV, currentAge)
		newKeyData := hash ^ newMoveData

		// Store in order: moveData first, then keyData
		// This ensures that a reader seeing the new keyData will also see valid moveData
		entry.moveData.Store(newMoveData)
		entry.keyData.Store(newKeyData)
	}
}

// NewSearch increments the age counter for a new search.
// This helps with replacement decisions.
func (tt *TranspositionTable) NewSearch() {
	tt.age.Add(1)
}

// Clear clears the transposition table.
func (tt *TranspositionTable) Clear() {
	for i := range tt.entries {
		tt.entries[i].keyData.Store(0)
		tt.entries[i].moveData.Store(0)
	}
	tt.age.Store(0)
	tt.hits.Store(0)
	tt.probes.Store(0)
}

// HashFull returns the permille (parts per thousand) of the table that is used.
func (tt *TranspositionTable) HashFull() int {
	// Sample first 1000 entries
	used := 0
	sampleSize := 1000
	if uint64(sampleSize) > tt.size {
		sampleSize = int(tt.size)
	}

	currentAge := uint8(tt.age.Load())
	for i := 0; i < sampleSize; i++ {
		moveData := tt.entries[i].moveData.Load()
		_, _, depth, _, _, age := unpackMoveData(moveData)
		if depth > 0 && age == currentAge {
			used++
		}
	}

	return (used * 1000) / sampleSize
}

// HitRate returns the cache hit rate as a percentage.
func (tt *TranspositionTable) HitRate() float64 {
	probes := tt.probes.Load()
	if probes == 0 {
		return 0
	}
	return float64(tt.hits.Load()) / float64(probes) * 100
}

// Size returns the number of entries in the table.
func (tt *TranspositionTable) Size() uint64 {
	return tt.size
}

// AdjustScore adjusts a score from/to the transposition table.
// Mate scores need to be adjusted based on ply distance.
func AdjustScoreFromTT(score int, ply int) int {
	if score > MateScore-MaxPly {
		return score - ply
	}
	if score < -MateScore+MaxPly {
		return score + ply
	}
	return score
}

// AdjustScoreToTT adjusts a score for storage in the transposition table.
func AdjustScoreToTT(score int, ply int) int {
	if score > MateScore-MaxPly {
		return score + ply
	}
	if score < -MateScore+MaxPly {
		return score - ply
	}
	return score
}
