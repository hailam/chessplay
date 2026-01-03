package engine

import (
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hailam/chessplay/internal/board"
	"github.com/hailam/chessplay/internal/book"
	"github.com/hailam/chessplay/internal/tablebase"
	"github.com/hailam/chessplay/sfnnue"
)

// NumWorkers is the number of parallel search workers (matches CPU cores).
var NumWorkers = runtime.GOMAXPROCS(0)

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
	MultiPV  int           // Number of principal variations to find (0 or 1 = single best move)
}

// SearchResult contains the result of a single PV search.
type SearchResult struct {
	Move  board.Move
	Score int
	PV    []board.Move
	Depth int
}

// Difficulty represents the AI difficulty level.
type Difficulty int

const (
	Easy   Difficulty = iota // ~2-3 ply, 500ms
	Medium                   // ~4-5 ply, 2s
	Hard                     // Maximum strength, 10s
)

// DifficultySettings maps difficulty to search limits.
var DifficultySettings = map[Difficulty]SearchLimits{
	Easy:   {Depth: 3, MoveTime: 500 * time.Millisecond},
	Medium: {Depth: 7, MoveTime: 1 * time.Second},
	Hard:   {Depth: 40, MoveTime: 3 * time.Second}, // Max strength (time-limited)
}

// Engine is the chess AI engine.
type Engine struct {
	// Workers for parallel search
	workers       []*Worker
	pawnTable     *PawnTable
	tt            *TranspositionTable
	sharedHistory *SharedHistory // Shared history for Lazy SMP
	stopFlag      atomic.Bool

	// Legacy single-threaded searcher (for Multi-PV compatibility)
	searcher *Searcher

	difficulty Difficulty
	book       *book.Book
	tablebase  tablebase.Prober

	// Position history for repetition detection
	rootPosHashes []uint64

	// NNUE evaluation
	useNNUE bool
	nnueNet *sfnnue.Networks // Shared networks (immutable after load)

	// Callbacks
	OnInfo func(SearchInfo)
}

// NewEngine creates a new chess engine with the given transposition table size in MB.
func NewEngine(ttSizeMB int) *Engine {
	tt := NewTranspositionTable(ttSizeMB)
	sharedHistory := NewSharedHistory()

	e := &Engine{
		tt:            tt,
		pawnTable:     NewPawnTable(1), // Shared pawn table for legacy searcher
		sharedHistory: sharedHistory,
		difficulty:    Medium,
		workers:       make([]*Worker, NumWorkers),
	}

	log.Printf("[Engine] Creating %d workers (GOMAXPROCS=%d)", NumWorkers, runtime.GOMAXPROCS(0))

	// Create workers, each with its own pawn table for thread safety
	for i := 0; i < NumWorkers; i++ {
		workerPawnTable := NewPawnTable(1) // 1MB per worker
		e.workers[i] = NewWorker(i, tt, workerPawnTable, sharedHistory, &e.stopFlag)
	}

	// Create legacy searcher for Multi-PV
	e.searcher = NewSearcher(tt)

	return e
}

// SetDifficulty sets the engine difficulty.
func (e *Engine) SetDifficulty(d Difficulty) {
	e.difficulty = d
}

// LoadBook loads an opening book from a Polyglot file.
func (e *Engine) LoadBook(filename string) error {
	b, err := book.LoadPolyglot(filename)
	if err != nil {
		return err
	}
	e.book = b
	return nil
}

// SetBook sets the opening book.
func (e *Engine) SetBook(b *book.Book) {
	e.book = b
}

// HasBook returns true if an opening book is loaded.
func (e *Engine) HasBook() bool {
	return e.book != nil
}

// SetTablebase sets the tablebase prober.
func (e *Engine) SetTablebase(tb tablebase.Prober) {
	e.tablebase = tb
}

// EnableLichessTablebase enables Lichess online tablebase lookups.
func (e *Engine) EnableLichessTablebase() {
	e.tablebase = tablebase.NewLichessProber()
}

