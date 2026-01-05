package engine

import (
	"time"

	"github.com/hailam/chessplay/internal/board"
)

// UCILimits contains UCI time control parameters.
type UCILimits struct {
	Time      [2]time.Duration // wtime, btime (remaining time for each color)
	Inc       [2]time.Duration // winc, binc (increment per move)
	MovesToGo int              // moves until next time control (0 = sudden death)
	MoveTime  time.Duration    // fixed time per move (overrides other time controls)
	Depth     int              // maximum search depth
	Nodes     uint64           // maximum nodes to search
	Infinite  bool             // search until stopped
	Ponder    bool             // ponder mode
}

// TimeManager handles time allocation for searches.
type TimeManager struct {
	optimumTime time.Duration // Target time for this move
	maximumTime time.Duration // Maximum time allowed
	startTime   time.Time     // When search started
}

// NewTimeManager creates a new time manager.
func NewTimeManager() *TimeManager {
	return &TimeManager{}
}

// Init initializes the time manager for a new search.
// ply is the current game ply (half-move number).
func (tm *TimeManager) Init(limits UCILimits, us board.Color, ply int) {
	tm.startTime = time.Now()

	// Fixed move time mode
	if limits.MoveTime > 0 {
		tm.optimumTime = limits.MoveTime
		tm.maximumTime = limits.MoveTime
		return
	}

	// Infinite or depth-limited mode
	if limits.Infinite || (limits.Time[us] == 0 && limits.MoveTime == 0) {
		tm.optimumTime = time.Hour
		tm.maximumTime = time.Hour
		return
	}

	// Calculate time allocation based on remaining time and increment
	timeLeft := limits.Time[us]
	inc := limits.Inc[us]

	// Estimate moves to go
	mtg := limits.MovesToGo
	if mtg == 0 {
		// Sudden death: estimate moves remaining based on game phase
		// Early game: more moves expected, late game: fewer
		mtg = 50 - ply/4
		if mtg < 10 {
			mtg = 10
		}
		if mtg > 50 {
			mtg = 50
		}
	}

	// Base time per move (simple division)
	baseTime := timeLeft / time.Duration(mtg)

	// Add most of the increment
	baseTime += inc * 9 / 10

	// Use baseTime directly as the optimum
	// No aggressive scaling - we need time to search!
	tm.optimumTime = baseTime

	// Slight reduction for very early moves (give some buffer)
	if ply < 8 {
		tm.optimumTime = baseTime * 85 / 100
	}

	// Maximum time: 5x optimum or 80% of remaining, whichever is smaller
	maxFromOptimum := tm.optimumTime * 5
	maxFromRemaining := timeLeft * 8 / 10

	if maxFromOptimum < maxFromRemaining {
		tm.maximumTime = maxFromOptimum
	} else {
		tm.maximumTime = maxFromRemaining
	}

	// Safety margin: never use more than 95% of remaining time
	safetyMargin := timeLeft * 95 / 100
	if tm.maximumTime > safetyMargin {
		tm.maximumTime = safetyMargin
	}

	// Minimum times
	if tm.optimumTime < 10*time.Millisecond {
		tm.optimumTime = 10 * time.Millisecond
	}
	if tm.maximumTime < 50*time.Millisecond {
		tm.maximumTime = 50 * time.Millisecond
	}
}

// Elapsed returns the time elapsed since search started.
func (tm *TimeManager) Elapsed() time.Duration {
	return time.Since(tm.startTime)
}

// OptimumTime returns the target time for this move.
func (tm *TimeManager) OptimumTime() time.Duration {
	return tm.optimumTime
}

// MaximumTime returns the maximum time allowed.
func (tm *TimeManager) MaximumTime() time.Duration {
	return tm.maximumTime
}

// ShouldStop returns true if we should stop searching.
func (tm *TimeManager) ShouldStop() bool {
	return tm.Elapsed() >= tm.maximumTime
}

// PastOptimum returns true if we've exceeded the optimum time.
func (tm *TimeManager) PastOptimum() bool {
	return tm.Elapsed() >= tm.optimumTime
}

// AdjustForStability adjusts time allocation based on best move stability.
// If the best move hasn't changed for several depths, we can stop earlier.
// stability: number of consecutive depths with same best move
func (tm *TimeManager) AdjustForStability(stability int) {
	if stability >= 6 {
		// Very stable: use only 40% of optimum
		tm.optimumTime = tm.optimumTime * 40 / 100
	} else if stability >= 4 {
		// Stable: use only 60% of optimum
		tm.optimumTime = tm.optimumTime * 60 / 100
	} else if stability >= 2 {
		// Somewhat stable: use 80% of optimum
		tm.optimumTime = tm.optimumTime * 80 / 100
	}
}

// AdjustForInstability increases time when best move keeps changing.
// changes: number of best move changes in recent depths
func (tm *TimeManager) AdjustForInstability(changes int) {
	if changes >= 4 {
		// Very unstable: use 200% of optimum (up to maximum)
		tm.optimumTime = tm.optimumTime * 200 / 100
		if tm.optimumTime > tm.maximumTime {
			tm.optimumTime = tm.maximumTime
		}
	} else if changes >= 2 {
		// Unstable: use 150% of optimum
		tm.optimumTime = tm.optimumTime * 150 / 100
		if tm.optimumTime > tm.maximumTime {
			tm.optimumTime = tm.maximumTime
		}
	}
}
