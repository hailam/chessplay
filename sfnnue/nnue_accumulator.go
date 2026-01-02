// NNUE Accumulator for incremental updates.
// Ported from Stockfish src/nnue/nnue_accumulator.h and .cpp

package sfnnue

// Accumulator holds the result of affine transformation of input features.
// Ported from nnue_accumulator.h:47-52
type Accumulator struct {
	// Accumulated values for each color [COLOR_NB][HalfDimensions]
	Accumulation [2][]int16

	// PSQT accumulated values for each color [COLOR_NB][PSQTBuckets]
	PSQTAccumulation [2][]int32

	// Whether each color's accumulator is computed
	Computed [2]bool

	// King squares when accumulator was computed (for detecting king moves)
	KingSq [2]int

	// Whether each perspective needs full refresh (king moved)
	NeedsRefresh [2]bool
}

// SQ_NONE represents no square (for king tracking)
const SQ_NONE = 64

// NewAccumulator creates a new accumulator with the given half dimensions
func NewAccumulator(halfDims int) *Accumulator {
	return &Accumulator{
		Accumulation: [2][]int16{
			make([]int16, halfDims),
			make([]int16, halfDims),
		},
		PSQTAccumulation: [2][]int32{
			make([]int32, PSQTBuckets),
			make([]int32, PSQTBuckets),
		},
		Computed:     [2]bool{false, false},
		KingSq:       [2]int{SQ_NONE, SQ_NONE},
		NeedsRefresh: [2]bool{true, true},
	}
}

// Reset marks the accumulator as not computed
func (a *Accumulator) Reset() {
	a.Computed[0] = false
	a.Computed[1] = false
	a.KingSq[0] = SQ_NONE
	a.KingSq[1] = SQ_NONE
	a.NeedsRefresh[0] = true
	a.NeedsRefresh[1] = true
}

// Copy copies values from another accumulator
func (a *Accumulator) Copy(other *Accumulator) {
	copy(a.Accumulation[0], other.Accumulation[0])
	copy(a.Accumulation[1], other.Accumulation[1])
	copy(a.PSQTAccumulation[0], other.PSQTAccumulation[0])
	copy(a.PSQTAccumulation[1], other.PSQTAccumulation[1])
	a.Computed[0] = other.Computed[0]
	a.Computed[1] = other.Computed[1]
	a.KingSq[0] = other.KingSq[0]
	a.KingSq[1] = other.KingSq[1]
	a.NeedsRefresh[0] = other.NeedsRefresh[0]
	a.NeedsRefresh[1] = other.NeedsRefresh[1]
}

// AccumulatorStack manages accumulators during search.
// Ported from nnue_accumulator.h:152-202
type AccumulatorStack struct {
	// Stack of accumulators for big network
	BigAccumulators []Accumulator

	// Stack of accumulators for small network
	SmallAccumulators []Accumulator

	// Current stack size
	Size int
}

// MaxStackSize is the maximum ply depth
const MaxStackSize = 256

// NewAccumulatorStack creates a new accumulator stack
func NewAccumulatorStack() *AccumulatorStack {
	stack := &AccumulatorStack{
		BigAccumulators:   make([]Accumulator, MaxStackSize),
		SmallAccumulators: make([]Accumulator, MaxStackSize),
		Size:              1,
	}

	// Initialize all accumulators
	for i := range stack.BigAccumulators {
		stack.BigAccumulators[i] = *NewAccumulator(TransformedFeatureDimensionsBig)
	}
	for i := range stack.SmallAccumulators {
		stack.SmallAccumulators[i] = *NewAccumulator(TransformedFeatureDimensionsSmall)
	}

	return stack
}

// Reset resets the stack to initial state
func (s *AccumulatorStack) Reset() {
	s.Size = 1
	s.BigAccumulators[0].Reset()
	s.SmallAccumulators[0].Reset()
}

// Push saves current state and prepares for a new position
func (s *AccumulatorStack) Push() {
	if s.Size < MaxStackSize {
		// Copy current accumulator to next level
		s.BigAccumulators[s.Size].Copy(&s.BigAccumulators[s.Size-1])
		s.SmallAccumulators[s.Size].Copy(&s.SmallAccumulators[s.Size-1])
		s.Size++
	}
}