// HasTablebase returns true if a tablebase is available.
func (e *Engine) HasTablebase() bool {
	return e.tablebase != nil && e.tablebase.Available()
}

// SetPositionHistory sets the position history for repetition detection.
// This should be called before Search() with hashes from the game's move history.
func (e *Engine) SetPositionHistory(hashes []uint64) {
	e.rootPosHashes = make([]uint64, len(hashes))
	copy(e.rootPosHashes, hashes)

	// Set for all workers
	for _, w := range e.workers {
		w.SetRootHistory(hashes)
	}

	// Set for legacy searcher
	e.searcher.SetRootHistory(hashes)
}

// Search finds the best move for the given position.
func (e *Engine) Search(pos *board.Position) board.Move {
	limits := DifficultySettings[e.difficulty]
	return e.SearchWithLimits(pos, limits)
}

// SearchWithLimits finds the best move with specific search limits.
// Uses Lazy SMP with multiple workers searching in parallel.
func (e *Engine) SearchWithLimits(pos *board.Position, limits SearchLimits) board.Move {
	log.Printf("[Search] Received position with SideToMove=%v", pos.SideToMove)

	// Try opening book first
	if e.book != nil {
		if move, ok := e.book.Probe(pos); ok {
			return move
		}
	}
	log.Printf("[Search] After book probe SideToMove=%v", pos.SideToMove)

	// Try tablebase for endgames
	if e.tablebase != nil && e.tablebase.Available() {
		pieceCount := tablebase.CountPieces(pos)
		if pieceCount <= e.tablebase.MaxPieces() {
			result := e.tablebase.ProbeRoot(pos)
			if result.Found && result.Move != board.NoMove {
				return result.Move
			}
		}
	}
	log.Printf("[Search] After tablebase probe SideToMove=%v", pos.SideToMove)

	// Reset for new search
	e.stopFlag.Store(false)
	e.tt.NewSearch()

	// Log evaluation mode
	if e.useNNUE && e.nnueNet != nil {
		log.Printf("[Engine] Starting search with NNUE evaluation")
	} else {
		log.Printf("[Engine] Starting search with Classical evaluation")
	}

	// Reset all workers
	for _, w := range e.workers {
		w.Reset()
	}

	startTime := time.Now()
	var bestMove board.Move
	var bestScore int
	var bestPV []board.Move
	var bestDepth int

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

	// Create result channel
	resultCh := make(chan WorkerResult, NumWorkers*maxDepth)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < NumWorkers; i++ {
		wg.Add(1)
		go e.workerSearch(i, pos, maxDepth, resultCh, &wg)
	}

	// Collect results in a separate goroutine
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(resultCh)
		close(done)
	}()

	// Track nodes across all workers
	var totalNodes uint64

	// Process results
resultLoop:
	for {
		select {
		case result, ok := <-resultCh:
			if !ok {
				break resultLoop
			}

			// Update total nodes
			totalNodes += result.Nodes

			// Update best result if this is deeper or same depth with better score
			if result.Move != board.NoMove {
				if result.Depth > bestDepth ||
					(result.Depth == bestDepth && result.Score > bestScore) {
					bestMove = result.Move
					bestScore = result.Score
					bestPV = result.PV
					bestDepth = result.Depth

					// Report info
					if e.OnInfo != nil {
						elapsed := time.Since(startTime)
						e.OnInfo(SearchInfo{
							Depth:    bestDepth,
							Score:    bestScore,
							Nodes:    e.getTotalNodes(),
							Time:     elapsed,
							PV:       bestPV,
							HashFull: e.tt.HashFull(),
						})
					}

					// Early termination: found mate
					if bestScore > MateScore-100 || bestScore < -MateScore+100 {
						e.stopFlag.Store(true)
						break resultLoop
					}
				}
			}

			// Check time limit
			if !deadline.IsZero() && time.Now().After(deadline) {
				e.stopFlag.Store(true)
				break resultLoop
			}

		case <-done:
			break resultLoop
		}
	}

	// Ensure all workers are stopped
	e.stopFlag.Store(true)

	// Wait for workers to finish
	<-done

	return bestMove
}

