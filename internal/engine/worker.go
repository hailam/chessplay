package engine

import (
	"log"
	"math"
	"sync/atomic"

	"github.com/hailam/chessplay/internal/board"
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
	undoStack [MaxPly]board.UndoInfo
	evalStack [MaxPly]int

	// Per-worker position history for repetition detection
	posHistory    []uint64
	rootPosHashes []uint64

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

	// Communication channel for results
	resultCh chan<- WorkerResult

	// Current search depth (for result reporting)
	depth int
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

// InitSearch initializes the worker for a new search with a position copy.
func (w *Worker) InitSearch(pos *board.Position) {
	log.Printf("[Worker %d] InitSearch received pos.SideToMove=%v", w.id, pos.SideToMove)
	w.pos = pos.Copy()
	log.Printf("[Worker %d] After copy w.pos.SideToMove=%v", w.id, w.pos.SideToMove)

	// Reset NNUE accumulator for new search to avoid stale state
	if w.nnueAcc != nil {
		w.nnueAcc.Reset()
	}

	// Initialize position history for this search
	w.posHistory = make([]uint64, 0, len(w.rootPosHashes)+MaxPly)
	w.posHistory = append(w.posHistory, w.rootPosHashes...)
	w.posHistory = append(w.posHistory, w.pos.Hash)
}

// SearchDepth performs search at the given depth and sends result via channel.
func (w *Worker) SearchDepth(depth, alpha, beta int) (board.Move, int) {
	w.depth = depth

	score := w.negamax(depth, 0, alpha, beta, board.NoMove)

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

	// Threefold repetition
	if len(w.posHistory) > 0 {
		currentHash := w.pos.Hash
		count := 0
		for _, h := range w.posHistory {
			if h == currentHash {
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
func (w *Worker) negamax(depth, ply int, alpha, beta int, prevMove board.Move) int {
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

	// Initialize PV length for this ply
	w.pv.length[ply] = ply

	// Check for draw
	if ply > 0 && w.isDraw() {
		return 0
	}

	// Probe transposition table
	var ttMove board.Move
	ttPv := false // Track if TT indicates this is a PV node
	ttEntry, found := w.tt.Probe(w.pos.Hash)
	if found {
		ttMove = ttEntry.BestMove
		ttPv = ttEntry.IsPV

		// Validate TT move before using (safety check for any edge cases)
		if ttMove != board.NoMove {
			piece := w.pos.PieceAt(ttMove.From())
			if piece == board.NoPiece || piece.Color() != w.pos.SideToMove {
				ttMove = board.NoMove // Invalidate bad TT move
			}
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

	// Internal Iterative Deepening (IID)
	if depth >= 4 && ttMove == board.NoMove {
		iidDepth := depth - 2
		if iidDepth < 1 {
			iidDepth = 1
		}
		w.negamax(iidDepth, ply, alpha, beta, prevMove)
		ttEntry, found = w.tt.Probe(w.pos.Hash)
		if found {
			ttMove = ttEntry.BestMove
		}
	}

	// Quiescence search at depth 0
	if depth <= 0 {
		return w.quiescence(ply, alpha, beta)
	}

	// Check if in check
	inCheck := w.pos.InCheck()

	// Check extension
	extension := 0
	if inCheck {
		extension = 1
	}

	// Threat extension
	if extension == 0 && depth >= threatExtensionMinDepth && ply > 0 {
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

	// Reverse Futility Pruning
	// Reduce aggressiveness in PV nodes (ttPv)
	if !inCheck && depth <= 6 && ply > 0 && !ttPv {
		rfpMargin := 80 * depth
		if !improving {
			rfpMargin -= 20
		}
		if staticEval-rfpMargin >= beta {
			return beta
		}
	}

	// Razoring
	if depth <= 2 && !inCheck && ply > 0 {
		razorMargin := 300 + 100*depth
		if staticEval+razorMargin <= alpha {
			score := w.quiescence(ply, alpha, beta)
			if score <= alpha {
				return score
			}
		}
	}

	// Null Move Pruning
	// Don't do NMP in PV nodes to preserve principal variation
	if !inCheck && depth >= 3 && ply > 0 && !ttPv && w.pos.HasNonPawnMaterial() {
		R := 2 + depth/4
		if R > depth-1 {
			R = depth - 1
		}

		nullUndo := w.pos.MakeNullMove()
		nullScore := -w.negamax(depth-1-R, ply+1, -beta, -beta+1, board.NoMove)
		w.pos.UnmakeNullMove(nullUndo)

		if nullScore >= beta {
			return beta
		}
	}

	// Probcut
	if depth >= probcutDepth && !inCheck && ply > 0 && abs(beta) < MateScore-100 {
		probcutBeta := beta + probcutMargin
		probcutSearchDepth := depth - probcutReduction
		if probcutSearchDepth < 1 {
			probcutSearchDepth = 1
		}

		captures := w.pos.GenerateCaptures()
		for i := 0; i < captures.Len(); i++ {
			capture := captures.Get(i)
			if SEE(w.pos, capture) < 0 {
				continue
			}

			w.nnuePush()
			undo := w.pos.MakeMove(capture)
			if !undo.Valid {
				w.nnuePop()
				continue
			}

			score := -w.negamax(probcutSearchDepth, ply+1, -probcutBeta, -probcutBeta+1, capture)
			w.pos.UnmakeMove(capture, undo)
			w.nnuePop()

			if score >= probcutBeta {
				return score
			}
		}
	}

	// Multi-Cut
	if depth >= multicutDepth && !inCheck && ply > 0 && abs(beta) < MateScore-100 {
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

			w.nnuePush()
			undo := w.pos.MakeMove(move)
			if !undo.Valid {
				w.nnuePop()
				continue
			}
			mcSearched++

			score := -w.negamax(mcSearchDepth, ply+1, -beta, -beta+1, move)
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

	// Futility Pruning flag
	pruneQuietMoves := false
	if depth <= 3 && !inCheck && ply > 0 {
		futilityMargin := []int{0, 200, 300, 500}
		if staticEval+futilityMargin[depth] <= alpha {
			pruneQuietMoves = true
		}
	}

	// Singular Extensions
	singularExtension := 0
	if depth >= 8 && ttMove != board.NoMove && !inCheck &&
		found && ttEntry.Depth >= int8(depth-3) && ttEntry.Flag != TTUpperBound {
		rBeta := int(ttEntry.Score) - 200
		singularDepth := (depth - 3) / 2
		if singularDepth < 1 {
			singularDepth = 1
		}
		singularScore := w.singularSearch(singularDepth, ply, rBeta-1, rBeta, prevMove, ttMove)
		if singularScore < rBeta {
			singularExtension = 1
		}
	}

	// Generate moves
	moves := w.pos.GenerateLegalMoves()

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

		isCapture := move.IsCapture(w.pos)
		isPromotion := move.IsPromotion()

		// Futility pruning
		if pruneQuietMoves && !isCapture && !isPromotion && bestMove != board.NoMove {
			continue
		}

		// SEE pruning
		if isCapture && depth <= 3 && !inCheck && movesSearched > 0 {
			if SEE(w.pos, move) < 0 {
				continue
			}
		}

		// Late Move Pruning (LMP)
		if depth <= 7 && !inCheck && movesSearched > 0 && !isCapture && !isPromotion && move != ttMove {
			threshold := lmpThreshold[depth]
			if !improving {
				threshold = threshold * 2 / 3
			}
			if movesSearched >= threshold {
				continue
			}
		}

		// History Pruning
		if depth <= 3 && !inCheck && movesSearched > 0 && !isCapture && !isPromotion && move != ttMove {
			if w.orderer.GetHistoryScore(move) < historyPruningThreshold {
				continue
			}
		}

		// Make move
		w.nnuePush()
		w.undoStack[ply] = w.pos.MakeMove(move)
		if !w.undoStack[ply].Valid {
			w.nnuePop()
			continue
		}

		w.posHistory = append(w.posHistory, w.pos.Hash)
		movesSearched++

		var score int
		newDepth := depth - 1 + extension

		if move == ttMove && singularExtension > 0 {
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

			// Adjustments
			if !improving {
				reduction++
			}
			if move == ttMove {
				reduction -= 2
			}

			// History-based adjustment (combine local and shared history)
			from := move.From()
			to := move.To()
			localHist := w.orderer.history[from][to]
			sharedHist := w.sharedHistory.Get(int(from), int(to))
			histScore := (localHist + sharedHist) / 2 // Average of local and shared
			reduction -= histScore / 8192

			// Ensure reduction is reasonable
			if reduction < 1 {
				reduction = 1
			}

			reducedDepth := newDepth - reduction
			if reducedDepth < 1 {
				reducedDepth = 1
			}

			score = -w.negamax(reducedDepth, ply+1, -alpha-1, -alpha, move)

			if score > alpha {
				score = -w.negamax(newDepth, ply+1, -beta, -alpha, move)
			}
		} else if movesSearched == 1 {
			score = -w.negamax(newDepth, ply+1, -beta, -alpha, move)
		} else {
			score = -w.negamax(newDepth, ply+1, -alpha-1, -alpha, move)
			if score > alpha && score < beta {
				score = -w.negamax(newDepth, ply+1, -beta, -alpha, move)
			}
		}

		w.posHistory = w.posHistory[:len(w.posHistory)-1]
		w.pos.UnmakeMove(move, w.undoStack[ply])
		w.nnuePop()

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
func (w *Worker) quiescenceInternal(ply, qPly int, alpha, beta int) int {
	const maxQuiescencePly = 32
	if ply >= MaxPly || qPly > maxQuiescencePly {
		return w.evaluate()
	}

	if w.stopFlag.Load() {
		return 0
	}

	w.nodes++

	// Lazy evaluation
	lazyEval := EvaluateMaterial(w.pos)
	if lazyEval-lazyEvalMargin >= beta {
		return beta
	}
	if lazyEval+lazyEvalMargin <= alpha {
		return alpha
	}

	// Stand pat
	standPat := w.evaluate()

	if standPat >= beta {
		return beta
	}

	if standPat > alpha {
		alpha = standPat
	}

	// Delta pruning
	bigDelta := QueenValue
	if standPat+bigDelta < alpha {
		return alpha
	}

	// Generate captures only
	moves := w.pos.GenerateCaptures()
	scores := w.orderer.ScoreMoves(w.pos, moves, ply, board.NoMove)

	for i := 0; i < moves.Len(); i++ {
		PickMove(moves, scores, i)
		move := moves.Get(i)

		// Delta pruning for individual moves
		if !w.pos.InCheck() {
			var captureValue int
			if move.IsEnPassant() {
				captureValue = PawnValue
			} else {
				capturedPiece := w.pos.PieceAt(move.To())
				if capturedPiece != board.NoPiece {
					captureValue = pieceValues[capturedPiece.Type()]
				}
			}
			if move.IsPromotion() {
				captureValue += QueenValue - PawnValue
			}
			if standPat+captureValue+200 < alpha {
				continue
			}
		}

		w.nnuePush()
		undo := w.pos.MakeMove(move)
		if !undo.Valid {
			w.nnuePop()
			continue
		}

		score := -w.quiescenceInternal(ply+1, qPly+1, -beta, -alpha)
		w.pos.UnmakeMove(move, undo)
		w.nnuePop()

		if score >= beta {
			return beta
		}

		if score > alpha {
			alpha = score
		}
	}

	// At first ply of quiescence, also search check-giving moves
	if qPly == 0 && !w.pos.InCheck() {
		checkMoves := w.pos.GenerateChecks()

		for i := 0; i < checkMoves.Len(); i++ {
			move := checkMoves.Get(i)

			if move.IsCapture(w.pos) {
				continue
			}

			w.nnuePush()
			undo := w.pos.MakeMove(move)
			if !undo.Valid {
				w.nnuePop()
				continue
			}

			if !w.pos.InCheck() {
				w.pos.UnmakeMove(move, undo)
				w.nnuePop()
				continue
			}

			score := -w.quiescenceInternal(ply+1, qPly+1, -beta, -alpha)
			w.pos.UnmakeMove(move, undo)
			w.nnuePop()

			if score >= beta {
				return beta
			}

			if score > alpha {
				alpha = score
			}
		}
	}

	return alpha
}

// singularSearch performs a search excluding a specific move.
func (w *Worker) singularSearch(depth, ply int, alpha, beta int, prevMove, excludedMove board.Move) int {
	moves := w.pos.GenerateLegalMoves()

	bestScore := -Infinity

	for i := 0; i < moves.Len(); i++ {
		move := moves.Get(i)

		if move == excludedMove {
			continue
		}

		w.nnuePush()
		w.undoStack[ply] = w.pos.MakeMove(move)
		if !w.undoStack[ply].Valid {
			w.nnuePop()
			continue
		}

		w.posHistory = append(w.posHistory, w.pos.Hash)

		score := -w.negamax(depth-1, ply+1, -beta, -alpha, move)

		w.posHistory = w.posHistory[:len(w.posHistory)-1]
		w.pos.UnmakeMove(move, w.undoStack[ply])
		w.nnuePop()

		if score > bestScore {
			bestScore = score
		}

		if score >= beta {
			return score
		}
	}

	if bestScore == -Infinity {
		return alpha
	}

	return bestScore
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