// Pop restores previous state
func (s *AccumulatorStack) Pop() {
	if s.Size > 1 {
		s.Size--
	}
}

// CurrentBig returns the current big network accumulator
func (s *AccumulatorStack) CurrentBig() *Accumulator {
	return &s.BigAccumulators[s.Size-1]
}

// CurrentSmall returns the current small network accumulator
func (s *AccumulatorStack) CurrentSmall() *Accumulator {
	return &s.SmallAccumulators[s.Size-1]
}

// PreviousBig returns the previous big network accumulator (for incremental updates)
func (s *AccumulatorStack) PreviousBig() *Accumulator {
	if s.Size > 1 {
		return &s.BigAccumulators[s.Size-2]
	}
	return nil
}

// PreviousSmall returns the previous small network accumulator (for incremental updates)
func (s *AccumulatorStack) PreviousSmall() *Accumulator {
	if s.Size > 1 {
		return &s.SmallAccumulators[s.Size-2]
	}
	return nil
}

// CanIncrementallyUpdate checks if we can do an incremental update for the given perspective
func (s *AccumulatorStack) CanIncrementallyUpdate(perspective int) bool {
	if s.Size <= 1 {
		return false
	}
	prev := s.PreviousBig()
	if prev == nil {
		return false
	}
	// Can incrementally update if previous was computed and no king move for this perspective
	return prev.Computed[perspective] && !s.CurrentBig().NeedsRefresh[perspective]
}

// AccumulatorCache provides per-king-square caches for efficient refresh.
// Ported from nnue_accumulator.h:61-106 (Finny Tables)
type AccumulatorCache struct {
	// Cache entries indexed by [king_square][color]
	Entries [64][2]AccumulatorCacheEntry
}

// AccumulatorCacheEntry stores cached accumulator state for a king position
type AccumulatorCacheEntry struct {
	Accumulation     []int16
	PSQTAccumulation []int32
	Pieces           [64]int // Piece on each square
	PieceBB          uint64  // Bitboard of pieces
}

// NewAccumulatorCache creates a new cache for the given dimensions
func NewAccumulatorCache(halfDims int, biases []int16) *AccumulatorCache {
	cache := &AccumulatorCache{}

	for sq := 0; sq < 64; sq++ {
		for c := 0; c < 2; c++ {
			entry := &cache.Entries[sq][c]
			entry.Accumulation = make([]int16, halfDims)
			entry.PSQTAccumulation = make([]int32, PSQTBuckets)

			// Initialize accumulation with biases
			copy(entry.Accumulation, biases)

			// Clear piece info
			for i := range entry.Pieces {
				entry.Pieces[i] = 0
			}
			entry.PieceBB = 0
		}
	}

	return cache
}

// Clear resets the cache with the given biases
func (c *AccumulatorCache) Clear(biases []int16) {
	for sq := 0; sq < 64; sq++ {
		for color := 0; color < 2; color++ {
			entry := &c.Entries[sq][color]
			copy(entry.Accumulation, biases)
			for i := range entry.PSQTAccumulation {
				entry.PSQTAccumulation[i] = 0
			}
			for i := range entry.Pieces {
				entry.Pieces[i] = 0
			}
			entry.PieceBB = 0
		}
	}
}

// GetEntry returns the cache entry for a king position and perspective
func (c *AccumulatorCache) GetEntry(kingSq, perspective int) *AccumulatorCacheEntry {
	return &c.Entries[kingSq][perspective]
}