// SearchWithUCILimits finds the best move using UCI time controls.
// Supports wtime/btime/winc/binc for proper tournament time management.
func (e *Engine) SearchWithUCILimits(pos *board.Position, limits UCILimits, ply int) board.Move {
	// Try opening book first
	if e.book != nil {
		if move, ok := e.book.Probe(pos); ok {
			return move
		}
	}

	// Try tablebase for endgames
	if e.tablebase != nil && e.tablebase.Available() {
		pieceCount := tablebase.CountPieces(pos)
		if pieceCount <= e.tablebase.MaxPieces() {
			result := e.tablebase.ProbeRoot(pos)
			if result.Found && result.Move != board.NoMove {
				return result.Move
			}
		}
	}

	// Initialize time manager
	tm := NewTimeManager()
	tm.Init(limits, pos.SideToMove, ply)

	// Reset for new search
	e.stopFlag.Store(false)
	e.tt.NewSearch()

	// Reset all workers
	for _, w := range e.workers {
		w.Reset()
	}

	startTime := time.Now()
	var bestMove board.Move
	var bestScore int
	var bestPV []board.Move
	var bestDepth int
	var lastBestMove board.Move
	var stabilityCount int
	var instabilityCount int

	// Determine maximum depth
	maxDepth := MaxPly
	if limits.Depth > 0 {
		maxDepth = limits.Depth
	}

	// Create result channel
	resultCh := make(chan WorkerResult, NumWorkers*maxDepth)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < NumWorkers; i++ {
		wg.Add(1)
		go e.workerSearch(i, pos, maxDepth, resultCh, &wg)
	}

	// Collect results in a separate goroutine
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(resultCh)
		close(done)
	}()

	// Process results
resultLoop:
	for {
		select {
		case result, ok := <-resultCh:
			if !ok {
				break resultLoop
			}

			// Update best result if this is deeper or same depth with better score
			if result.Move != board.NoMove {
				if result.Depth > bestDepth ||
					(result.Depth == bestDepth && result.Score > bestScore) {

					// Track move stability
					if result.Depth > bestDepth {
						if result.Move == lastBestMove {
							stabilityCount++
							instabilityCount = 0
						} else {
							instabilityCount++
							stabilityCount = 0
						}
						lastBestMove = result.Move
					}

					bestMove = result.Move
					bestScore = result.Score
					bestPV = result.PV
					bestDepth = result.Depth

					// Report info
					if e.OnInfo != nil {
						elapsed := time.Since(startTime)
						e.OnInfo(SearchInfo{
							Depth:    bestDepth,
							Score:    bestScore,
							Nodes:    e.getTotalNodes(),
							Time:     elapsed,
							PV:       bestPV,
							HashFull: e.tt.HashFull(),
						})
					}

					// Early termination: found mate
					if bestScore > MateScore-100 || bestScore < -MateScore+100 {
						e.stopFlag.Store(true)
						break resultLoop
					}

					// Time management: check if we should stop based on stability
					if tm.PastOptimum() {
						if stabilityCount >= 4 {
							// Move is very stable, stop early
							e.stopFlag.Store(true)
							break resultLoop
						}
					}
				}
			}

			// Check time limit
			if tm.ShouldStop() {
				e.stopFlag.Store(true)
				break resultLoop
			}

			// Node limit check
			if limits.Nodes > 0 && e.getTotalNodes() >= limits.Nodes {
				e.stopFlag.Store(true)
				break resultLoop
			}

		case <-done:
			break resultLoop
		}
	}

	// Ensure all workers are stopped
	e.stopFlag.Store(true)
	<-done

	return bestMove
}

