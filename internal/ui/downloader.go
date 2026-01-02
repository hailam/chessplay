package ui

import (
	"fmt"
	"image/color"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/hailam/chessplay/internal/storage"
)

// NNUE network file URLs and sizes
const (
	SmallNetURL  = "https://tests.stockfishchess.org/api/nn/nn-37f18f62d772.nnue"
	SmallNetName = "nn-37f18f62d772.nnue"
	SmallNetSize = 3674624 // ~3.5 MB

	BigNetURL  = "https://tests.stockfishchess.org/api/nn/nn-c288c895ea92.nnue"
	BigNetName = "nn-c288c895ea92.nnue"
	BigNetSize = 113246144 // ~108 MB
)

// DownloadState represents the current download state.
type DownloadState int

const (
	DownloadIdle DownloadState = iota
	DownloadInProgress
	DownloadComplete
	DownloadError
)

// Downloader dimensions
const (
	DownloaderWidth  = 420
	DownloaderHeight = 180
	DownloaderPadX   = 24
	DownloaderPadY   = 20
)

// DownloadProgress tracks download progress.
type DownloadProgress struct {
	CurrentFile   string
	TotalFiles    int
	CurrentFileNo int
	BytesReceived int64
	TotalBytes    int64
	State         DownloadState
	Error         error
}

// Downloader manages NNUE network file downloads.
type Downloader struct {
	visible  bool
	progress DownloadProgress
	mu       sync.RWMutex

	// Position (centered)
	x, y int

	// Callbacks
	onComplete func()
	onCancel   func()

	// Cancel channel
	cancelCh chan struct{}
}

// NewDownloader creates a new downloader.
func NewDownloader() *Downloader {
	d := &Downloader{}
	d.calculatePosition()
	return d
}

// calculatePosition centers the downloader on screen.
func (d *Downloader) calculatePosition() {
	d.x = (ScreenWidth - DownloaderWidth) / 2
	d.y = (ScreenHeight - DownloaderHeight) / 2
}

// IsVisible returns true if the downloader is visible.
func (d *Downloader) IsVisible() bool {
	return d.visible
}

// Show displays the downloader and starts downloading.
func (d *Downloader) Show(onComplete func(), onCancel func()) {
	d.visible = true
	d.onComplete = onComplete
	d.onCancel = onCancel
	d.cancelCh = make(chan struct{})

	d.mu.Lock()
	d.progress = DownloadProgress{
		State:      DownloadInProgress,
		TotalFiles: 2,
	}
	d.mu.Unlock()

	// Start download in background
	go d.downloadNetworks()
}

// Hide closes the downloader.
func (d *Downloader) Hide() {
	d.visible = false
}

// Cancel cancels the download.
func (d *Downloader) Cancel() {
	if d.cancelCh != nil {
		close(d.cancelCh)
	}
	if d.onCancel != nil {
		d.onCancel()
	}
	d.Hide()
}

// downloadNetworks downloads both NNUE network files.
func (d *Downloader) downloadNetworks() {
	nnueDir, err := storage.GetNNUEDir()
	if err != nil {
		d.setError(fmt.Errorf("failed to get NNUE directory: %w", err))
		return
	}

	// Download small network first
	d.updateProgress(SmallNetName, 1, SmallNetSize)
	if err := d.downloadFile(SmallNetURL, filepath.Join(nnueDir, SmallNetName), SmallNetSize); err != nil {
		d.setError(err)
		return
	}

	// Check for cancel
	select {
	case <-d.cancelCh:
		return
	default:
	}

	// Download big network
	d.updateProgress(BigNetName, 2, BigNetSize)
	if err := d.downloadFile(BigNetURL, filepath.Join(nnueDir, BigNetName), BigNetSize); err != nil {
		d.setError(err)
		return
	}

	// Complete
	d.mu.Lock()
	d.progress.State = DownloadComplete
	d.mu.Unlock()

	if d.onComplete != nil {
		d.onComplete()
	}
}

// updateProgress updates the current download progress.
func (d *Downloader) updateProgress(filename string, fileNo int, totalBytes int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.progress.CurrentFile = filename
	d.progress.CurrentFileNo = fileNo
	d.progress.TotalBytes = totalBytes
	d.progress.BytesReceived = 0
}

