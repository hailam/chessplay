package engine

import (
	"sync/atomic"

	"github.com/hailam/chessplay/internal/board"
)

// Search constants
const (
	Infinity  = 30000
	MateScore = 29000
	MaxPly    = 128
)

// Pruning constants
const (
	lazyEvalMargin           = 150   // Lazy eval margin for quiescence
	historyPruningThreshold  = -4000 // History pruning threshold
	probcutDepth             = 8     // Minimum depth for probcut
	probcutMargin            = 200   // Probcut margin above beta
	probcutReduction         = 4     // Probcut depth reduction
	multicutDepth            = 8     // Minimum depth for multi-cut
	multicutMoves            = 6     // Number of moves to try
	multicutRequired         = 3     // Number of cutoffs needed
)

// LMP (Late Move Pruning) thresholds by depth
// At depth d, prune quiet moves after lmpThreshold[d] moves
var lmpThreshold = [8]int{0, 3, 5, 9, 15, 23, 33, 45}

// Threat extension constants
const (
	threatExtensionMinDepth  = 4   // Minimum depth to consider threat extensions
	threatExtensionThreshold = 200 // Minimum material value to trigger extension (Knight/Bishop value)
)

// PVTable stores the principal variation.
type PVTable struct {
	length [MaxPly]int
	moves  [MaxPly][MaxPly]board.Move
}

// Searcher performs the alpha-beta search.
// It wraps a single Worker for backwards compatibility.
type Searcher struct {
	worker   *Worker
	stopFlag atomic.Bool
}

// NewSearcher creates a new searcher.
func NewSearcher(tt *TranspositionTable) *Searcher {
	pawnTable := NewPawnTable(1)       // 1MB pawn hash table
	sharedHistory := NewSharedHistory() // Own history for single-threaded search
	s := &Searcher{}
	s.worker = NewWorker(0, tt, pawnTable, sharedHistory, &s.stopFlag)
	return s
}

// Stop signals the search to stop.
func (s *Searcher) Stop() {
	s.stopFlag.Store(true)
}

// Reset resets the searcher for a new search.
func (s *Searcher) Reset() {
	s.stopFlag.Store(false)
	s.worker.Reset()
}

// Nodes returns the number of nodes searched.
func (s *Searcher) Nodes() uint64 {
	return s.worker.Nodes()
}

// Search performs the search at the given depth.
func (s *Searcher) Search(pos *board.Position, depth int) (board.Move, int) {
	return s.SearchWithBounds(pos, depth, -Infinity, Infinity)
}

// SetRootHistory sets the position history from the game (for repetition detection).
// This should be called before Search() with hashes from the game's move history.
func (s *Searcher) SetRootHistory(hashes []uint64) {
	s.worker.SetRootHistory(hashes)
}

// SetExcludedMoves sets the moves to exclude at root (for Multi-PV).
func (s *Searcher) SetExcludedMoves(moves []board.Move) {
	s.worker.SetExcludedMoves(moves)
}

// SearchWithBounds performs search with custom alpha/beta bounds (for aspiration windows).
func (s *Searcher) SearchWithBounds(pos *board.Position, depth, alpha, beta int) (board.Move, int) {
	s.worker.InitSearch(pos)
	return s.worker.SearchDepth(depth, alpha, beta)
}

// GetPV returns the principal variation from the last search.
func (s *Searcher) GetPV() []board.Move {
	return s.worker.GetPV()
}

// ClearOrderer clears the move orderer state.
func (s *Searcher) ClearOrderer() {
	s.worker.orderer.Clear()
}

// IsStopped returns true if the search has been stopped.
func (s *Searcher) IsStopped() bool {
	return s.stopFlag.Load()
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