// workerSearch runs iterative deepening search in a worker goroutine.
// Uses depth staggering: workers start at different depths to reduce redundant shallow work.
func (e *Engine) workerSearch(workerID int, pos *board.Position, maxDepth int, resultCh chan<- WorkerResult, wg *sync.WaitGroup) {
	defer wg.Done()

	worker := e.workers[workerID]
	worker.InitSearch(pos)

	var prevScore int

	// Depth staggering: helper workers skip shallow depths
	// Worker 0 (main): starts at depth 1
	// Workers 1-2: start at depth 2
	// Workers 3-5: start at depth 3
	// Workers 6+: start at depth 4
	startDepth := 1
	if workerID >= 6 {
		startDepth = 4
	} else if workerID >= 3 {
		startDepth = 3
	} else if workerID >= 1 {
		startDepth = 2
	}

	// Track recent scores for volatility calculation
	recentScores := make([]int, 0, 10)

	for depth := startDepth; depth <= maxDepth; depth++ {
		if e.stopFlag.Load() {
			return
		}

		var move board.Move
		var score int

		// Use dynamic aspiration windows after depth 4
		// Window size adapts based on score volatility
		if depth >= 5 && prevScore != 0 {
			// Calculate volatility from recent scores
			volatility := 0
			if len(recentScores) >= 2 {
				minScore, maxScore := recentScores[0], recentScores[0]
				for _, s := range recentScores {
					if s < minScore {
						minScore = s
					}
					if s > maxScore {
						maxScore = s
					}
				}
				volatility = maxScore - minScore
			}

			// Dynamic window size based on volatility
			var window int
			if volatility > 400 {
				// High volatility (tactical position): use wider window
				window = 150 + volatility/4
			} else if volatility < 50 {
				// Stable position: use tight window
				window = 25
			} else {
				// Normal: moderate window
				window = 50 + volatility/8
			}

			// Add worker-specific variation for search diversity
			window += (workerID % 8) * 3

			alpha := prevScore - window
			beta := prevScore + window
			retryCount := 0

			for {
				move, score = worker.SearchDepth(depth, alpha, beta)

				if e.stopFlag.Load() {
					return
				}

				if score <= alpha {
					// Failed low: gradually expand alpha
					retryCount++
					if retryCount >= 2 {
						alpha = -Infinity
					} else {
						alpha = prevScore - window*2
					}
				} else if score >= beta {
					// Failed high: gradually expand beta
					retryCount++
					if retryCount >= 2 {
						beta = Infinity
					} else {
						beta = prevScore + window*2
					}
				} else {
					break
				}

				if alpha == -Infinity && beta == Infinity {
					break
				}
			}
		} else {
			move, score = worker.SearchDepth(depth, -Infinity, Infinity)
		}

		if e.stopFlag.Load() {
			return
		}

		prevScore = score

		// Track score for volatility calculation
		recentScores = append(recentScores, score)
		if len(recentScores) > 10 {
			recentScores = recentScores[1:] // Keep last 10 scores
		}

		// Send result
		pv := worker.GetPV()
		resultCh <- WorkerResult{
			WorkerID: workerID,
			Depth:    depth,
			Score:    score,
			Move:     move,
			PV:       pv,
			Nodes:    worker.Nodes(),
		}
	}
}

// getTotalNodes returns the total nodes searched by all workers.
func (e *Engine) getTotalNodes() uint64 {
	var total uint64
	for _, w := range e.workers {
		total += w.Nodes()
	}
	return total
}