// setError sets the download error state.
func (d *Downloader) setError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.progress.State = DownloadError
	d.progress.Error = err
}

// downloadFile downloads a single file with progress tracking.
func (d *Downloader) downloadFile(url, destPath string, expectedSize int64) error {
	// Check if file already exists with reasonable size (> half expected size)
	// Using threshold instead of exact size to handle minor size variations
	if info, err := os.Stat(destPath); err == nil && info.Size() > expectedSize/2 {
		// File exists and has reasonable size, skip download
		d.mu.Lock()
		d.progress.BytesReceived = info.Size()
		d.mu.Unlock()
		return nil
	}

	// Create request
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", filepath.Base(destPath), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create destination file
	tmpPath := destPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	// Download with progress
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-d.cancelCh:
			out.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("download cancelled")
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				out.Close()
				os.Remove(tmpPath)
				return fmt.Errorf("failed to write: %w", writeErr)
			}

			d.mu.Lock()
			d.progress.BytesReceived += int64(n)
			d.mu.Unlock()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			out.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("download error: %w", err)
		}
	}

	out.Close()

	// Rename temp file to final name
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// Update handles input for the downloader.
func (d *Downloader) Update(input *InputHandler) bool {
	if !d.visible {
		return false
	}

	// Handle escape to cancel
	if IsKeyJustPressed(ebiten.KeyEscape) {
		d.Cancel()
		return true
	}

	// Check for completion or error
	d.mu.RLock()
	state := d.progress.State
	d.mu.RUnlock()

	if state == DownloadComplete || state == DownloadError {
		if input.IsLeftJustPressed() {
			d.Hide()
			return true
		}
	}

	return true
}

// Draw renders the downloader.
func (d *Downloader) Draw(screen *ebiten.Image, glass *GlassEffect) {
	if !d.visible {
		return
	}

	// Full-screen blur overlay with glass effect
	if glass != nil && glass.IsEnabled() {
		tint := color.RGBA{0, 0, 0, 100} // Dark tint for modal backdrop
		glass.DrawGlass(screen, 0, 0, scaleI(ScreenWidth), scaleI(ScreenHeight),
			tint, 3.0, 4.0) // sigma=3.0, refraction=4.0
	} else {
		// Fallback: semi-transparent overlay
		vector.DrawFilledRect(screen, 0, 0, scaleF(ScreenWidth), scaleF(ScreenHeight), modalOverlay, false)
	}

	// Modal background
	vector.DrawFilledRect(screen, scaleF(d.x), scaleF(d.y), scaleF(DownloaderWidth), scaleF(DownloaderHeight), modalBg, false)
	vector.StrokeRect(screen, scaleF(d.x), scaleF(d.y), scaleF(DownloaderWidth), scaleF(DownloaderHeight), float32(UIScale*2), modalBorder, false)

	// Get current progress
	d.mu.RLock()
	progress := d.progress
	d.mu.RUnlock()

	contentX := d.x + DownloaderPadX
	contentY := d.y + DownloaderPadY

	face := GetRegularFace()
	if face == nil {
		return
	}

	switch progress.State {
	case DownloadInProgress:
		d.drawInProgress(screen, contentX, contentY, progress, face)
	case DownloadComplete:
		d.drawComplete(screen, contentX, contentY, face)
	case DownloadError:
		d.drawError(screen, contentX, contentY, progress, face)
	}
}

