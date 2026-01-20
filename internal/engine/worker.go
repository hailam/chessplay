package engine

import (
	"log"
	"math"
	"sync/atomic"

	"github.com/hailam/chessplay/internal/board"
	"github.com/hailam/chessplay/internal/tablebase"
	"github.com/hailam/chessplay/sfnnue"
)

// LMR reduction table - precomputed logarithmic reductions
// Based on Stockfish's formula: 21.46 * log(depth) * log(moveCount) / 1024
var lmrReductions [64][64]int

func init() {
	for d := 1; d < 64; d++ {
		for m := 1; m < 64; m++ {
			// Stockfish-like formula
			lmrReductions[d][m] = int(21.46 * math.Log(float64(d)) * math.Log(float64(m)) / 1024.0)
		}
	}
}

// SearchStack stores per-ply search state for continuation history tracking.
// Ported from Stockfish's Stack structure.
type SearchStack struct {
	// Current move at this ply
	currentMove board.Move

	// Piece that moved at this ply
	movedPiece board.Piece

	// Destination square of the move
	moveTo board.Square

	// Pointer to continuation history table for this move's piece/to
	// Used by child nodes to look up move patterns
	continuationHistory *PieceToHistory

	// Statistical score for history-based decisions
	statScore int

	// Reduction applied at this ply (for hindsight depth adjustment)
	reduction int

	// Count of beta cutoffs at this ply (for LMR scaling)
	cutoffCnt int
}

// Worker represents a search worker for parallel Lazy SMP search.
// Each worker has its own state but shares the transposition table and history.
type Worker struct {
	id int

	// Per-worker position copy
	pos *board.Position

	// Per-worker move ordering (killers stay local, history shared)
	orderer *MoveOrderer

	// Per-worker search state
	nodes uint64
	pv    PVTable

	// Per-worker stacks
	undoStack   [MaxPly]board.UndoInfo
	evalStack   [MaxPly]int
	searchStack [MaxPly]SearchStack // For continuation history tracking

	// Per-worker position history for repetition detection
	// Pre-allocated buffer avoids allocation per move in negamax
	// Size: MaxPly (128) + 640 for root history = 768
	posHistoryBuffer [768]uint64
	posHistoryLen    int
	rootPosHashes    []uint64

	// Multi-PV support: moves to exclude at root
	excludedRootMoves []board.Move

	// Shared resources (pointers to engine's shared state)
	tt            *TranspositionTable
	pawnTable     *PawnTable
	sharedHistory *SharedHistory    // Shared history for Lazy SMP
	corrHistory   *CorrectionHistory // Correction history for eval adjustment
	stopFlag      *atomic.Bool

	// NNUE evaluation (per-worker for thread safety)
	useNNUE  bool
	nnueNet  *sfnnue.Networks
	nnueAcc  *sfnnue.AccumulatorStack

	// Pre-allocated buffer for active feature indices (avoids allocation per computeAccumulator call)
	// Max 32 pieces on the board, but features can have more indices due to king-relative positions
	activeIndicesBuffer [64]int

	// Dirty piece tracking for incremental NNUE updates
	dirtyState DirtyState

	// Tablebase probing
	tbProber   tablebase.Prober
	tbProbeDepth int // Minimum depth to probe TB (default: 1)

	// Debug mode
	debug bool

	// Communication channel for results
	resultCh chan<- WorkerResult

	// Current search depth (for result reporting)
	depth int

	// Optimism tracking (Stockfish evaluate.cpp)
	// Used for material scaling: includes optimism term based on running average of root scores
	optimism [2]int // Per-side optimism: [White=0, Black=1]
	avgScore int    // Running average of root move score (initialized to -Infinity)

	// Root delta for LMR scaling (Stockfish search.cpp)
	// Width of the initial aspiration window at root, used to scale reductions
	rootDelta int

	// NMP verification: minimum ply where NMP is allowed (Stockfish search.cpp:892-925)
	// When set > 0, NMP is disabled until ply exceeds this value
	nmpMinPly int
}

// WorkerResult contains the result from a worker's search at a given depth.
type WorkerResult struct {
	WorkerID int
	Depth    int
	Score    int
	Move     board.Move
	PV       []board.Move
	Nodes    uint64
}

// NewWorker creates a new search worker.
func NewWorker(id int, tt *TranspositionTable, pawnTable *PawnTable, sharedHistory *SharedHistory, stopFlag *atomic.Bool) *Worker {
	return &Worker{
		id:            id,
		orderer:       NewMoveOrderer(),
		tt:            tt,
		pawnTable:     pawnTable,
		sharedHistory: sharedHistory,
		corrHistory:   NewCorrectionHistory(),
		stopFlag:      stopFlag,
	}
}

// initNNUE initializes NNUE evaluation for this worker.
func (w *Worker) initNNUE(nets *sfnnue.Networks) {
	w.nnueNet = nets
	w.nnueAcc = sfnnue.NewAccumulatorStack()
}

// SetTablebase sets the tablebase prober for this worker.
func (w *Worker) SetTablebase(prober tablebase.Prober, probeDepth int) {
	w.tbProber = prober
	w.tbProbeDepth = probeDepth
	if w.tbProbeDepth < 1 {
		w.tbProbeDepth = 1
	}
}

// ID returns the worker's ID.
func (w *Worker) ID() int {
	return w.id
}

// Nodes returns the number of nodes searched by this worker.
func (w *Worker) Nodes() uint64 {
	return w.nodes
}

// Reset resets the worker for a new search.
func (w *Worker) Reset() {
	w.nodes = 0
	w.orderer.Clear()
	// Reset optimism tracking for new search
	w.avgScore = -Infinity // Will be set to first score
	w.optimism[0] = 0
	w.optimism[1] = 0
}

// UpdateOptimism calculates optimism for the current iteration based on avgScore.
// Should be called before each depth in iterative deepening.
// Ported from Stockfish search.cpp iterative deepening loop.
func (w *Worker) UpdateOptimism() {
	avg := w.avgScore
	if avg == -Infinity {
		// No score yet - use 0 optimism
		w.optimism[0] = 0
		w.optimism[1] = 0
		return
	}

	// Stockfish formula: 142 * avg / (abs(avg) + 91)
	us := 0 // White = 0, Black = 1
	if w.pos.SideToMove == board.Black {
		us = 1
	}

	absAvg := avg
	if absAvg < 0 {
		absAvg = -absAvg
	}
	w.optimism[us] = (142 * avg) / (absAvg + 91)
	w.optimism[1-us] = -w.optimism[us]
}

