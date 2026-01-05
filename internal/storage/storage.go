package storage

import (
	"encoding/json"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// Storage keys
const (
	keyPreferences = "preferences"
	keyStats       = "stats"
	keyFirstLaunch = "first_launch"
)

// EvalMode represents the evaluation engine mode
type EvalMode int

const (
	EvalClassical EvalMode = iota
	EvalNNUE
)

// GameMode represents the game mode
type GameMode int

const (
	ModeHumanVsHuman GameMode = iota
	ModeHumanVsComputer
)

// Difficulty represents AI difficulty level
type Difficulty int

const (
	DifficultyEasy Difficulty = iota
	DifficultyMedium
	DifficultyHard
)

// PlayerColor represents which color the human plays
type PlayerColor int

const (
	ColorWhite PlayerColor = iota
	ColorBlack
)

// UserPreferences stores user settings
type UserPreferences struct {
	Username     string      `json:"username"`
	Difficulty   Difficulty  `json:"difficulty"`
	GameMode     GameMode    `json:"game_mode"`
	EvalMode     EvalMode    `json:"eval_mode"`
	PlayerColor  PlayerColor `json:"player_color"`
	SoundEnabled bool        `json:"sound_enabled"`
	LastPlayed   time.Time   `json:"last_played"`
}

// DefaultPreferences returns default user preferences
func DefaultPreferences() *UserPreferences {
	return &UserPreferences{
		Username:     "Player",
		Difficulty:   DifficultyMedium,
		GameMode:     ModeHumanVsComputer,
		EvalMode:     EvalClassical,
		PlayerColor:  ColorWhite,
		SoundEnabled: true,
		LastPlayed:   time.Now(),
	}
}

// GameStats stores game statistics
type GameStats struct {
	GamesPlayed    int            `json:"games_played"`
	Wins           int            `json:"wins"`
	Losses         int            `json:"losses"`
	Draws          int            `json:"draws"`
	WinsByMode     map[string]int `json:"wins_by_mode"`
	WinsByDiff     map[string]int `json:"wins_by_difficulty"`
	TotalPlayTime  time.Duration  `json:"total_play_time"`
	LongestWinStrk int            `json:"longest_win_streak"`
	CurrentStreak  int            `json:"current_streak"`
}

// NewGameStats returns empty game statistics
func NewGameStats() *GameStats {
	return &GameStats{
		WinsByMode: make(map[string]int),
		WinsByDiff: make(map[string]int),
	}
}

// GameResult represents the result of a completed game
type GameResult struct {
	Won        bool
	Draw       bool
	Mode       GameMode
	Difficulty Difficulty
	EvalMode   EvalMode
	Duration   time.Duration
}

// Storage wraps BadgerDB for persistent storage
type Storage struct {
	db *badger.DB
}

// NewStorage creates a new storage instance
func NewStorage() (*Storage, error) {
	dbDir, err := GetDatabaseDir()
	if err != nil {
		return nil, err
	}

	opts := badger.DefaultOptions(dbDir)
	opts.Logger = nil // Disable logging

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &Storage{db: db}, nil
}

// Close closes the database
func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// IsFirstLaunch returns true if this is the first launch
func (s *Storage) IsFirstLaunch() (bool, error) {
	var firstLaunch bool = true

	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(keyFirstLaunch))
		if err == badger.ErrKeyNotFound {
			firstLaunch = true
			return nil
		}
		if err != nil {
			return err
		}
		firstLaunch = false
		return nil
	})

	return firstLaunch, err
}

// MarkFirstLaunchComplete marks that first launch setup is complete
func (s *Storage) MarkFirstLaunchComplete() error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(keyFirstLaunch), []byte("done"))
	})
}

// SavePreferences saves user preferences
func (s *Storage) SavePreferences(prefs *UserPreferences) error {
	prefs.LastPlayed = time.Now()

	data, err := json.Marshal(prefs)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(keyPreferences), data)
	})
}

// LoadPreferences loads user preferences, returns defaults if not found
func (s *Storage) LoadPreferences() (*UserPreferences, error) {
	prefs := DefaultPreferences()

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPreferences))
		if err == badger.ErrKeyNotFound {
			return nil // Use defaults
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, prefs)
		})
	})

	return prefs, err
}

// SaveStats saves game statistics
func (s *Storage) SaveStats(stats *GameStats) error {
	data, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(keyStats), data)
	})
}

// LoadStats loads game statistics, returns empty stats if not found
func (s *Storage) LoadStats() (*GameStats, error) {
	stats := NewGameStats()

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyStats))
		if err == badger.ErrKeyNotFound {
			return nil // Use empty stats
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, stats)
		})
	})

	return stats, err
}

// RecordGame records a completed game and updates statistics
func (s *Storage) RecordGame(result GameResult) error {
	stats, err := s.LoadStats()
	if err != nil {
		return err
	}

	stats.GamesPlayed++
	stats.TotalPlayTime += result.Duration

	// Mode key for stats
	modeKey := "hvh"
	if result.Mode == ModeHumanVsComputer {
		modeKey = "hvc"
	}

	// Difficulty key
	diffKey := "easy"
	switch result.Difficulty {
	case DifficultyMedium:
		diffKey = "medium"
	case DifficultyHard:
		diffKey = "hard"
	}

	if result.Draw {
		stats.Draws++
		stats.CurrentStreak = 0
	} else if result.Won {
		stats.Wins++
		stats.CurrentStreak++
		if stats.CurrentStreak > stats.LongestWinStrk {
			stats.LongestWinStrk = stats.CurrentStreak
		}
		stats.WinsByMode[modeKey]++
		stats.WinsByDiff[diffKey]++
	} else {
		stats.Losses++
		stats.CurrentStreak = 0
	}

	return s.SaveStats(stats)
}

// GetWinRate returns the win rate as a percentage (0-100)
func (s *GameStats) GetWinRate() float64 {
	if s.GamesPlayed == 0 {
		return 0
	}
	return float64(s.Wins) / float64(s.GamesPlayed) * 100
}