// drawInProgress draws the in-progress state.
func (d *Downloader) drawInProgress(screen *ebiten.Image, x, y int, progress DownloadProgress, face *text.GoTextFace) {
	// Title
	d.drawCenteredText(screen, "Downloading NNUE Networks...", d.x+DownloaderWidth/2, y+10, textPrimary, face)

	// Progress bar
	barX := x
	barY := y + 50
	barW := DownloaderWidth - DownloaderPadX*2
	barH := 24

	// Background
	vector.DrawFilledRect(screen, scaleF(barX), scaleF(barY), scaleF(barW), scaleF(barH), widgetBg, false)
	vector.StrokeRect(screen, scaleF(barX), scaleF(barY), scaleF(barW), scaleF(barH), float32(UIScale), widgetBorder, false)

	// Progress fill
	if progress.TotalBytes > 0 {
		progressPct := float32(progress.BytesReceived) / float32(progress.TotalBytes)
		fillW := scaleF(barW-4) * progressPct
		vector.DrawFilledRect(screen, scaleF(barX+2), scaleF(barY+2), fillW, scaleF(barH-4), accentColor, false)
	}

	// Percentage text
	pct := 0.0
	if progress.TotalBytes > 0 {
		pct = float64(progress.BytesReceived) / float64(progress.TotalBytes) * 100
	}
	pctText := fmt.Sprintf("%.0f%%", pct)
	d.drawCenteredText(screen, pctText, barX+barW/2, barY+barH/2-2, textPrimary, face)

	// File info
	fileInfo := fmt.Sprintf("File %d of %d: %s", progress.CurrentFileNo, progress.TotalFiles, progress.CurrentFile)
	d.drawCenteredText(screen, fileInfo, d.x+DownloaderWidth/2, barY+barH+20, textSecondary, face)

	// Size info
	receivedMB := float64(progress.BytesReceived) / 1024 / 1024
	totalMB := float64(progress.TotalBytes) / 1024 / 1024
	sizeText := fmt.Sprintf("%.1f MB / %.1f MB", receivedMB, totalMB)
	d.drawCenteredText(screen, sizeText, d.x+DownloaderWidth/2, barY+barH+44, textMuted, face)
}

// drawComplete draws the complete state.
func (d *Downloader) drawComplete(screen *ebiten.Image, x, y int, face *text.GoTextFace) {
	successColor := color.RGBA{76, 175, 120, 255}
	d.drawCenteredText(screen, "Download Complete!", d.x+DownloaderWidth/2, y+40, successColor, face)
	d.drawCenteredText(screen, "NNUE networks are ready to use.", d.x+DownloaderWidth/2, y+70, textSecondary, face)
	d.drawCenteredText(screen, "Click anywhere to close.", d.x+DownloaderWidth/2, y+110, textMuted, face)
}

// drawError draws the error state.
func (d *Downloader) drawError(screen *ebiten.Image, x, y int, progress DownloadProgress, face *text.GoTextFace) {
	errorColor := color.RGBA{255, 100, 100, 255}
	d.drawCenteredText(screen, "Download Failed", d.x+DownloaderWidth/2, y+40, errorColor, face)

	errMsg := "Unknown error"
	if progress.Error != nil {
		errMsg = progress.Error.Error()
		if len(errMsg) > 40 {
			errMsg = errMsg[:40] + "..."
		}
	}
	d.drawCenteredText(screen, errMsg, d.x+DownloaderWidth/2, y+70, textSecondary, face)
	d.drawCenteredText(screen, "Click anywhere to close.", d.x+DownloaderWidth/2, y+110, textMuted, face)
}

// drawCenteredText draws centered text.
func (d *Downloader) drawCenteredText(screen *ebiten.Image, s string, centerX, centerY int, c color.Color, face *text.GoTextFace) {
	w, h := MeasureText(s, face)
	op := &text.DrawOptions{}
	op.GeoM.Translate(scaleD(centerX)-w/2, scaleD(centerY)-h/2)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(screen, s, face, op)
}

// CheckNNUENetworks checks if NNUE networks are available.
func CheckNNUENetworks() (smallExists, bigExists bool, err error) {
	nnueDir, err := storage.GetNNUEDir()
	if err != nil {
		return false, false, err
	}

	smallPath := filepath.Join(nnueDir, SmallNetName)
	bigPath := filepath.Join(nnueDir, BigNetName)

	// Check if files exist with reasonable size (> 1MB for small, > 50MB for big)
	// Using thresholds instead of exact sizes to handle minor size variations
	if info, err := os.Stat(smallPath); err == nil && info.Size() > 1*1024*1024 {
		smallExists = true
	}
	if info, err := os.Stat(bigPath); err == nil && info.Size() > 50*1024*1024 {
		bigExists = true
	}

	return smallExists, bigExists, nil
}

// GetNNUEPaths returns the paths to the NNUE network files.
func GetNNUEPaths() (smallPath, bigPath string, err error) {
	nnueDir, err := storage.GetNNUEDir()
	if err != nil {
		return "", "", err
	}

	return filepath.Join(nnueDir, SmallNetName), filepath.Join(nnueDir, BigNetName), nil
}