// UpdateAvgScore updates the running average score after each iteration.
// Ported from Stockfish search.cpp.
func (w *Worker) UpdateAvgScore(score int) {
	if w.avgScore == -Infinity {
		w.avgScore = score
	} else {
		// Running average: (score + avgScore) / 2
		w.avgScore = (score + w.avgScore) / 2
	}
}

// SetRootHistory sets the position history from the game (for repetition detection).
func (w *Worker) SetRootHistory(hashes []uint64) {
	w.rootPosHashes = make([]uint64, len(hashes))
	copy(w.rootPosHashes, hashes)
}

// SetResultChannel sets the channel for sending search results.
func (w *Worker) SetResultChannel(ch chan<- WorkerResult) {
	w.resultCh = ch
}

// SetExcludedMoves sets the moves to exclude at root (for Multi-PV).
func (w *Worker) SetExcludedMoves(moves []board.Move) {
	w.excludedRootMoves = moves
}

// InitSearch initializes the worker for a new search.
// IMPORTANT: pos must be a dedicated copy for this worker (not shared with other goroutines).
// The caller (engine.workerSearch) is responsible for providing an isolated copy.
func (w *Worker) InitSearch(pos *board.Position) {
	w.pos = pos // Use directly - caller provides dedicated copy

	// Reset NNUE accumulator for new search to avoid stale state
	if w.nnueAcc != nil {
		w.nnueAcc.Reset()
	}

	// Initialize position history using pre-allocated buffer (avoids allocation per search)
	// Copy root position hashes (game history) into buffer
	rootLen := len(w.rootPosHashes)
	if rootLen > 640 {
		// Truncate to most recent 640 hashes (extremely long games)
		rootLen = 640
		copy(w.posHistoryBuffer[:rootLen], w.rootPosHashes[len(w.rootPosHashes)-640:])
	} else {
		copy(w.posHistoryBuffer[:rootLen], w.rootPosHashes)
	}
	// Add current position hash
	w.posHistoryBuffer[rootLen] = w.pos.Hash
	w.posHistoryLen = rootLen + 1
}

// Pos returns the current position (for debugging).
func (w *Worker) Pos() *board.Position {
	return w.pos
}

// SearchDepth performs search at the given depth and sends result via channel.
func (w *Worker) SearchDepth(depth, alpha, beta int) (board.Move, int) {
	w.depth = depth

	// DEBUG: Verify King exists at root
	if board.DebugMoveValidation {
		if w.pos.Pieces[board.White][board.King] == 0 {
			log.Printf("ROOT: White King MISSING at root! depth=%d hash=%x", depth, w.pos.Hash)
		}
		if w.pos.Pieces[board.Black][board.King] == 0 {
			log.Printf("ROOT: Black King MISSING at root! depth=%d hash=%x", depth, w.pos.Hash)
		}
	}

	score := w.negamax(depth, 0, alpha, beta, board.NoMove, board.NoMove, false)

	var bestMove board.Move
	if w.pv.length[0] > 0 {
		bestMove = w.pv.moves[0][0]
	}

	// Safety fallback: if no PV but legal moves exist, use first legal move
	if bestMove == board.NoMove && !w.stopFlag.Load() {
		moves := w.pos.GenerateLegalMoves()
		if moves.Len() > 0 {
			bestMove = moves.Get(0)
		}
	}

	// Send result if channel is set
	if w.resultCh != nil && !w.stopFlag.Load() {
		pv := make([]board.Move, w.pv.length[0])
		for i := 0; i < w.pv.length[0]; i++ {
			pv[i] = w.pv.moves[0][i]
		}
		w.resultCh <- WorkerResult{
			WorkerID: w.id,
			Depth:    depth,
			Score:    score,
			Move:     bestMove,
			PV:       pv,
			Nodes:    w.nodes,
		}
	}

	return bestMove, score
}

// evaluate returns the static evaluation using cached pawn structure or NNUE.
func (w *Worker) evaluate() int {
	if w.useNNUE && w.nnueNet != nil {
		return w.nnueEvaluate()
	}
	return EvaluateWithPawnTable(w.pos, w.pawnTable)
}

// stopped returns true if search should stop.
func (w *Worker) stopped() bool {
	return w.stopFlag.Load()
}

// GetPV returns the principal variation from the last search.
func (w *Worker) GetPV() []board.Move {
	pv := make([]board.Move, w.pv.length[0])
	for i := 0; i < w.pv.length[0]; i++ {
		pv[i] = w.pv.moves[0][i]
	}
	return pv
}

// isExcludedRootMove checks if a move is in the excluded list (for Multi-PV).
func (w *Worker) isExcludedRootMove(move board.Move) bool {
	for _, excluded := range w.excludedRootMoves {
		if move == excluded {
			return true
		}
	}
	return false
}

// isDraw checks for draw by repetition or 50-move rule.
func (w *Worker) isDraw() bool {
	// 50-move rule
	if w.pos.HalfMoveClock >= 100 {
		return true
	}

	// Insufficient material
	if w.pos.IsInsufficientMaterial() {
		return true
	}

	// Threefold repetition (use pre-allocated buffer)
	if w.posHistoryLen > 0 {
		currentHash := w.pos.Hash
		count := 0
		for i := 0; i < w.posHistoryLen; i++ {
			if w.posHistoryBuffer[i] == currentHash {
				count++
				if count >= 2 {
					return true
				}
			}
		}
	}

	return false
}

