package engine

import (
	"time"

	"github.com/hailam/chessplay/internal/board"
)

// SearchInfo contains information about the current search.
type SearchInfo struct {
	Depth    int
	Score    int
	Nodes    uint64
	Time     time.Duration
	PV       []board.Move
	HashFull int // Permille of hash table used
}

// SearchLimits specifies constraints on the search.
type SearchLimits struct {
	Depth    int           // Maximum depth (0 = no limit)
	Nodes    uint64        // Maximum nodes (0 = no limit)
	MoveTime time.Duration // Time for this move (0 = no limit)
	Infinite bool          // Search until stopped
}

// Difficulty represents the AI difficulty level.
type Difficulty int

const (
	Easy   Difficulty = iota // ~2-3 ply, 500ms
	Medium                   // ~4-5 ply, 2s
	Hard                     // ~6+ ply, 5s
)

// DifficultySettings maps difficulty to search limits.
var DifficultySettings = map[Difficulty]SearchLimits{
	Easy:   {Depth: 3, MoveTime: 500 * time.Millisecond},
	Medium: {Depth: 5, MoveTime: 2 * time.Second},
	Hard:   {Depth: 7, MoveTime: 5 * time.Second},
}

// Engine is the chess AI engine.
type Engine struct {
	searcher   *Searcher
	tt         *TranspositionTable
	difficulty Difficulty

	// Callbacks
	OnInfo func(SearchInfo)
}

// NewEngine creates a new chess engine with the given transposition table size in MB.
func NewEngine(ttSizeMB int) *Engine {
	tt := NewTranspositionTable(ttSizeMB)
	return &Engine{
		searcher:   NewSearcher(tt),
		tt:         tt,
		difficulty: Medium,
	}
}

// SetDifficulty sets the engine difficulty.
func (e *Engine) SetDifficulty(d Difficulty) {
	e.difficulty = d
}

// Search finds the best move for the given position.
func (e *Engine) Search(pos *board.Position) board.Move {
	limits := DifficultySettings[e.difficulty]
	return e.SearchWithLimits(pos, limits)
}

// SearchWithLimits finds the best move with specific search limits.
func (e *Engine) SearchWithLimits(pos *board.Position, limits SearchLimits) board.Move {
	e.searcher.Reset()
	e.tt.NewSearch()

	startTime := time.Now()
	var bestMove board.Move
	var bestScore int

	// Determine maximum depth
	maxDepth := MaxPly
	if limits.Depth > 0 {
		maxDepth = limits.Depth
	}

	// Determine deadline
	var deadline time.Time
	if limits.MoveTime > 0 {
		deadline = startTime.Add(limits.MoveTime)
	}

	// Aspiration window parameters
	const initialWindow = 50 // Start with ±50 centipawns

	// Iterative deepening
	for depth := 1; depth <= maxDepth; depth++ {
		// Check time before starting new iteration
		if !deadline.IsZero() && time.Now().After(deadline) {
			break
		}

		var move board.Move
		var score int

		// Use aspiration windows after depth 4 and when we have a previous score
		if depth >= 5 && bestMove != board.NoMove {
			window := initialWindow
			alpha := bestScore - window
			beta := bestScore + window

			// Aspiration window search with widening
			for {
				move, score = e.searcher.SearchWithBounds(pos, depth, alpha, beta)

				// Check if search was stopped
				if e.searcher.stopFlag.Load() {
					break
				}

				if score <= alpha {
					// Fail low - widen window down
					alpha = -Infinity
				} else if score >= beta {
					// Fail high - widen window up
					beta = Infinity
				} else {
					// Score within window, we're done
					break
				}

				// If both bounds are infinite, we've done a full search
				if alpha == -Infinity && beta == Infinity {
					break
				}
			}
		} else {
			// Full window search for early depths
			move, score = e.searcher.Search(pos, depth)
		}

		// Check if search was stopped
		if e.searcher.stopFlag.Load() {
			break
		}

		// Update best move
		if move != board.NoMove {
			bestMove = move
			bestScore = score
		}

		// Report info
		if e.OnInfo != nil {
			elapsed := time.Since(startTime)
			e.OnInfo(SearchInfo{
				Depth:    depth,
				Score:    bestScore,
				Nodes:    e.searcher.Nodes(),
				Time:     elapsed,
				PV:       e.searcher.GetPV(),
				HashFull: e.tt.HashFull(),
			})
		}

		// Early termination: found mate
		if score > MateScore-100 || score < -MateScore+100 {
			break
		}

		// Check time after iteration
		if !deadline.IsZero() {
			elapsed := time.Since(startTime)
			remaining := limits.MoveTime - elapsed

			// If we've used more than half the time, don't start another iteration
			if remaining < elapsed {
				break
			}
		}
	}

	return bestMove
}

// Stop stops the current search.
func (e *Engine) Stop() {
	e.searcher.Stop()
}

// Clear clears the transposition table and other caches.
func (e *Engine) Clear() {
	e.tt.Clear()
	e.searcher.orderer.Clear()
}

// Perft performs a perft test (for debugging move generation).
func (e *Engine) Perft(pos *board.Position, depth int) uint64 {
	if depth == 0 {
		return 1
	}

	moves := pos.GenerateLegalMoves()
	if depth == 1 {
		return uint64(moves.Len())
	}

	var nodes uint64
	for i := 0; i < moves.Len(); i++ {
		move := moves.Get(i)
		undo := pos.MakeMove(move)
		nodes += e.Perft(pos, depth-1)
		pos.UnmakeMove(move, undo)
	}

	return nodes
}

// Evaluate returns the static evaluation of a position.
func (e *Engine) Evaluate(pos *board.Position) int {
	return Evaluate(pos)
}

// ScoreToString converts a score to a human-readable string.
func ScoreToString(score int) string {
	if score > MateScore-100 {
		mateIn := (MateScore - score + 1) / 2
		return "Mate in " + itoa(mateIn)
	}
	if score < -MateScore+100 {
		mateIn := (MateScore + score + 1) / 2
		return "Mated in " + itoa(mateIn)
	}

	// Convert centipawns to pawns
	sign := ""
	if score < 0 {
		sign = "-"
		score = -score
	}
	pawns := score / 100
	centipawns := score % 100

	return sign + itoa(pawns) + "." + itoa(centipawns)
}

// Simple integer to string (avoid fmt import)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	s := ""
	for n > 0 {
		s = string('0'+byte(n%10)) + s
		n /= 10
	}
	return s
}
