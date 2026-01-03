package tablebase

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hailam/chessplay/internal/board"
)

// SyzygyProber probes local Syzygy tablebase files.
// Uses the Lichess API as fallback when local files are not available.
type SyzygyProber struct {
	path       string
	maxPieces  int
	available  bool
	fallback   Prober // Fallback to Lichess API when local files unavailable
	mu         sync.RWMutex
	downloader *SyzygyDownloader
}

// NewSyzygyProber creates a new Syzygy prober with the given path.
// If path is empty, uses the default cache directory.
// Falls back to Lichess API for positions not in local files.
func NewSyzygyProber(path string) *SyzygyProber {
	if path == "" {
		path = DefaultCacheDir()
	}

	sp := &SyzygyProber{
		path:       path,
		fallback:   NewCachedLichessProber(),
		downloader: NewSyzygyDownloader(path),
	}

	// Check what's available
	sp.refresh()

	return sp
}

// refresh checks available tablebase files and updates maxPieces.
func (sp *SyzygyProber) refresh() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Check if directory exists
	if _, err := os.Stat(sp.path); os.IsNotExist(err) {
		sp.available = false
		sp.maxPieces = 0
		log.Printf("[Syzygy] Path does not exist: %s, using Lichess API fallback", sp.path)
		return
	}

	// Count max pieces available
	sp.maxPieces = sp.downloader.MaxPiecesAvailable()
	sp.available = sp.maxPieces > 0

	if sp.available {
		log.Printf("[Syzygy] Found local tablebases at %s (max %d pieces)", sp.path, sp.maxPieces)
	} else {
		log.Printf("[Syzygy] No local tablebases found at %s, using Lichess API fallback", sp.path)
	}
}

// SetPath updates the tablebase path and refreshes available files.
func (sp *SyzygyProber) SetPath(path string) {
	if path == "" {
		path = DefaultCacheDir()
	}
	sp.path = path
	sp.downloader = NewSyzygyDownloader(path)
	sp.refresh()
}

// Probe looks up a position in the tablebase.
// Currently uses Lichess API as the probing backend since we don't have
// a pure Go Syzygy file reader. Local files can be used once a pure Go
// implementation is integrated.
func (sp *SyzygyProber) Probe(pos *board.Position) ProbeResult {
	pieceCount := CountPieces(pos)

	// Check if position is within tablebase range
	if pieceCount > 7 {
		return ProbeResult{Found: false}
	}

	// Use Lichess API for now (cached)
	// TODO: Add pure Go local file reading when library is available
	return sp.fallback.Probe(pos)
}

// ProbeRoot finds the best move from the tablebase.
func (sp *SyzygyProber) ProbeRoot(pos *board.Position) RootResult {
	pieceCount := CountPieces(pos)

	if pieceCount > 7 {
		return RootResult{Found: false}
	}

	// Use Lichess API for now (cached)
	return sp.fallback.ProbeRoot(pos)
}

// MaxPieces returns the maximum number of pieces supported.
func (sp *SyzygyProber) MaxPieces() int {
	// Lichess API supports up to 7-piece tablebases
	return 7
}

// Available returns true if tablebase probing is available.
func (sp *SyzygyProber) Available() bool {
	// Always available via Lichess fallback
	return true
}

// LocalMaxPieces returns the max pieces available locally.
func (sp *SyzygyProber) LocalMaxPieces() int {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.maxPieces
}

// HasLocalFiles returns true if local tablebase files exist.
func (sp *SyzygyProber) HasLocalFiles() bool {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.available
}

// Path returns the current tablebase path.
func (sp *SyzygyProber) Path() string {
	return sp.path
}

// Download5Piece downloads all 5-piece tablebase files.
// Returns a channel for progress updates.
func (sp *SyzygyProber) Download5Piece() (<-chan DownloadProgress, error) {
	if err := sp.downloader.EnsureCacheDir(); err != nil {
		return nil, err
	}

	progress := make(chan DownloadProgress, 100)

	go func() {
		defer close(progress)
		if err := sp.downloader.Download5Piece(progress); err != nil {
			progress <- DownloadProgress{Error: err}
		}
		// Refresh after download
		sp.refresh()
	}()

	return progress, nil
}

// HybridProber combines local Syzygy files with Lichess API fallback.
// Prefers local files when available, falls back to API for missing endgames.
type HybridProber struct {
	local    *SyzygyProber
	online   *CachedProber
	useLocal bool
}

// NewHybridProber creates a prober that uses local files when available.
func NewHybridProber(syzygyPath string) *HybridProber {
	local := NewSyzygyProber(syzygyPath)
	online := NewCachedLichessProber()

	return &HybridProber{
		local:    local,
		online:   online,
		useLocal: local.HasLocalFiles(),
	}
}

func (hp *HybridProber) Probe(pos *board.Position) ProbeResult {
	// Currently both use Lichess API under the hood
	// Local file reading will be added when pure Go library is integrated
	return hp.online.Probe(pos)
}

func (hp *HybridProber) ProbeRoot(pos *board.Position) RootResult {
	return hp.online.ProbeRoot(pos)
}

func (hp *HybridProber) MaxPieces() int {
	return 7 // Lichess supports 7-piece
}

func (hp *HybridProber) Available() bool {
	return true
}

// CacheHitRate returns the API cache hit rate.
func (hp *HybridProber) CacheHitRate() float64 {
	return hp.online.HitRate()
}

// ClearCache clears the API cache.
func (hp *HybridProber) ClearCache() {
	hp.online.Clear()
}

// positionToMaterial converts a position to a material key like "KQvKR".
// This is used for tablebase file lookup.
func positionToMaterial(pos *board.Position) string {
	var white, black strings.Builder

	// Count pieces for each side
	for pt := board.Queen; pt >= board.Pawn; pt-- {
		count := (pos.Pieces[board.White][pt]).PopCount()
		for i := 0; i < count; i++ {
			white.WriteByte(pieceChar(pt))
		}
	}

	for pt := board.Queen; pt >= board.Pawn; pt-- {
		count := (pos.Pieces[board.Black][pt]).PopCount()
		for i := 0; i < count; i++ {
			black.WriteByte(pieceChar(pt))
		}
	}

	// Format: KXXvKYY (always include kings)
	return "K" + white.String() + "vK" + black.String()
}

func pieceChar(pt board.PieceType) byte {
	switch pt {
	case board.Queen:
		return 'Q'
	case board.Rook:
		return 'R'
	case board.Bishop:
		return 'B'
	case board.Knight:
		return 'N'
	case board.Pawn:
		return 'P'
	default:
		return '?'
	}
}

// checkLocalFile checks if a tablebase file exists locally.
func (sp *SyzygyProber) checkLocalFile(material string) bool {
	wdlPath := filepath.Join(sp.path, material+".rtbw")
	dtzPath := filepath.Join(sp.path, material+".rtbz")

	_, wdlErr := os.Stat(wdlPath)
	_, dtzErr := os.Stat(dtzPath)

	return wdlErr == nil && dtzErr == nil
}