// negamax implements the negamax algorithm with alpha-beta pruning.
// excludedMove is used for singular extension search - if not NoMove, this move will be skipped.
// cutNode indicates expected node type: true if we expect a beta cutoff (most children are cut-nodes).
func (w *Worker) negamax(depth, ply int, alpha, beta int, prevMove, excludedMove board.Move, cutNode bool) int {
	// Bounds check to prevent array overflow (can happen with high depth + extensions)
	// Use MaxPly-1 because we access pv.length[ply+1] inside this function
	if ply >= MaxPly-1 {
		return w.evaluate()
	}

	// Check for stop signal periodically
	if w.nodes&4095 == 0 && w.stopFlag.Load() {
		return 0
	}

	w.nodes++

	// DEBUG: Comprehensive position validation at EVERY ply
	if board.DebugMoveValidation {
		us := w.pos.SideToMove
		// Check that pieces for "us" are ACTUALLY in Occupied[us]
		for pt := board.Pawn; pt <= board.King; pt++ {
			pieceBB := w.pos.Pieces[us][pt]
			if pieceBB&^w.pos.Occupied[us] != 0 {
				log.Printf("NEGAMAX ENTRY CORRUPT: %v %v pieces not in Occupied[%v]! ply=%d depth=%d hash=%x prevMove=%v",
					us, pt, us, ply, depth, w.pos.Hash, prevMove)
				log.Printf("  PieceBB=%x Occupied[%v]=%x Diff=%x",
					pieceBB, us, w.pos.Occupied[us], pieceBB&^w.pos.Occupied[us])
			}
		}
		// Check that Occupied[us] matches sum of our pieces
		var ourSum board.Bitboard
		for pt := board.Pawn; pt <= board.King; pt++ {
			ourSum |= w.pos.Pieces[us][pt]
		}
		if ourSum != w.pos.Occupied[us] {
			log.Printf("NEGAMAX ENTRY CORRUPT: %v Occupied mismatch! ply=%d depth=%d hash=%x prevMove=%v",
				us, ply, depth, w.pos.Hash, prevMove)
			log.Printf("  Sum=%x Occupied=%x", ourSum, w.pos.Occupied[us])
		}
	}

	// Initialize PV length for this ply
	w.pv.length[ply] = ply

	// Check for draw
	if ply > 0 && w.isDraw() {
		return 0
	}

	// Tablebase probing (only in endgame positions)
	if ply > 0 && w.tbProber != nil && depth >= w.tbProbeDepth {
		pieceCount := tablebase.CountPieces(w.pos)
		if pieceCount <= w.tbProber.MaxPieces() {
			tbResult := w.tbProber.Probe(w.pos)
			if tbResult.Found {
				tbScore := tablebase.WDLToScore(tbResult.WDL, ply)

				// Determine TT flag based on WDL
				var ttFlag TTFlag
				switch tbResult.WDL {
				case tablebase.WDLWin, tablebase.WDLCursedWin:
					// Winning - this is a lower bound (we might find better)
					if tbScore >= beta {
						// Store in TT and return
						w.tt.Store(w.pos.Hash, MaxPly, AdjustScoreToTT(tbScore, ply), TTLowerBound, board.NoMove, true)
						return tbScore
					}
					ttFlag = TTLowerBound
					if tbScore > alpha {
						alpha = tbScore
					}
				case tablebase.WDLLoss, tablebase.WDLBlessedLoss:
					// Losing - this is an upper bound
					if tbScore <= alpha {
						w.tt.Store(w.pos.Hash, MaxPly, AdjustScoreToTT(tbScore, ply), TTUpperBound, board.NoMove, true)
						return tbScore
					}
					ttFlag = TTUpperBound
					if tbScore < beta {
						beta = tbScore
					}
				default:
					// Draw - exact score
					w.tt.Store(w.pos.Hash, MaxPly, AdjustScoreToTT(tbScore, ply), TTExact, board.NoMove, true)
					return tbScore
				}
				_ = ttFlag // Used for potential future improvements
			}
		}
	}

	// Probe transposition table
	var ttMove board.Move
	ttPv := false // Track if TT indicates this is a PV node
	ttEntry, found := w.tt.Probe(w.pos.Hash)
	if found {
		ttMove = ttEntry.BestMove
		ttPv = ttEntry.IsPV

		// Validate TT move immediately (like Stockfish's movepick.cpp)
		// TT moves can be corrupted due to hash collisions or race conditions
		if ttMove != board.NoMove && !w.pos.PseudoLegal(ttMove) {
			ttMove = board.NoMove
		}

		// Multi-PV: don't use TT cutoffs at root if TT move is excluded
		ttCutoffAllowed := ply > 0 || !w.isExcludedRootMove(ttMove)

		if int(ttEntry.Depth) >= depth && ttCutoffAllowed {
			score := AdjustScoreFromTT(int(ttEntry.Score), ply)
			switch ttEntry.Flag {
			case TTExact:
				if ply == 0 && ttMove != board.NoMove {
					w.pv.moves[0][0] = ttMove
					w.pv.length[0] = 1
				}
				return score
			case TTLowerBound:
				if score > alpha {
					alpha = score
				}
			case TTUpperBound:
				if score < beta {
					beta = score
				}
			}
			if alpha >= beta {
				if ply == 0 && ttMove != board.NoMove {
					w.pv.moves[0][0] = ttMove
					w.pv.length[0] = 1
				}
				return score
			}
		}
	}

	// Quiescence search at depth 0
	if depth <= 0 {
		return w.quiescence(ply, alpha, beta)
	}

	// Check if in check
	inCheck := w.pos.InCheck()

	// Internal Iterative Reductions (IIR) - Stockfish approach
	// When no TT move is available, reduce depth instead of doing recursive search
	// This avoids undoStack[ply] collision that occurred with recursive IID
	if depth >= 4 && ttMove == board.NoMove && !inCheck {
		depth -= 2
	}

	// Check extension
	extension := 0
	if inCheck {
		extension = 1
	}

	// Threat extension
	if EnableThreatExt && extension == 0 && depth >= threatExtensionMinDepth && ply > 0 {
		if w.detectSeriousThreats() {
			extension = 1
		}
	}

	// Static evaluation for pruning decisions
	rawEval := w.evaluate()
	// Apply correction history adjustment
	correction := w.corrHistory.Get(w.pos)
	staticEval := rawEval + correction
	w.evalStack[ply] = staticEval

	// Improving heuristic
	improving := false
	if ply >= 2 {
		improving = staticEval > w.evalStack[ply-2]
	}

	// opponentWorsening heuristic (Stockfish search.cpp:751)
	// True if opponent's position is worsening (our eval improved vs their last eval)
	opponentWorsening := false
	if ply >= 1 {
		opponentWorsening = staticEval > -w.evalStack[ply-1]
	}

	// Hindsight depth adjustment (Stockfish search.cpp:754-757)
	// Adjust depth based on how the previous ply's LMR prediction turned out
	if EnableHindsightDepth && ply >= 1 {
		priorReduction := w.searchStack[ply-1].reduction
		// If we reduced a lot and opponent isn't getting worse, search deeper
		if priorReduction >= 3 && !opponentWorsening {
			depth++
		}
		// If we reduced and position eval sum suggests stability, search shallower
		if priorReduction >= 2 && depth >= 2 {
			evalSum := staticEval + w.evalStack[ply-1]
			if evalSum > 173 {
				depth--
			}
		}
	}

	// Initialize cutoffCnt for grandchild nodes (Stockfish search.cpp:699)
	if ply+2 < MaxPly {
		w.searchStack[ply+2].cutoffCnt = 0
	}

	// Reverse Futility Pruning
	// Reduce aggressiveness in PV nodes (ttPv)
	if EnableRFP && !inCheck && depth <= 6 && ply > 0 && !ttPv {
		rfpMargin := 80 * depth
		if !improving {
			rfpMargin -= 20
		}
		if staticEval-rfpMargin >= beta {
			return beta
		}
	}

	// Razoring (Stockfish search.cpp:873)
	// Use quadratic formula: 485 + 281*depth*depth (much more aggressive)
	if EnableRazoring && depth <= 5 && !inCheck && ply > 0 && !ttPv {
		razorMargin := 485 + 281*depth*depth
		if staticEval+razorMargin <= alpha {
			score := w.quiescence(ply, alpha, beta)
			if score <= alpha {
				return score
			}
		}
	}

	// Null Move Pruning (Stockfish search.cpp:893-924)
	// Don't do NMP in PV nodes to preserve principal variation
	if EnableNMP && !inCheck && depth >= 3 && ply > 0 && !ttPv && w.pos.HasNonPawnMaterial() {
		// Stockfish: R = 7 + depth/3 (more aggressive than our previous 2 + depth/4)
		R := 7 + depth/3
		if R > depth-1 {
			R = depth - 1
		}

		nullUndo := w.pos.MakeNullMove()
		nullScore := -w.negamax(depth-1-R, ply+1, -beta, -beta+1, board.NoMove, board.NoMove, !cutNode)
		w.pos.UnmakeNullMove(nullUndo)

		if nullScore >= beta {
			return nullScore
		}
	}

	// Probcut - prune if a shallow search of captures exceeds beta by a margin
	// Stockfish (search.cpp:938): probCutBeta = beta + 235 - 63 * improving
	// probCutDepth = clamp(depth - 5 - (staticEval-beta)/315, 0, depth)
	if EnableProbcut && depth >= probcutDepth && !inCheck && ply > 0 && abs(beta) < MateScore-100 {
		// Adaptive margin: 235 - 63 when improving, 235 when not
		adaptiveMargin := 235
		if improving {
			adaptiveMargin -= 63
		}
		probcutBeta := beta + adaptiveMargin

		// Adaptive depth based on eval (Stockfish formula)
		evalDiff := staticEval - beta
		probcutSearchDepth := depth - 5 - evalDiff/315
		if probcutSearchDepth < 1 {
			probcutSearchDepth = 1
		}
		if probcutSearchDepth > depth {
			probcutSearchDepth = depth
		}

		captures := w.pos.GenerateCaptures()
		for i := 0; i < captures.Len(); i++ {
			capture := captures.Get(i)
			if SEE(w.pos, capture) < 0 {
				continue
			}

			w.computeDirtyPieces(capture) // Track piece changes for incremental NNUE
			w.nnuePush()
			undo := w.pos.MakeMove(capture)
			if !undo.Valid {
				// Move is illegal - undo and try next
				w.pos.UnmakeMove(capture, undo)
				w.nnuePop()
				continue
			}

			score := -w.negamax(probcutSearchDepth, ply+1, -probcutBeta, -probcutBeta+1, capture, board.NoMove, !cutNode)
			w.pos.UnmakeMove(capture, undo)
			w.nnuePop()

			if score >= probcutBeta {
				return score
			}
		}
	}

	// Multi-Cut - if multiple moves fail high at reduced depth, prune
	if EnableMulticut && depth >= multicutDepth && !inCheck && ply > 0 && abs(beta) < MateScore-100 {
		mcMoves := w.pos.GenerateLegalMoves()
		mcScores := w.orderer.ScoreMovesWithCounter(w.pos, mcMoves, ply, ttMove, prevMove)

		mcCutoffs := 0
		mcSearched := 0
		mcSearchDepth := depth - 4
		if mcSearchDepth < 1 {
			mcSearchDepth = 1
		}

		for i := 0; i < mcMoves.Len() && mcSearched < multicutMoves; i++ {
			PickMove(mcMoves, mcScores, i)
			move := mcMoves.Get(i)

			w.computeDirtyPieces(move) // Track piece changes for incremental NNUE
			w.nnuePush()
			undo := w.pos.MakeMove(move)
			if !undo.Valid {
				// Move is illegal - undo and try next
				w.pos.UnmakeMove(move, undo)
				w.nnuePop()
				continue
			}
			mcSearched++

			score := -w.negamax(mcSearchDepth, ply+1, -beta, -beta+1, move, board.NoMove, !cutNode)
			w.pos.UnmakeMove(move, undo)
			w.nnuePop()

			if score >= beta {
				mcCutoffs++
				if mcCutoffs >= multicutRequired {
					return beta
				}
			}
		}
	}

	// Futility Pruning flag (Stockfish: depth <= 5)
	pruneQuietMoves := false
	if EnableFutilityPruning && depth <= 5 && !inCheck && ply > 0 {
		futilityMargin := []int{0, 200, 300, 500, 700, 900}
		if staticEval+futilityMargin[depth] <= alpha {
			pruneQuietMoves = true
		}
	}

	// Singular Extensions (Stockfish search.cpp:1129-1157)
	// When TT move is significantly better than alternatives, extend it
	singularExtension := 0
	if EnableSingularExt && depth >= 6 && ttMove != board.NoMove && excludedMove == board.NoMove && found {
		// Check TT entry conditions:
		// - TT depth is recent enough
		// - TT bound includes lower bound (we know it's at least this good)
		if int(ttEntry.Depth) >= depth-3 && (ttEntry.Flag == TTLowerBound || ttEntry.Flag == TTExact) {
			// Stockfish formula: ttValue - (53 + 75*(ttPv && !PvNode)) * depth / 60
			// isPvNode = true when we have full window (alpha < beta - 1)
			isPvNode := alpha < beta-1
			margin := 53
			if ttPv && !isPvNode {
				margin = 128 // 53 + 75
			}
			ttValue := AdjustScoreFromTT(int(ttEntry.Score), ply)
			singularBeta := ttValue - margin*depth/60

			// Search at reduced depth excluding the TT move
			singularDepth := (depth - 1) / 2
			singularScore := w.negamax(singularDepth, ply, singularBeta-1, singularBeta, prevMove, ttMove, cutNode)

			// If all other moves fail low, extend the TT move (Stockfish double/triple extension)
			if singularScore < singularBeta {
				// Check if TT move is a capture for margin calculations
				ttCapture := ttMove.IsCapture(w.pos)

				// Stockfish's complex margin formulas (search.cpp:1140-1157)
				// doubleMargin: -4 + 199*PvNode - 201*!ttCapture
				// tripleMargin: 73 + 302*PvNode - 248*!ttCapture + 90*ttPv
				doubleMargin := -4
				if isPvNode {
					doubleMargin += 199
				}
				if !ttCapture {
					doubleMargin -= 201
				}

				tripleMargin := 73
				if isPvNode {
					tripleMargin += 302
				}
				if !ttCapture {
					tripleMargin -= 248
				}
				if ttPv {
					tripleMargin += 90
				}

				// Calculate extension level
				singularExtension = 1
				if singularScore < singularBeta-doubleMargin {
					singularExtension = 2
				}
				if singularScore < singularBeta-tripleMargin {
					singularExtension = 3
				}
			} else {
				// Negative extensions (Stockfish search.cpp:1158-1165)
				// TT move is NOT singular - other moves are also good
				// Reduce depth instead of extending
				ttValue := AdjustScoreFromTT(int(ttEntry.Score), ply)
				if ttValue >= beta {
					singularExtension = -3 // Strong reduction when TT value beats beta
				} else if cutNode {
					singularExtension = -2 // Moderate reduction at cut nodes
				}
			}
		}
	}

	// Generate moves
	moves := w.pos.GenerateLegalMoves()

	// DEBUG: Verify KingSquare matches King bitboard after move generation
	if board.DebugMoveValidation {
		whiteKingBB := w.pos.Pieces[board.White][board.King]
		blackKingBB := w.pos.Pieces[board.Black][board.King]
		whiteKingSq := whiteKingBB.LSB()
		blackKingSq := blackKingBB.LSB()
		if w.pos.KingSquare[board.White] != whiteKingSq {
			log.Printf("KINGSQ MISMATCH after movegen! White cached=%v actual=%v ply=%d depth=%d hash=%x",
				w.pos.KingSquare[board.White], whiteKingSq, ply, depth, w.pos.Hash)
		}
		if w.pos.KingSquare[board.Black] != blackKingSq {
			log.Printf("KINGSQ MISMATCH after movegen! Black cached=%v actual=%v ply=%d depth=%d hash=%x",
				w.pos.KingSquare[board.Black], blackKingSq, ply, depth, w.pos.Hash)
		}
	}

	// Checkmate or stalemate
	if moves.Len() == 0 {
		if inCheck {
			return -MateScore + ply
		}
		return 0
	}

	// Score and sort moves
	scores := w.orderer.ScoreMovesWithCounter(w.pos, moves, ply, ttMove, prevMove)

	bestScore := -Infinity
	bestMove := board.NoMove
	flag := TTUpperBound
	movesSearched := 0

	for i := 0; i < moves.Len(); i++ {
		PickMove(moves, scores, i)
		move := moves.Get(i)

		// Multi-PV: skip excluded moves at root
		if ply == 0 && w.isExcludedRootMove(move) {
			continue
		}

		// Singular extension: skip the excluded move
		if move == excludedMove {
			continue
		}

		isCapture := move.IsCapture(w.pos)
		isPromotion := move.IsPromotion()

		// Futility pruning (in move loop)
		if EnableFutilityPruning && pruneQuietMoves && !isCapture && !isPromotion && bestMove != board.NoMove {
			continue
		}

		// SEE pruning - prune bad captures at low depths (Stockfish: depth <= 7)
		if EnableSEEPruning && isCapture && depth <= 7 && !inCheck && movesSearched > 0 {
			// Scale threshold based on depth: deeper = more permissive
			seeThreshold := -20 * depth
			if SEE(w.pos, move) < seeThreshold {
				continue
			}
		}

		// Late Move Pruning (LMP)
		if EnableLMP && depth <= 7 && !inCheck && movesSearched > 0 && !isCapture && !isPromotion && move != ttMove {
			threshold := lmpThreshold[depth]
			if !improving {
				threshold = threshold * 2 / 3
			}
			if movesSearched >= threshold {
				continue
			}
		}

		// History Pruning
		if EnableHistoryPruning && depth <= 3 && !inCheck && movesSearched > 0 && !isCapture && !isPromotion && move != ttMove {
			if w.orderer.GetHistoryScore(move) < historyPruningThreshold {
				continue
			}
		}

		// SEE-based quiet pruning disabled: Our SEE only handles captures
		// TODO: Implement proper SEE for quiet moves (check if piece is safe on destination)

		// DEBUG: Verify position consistency BEFORE checking piece
		{
			var whiteSum, blackSum board.Bitboard
			for pt := board.Pawn; pt <= board.King; pt++ {
				whiteSum |= w.pos.Pieces[board.White][pt]
				blackSum |= w.pos.Pieces[board.Black][pt]
			}
			if whiteSum != w.pos.Occupied[board.White] {
				log.Printf("PRE-MOVE: White Occupied mismatch! sum=%x Occupied=%x ply=%d depth=%d move=%v hash=%x",
					whiteSum, w.pos.Occupied[board.White], ply, depth, move, w.pos.Hash)
			}
			if blackSum != w.pos.Occupied[board.Black] {
				log.Printf("PRE-MOVE: Black Occupied mismatch! sum=%x Occupied=%x ply=%d depth=%d move=%v hash=%x",
					blackSum, w.pos.Occupied[board.Black], ply, depth, move, w.pos.Hash)
			}
			if (whiteSum | blackSum) != w.pos.AllOccupied {
				log.Printf("PRE-MOVE: AllOccupied mismatch! sum=%x AllOccupied=%x ply=%d depth=%d move=%v hash=%x",
					whiteSum|blackSum, w.pos.AllOccupied, ply, depth, move, w.pos.Hash)
			}
		}

		// Make move
		movingPiece := w.pos.PieceAt(move.From())
		moveTo := move.To()

		// Defensive skip: validate move matches current side to move
		// This catches position corruption or stale move data
		if movingPiece == board.NoPiece || movingPiece.Color() != w.pos.SideToMove {
			if board.DebugMoveValidation {
				fromSq := move.From()
				fromBB := board.SquareBB(fromSq)
				log.Printf("ERROR: Invalid move! SideToMove=%v, PieceColor=%v, Move=%v, FromSq=%v, Ply=%d, Depth=%d, Hash=%x",
					w.pos.SideToMove, movingPiece.Color(), move, fromSq, ply, depth, w.pos.Hash)
				log.Printf("ERROR DETAIL: FromBB=%x AllOccupied=%x InAll=%v InWhite=%v InBlack=%v",
					fromBB, w.pos.AllOccupied,
					w.pos.AllOccupied&fromBB != 0,
					w.pos.Occupied[board.White]&fromBB != 0,
					w.pos.Occupied[board.Black]&fromBB != 0)
				// Print what's actually on f1 (sq 5 in standard notation)
				log.Printf("ERROR: KingSquares White=%v Black=%v", w.pos.KingSquare[board.White], w.pos.KingSquare[board.Black])
			}
			continue
		}

		// DEBUG: Save hash before MakeMove to verify restoration
		hashBeforeMove := w.pos.Hash

		// DEBUG: Verify King exists BEFORE MakeMove
		if board.DebugMoveValidation {
			whiteKingBB := w.pos.Pieces[board.White][board.King]
			if whiteKingBB == 0 {
				log.Printf("KING GONE BEFORE MakeMove! ply=%d depth=%d move=%v hash=%x", ply, depth, move, w.pos.Hash)
			}
		}

		w.computeDirtyPieces(move) // Track piece changes for incremental NNUE
		w.nnuePush()
		w.undoStack[ply] = w.pos.MakeMove(move)
		if !w.undoStack[ply].Valid {
			// Move is illegal - undo the position change and try next move
			w.pos.UnmakeMove(move, w.undoStack[ply])
			w.nnuePop()
			continue
		}

		// Store hashBeforeMove in undoStack for verification (reusing the Hash field)
		_ = hashBeforeMove // Will check after UnmakeMove

		// Store move info in search stack for continuation history
		w.searchStack[ply].currentMove = move
		w.searchStack[ply].movedPiece = movingPiece
		w.searchStack[ply].moveTo = moveTo
		w.searchStack[ply].continuationHistory = w.orderer.GetContinuationHistoryTable(movingPiece, moveTo)

		w.posHistoryBuffer[w.posHistoryLen] = w.pos.Hash
		w.posHistoryLen++
		movesSearched++

		var score int
		newDepth := depth - 1 + extension

		// Apply singular extension (positive) or negative extension (reduction)
		if move == ttMove && singularExtension != 0 {
			newDepth += singularExtension
		}

		// Late Move Reduction (LMR) - logarithmic formula based on Stockfish
		if movesSearched > 4 && depth >= 3 && !inCheck && !isCapture && !isPromotion {
			// Get base reduction from precomputed table
			d := depth
			if d > 63 {
				d = 63
			}
			m := movesSearched
			if m > 63 {
				m = 63
			}
			reduction := lmrReductions[d][m]

			// Stockfish's rootDelta scaling (search.cpp:1736)
			// Scales reduction inversely with aspiration window width
			// Narrower windows (confident positions) get less reduction
			if w.rootDelta > 0 && w.rootDelta < Infinity {
				delta := beta - alpha
				reduction -= delta * 608 / w.rootDelta
			}

			// Adjustments based on node type and position
			if !improving {
				reduction++
			}
			if move == ttMove {
				reduction -= 2
			}
			if ttPv {
				// Reduce less in TT PV positions (Stockfish: + 946 / 1024)
				reduction--
			}

			// Stockfish cutNode scaling (search.cpp:1199)
			// Cut nodes get extra reduction: r += 3372 + 997 * !ttMove (in 1024 units)
			if cutNode {
				extra := 3372
				if ttMove == board.NoMove {
					extra += 997
				}
				reduction += extra / 1024
			}

			// allNode classification: nodes that are neither PV nor cut
			// These nodes expect to search all moves, so reduce more aggressively
			isPvNode := alpha < beta-1
			allNode := !isPvNode && !cutNode
			if allNode && depth > 2 {
				// Extra reduction proportional to depth for all-nodes
				reduction += reduction / (depth + 1)
			}

			// cutoffCnt scaling (Stockfish search.cpp:1208-1210)
			// If next ply had multiple cutoffs, increase reduction
			if ply+1 < MaxPly {
				cutoffCnt := w.searchStack[ply+1].cutoffCnt
				if cutoffCnt > 1 {
					extra := 120
					if cutoffCnt > 2 {
						extra += 1024
					}
					if cutoffCnt > 3 {
						extra += 100
					}
					if allNode {
						extra += 1024
					}
					reduction += extra / 1024
				}
			}

			// Calculate statScore: combine main history + continuation histories
			// Ported from Stockfish's statScore calculation
			from := move.From()
			to := move.To()
			localHist := w.orderer.history[from][to]
			sharedHist := w.sharedHistory.Get(int(from), int(to))
			mainHist := (localHist + sharedHist) / 2

			// Add continuation history contributions from 1-ply and 2-ply back
			contHist0 := 0
			contHist1 := 0
			if ply >= 1 && w.searchStack[ply-1].continuationHistory != nil {
				contHist0 = w.searchStack[ply-1].continuationHistory[movingPiece][moveTo]
			}
			if ply >= 2 && w.searchStack[ply-2].continuationHistory != nil {
				contHist1 = w.searchStack[ply-2].continuationHistory[movingPiece][moveTo]
			}

			// Combine: 2*mainHist + contHist[0] + contHist[1] (Stockfish formula)
			statScore := 2*mainHist + contHist0 + contHist1
			w.searchStack[ply].statScore = statScore

			// Apply statScore to reduction (Stockfish: r -= statScore * 850 / 8192)
			reduction -= statScore * 850 / 8192

			// Stockfish: reduce more for later moves in move ordering (moveCount factor)
			reduction -= movesSearched * 73 / 1024

			// Ensure reduction is reasonable
			if reduction < 1 {
				reduction = 1
			}

			reducedDepth := newDepth - reduction
			if reducedDepth < 1 {
				reducedDepth = 1
			}

			// Store reduction for hindsight depth adjustment (Stockfish search.cpp:754-757)
			w.searchStack[ply].reduction = reduction

			score = -w.negamax(reducedDepth, ply+1, -alpha-1, -alpha, move, board.NoMove, !cutNode)

			if score > alpha {
				score = -w.negamax(newDepth, ply+1, -beta, -alpha, move, board.NoMove, false)
			}
		} else if movesSearched == 1 {
			// First move: PV node, cutNode=false
			score = -w.negamax(newDepth, ply+1, -beta, -alpha, move, board.NoMove, false)
		} else {
			// PVS: null window search with flipped cutNode
			score = -w.negamax(newDepth, ply+1, -alpha-1, -alpha, move, board.NoMove, !cutNode)
			if score > alpha && score < beta {
				// Re-search with full window: PV-like, cutNode=false
				score = -w.negamax(newDepth, ply+1, -beta, -alpha, move, board.NoMove, false)
			}
		}

		w.posHistoryLen--
		w.pos.UnmakeMove(move, w.undoStack[ply])
		w.nnuePop()

		// DEBUG: Verify King exists AFTER UnmakeMove
		if board.DebugMoveValidation {
			whiteKingBB := w.pos.Pieces[board.White][board.King]
			if whiteKingBB == 0 {
				log.Printf("KING GONE AFTER UnmakeMove! ply=%d depth=%d move=%v hash=%x undoHash=%x",
					ply, depth, move, w.pos.Hash, w.undoStack[ply].Hash)
			}
		}

		// DEBUG: Verify position hash was restored correctly after UnmakeMove
		// The UndoInfo.Hash should contain the hash BEFORE the move was made
		if w.pos.Hash != w.undoStack[ply].Hash {
			log.Printf("HASH MISMATCH: Expected=%x Got=%x ply=%d move=%v depth=%d",
				w.undoStack[ply].Hash, w.pos.Hash, ply, move, depth)
		}

		// Verify position consistency after UnmakeMove
		if w.pos.AllOccupied != (w.pos.Occupied[board.White] | w.pos.Occupied[board.Black]) {
			log.Printf("CORRUPTION: AllOccupied mismatch after UnmakeMove! ply=%d, move=%v", ply, move)
		}
		// Verify Occupied[color] matches sum of Pieces[color][*]
		var whiteOcc, blackOcc board.Bitboard
		for pt := board.Pawn; pt <= board.King; pt++ {
			whiteOcc |= w.pos.Pieces[board.White][pt]
			blackOcc |= w.pos.Pieces[board.Black][pt]
		}
		if w.pos.Occupied[board.White] != whiteOcc {
			log.Printf("CORRUPTION: White Occupied mismatch! Occupied=%x, Pieces sum=%x, ply=%d, move=%v",
				w.pos.Occupied[board.White], whiteOcc, ply, move)
		}
		if w.pos.Occupied[board.Black] != blackOcc {
			log.Printf("CORRUPTION: Black Occupied mismatch! Occupied=%x, Pieces sum=%x, ply=%d, move=%v",
				w.pos.Occupied[board.Black], blackOcc, ply, move)
		}

		if w.stopFlag.Load() {
			return 0
		}

		if score > bestScore {
			bestScore = score
			bestMove = move

			if score > alpha {
				alpha = score
				flag = TTExact

				w.pv.moves[ply][ply] = move
				for j := ply + 1; j < w.pv.length[ply+1]; j++ {
					w.pv.moves[ply][j] = w.pv.moves[ply+1][j]
				}
				w.pv.length[ply] = w.pv.length[ply+1]
			}
		}

		// Beta cutoff
		if score >= beta {
			// Update cutoffCnt (Stockfish search.cpp:1375)
			// Increment when extension < 2 or at PV nodes
			isPvNode := alpha < beta-1
			if extension < 2 || isPvNode {
				w.searchStack[ply].cutoffCnt++
			}

			if ply == 0 && bestMove != board.NoMove {
				w.pv.moves[0][0] = bestMove
				w.pv.length[0] = 1
			}

			w.tt.Store(w.pos.Hash, depth, AdjustScoreToTT(score, ply), TTLowerBound, bestMove, false)

			if isCapture {
				attackerPiece := w.pos.PieceAt(move.From())
				var capturedType board.PieceType
				if move.IsEnPassant() {
					capturedType = board.Pawn
				} else {
					capturedPiece := w.pos.PieceAt(move.To())
					if capturedPiece != board.NoPiece {
						capturedType = capturedPiece.Type()
					}
				}
				w.orderer.UpdateCaptureHistory(attackerPiece, move.To(), capturedType, depth, true)
			} else {
				w.orderer.UpdateKillers(move, ply)
				w.orderer.UpdateHistory(move, depth, true)
				// Update low-ply history for better root move ordering
				w.orderer.UpdateLowPlyHistory(move, ply, depth, true)
				// Also update shared history for Lazy SMP collective learning
				bonus := depth * depth
				w.sharedHistory.Update(int(move.From()), int(move.To()), bonus)
				w.orderer.UpdateCounterMove(prevMove, move, w.pos)

				if prevMove != board.NoMove {
					prevPiece := w.pos.PieceAt(prevMove.To())
					movePiece := w.pos.PieceAt(move.To())
					w.orderer.UpdateCountermoveHistory(prevMove, move, prevPiece, movePiece, depth, true)
				}

				// Update continuation history for multiple plies back (Stockfish style)
				// This learns move pair patterns at different ply distances
				w.updateContinuationHistories(ply, movingPiece, moveTo, depth, true)
			}

			return score
		}
	}

	// Safety fallback
	if bestMove == board.NoMove && moves.Len() > 0 {
		bestMove = moves.Get(0)
		if bestScore == -Infinity {
			bestScore = alpha
		}
	}

	// Update correction history when we have an exact score
	// This helps the engine learn from eval errors
	if flag == TTExact && !inCheck && depth >= 2 {
		w.corrHistory.Update(w.pos, bestScore, rawEval, depth)
	}

	// isPV = true when we found an exact score (improved alpha without beta cutoff)
	isPV := flag == TTExact
	w.tt.Store(w.pos.Hash, depth, AdjustScoreToTT(bestScore, ply), flag, bestMove, isPV)

	return bestScore
}

