package tablebase

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SyzygyDownloader downloads Syzygy tablebase files from Lichess CDN.
type SyzygyDownloader struct {
	CacheDir string // Directory to cache files (e.g., ~/.chessplay/syzygy/)
	BaseURL  string // Base URL for downloading (e.g., https://tablebase.lichess.ovh/tables/)
	Client   *http.Client
}

// NewSyzygyDownloader creates a new downloader with default settings.
func NewSyzygyDownloader(cacheDir string) *SyzygyDownloader {
	return &SyzygyDownloader{
		CacheDir: cacheDir,
		BaseURL:  "https://tablebase.lichess.ovh/tables/standard/",
		Client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

// DefaultCacheDir returns the default cache directory for Syzygy files.
func DefaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./syzygy"
	}
	return filepath.Join(home, ".chessplay", "syzygy")
}

// EnsureCacheDir creates the cache directory if it doesn't exist.
func (d *SyzygyDownloader) EnsureCacheDir() error {
	return os.MkdirAll(d.CacheDir, 0755)
}

// 5-piece tablebase file names (145 files, ~939MB total)
var FivePieceFiles = []string{
	// KXvK endings
	"KQvK", "KRvK", "KBvK", "KNvK", "KPvK",
	// KXXvK endings
	"KQQvK", "KQRvK", "KQBvK", "KQNvK", "KQPvK",
	"KRRvK", "KRBvK", "KRNvK", "KRPvK",
	"KBBvK", "KBNvK", "KBPvK",
	"KNNvK", "KNPvK",
	"KPPvK",
	// KXvKX endings
	"KQvKQ", "KQvKR", "KQvKB", "KQvKN", "KQvKP",
	"KRvKR", "KRvKB", "KRvKN", "KRvKP",
	"KBvKB", "KBvKN", "KBvKP",
	"KNvKN", "KNvKP",
	"KPvKP",
	// KXXvKX endings
	"KQQvKQ", "KQQvKR", "KQQvKB", "KQQvKN", "KQQvKP",
	"KQRvKQ", "KQRvKR", "KQRvKB", "KQRvKN", "KQRvKP",
	"KQBvKQ", "KQBvKR", "KQBvKB", "KQBvKN", "KQBvKP",
	"KQNvKQ", "KQNvKR", "KQNvKB", "KQNvKN", "KQNvKP",
	"KQPvKQ", "KQPvKR", "KQPvKB", "KQPvKN", "KQPvKP",
	"KRRvKQ", "KRRvKR", "KRRvKB", "KRRvKN", "KRRvKP",
	"KRBvKQ", "KRBvKR", "KRBvKB", "KRBvKN", "KRBvKP",
	"KRNvKQ", "KRNvKR", "KRNvKB", "KRNvKN", "KRNvKP",
	"KRPvKQ", "KRPvKR", "KRPvKB", "KRPvKN", "KRPvKP",
	"KBBvKQ", "KBBvKR", "KBBvKB", "KBBvKN", "KBBvKP",
	"KBNvKQ", "KBNvKR", "KBNvKB", "KBNvKN", "KBNvKP",
	"KBPvKQ", "KBPvKR", "KBPvKB", "KBPvKN", "KBPvKP",
	"KNNvKQ", "KNNvKR", "KNNvKB", "KNNvKN", "KNNvKP",
	"KNPvKQ", "KNPvKR", "KNPvKB", "KNPvKN", "KNPvKP",
	"KPPvKQ", "KPPvKR", "KPPvKB", "KPPvKN", "KPPvKP",
	// KXvKXX endings (symmetric)
	"KQvKQQ", "KQvKQR", "KQvKQB", "KQvKQN", "KQvKQP",
	"KQvKRR", "KQvKRB", "KQvKRN", "KQvKRP",
	"KQvKBB", "KQvKBN", "KQvKBP",
	"KQvKNN", "KQvKNP",
	"KQvKPP",
	"KRvKQR", "KRvKQB", "KRvKQN", "KRvKQP",
	"KRvKRR", "KRvKRB", "KRvKRN", "KRvKRP",
	"KRvKBB", "KRvKBN", "KRvKBP",
	"KRvKNN", "KRvKNP",
	"KRvKPP",
	"KBvKQB", "KBvKQN", "KBvKQP",
	"KBvKRB", "KBvKRN", "KBvKRP",
	"KBvKBB", "KBvKBN", "KBvKBP",
	"KBvKNN", "KBvKNP",
	"KBvKPP",
	"KNvKQN", "KNvKQP",
	"KNvKRN", "KNvKRP",
	"KNvKBN", "KNvKBP",
	"KNvKNN", "KNvKNP",
	"KNvKPP",
	"KPvKQP",
	"KPvKRP",
	"KPvKBP",
	"KPvKNP",
	"KPvKPP",
}