// UpdateFromCache updates an accumulator from a cache entry.
// Returns the number of pieces that changed (for deciding if incremental update is worthwhile).
func (c *AccumulatorCache) UpdateFromCache(
	entry *AccumulatorCacheEntry,
	acc *Accumulator,
	currentPieceBB uint64,
	currentPieces [64]int,
	perspective int,
	halfDims int,
	weights []int16,
	psqtWeights []int32,
	makeIndexFn func(perspective, sq, piece, kingSq int) int,
	kingSq int,
) int {
	// Find pieces that changed
	changedBB := entry.PieceBB ^ currentPieceBB

	changedCount := 0
	// Count bits in changedBB
	bb := changedBB
	for bb != 0 {
		bb &= bb - 1
		changedCount++
	}

	// If too many pieces changed, it's faster to do a full refresh
	// (typically if more than 3-4 pieces changed)
	if changedCount > 4 {
		return changedCount
	}

	// Copy from cache
	copy(acc.Accumulation[perspective], entry.Accumulation)
	copy(acc.PSQTAccumulation[perspective], entry.PSQTAccumulation)

	// Apply changes
	bb = changedBB
	for bb != 0 {
		sq := trailingZeros64(bb)
		bb &= bb - 1

		// Check if piece was removed or added
		wasPresent := (entry.PieceBB & (1 << sq)) != 0
		isPresent := (currentPieceBB & (1 << sq)) != 0

		if wasPresent && !isPresent {
			// Piece was removed
			pc := entry.Pieces[sq]
			if pc != 0 {
				idx := makeIndexFn(perspective, sq, pc, kingSq)
				offset := idx * halfDims
				for i := 0; i < halfDims; i++ {
					acc.Accumulation[perspective][i] -= weights[offset+i]
				}
				psqtOffset := idx * 8 // PSQTBuckets
				for b := 0; b < 8; b++ {
					acc.PSQTAccumulation[perspective][b] -= psqtWeights[psqtOffset+b]
				}
			}
		} else if !wasPresent && isPresent {
			// Piece was added
			pc := currentPieces[sq]
			if pc != 0 {
				idx := makeIndexFn(perspective, sq, pc, kingSq)
				offset := idx * halfDims
				for i := 0; i < halfDims; i++ {
					acc.Accumulation[perspective][i] += weights[offset+i]
				}
				psqtOffset := idx * 8 // PSQTBuckets
				for b := 0; b < 8; b++ {
					acc.PSQTAccumulation[perspective][b] += psqtWeights[psqtOffset+b]
				}
			}
		} else if wasPresent && isPresent {
			// Piece changed (e.g., promotion or different piece)
			oldPc := entry.Pieces[sq]
			newPc := currentPieces[sq]
			if oldPc != newPc {
				// Remove old
				if oldPc != 0 {
					idx := makeIndexFn(perspective, sq, oldPc, kingSq)
					offset := idx * halfDims
					for i := 0; i < halfDims; i++ {
						acc.Accumulation[perspective][i] -= weights[offset+i]
					}
					psqtOffset := idx * 8
					for b := 0; b < 8; b++ {
						acc.PSQTAccumulation[perspective][b] -= psqtWeights[psqtOffset+b]
					}
				}
				// Add new
				if newPc != 0 {
					idx := makeIndexFn(perspective, sq, newPc, kingSq)
					offset := idx * halfDims
					for i := 0; i < halfDims; i++ {
						acc.Accumulation[perspective][i] += weights[offset+i]
					}
					psqtOffset := idx * 8
					for b := 0; b < 8; b++ {
						acc.PSQTAccumulation[perspective][b] += psqtWeights[psqtOffset+b]
					}
				}
			}
		}
	}

	acc.Computed[perspective] = true
	return changedCount
}

// SaveToCache saves the current accumulator state to the cache entry
func (c *AccumulatorCache) SaveToCache(
	entry *AccumulatorCacheEntry,
	acc *Accumulator,
	currentPieceBB uint64,
	currentPieces [64]int,
	perspective int,
) {
	copy(entry.Accumulation, acc.Accumulation[perspective])
	copy(entry.PSQTAccumulation, acc.PSQTAccumulation[perspective])
	entry.PieceBB = currentPieceBB
	copy(entry.Pieces[:], currentPieces[:])
}

// trailingZeros64 returns the number of trailing zeros in a 64-bit integer
func trailingZeros64(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	if x&0xFFFFFFFF == 0 {
		n += 32
		x >>= 32
	}
	if x&0xFFFF == 0 {
		n += 16
		x >>= 16
	}
	if x&0xFF == 0 {
		n += 8
		x >>= 8
	}
	if x&0xF == 0 {
		n += 4
		x >>= 4
	}
	if x&0x3 == 0 {
		n += 2
		x >>= 2
	}
	if x&0x1 == 0 {
		n += 1
	}
	return n
}