// quiescence searches captures to avoid horizon effect.
func (w *Worker) quiescence(ply int, alpha, beta int) int {
	return w.quiescenceInternal(ply, 0, alpha, beta)
}

// quiescenceInternal is the internal quiescence search with qPly tracking.
// Fixed to match Stockfish: TT probe, proper in-check handling, SEE pruning.
func (w *Worker) quiescenceInternal(ply, qPly int, alpha, beta int) int {
	const maxQuiescencePly = 32
	if ply >= MaxPly || qPly > maxQuiescencePly {
		return w.evaluate()
	}

	if w.stopFlag.Load() {
		return 0
	}

	w.nodes++
	originalAlpha := alpha

	// TT Probe - critical for QS performance
	var ttMove board.Move
	ttEntry, ttHit := w.tt.Probe(w.pos.Hash)
	if ttHit {
		ttMove = ttEntry.BestMove
		// Validate TT move (can be corrupted by hash collision)
		if ttMove != board.NoMove && !w.pos.PseudoLegal(ttMove) {
			ttMove = board.NoMove
		}
		// TT cutoff - depth >= 0 is sufficient for QS
		if ttEntry.Depth >= 0 {
			score := AdjustScoreFromTT(int(ttEntry.Score), ply)
			switch ttEntry.Flag {
			case TTExact:
				return score
			case TTLowerBound:
				if score >= beta {
					return score
				}
			case TTUpperBound:
				if score <= alpha {
					return score
				}
			}
		}
	}

	// Check detection - critical: NO standing pat when in check
	inCheck := w.pos.InCheck()

	var standPat, bestValue int
	var bestMove board.Move

	if inCheck {
		// When in check, we MUST make a move - no standing pat allowed
		// Start with worst possible score (will be checkmate if no legal moves)
		bestValue = -MateScore + ply
		standPat = bestValue
	} else {
		// Lazy evaluation cutoff (only when not in check)
		lazyEval := EvaluateMaterial(w.pos)
		if lazyEval-lazyEvalMargin >= beta {
			return beta
		}
		if lazyEval+lazyEvalMargin <= alpha {
			return alpha
		}

		// Stand pat - can choose not to capture
		standPat = w.evaluate()
		bestValue = standPat

		if standPat >= beta {
			// Store stand pat cutoff in TT
			w.tt.Store(w.pos.Hash, 0, AdjustScoreToTT(standPat, ply), TTLowerBound, board.NoMove, false)
			return beta
		}

		if standPat > alpha {
			alpha = standPat
		}

		// Big delta pruning - if even capturing a queen can't raise alpha, give up
		if standPat+QueenValue < alpha {
			return alpha
		}
	}

	// Move generation: evasions when in check, captures otherwise
	var moves *board.MoveList
	if inCheck {
		// When in check, must search ALL legal moves (evasions)
		moves = w.pos.GenerateLegalMoves()
	} else {
		// Normal QS: only captures
		moves = w.pos.GenerateCaptures()
	}

	// Move ordering with TT move priority
	scores := w.orderer.ScoreMoves(w.pos, moves, ply, ttMove)

	for i := 0; i < moves.Len(); i++ {
		PickMove(moves, scores, i)
		move := moves.Get(i)

		// Pruning only when NOT in check and move is a capture
		if !inCheck && move.IsCapture(w.pos) {
			captureValue := qsCaptureValue(w.pos, move)
			futilityBase := standPat + 351 // Stockfish constant

			// Delta pruning: skip if even this capture can't reach alpha
			if standPat+captureValue+200 < alpha && !move.IsPromotion() {
				if captureValue+futilityBase > bestValue {
					bestValue = captureValue + futilityBase
				}
				continue
			}

			// SEE pruning: skip losing captures
			seeValue := SEE(w.pos, move)
			if seeValue < 0 {
				continue
			}

			// SEE futility: if base + SEE can't reach alpha, skip
			if futilityBase+seeValue <= alpha {
				if futilityBase > bestValue {
					bestValue = futilityBase
				}
				continue
			}
		}

		w.computeDirtyPieces(move)
		w.nnuePush()
		undo := w.pos.MakeMove(move)
		if !undo.Valid {
			w.pos.UnmakeMove(move, undo)
			w.nnuePop()
			continue
		}

		score := -w.quiescenceInternal(ply+1, qPly+1, -beta, -alpha)
		w.pos.UnmakeMove(move, undo)
		w.nnuePop()

		if score > bestValue {
			bestValue = score
			bestMove = move

			if score > alpha {
				alpha = score
				if score >= beta {
					break // Beta cutoff
				}
			}
		}
	}

	// Checkmate detection: if in check and no legal moves found
	if inCheck && bestValue == -MateScore+ply {
		return -MateScore + ply // Checkmate
	}

	// Store result in TT
	var ttFlag TTFlag
	if bestValue >= beta {
		ttFlag = TTLowerBound
	} else if bestValue > originalAlpha {
		ttFlag = TTExact
	} else {
		ttFlag = TTUpperBound
	}
	w.tt.Store(w.pos.Hash, 0, AdjustScoreToTT(bestValue, ply), ttFlag, bestMove, false)

	return bestValue
}

