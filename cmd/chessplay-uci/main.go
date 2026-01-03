package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/hailam/chessplay/internal/engine"
	"github.com/hailam/chessplay/internal/uci"
)

// Default NNUE file names (Stockfish compatible)
const (
	defaultBigNet   = "nn-c288c895ea92.nnue" // ~108MB
	defaultSmallNet = "nn-37f18f62d772.nnue" // ~3.5MB
)

func main() {
	// Create engine with 64MB hash table
	eng := engine.NewEngine(64)

	// Auto-load NNUE from default locations
	if err := autoLoadNNUE(eng); err != nil {
		log.Printf("Warning: NNUE not loaded: %v (using classical evaluation)", err)
	}

	// Create and run UCI protocol handler
	protocol := uci.New(eng)
	protocol.Run()
}

// autoLoadNNUE attempts to load NNUE weights from standard locations
func autoLoadNNUE(eng *engine.Engine) error {
	// Try multiple locations in order of preference
	searchPaths := []string{
		getAppSupportDir(),                    // ~/Library/Application Support/chessplay/nnue/
		filepath.Join(getHomeDir(), ".chessplay", "nnue"), // ~/.chessplay/nnue/
		"./nnue",                              // ./nnue/ (current directory)
		".",                                   // current directory
	}

	for _, dir := range searchPaths {
		bigPath := filepath.Join(dir, defaultBigNet)
		smallPath := filepath.Join(dir, defaultSmallNet)

		// Check if both files exist
		if fileExists(bigPath) && fileExists(smallPath) {
			if err := eng.LoadNNUE(bigPath, smallPath); err != nil {
				log.Printf("Failed to load NNUE from %s: %v", dir, err)
				continue
			}
			eng.SetUseNNUE(true)
			log.Printf("NNUE loaded from %s", dir)
			return nil
		}
	}

	return os.ErrNotExist
}

// getAppSupportDir returns the application support directory for chessplay
func getAppSupportDir() string {
	home := getHomeDir()
	// macOS: ~/Library/Application Support/chessplay/nnue/
	return filepath.Join(home, "Library", "Application Support", "chessplay", "nnue")
}

// getHomeDir returns the user's home directory
func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
