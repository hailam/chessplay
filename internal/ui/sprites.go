// Package ui implements the chess game UI using Ebitengine.
package ui

import (
	"bytes"
	"embed"
	"image"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hailam/chessplay/internal/board"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

//go:embed assets/pieces/*.svg
var pieceAssets embed.FS

// SpriteManager manages piece sprites.
type SpriteManager struct {
	pieces      map[board.Piece]*ebiten.Image
	size        int     // Display size (e.g., 80)
	renderScale float64 // Render at higher resolution for quality (e.g., 3.0)
}

// NewSpriteManager creates a new sprite manager with pieces of the given size.
func NewSpriteManager(size int) *SpriteManager {
	sm := &SpriteManager{
		pieces:      make(map[board.Piece]*ebiten.Image),
		size:        size,
		renderScale: 3.0, // Render at 3x resolution for sharp scaling
	}
	sm.loadPieces()
	return sm
}

// GetPiece returns the sprite for a piece.
func (sm *SpriteManager) GetPiece(p board.Piece) *ebiten.Image {
	return sm.pieces[p]
}

// pieceFiles maps pieces to their asset file paths.
var pieceFiles = map[board.Piece]string{
	board.NewPiece(board.Pawn, board.White):   "assets/pieces/wP.svg",
	board.NewPiece(board.Knight, board.White): "assets/pieces/wN.svg",
	board.NewPiece(board.Bishop, board.White): "assets/pieces/wB.svg",
	board.NewPiece(board.Rook, board.White):   "assets/pieces/wR.svg",
	board.NewPiece(board.Queen, board.White):  "assets/pieces/wQ.svg",
	board.NewPiece(board.King, board.White):   "assets/pieces/wK.svg",
	board.NewPiece(board.Pawn, board.Black):   "assets/pieces/bP.svg",
	board.NewPiece(board.Knight, board.Black): "assets/pieces/bN.svg",
	board.NewPiece(board.Bishop, board.Black): "assets/pieces/bB.svg",
	board.NewPiece(board.Rook, board.Black):   "assets/pieces/bR.svg",
	board.NewPiece(board.Queen, board.Black):  "assets/pieces/bQ.svg",
	board.NewPiece(board.King, board.Black):   "assets/pieces/bK.svg",
}

// loadPieces loads all piece sprites from embedded SVG files.
func (sm *SpriteManager) loadPieces() {
	// Render at higher resolution for better quality when scaled
	renderSize := int(float64(sm.size) * sm.renderScale)

	for piece, path := range pieceFiles {
		data, err := pieceAssets.ReadFile(path)
		if err != nil {
			log.Printf("Failed to read piece asset %s: %v", path, err)
			continue
		}

		// Parse SVG
		icon, err := oksvg.ReadIconStream(bytes.NewReader(data))
		if err != nil {
			log.Printf("Failed to parse SVG %s: %v", path, err)
			continue
		}

		// Set target size at higher resolution for quality
		icon.SetTarget(0, 0, float64(renderSize), float64(renderSize))

		// Create RGBA image and render with anti-aliasing at high resolution
		rgba := image.NewRGBA(image.Rect(0, 0, renderSize, renderSize))
		scanner := rasterx.NewScannerGV(renderSize, renderSize, rgba, rgba.Bounds())
		raster := rasterx.NewDasher(renderSize, renderSize, scanner)
		icon.Draw(raster, 1.0)

		sm.pieces[piece] = ebiten.NewImageFromImage(rgba)
	}
}

// DrawPieceAt draws a piece at the given pixel coordinates.
func (sm *SpriteManager) DrawPieceAt(screen *ebiten.Image, p board.Piece, x, y int) {
	if p == board.NoPiece {
		return
	}
	sprite := sm.GetPiece(p)
	if sprite == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	// Scale down from render resolution to display size
	scale := 1.0 / sm.renderScale
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(x), float64(y))
	// Use linear filtering for smooth scaling
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(sprite, op)
}

// Size returns the size of piece sprites.
func (sm *SpriteManager) Size() int {
	return sm.size
}

// GetHighlightedPiece returns a version of the piece with a highlight effect.
func (sm *SpriteManager) GetHighlightedPiece(p board.Piece) *ebiten.Image {
	base := sm.GetPiece(p)
	if base == nil {
		return nil
	}

	bounds := base.Bounds()
	highlighted := ebiten.NewImage(bounds.Dx(), bounds.Dy())

	// Draw a subtle glow effect
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.Scale(1.2, 1.2, 1.0, 1.0) // Slightly brighter
	highlighted.DrawImage(base, op)

	return highlighted
}
