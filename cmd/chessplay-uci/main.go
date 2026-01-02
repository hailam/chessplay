package main

import (
	"github.com/hailam/chessplay/internal/engine"
	"github.com/hailam/chessplay/internal/uci"
)

func main() {
	// Create engine with 64MB hash table
	eng := engine.NewEngine(64)

	// Create and run UCI protocol handler
	protocol := uci.New(eng)
	protocol.Run()
}