// qsCaptureValue returns the material value of a capture for QS pruning.
func qsCaptureValue(pos *board.Position, move board.Move) int {
	var value int
	if move.IsEnPassant() {
		value = PawnValue
	} else {
		captured := pos.PieceAt(move.To())
		if captured != board.NoPiece {
			value = pieceValues[captured.Type()]
		}
	}
	if move.IsPromotion() {
		value += pieceValues[move.Promotion()] - PawnValue
	}
	return value
}

// detectSeriousThreats checks if opponent has serious threats against our pieces.
func (w *Worker) detectSeriousThreats() bool {
	pos := w.pos
	us := pos.SideToMove
	them := us.Other()
	occupied := pos.AllOccupied

	enemyPawnAttacks := computePawnAttacksBB(pos, them)
	enemyKnightAttacks := computeKnightAttacksBB(pos, them)
	enemyBishopAttacks := computeBishopAttacksBB(pos, them, occupied)
	enemyRookAttacks := computeRookAttacksBB(pos, them, occupied)
	enemyQueenAttacks := computeQueenAttacksBB(pos, them, occupied)

	enemyAttacks := enemyPawnAttacks | enemyKnightAttacks | enemyBishopAttacks |
		enemyRookAttacks | enemyQueenAttacks

	ourPawnAttacks := computePawnAttacksBB(pos, us)
	ourKnightAttacks := computeKnightAttacksBB(pos, us)
	ourBishopAttacks := computeBishopAttacksBB(pos, us, occupied)
	ourRookAttacks := computeRookAttacksBB(pos, us, occupied)
	ourQueenAttacks := computeQueenAttacksBB(pos, us, occupied)
	ourKingAttacks := board.KingAttacks(pos.KingSquare[us])

	ourDefenses := ourPawnAttacks | ourKnightAttacks | ourBishopAttacks |
		ourRookAttacks | ourQueenAttacks | ourKingAttacks

	ourPieces := pos.Occupied[us] &^ board.SquareBB(pos.KingSquare[us])

	hangingPieces := ourPieces & enemyAttacks & ^ourDefenses

	for hangingPieces != 0 {
		sq := hangingPieces.PopLSB()
		piece := pos.PieceAt(sq)
		if piece != board.NoPiece && pieceValues[piece.Type()] >= threatExtensionThreshold {
			return true
		}
	}

	queens := pos.Pieces[us][board.Queen]
	if queens&(enemyPawnAttacks|enemyKnightAttacks|enemyBishopAttacks|enemyRookAttacks) != 0 {
		return true
	}

	rooks := pos.Pieces[us][board.Rook]
	if rooks&(enemyPawnAttacks|enemyKnightAttacks|enemyBishopAttacks) != 0 {
		return true
	}

	return false
}

// updateContinuationHistories updates continuation history for multiple plies back.
// Ported from Stockfish's update_continuation_histories function.
// Updates plies 1, 2, 3, 4, 5, 6 with weighted bonuses.
func (w *Worker) updateContinuationHistories(ply int, piece board.Piece, toSq board.Square, depth int, isGood bool) {
	// Update continuation history for plies 1-6 back
	for plyBack := 1; plyBack <= 6; plyBack++ {
		targetPly := ply - plyBack
		if targetPly < 0 {
			break
		}

		ss := &w.searchStack[targetPly]
		if ss.currentMove == board.NoMove || ss.movedPiece == board.NoPiece {
			continue
		}

		// Update the continuation history entry
		w.orderer.UpdateContinuationHistory(
			ss.movedPiece,
			ss.moveTo,
			piece,
			toSq,
			depth,
			plyBack,
			isGood,
		)
	}
}
