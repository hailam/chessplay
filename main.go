// ChessPlay - A chess game built with Ebitengine
package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hailam/chessplay/internal/ui"
)

func main() {
	game := ui.NewGame()

	ebiten.SetWindowSize(ui.ScreenWidth, ui.ScreenHeight)
	ebiten.SetWindowTitle("ChessPlay")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	// Enable smooth scaling when window is resized or fullscreen
	ebiten.SetScreenFilterEnabled(true)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