// DownloadProgress tracks download progress.
type DownloadProgress struct {
	File          string
	BytesReceived int64
	TotalBytes    int64
	Done          bool
	Error         error
}

// HasFile checks if a tablebase file is already downloaded.
func (d *SyzygyDownloader) HasFile(name string) bool {
	wdlPath := filepath.Join(d.CacheDir, name+".rtbw")
	dtzPath := filepath.Join(d.CacheDir, name+".rtbz")

	_, wdlErr := os.Stat(wdlPath)
	_, dtzErr := os.Stat(dtzPath)

	return wdlErr == nil && dtzErr == nil
}

// DownloadFile downloads a single tablebase (both WDL and DTZ).
func (d *SyzygyDownloader) DownloadFile(name string, progress chan<- DownloadProgress) error {
	if err := d.EnsureCacheDir(); err != nil {
		return err
	}

	// Download WDL file (.rtbw)
	wdlURL := d.BaseURL + "wdl/" + name + ".rtbw"
	wdlPath := filepath.Join(d.CacheDir, name+".rtbw")
	if err := d.downloadSingleFile(wdlURL, wdlPath, name+".rtbw", progress); err != nil {
		return fmt.Errorf("downloading WDL: %w", err)
	}

	// Download DTZ file (.rtbz)
	dtzURL := d.BaseURL + "dtz/" + name + ".rtbz"
	dtzPath := filepath.Join(d.CacheDir, name+".rtbz")
	if err := d.downloadSingleFile(dtzURL, dtzPath, name+".rtbz", progress); err != nil {
		return fmt.Errorf("downloading DTZ: %w", err)
	}

	return nil
}

func (d *SyzygyDownloader) downloadSingleFile(url, path, name string, progress chan<- DownloadProgress) error {
	// Check if already exists
	if _, err := os.Stat(path); err == nil {
		if progress != nil {
			progress <- DownloadProgress{File: name, Done: true}
		}
		return nil
	}

	// Create temporary file
	tmpPath := path + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Start download
	resp, err := d.Client.Get(url)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpPath)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Copy with progress
	var written int64
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				os.Remove(tmpPath)
				return werr
			}
			written += int64(n)
			if progress != nil {
				progress <- DownloadProgress{
					File:          name,
					BytesReceived: written,
					TotalBytes:    resp.ContentLength,
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(tmpPath)
			return err
		}
	}

	// Rename to final path
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if progress != nil {
		progress <- DownloadProgress{File: name, Done: true}
	}
	return nil
}

// Download5Piece downloads all 5-piece tablebases.
func (d *SyzygyDownloader) Download5Piece(progress chan<- DownloadProgress) error {
	for _, name := range FivePieceFiles {
		if d.HasFile(name) {
			continue
		}
		if err := d.DownloadFile(name, progress); err != nil {
			return fmt.Errorf("downloading %s: %w", name, err)
		}
	}
	return nil
}

// GetAvailableFiles returns the list of available tablebase files in cache.
func (d *SyzygyDownloader) GetAvailableFiles() []string {
	var files []string
	entries, err := os.ReadDir(d.CacheDir)
	if err != nil {
		return files
	}

	seen := make(map[string]int)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".rtbw") {
			base := strings.TrimSuffix(name, ".rtbw")
			seen[base]++
		} else if strings.HasSuffix(name, ".rtbz") {
			base := strings.TrimSuffix(name, ".rtbz")
			seen[base]++
		}
	}

	for base, count := range seen {
		if count >= 2 { // Both WDL and DTZ present
			files = append(files, base)
		}
	}

	sort.Strings(files)
	return files
}

// MaxPiecesAvailable returns the maximum piece count available in cache.
func (d *SyzygyDownloader) MaxPiecesAvailable() int {
	files := d.GetAvailableFiles()
	maxPieces := 0
	for _, f := range files {
		pieces := countPiecesFromName(f)
		if pieces > maxPieces {
			maxPieces = pieces
		}
	}
	return maxPieces
}

// countPiecesFromName counts pieces in a tablebase name like "KQRvKR".
func countPiecesFromName(name string) int {
	count := 0
	for _, c := range strings.ToUpper(name) {
		switch c {
		case 'K', 'Q', 'R', 'B', 'N', 'P':
			count++
		}
	}
	return count
}

// TotalDownloadSize returns the approximate total size for 5-piece tables.
func TotalDownloadSize5Piece() int64 {
	return 939 * 1024 * 1024 // ~939 MB
}

// FormatBytes formats bytes to human readable string.
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
