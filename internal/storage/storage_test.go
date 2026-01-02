package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorage(t *testing.T) {
	// Use temp directory for test
	tmpDir, err := os.MkdirTemp("", "chessplay-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override the data dir for testing
	dbDir := filepath.Join(tmpDir, "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("Failed to create db dir: %v", err)
	}

	// We can't easily test with the real GetDatabaseDir, so we'll test the structs directly
	t.Run("DefaultPreferences", func(t *testing.T) {
		prefs := DefaultPreferences()
		if prefs.Username != "Player" {
			t.Errorf("Expected username 'Player', got '%s'", prefs.Username)
		}
		if prefs.Difficulty != DifficultyMedium {
			t.Errorf("Expected medium difficulty")
		}
		if prefs.EvalMode != EvalClassical {
			t.Errorf("Expected classical eval mode")
		}
		if !prefs.SoundEnabled {
			t.Errorf("Expected sound enabled by default")
		}
	})

	t.Run("NewGameStats", func(t *testing.T) {
		stats := NewGameStats()
		if stats.GamesPlayed != 0 {
			t.Errorf("Expected 0 games played")
		}
		if stats.GetWinRate() != 0 {
			t.Errorf("Expected 0 win rate")
		}
	})

	t.Run("WinRate", func(t *testing.T) {
		stats := &GameStats{
			GamesPlayed: 10,
			Wins:        5,
			Losses:      3,
			Draws:       2,
		}
		rate := stats.GetWinRate()
		if rate != 50 {
			t.Errorf("Expected 50%% win rate, got %.2f%%", rate)
		}
	})
}

func TestDataPaths(t *testing.T) {
	// Test that GetDataDir returns a valid path
	dataDir, err := GetDataDir()
	if err != nil {
		t.Fatalf("GetDataDir failed: %v", err)
	}
	if dataDir == "" {
		t.Error("GetDataDir returned empty path")
	}

	// Verify directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Errorf("Data directory was not created: %s", dataDir)
	}

	t.Logf("Data directory: %s", dataDir)
}
