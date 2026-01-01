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

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