// SearchMultiPV finds multiple best moves (principal variations) for analysis.
func (e *Engine) SearchMultiPV(pos *board.Position, limits SearchLimits) []SearchResult {
	numPV := limits.MultiPV
	if numPV <= 0 {
		numPV = 1
	}

	results := make([]SearchResult, 0, numPV)
	excludedMoves := make([]board.Move, 0, numPV)

	for i := 0; i < numPV; i++ {
		// Search excluding already-found best moves
		move, score, pv, depth := e.searchWithExclusions(pos, limits, excludedMoves)
		if move == board.NoMove {
			break
		}

		results = append(results, SearchResult{
			Move:  move,
			Score: score,
			PV:    pv,
			Depth: depth,
		})
		excludedMoves = append(excludedMoves, move)
	}

	// Sort results by score (descending) to ensure best moves are first
	for i := 0; i < len(results)-1; i++ {
		maxIdx := i
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[maxIdx].Score {
				maxIdx = j
			}
		}
		if maxIdx != i {
			results[i], results[maxIdx] = results[maxIdx], results[i]
		}
	}

	return results
}

// searchWithExclusions searches for best move excluding certain moves at the root.
func (e *Engine) searchWithExclusions(pos *board.Position, limits SearchLimits, excluded []board.Move) (board.Move, int, []board.Move, int) {
	e.searcher.Reset()
	e.searcher.SetExcludedMoves(excluded)
	e.tt.NewSearch()

	startTime := time.Now()
	var bestMove board.Move
	var bestScore int
	var bestDepth int

	maxDepth := MaxPly
	if limits.Depth > 0 {
		maxDepth = limits.Depth
	}

	var deadline time.Time
	if limits.MoveTime > 0 {
		deadline = startTime.Add(limits.MoveTime)
	}

	for depth := 1; depth <= maxDepth; depth++ {
		if !deadline.IsZero() && time.Now().After(deadline) {
			break
		}

		move, score := e.searcher.Search(pos, depth)

		if e.searcher.IsStopped() {
			break
		}

		if move != board.NoMove {
			bestMove = move
			bestScore = score
			bestDepth = depth
		}

		if score > MateScore-100 || score < -MateScore+100 {
			break
		}

		if !deadline.IsZero() {
			elapsed := time.Since(startTime)
			remaining := limits.MoveTime - elapsed
			if remaining < elapsed {
				break
			}
		}
	}

	pv := e.searcher.GetPV()
	e.searcher.SetExcludedMoves(nil) // Clear exclusions

	return bestMove, bestScore, pv, bestDepth
}

// Stop stops the current search.
func (e *Engine) Stop() {
	e.stopFlag.Store(true)
	e.searcher.Stop()
}

// Clear clears the transposition table and other caches.
func (e *Engine) Clear() {
	e.tt.Clear()
	// Clear all worker orderers
	for _, w := range e.workers {
		w.orderer.Clear()
	}
	e.searcher.ClearOrderer()
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

// LoadNNUE loads NNUE network files.
func (e *Engine) LoadNNUE(bigPath, smallPath string) error {
	log.Printf("[Engine] Loading NNUE networks...")
	log.Printf("[Engine]   Big network: %s", bigPath)
	log.Printf("[Engine]   Small network: %s", smallPath)

	nets, err := sfnnue.LoadNetworks(bigPath, smallPath)
	if err != nil {
		log.Printf("[Engine] Failed to load NNUE: %v", err)
		return err
	}
	e.nnueNet = nets

	// Initialize NNUE evaluators for all workers
	for _, w := range e.workers {
		w.initNNUE(nets)
	}

	// Initialize for legacy searcher
	e.searcher.worker.initNNUE(nets)

	log.Printf("[Engine] NNUE networks loaded successfully")
	return nil
}

// SetUseNNUE enables or disables NNUE evaluation.
func (e *Engine) SetUseNNUE(use bool) {
	e.useNNUE = use
	for _, w := range e.workers {
		w.useNNUE = use
	}
	e.searcher.worker.useNNUE = use

	if use {
		log.Printf("[Engine] Evaluation mode: NNUE")
	} else {
		log.Printf("[Engine] Evaluation mode: Classical")
	}
}

// UseNNUE returns whether NNUE evaluation is enabled.
func (e *Engine) UseNNUE() bool {
	return e.useNNUE
}

// HasNNUE returns whether NNUE networks are loaded.
func (e *Engine) HasNNUE() bool {
	return e.nnueNet != nil
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
