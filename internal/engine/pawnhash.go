package engine

// PawnEntry stores cached pawn structure evaluation.
type PawnEntry struct {
	Key     uint64
	MgScore int16 // Middlegame score
	EgScore int16 // Endgame score
}

// PawnTable is a hash table for caching pawn structure evaluations.
type PawnTable struct {
	entries []PawnEntry
	mask    uint64
}

// NewPawnTable creates a new pawn hash table with the given size in MB.
func NewPawnTable(sizeMB int) *PawnTable {
	// Each entry is 12 bytes (8 + 2 + 2), round to power of 2
	entrySize := 12
	numEntries := (sizeMB * 1024 * 1024) / entrySize

	// Round down to power of 2
	size := 1
	for size*2 <= numEntries {
		size *= 2
	}

	return &PawnTable{
		entries: make([]PawnEntry, size),
		mask:    uint64(size - 1),
	}
}

// Probe looks up a pawn structure evaluation in the hash table.
// Returns the middlegame and endgame scores if found.
func (pt *PawnTable) Probe(key uint64) (mg, eg int, found bool) {
	entry := &pt.entries[key&pt.mask]
	if entry.Key == key {
		return int(entry.MgScore), int(entry.EgScore), true
	}
	return 0, 0, false
}

// Store saves a pawn structure evaluation in the hash table.
func (pt *PawnTable) Store(key uint64, mg, eg int) {
	entry := &pt.entries[key&pt.mask]
	entry.Key = key
	entry.MgScore = int16(mg)
	entry.EgScore = int16(eg)
}

// Clear clears the pawn hash table.
func (pt *PawnTable) Clear() {
	for i := range pt.entries {
		pt.entries[i] = PawnEntry{}
	}
}
