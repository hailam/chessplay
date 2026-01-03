// Package storage provides persistent storage for user preferences and game statistics.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const appName = "chessplay"

// GetDataDir returns the platform-specific data directory for the application.
// - macOS: ~/Library/Application Support/chessplay/
// - Linux: ~/.local/share/chessplay/
// - Windows: %APPDATA%/chessplay/
func GetDataDir() (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "darwin":
		// macOS: ~/Library/Application Support/
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(homeDir, "Library", "Application Support")

	case "windows":
		// Windows: %APPDATA%
		baseDir = os.Getenv("APPDATA")
		if baseDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			baseDir = filepath.Join(homeDir, "AppData", "Roaming")
		}

	default:
		// Linux and other Unix-like: ~/.local/share/
		// Check XDG_DATA_HOME first
		baseDir = os.Getenv("XDG_DATA_HOME")
		if baseDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			baseDir = filepath.Join(homeDir, ".local", "share")
		}
	}

	dataDir := filepath.Join(baseDir, appName)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", err
	}

	return dataDir, nil
}

// GetNNUEDir returns the directory for storing NNUE network files.
func GetNNUEDir() (string, error) {
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}

	nnueDir := filepath.Join(dataDir, "nnue")
	if err := os.MkdirAll(nnueDir, 0755); err != nil {
		return "", err
	}

	return nnueDir, nil
}

// GetDatabaseDir returns the directory for storing the BadgerDB database.
func GetDatabaseDir() (string, error) {
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}

	dbDir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return "", err
	}

	fmt.Printf("Database directory: %s\n", dbDir)

	return dbDir, nil
}
