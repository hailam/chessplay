package tablebase

import (
	"sync"

	"github.com/hailam/chessplay/internal/board"
)

// CachedProber wraps another prober with an LRU cache.
// This reduces API calls for frequently probed positions.
type CachedProber struct {
	inner     Prober
	cache     map[uint64]ProbeResult
	mu        sync.RWMutex
	maxSize   int
	hits      uint64
	misses    uint64
}

// NewCachedProber creates a cached prober wrapping the given prober.
func NewCachedProber(inner Prober, cacheSize int) *CachedProber {
	return &CachedProber{
		inner:   inner,
		cache:   make(map[uint64]ProbeResult, cacheSize),
		maxSize: cacheSize,
	}
}

// NewCachedLichessProber creates a cached Lichess prober with default cache size.
func NewCachedLichessProber() *CachedProber {
	return NewCachedProber(NewLichessProber(), 100000)
}

func (cp *CachedProber) Probe(pos *board.Position) ProbeResult {
	// Check cache first
	cp.mu.RLock()
	if result, ok := cp.cache[pos.Hash]; ok {
		cp.mu.RUnlock()
		cp.mu.Lock()
		cp.hits++
		cp.mu.Unlock()
		return result
	}
	cp.mu.RUnlock()

	// Cache miss - probe underlying
	result := cp.inner.Probe(pos)

	// Store in cache
	cp.mu.Lock()
	cp.misses++
	if len(cp.cache) >= cp.maxSize {
		// Simple eviction: clear half the cache
		i := 0
		for k := range cp.cache {
			if i >= cp.maxSize/2 {
				break
			}
			delete(cp.cache, k)
			i++
		}
	}
	cp.cache[pos.Hash] = result
	cp.mu.Unlock()

	return result
}

func (cp *CachedProber) ProbeRoot(pos *board.Position) RootResult {
	// Root probing is not cached (needs move info)
	return cp.inner.ProbeRoot(pos)
}

func (cp *CachedProber) MaxPieces() int {
	return cp.inner.MaxPieces()
}

func (cp *CachedProber) Available() bool {
	return cp.inner.Available()
}

// HitRate returns the cache hit rate as a percentage.
func (cp *CachedProber) HitRate() float64 {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	total := cp.hits + cp.misses
	if total == 0 {
		return 0
	}
	return float64(cp.hits) / float64(total) * 100
}

// CacheSize returns the current number of cached entries.
func (cp *CachedProber) CacheSize() int {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return len(cp.cache)
}

// Clear clears the cache.
func (cp *CachedProber) Clear() {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.cache = make(map[uint64]ProbeResult, cp.maxSize)
	cp.hits = 0
	cp.misses = 0
}
