package engine

import (
	"sync"
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

// Number of shards for TT locking (power of 2 for fast modulo)
const ttShardCount = 256
const ttShardMask = ttShardCount - 1

// TTEntry represents an entry in the transposition table.
type TTEntry struct {
	Key      uint64     // Full 64-bit Zobrist hash for verification (eliminates collisions)
	BestMove board.Move // Best move found
	Score    int16      // Score (bounded by flag)
	Depth    int8       // Search depth
	Flag     TTFlag     // Type of bound
	Age      uint8      // Generation for replacement
}

// TranspositionTable is a hash table for storing search results.
// Uses sharded locking for thread-safety in Lazy SMP.
type TranspositionTable struct {
	entries []TTEntry
	shards  [ttShardCount]sync.RWMutex // Sharded locks
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
	entrySize := uint64(16) // Size of TTEntry with 64-bit key
	numEntries := (uint64(sizeMB) * 1024 * 1024) / entrySize

	// Round down to power of 2 for fast modulo
	numEntries = roundDownToPowerOf2(numEntries)

	return &TranspositionTable{
		entries: make([]TTEntry, numEntries),
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

// shardIndex returns the shard index for a given entry index.
func (tt *TranspositionTable) shardIndex(idx uint64) int {
	return int(idx & ttShardMask)
}

// Probe looks up a position in the transposition table.
// Returns the entry and true if found, otherwise returns empty entry and false.
func (tt *TranspositionTable) Probe(hash uint64) (TTEntry, bool) {
	tt.probes.Add(1)

	idx := hash & tt.mask
	shard := tt.shardIndex(idx)

	tt.shards[shard].RLock()
	entry := tt.entries[idx]
	tt.shards[shard].RUnlock()

	// Verify the full 64-bit key matches (eliminates hash collisions)
	if entry.Key == hash && entry.Depth > 0 {
		tt.hits.Add(1)
		return entry, true
	}

	return TTEntry{}, false
}

// Store saves a position in the transposition table.
func (tt *TranspositionTable) Store(hash uint64, depth int, score int, flag TTFlag, bestMove board.Move) {
	idx := hash & tt.mask
	shard := tt.shardIndex(idx)

	tt.shards[shard].Lock()
	entry := &tt.entries[idx]

	// Replacement strategy:
	// - Always replace if new entry is from current search and deeper or equal depth
	// - Always replace if existing entry is from old search
	// - Never replace if existing entry is deeper and from current search

	currentAge := uint8(tt.age.Load())
	if entry.Age != currentAge || depth >= int(entry.Depth) {
		entry.Key = hash // Store full 64-bit hash
		entry.BestMove = bestMove
		entry.Score = int16(score)
		entry.Depth = int8(depth)
		entry.Flag = flag
		entry.Age = currentAge
	}
	tt.shards[shard].Unlock()
}

// NewSearch increments the age counter for a new search.
// This helps with replacement decisions.
func (tt *TranspositionTable) NewSearch() {
	tt.age.Add(1)
}

// Clear clears the transposition table.
func (tt *TranspositionTable) Clear() {
	for i := range tt.entries {
		tt.entries[i] = TTEntry{}
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
		if tt.entries[i].Depth > 0 && tt.entries[i].Age == currentAge {
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
