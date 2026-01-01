// Package ui implements the chess game UI using Ebitengine.
package ui

import (
	"bytes"
	"log"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/gofont/gobold"
)

var (
	// Font faces for text rendering
	regularFace *text.GoTextFace
	boldFace    *text.GoTextFace
)

const (
	defaultFontSize = 14.0
	titleFontSize   = 16.0
)

func init() {
	initFonts()
}

func initFonts() {
	// Load regular font
	regularSource, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		log.Printf("Failed to load regular font: %v", err)
		return
	}
	regularFace = &text.GoTextFace{
		Source: regularSource,
		Size:   defaultFontSize,
	}

	// Load bold font
	boldSource, err := text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF))
	if err != nil {
		log.Printf("Failed to load bold font: %v", err)
		return
	}
	boldFace = &text.GoTextFace{
		Source: boldSource,
		Size:   titleFontSize,
	}
}

// GetRegularFace returns the regular font face.
func GetRegularFace() *text.GoTextFace {
	return regularFace
}

// GetBoldFace returns the bold font face.
func GetBoldFace() *text.GoTextFace {
	return boldFace
}

// GetFaceWithSize returns a font face with a custom size.
func GetFaceWithSize(size float64) *text.GoTextFace {
	if regularFace == nil {
		return nil
	}
	return &text.GoTextFace{
		Source: regularFace.Source,
		Size:   size,
	}
}

// MeasureText returns the width and height of the given text.
func MeasureText(s string, face *text.GoTextFace) (width, height float64) {
	if face == nil {
		return 0, 0
	}
	w, h := text.Measure(s, face, 0)
	return w, h
}
