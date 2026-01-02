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
	lazyEvalMargin           = 150  // Lazy eval margin for quiescence
	historyPruningThreshold  = -4000 // History pruning threshold
	probcutDepth             = 8    // Minimum depth for probcut
	probcutMargin            = 200  // Probcut margin above beta
	probcutReduction         = 4    // Probcut depth reduction
	multicutDepth            = 8    // Minimum depth for multi-cut
	multicutMoves            = 6    // Number of moves to try
	multicutRequired         = 3    // Number of cutoffs needed
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
type Searcher struct {
	pos       *board.Position
	tt        *TranspositionTable
	pawnTable *PawnTable
	orderer   *MoveOrderer

	// Search state
	nodes    uint64
	stopFlag atomic.Bool

	// PV tracking
	pv PVTable

	// Undo stack
	undoStack [MaxPly]board.UndoInfo

	// Eval stack for improving heuristic
	evalStack [MaxPly]int

	// Position history for repetition detection
	posHistory    []uint64
	rootPosHashes []uint64 // Position hashes from game history (before search)

	// Multi-PV support: moves to exclude at root
	excludedRootMoves []board.Move
}

// NewSearcher creates a new searcher.
func NewSearcher(tt *TranspositionTable) *Searcher {
	return &Searcher{
		tt:        tt,
		pawnTable: NewPawnTable(1), // 1MB pawn hash table
		orderer:   NewMoveOrderer(),
	}
}

// Stop signals the search to stop.
func (s *Searcher) Stop() {
	s.stopFlag.Store(true)
}

// Reset resets the searcher for a new search.
func (s *Searcher) Reset() {
	s.stopFlag.Store(false)
	s.nodes = 0
	s.orderer.Clear()
}

// Nodes returns the number of nodes searched.
func (s *Searcher) Nodes() uint64 {
	return s.nodes
}

// evaluate returns the static evaluation using cached pawn structure.
func (s *Searcher) evaluate() int {
	return EvaluateWithPawnTable(s.pos, s.pawnTable)
}

// Search performs the search at the given depth.
func (s *Searcher) Search(pos *board.Position, depth int) (board.Move, int) {
	return s.SearchWithBounds(pos, depth, -Infinity, Infinity)
}

// SetRootHistory sets the position history from the game (for repetition detection).
// This should be called before Search() with hashes from the game's move history.
func (s *Searcher) SetRootHistory(hashes []uint64) {
	s.rootPosHashes = make([]uint64, len(hashes))
	copy(s.rootPosHashes, hashes)
}

// SearchWithBounds performs search with custom alpha/beta bounds (for aspiration windows).
func (s *Searcher) SearchWithBounds(pos *board.Position, depth, alpha, beta int) (board.Move, int) {
	s.pos = pos.Copy()
	// Note: Reset() is called once by SearchWithLimits, not here per-depth

	// Initialize position history for this search
	// Start with game history and add current position
	s.posHistory = make([]uint64, 0, len(s.rootPosHashes)+MaxPly)
	s.posHistory = append(s.posHistory, s.rootPosHashes...)
	s.posHistory = append(s.posHistory, s.pos.Hash)

	score := s.negamax(depth, 0, alpha, beta, board.NoMove)

	var bestMove board.Move
	if s.pv.length[0] > 0 {
		bestMove = s.pv.moves[0][0]
	}

	// Safety fallback: if no PV but legal moves exist, use first legal move
	if bestMove == board.NoMove {
		moves := s.pos.GenerateLegalMoves()
		if moves.Len() > 0 {
			bestMove = moves.Get(0)
		}
	}

	return bestMove, score
}

// negamax implements the negamax algorithm with alpha-beta pruning.
// Includes: null move pruning, futility pruning, LMR, counter-moves.
func (s *Searcher) negamax(depth, ply int, alpha, beta int, prevMove board.Move) int {
	// Check for stop signal periodically
	if s.nodes&4095 == 0 && s.stopFlag.Load() {
		return 0
	}

	s.nodes++

	// Initialize PV length for this ply
	s.pv.length[ply] = ply

	// Check for draw
	if ply > 0 && s.isDraw() {
		return 0
	}

	// Probe transposition table
	var ttMove board.Move
	ttEntry, found := s.tt.Probe(s.pos.Hash)
	if found {
		ttMove = ttEntry.BestMove

		// Multi-PV: don't use TT cutoffs at root if TT move is excluded
		ttCutoffAllowed := ply > 0 || !s.isExcludedRootMove(ttMove)

		if int(ttEntry.Depth) >= depth && ttCutoffAllowed {
			score := AdjustScoreFromTT(int(ttEntry.Score), ply)
			switch ttEntry.Flag {
			case TTExact:
				// At root, populate PV from TT move before returning
				if ply == 0 && ttMove != board.NoMove {
					s.pv.moves[0][0] = ttMove
					s.pv.length[0] = 1
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
				// At root, populate PV from TT move before returning
				if ply == 0 && ttMove != board.NoMove {
					s.pv.moves[0][0] = ttMove
					s.pv.length[0] = 1
				}
				return score
			}
		}
	}

	// Internal Iterative Deepening (IID)
	// When no TT move at sufficient depth, search at reduced depth first
	if depth >= 4 && ttMove == board.NoMove {
		iidDepth := depth - 2
		if iidDepth < 1 {
			iidDepth = 1
		}
		// Search at reduced depth (results stored in TT)
		s.negamax(iidDepth, ply, alpha, beta, prevMove)
		// Re-probe TT to get the move found by IID
		ttEntry, found = s.tt.Probe(s.pos.Hash)
		if found {
			ttMove = ttEntry.BestMove
		}
	}

	// Quiescence search at depth 0
	if depth <= 0 {
		return s.quiescence(ply, alpha, beta)
	}

	// Check if in check
	inCheck := s.pos.InCheck()

	// Check extension: extend search when in check
	extension := 0
	if inCheck {
		extension = 1
	}

	// Threat extension: extend when opponent has serious threats
	if extension == 0 && depth >= threatExtensionMinDepth && ply > 0 {
		if s.detectSeriousThreats() {
			extension = 1
		}
	}

	// Static evaluation for pruning decisions
	staticEval := s.evaluate()

	// Store eval in stack for improving heuristic
	s.evalStack[ply] = staticEval

	// Improving heuristic: is current eval better than 2 plies ago?
	improving := false
	if ply >= 2 {
		improving = staticEval > s.evalStack[ply-2]
	}

	// Reverse Futility Pruning (Static Null Move Pruning)
	// If static eval is far above beta at shallow depth, return beta
	if !inCheck && depth <= 6 && ply > 0 {
		rfpMargin := 80 * depth
		if !improving {
			rfpMargin -= 20 // Stricter when not improving
		}
		if staticEval-rfpMargin >= beta {
			return beta
		}
	}

	// Razoring: if static eval is far below alpha at shallow depth, drop to quiescence
	if depth <= 2 && !inCheck && ply > 0 {
		razorMargin := 300 + 100*depth // 400 at d1, 500 at d2
		if staticEval+razorMargin <= alpha {
			score := s.quiescence(ply, alpha, beta)
			if score <= alpha {
				return score // Confirmed hopeless
			}
		}
	}

	// Null Move Pruning
	// Skip if: in check, at root, no non-pawn material (zugzwang risk)
	if !inCheck && depth >= 3 && ply > 0 && s.pos.HasNonPawnMaterial() {
		// Reduction based on depth
		R := 2 + depth/4
		if R > depth-1 {
			R = depth - 1
		}

		s.pos.MakeNullMove()
		nullScore := -s.negamax(depth-1-R, ply+1, -beta, -beta+1, board.NoMove)
		s.pos.UnmakeNullMove()

		if nullScore >= beta {
			return beta // Null move cutoff
		}
	}

	// Probcut: at high depths, try captures that might fail high
	// If a capture scores well at reduced depth, we can prune
	if depth >= probcutDepth && !inCheck && ply > 0 && abs(beta) < MateScore-100 {
		probcutBeta := beta + probcutMargin
		probcutSearchDepth := depth - probcutReduction
		if probcutSearchDepth < 1 {
			probcutSearchDepth = 1
		}

		// Try captures with positive SEE
		captures := s.pos.GenerateCaptures()
		for i := 0; i < captures.Len(); i++ {
			capture := captures.Get(i)
			if SEE(s.pos, capture) < 0 {
				continue // Skip losing captures
			}

			undo := s.pos.MakeMove(capture)
			if !undo.Valid {
				continue
			}

			// Search at reduced depth
			score := -s.negamax(probcutSearchDepth, ply+1, -probcutBeta, -probcutBeta+1, capture)
			s.pos.UnmakeMove(capture, undo)

			if score >= probcutBeta {
				return score // Probcut cutoff
			}
		}
	}

	// Multi-Cut: at high depths, if many moves fail high, prune
	if depth >= multicutDepth && !inCheck && ply > 0 && abs(beta) < MateScore-100 {
		mcMoves := s.pos.GenerateLegalMoves()
		mcScores := s.orderer.ScoreMovesWithCounter(s.pos, mcMoves, ply, ttMove, prevMove)

		mcCutoffs := 0
		mcSearched := 0
		mcSearchDepth := depth - 4
		if mcSearchDepth < 1 {
			mcSearchDepth = 1
		}

		for i := 0; i < mcMoves.Len() && mcSearched < multicutMoves; i++ {
			PickMove(mcMoves, mcScores, i)
			move := mcMoves.Get(i)

			undo := s.pos.MakeMove(move)
			if !undo.Valid {
				continue
			}
			mcSearched++

			score := -s.negamax(mcSearchDepth, ply+1, -beta, -beta+1, move)
			s.pos.UnmakeMove(move, undo)

			if score >= beta {
				mcCutoffs++
				if mcCutoffs >= multicutRequired {
					return beta // Multi-cut pruning
				}
			}
		}
	}

	// Futility Pruning
	// At shallow depths, if static eval + margin can't reach alpha, prune quiet moves
	pruneQuietMoves := false
	if depth <= 3 && !inCheck && ply > 0 {
		futilityMargin := []int{0, 200, 300, 500}
		if staticEval+futilityMargin[depth] <= alpha {
			pruneQuietMoves = true
		}
	}

	// Singular Extensions
	// At high depth with a TT move, verify if the TT move is singular (much better than alternatives)
	singularExtension := 0
	if depth >= 8 && ttMove != board.NoMove && !inCheck &&
		found && ttEntry.Depth >= int8(depth-3) && ttEntry.Flag != TTUpperBound {
		// rBeta is the threshold - if all other moves fail below this, TT move is singular
		rBeta := int(ttEntry.Score) - 200
		singularDepth := (depth - 3) / 2
		if singularDepth < 1 {
			singularDepth = 1
		}
		// Search all moves except TT move at reduced depth
		singularScore := s.singularSearch(singularDepth, ply, rBeta-1, rBeta, prevMove, ttMove)
		if singularScore < rBeta {
			singularExtension = 1 // TT move is singular, extend it
		}
	}

	// Generate moves
	moves := s.pos.GenerateLegalMoves()

	// Check for checkmate or stalemate
	if moves.Len() == 0 {
		if inCheck {
			return -MateScore + ply // Checkmate
		}
		return 0 // Stalemate
	}

	// Score and sort moves (including counter-move bonus)
	scores := s.orderer.ScoreMovesWithCounter(s.pos, moves, ply, ttMove, prevMove)

	bestScore := -Infinity
	bestMove := board.NoMove
	flag := TTUpperBound
	movesSearched := 0

	for i := 0; i < moves.Len(); i++ {
		// Pick the best remaining move
		PickMove(moves, scores, i)
		move := moves.Get(i)

		// Multi-PV: skip excluded moves at root
		if ply == 0 && s.isExcludedRootMove(move) {
			continue
		}

		isCapture := move.IsCapture(s.pos)
		isPromotion := move.IsPromotion()

		// Futility pruning: skip quiet moves if we can't improve alpha
		// Only prune after we have found at least one valid move
		if pruneQuietMoves && !isCapture && !isPromotion && bestMove != board.NoMove {
			continue
		}

		// SEE pruning: skip losing captures at shallow depths
		// Only prune after we have at least one move searched
		if isCapture && depth <= 3 && !inCheck && movesSearched > 0 {
			if SEE(s.pos, move) < 0 {
				continue // Skip losing capture
			}
		}

		// Late Move Pruning (LMP): skip late quiet moves at shallow depths
		if depth <= 7 && !inCheck && movesSearched > 0 && !isCapture && !isPromotion && move != ttMove {
			threshold := lmpThreshold[depth]
			if !improving {
				threshold = threshold * 2 / 3 // Prune more aggressively when not improving
			}
			if movesSearched >= threshold {
				continue
			}
		}

		// History Pruning: skip quiet moves with very negative history at shallow depths
		if depth <= 3 && !inCheck && movesSearched > 0 && !isCapture && !isPromotion && move != ttMove {
			if s.orderer.GetHistoryScore(move) < historyPruningThreshold {
				continue
			}
		}

		// Make move
		s.undoStack[ply] = s.pos.MakeMove(move)

		// Skip if move was invalid (no piece at from square)
		if !s.undoStack[ply].Valid {
			continue
		}

		// Track position hash for repetition detection
		s.posHistory = append(s.posHistory, s.pos.Hash)

		movesSearched++
		var score int
		newDepth := depth - 1 + extension

		// Apply singular extension for TT move
		if move == ttMove && singularExtension > 0 {
			newDepth += singularExtension
		}

		// Late Move Reduction (LMR)
		// Search late quiet moves at reduced depth first
		if movesSearched > 4 && depth >= 3 && !inCheck && !isCapture && !isPromotion {
			// Calculate reduction
			reduction := 1
			if movesSearched > 10 {
				reduction = 2
			}
			if depth > 6 {
				reduction++
			}

			reducedDepth := newDepth - reduction
			if reducedDepth < 1 {
				reducedDepth = 1
			}

			// Search with reduction (null window)
			score = -s.negamax(reducedDepth, ply+1, -alpha-1, -alpha, move)

			// If score is promising, re-search at full depth with full window
			if score > alpha {
				score = -s.negamax(newDepth, ply+1, -beta, -alpha, move)
			}
		} else if movesSearched == 1 {
			// PVS: First move - search with full window
			score = -s.negamax(newDepth, ply+1, -beta, -alpha, move)
		} else {
			// PVS: Later moves - search with zero window first
			score = -s.negamax(newDepth, ply+1, -alpha-1, -alpha, move)

			// If zero window fails high, re-search with full window
			if score > alpha && score < beta {
				score = -s.negamax(newDepth, ply+1, -beta, -alpha, move)
			}
		}

		// Pop position hash before unmake
		s.posHistory = s.posHistory[:len(s.posHistory)-1]

		// Unmake move
		s.pos.UnmakeMove(move, s.undoStack[ply])

		// Check for stop
		if s.stopFlag.Load() {
			return 0
		}

		if score > bestScore {
			bestScore = score
			bestMove = move

			if score > alpha {
				alpha = score
				flag = TTExact

				// Update PV
				s.pv.moves[ply][ply] = move
				for j := ply + 1; j < s.pv.length[ply+1]; j++ {
					s.pv.moves[ply][j] = s.pv.moves[ply+1][j]
				}
				s.pv.length[ply] = s.pv.length[ply+1]
			}
		}

		// Beta cutoff
		if score >= beta {
			// At root, ensure PV is populated before returning
			if ply == 0 && bestMove != board.NoMove {
				s.pv.moves[0][0] = bestMove
				s.pv.length[0] = 1
			}

			// Store in TT
			s.tt.Store(s.pos.Hash, depth, AdjustScoreToTT(score, ply), TTLowerBound, bestMove)

			if isCapture {
				// Update capture history for successful captures
				attackerPiece := s.pos.PieceAt(move.From())
				var capturedType board.PieceType
				if move.IsEnPassant() {
					capturedType = board.Pawn
				} else {
					capturedPiece := s.pos.PieceAt(move.To())
					if capturedPiece != board.NoPiece {
						capturedType = capturedPiece.Type()
					}
				}
				s.orderer.UpdateCaptureHistory(attackerPiece, move.To(), capturedType, depth, true)
			} else {
				// Update killer, history, counter-move, and CMH for quiet moves
				s.orderer.UpdateKillers(move, ply)
				s.orderer.UpdateHistory(move, depth, true)
				s.orderer.UpdateCounterMove(prevMove, move, s.pos)

				// Update countermove history
				if prevMove != board.NoMove {
					prevPiece := s.pos.PieceAt(prevMove.To())
					movePiece := s.pos.PieceAt(move.To()) // Piece is now at 'to' after move
					s.orderer.UpdateCountermoveHistory(prevMove, move, prevPiece, movePiece, depth, true)
				}
			}

			return score
		}
	}

	// Safety: ensure we return a valid move if legal moves exist
	if bestMove == board.NoMove && moves.Len() > 0 {
		bestMove = moves.Get(0)
		if bestScore == -Infinity {
			bestScore = alpha
		}
	}

	// Store in TT
	s.tt.Store(s.pos.Hash, depth, AdjustScoreToTT(bestScore, ply), flag, bestMove)

	return bestScore
}

// quiescence searches captures (and checks at qPly 0) to avoid horizon effect.
func (s *Searcher) quiescence(ply int, alpha, beta int) int {
	return s.quiescenceInternal(ply, 0, alpha, beta)
}

// quiescenceInternal is the internal quiescence search with qPly tracking.
func (s *Searcher) quiescenceInternal(ply, qPly int, alpha, beta int) int {
	// Depth limit to prevent infinite recursion
	const maxQuiescencePly = 32
	if ply >= MaxPly || qPly > maxQuiescencePly {
		return s.evaluate()
	}

	// Check for stop
	if s.stopFlag.Load() {
		return 0
	}

	s.nodes++

	// Lazy evaluation: quick material check before full eval
	lazyEval := EvaluateMaterial(s.pos)
	if lazyEval-lazyEvalMargin >= beta {
		return beta
	}
	if lazyEval+lazyEvalMargin <= alpha {
		return alpha
	}

	// Stand pat (evaluate current position)
	standPat := s.evaluate()

	if standPat >= beta {
		return beta
	}

	if standPat > alpha {
		alpha = standPat
	}

	// Delta pruning: if we're very far behind, prune
	bigDelta := QueenValue
	if standPat+bigDelta < alpha {
		return alpha
	}

	// Generate captures only
	moves := s.pos.GenerateCaptures()

	// Score captures using MVV-LVA
	scores := s.orderer.ScoreMoves(s.pos, moves, ply, board.NoMove)

	for i := 0; i < moves.Len(); i++ {
		PickMove(moves, scores, i)
		move := moves.Get(i)

		// Delta pruning for individual moves
		// Skip captures that can't improve alpha significantly
		if !s.pos.InCheck() {
			var captureValue int
			if move.IsEnPassant() {
				captureValue = PawnValue
			} else {
				capturedPiece := s.pos.PieceAt(move.To())
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

		// Make move
		undo := s.pos.MakeMove(move)

		// Skip if move was invalid (no piece at from square)
		if !undo.Valid {
			continue
		}

		// Recursive search
		score := -s.quiescenceInternal(ply+1, qPly+1, -beta, -alpha)

		// Unmake move
		s.pos.UnmakeMove(move, undo)

		if score >= beta {
			return beta
		}

		if score > alpha {
			alpha = score
		}
	}

	// At first ply of quiescence, also search check-giving moves
	if qPly == 0 && !s.pos.InCheck() {
		checkMoves := s.pos.GenerateChecks()

		for i := 0; i < checkMoves.Len(); i++ {
			move := checkMoves.Get(i)

			// Skip if already searched as capture
			if move.IsCapture(s.pos) {
				continue
			}

			undo := s.pos.MakeMove(move)
			if !undo.Valid {
				continue
			}

			// Verify it actually gives check
			if !s.pos.InCheck() {
				s.pos.UnmakeMove(move, undo)
				continue
			}

			score := -s.quiescenceInternal(ply+1, qPly+1, -beta, -alpha)
			s.pos.UnmakeMove(move, undo)

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

// isDraw checks for draw by repetition or 50-move rule.
func (s *Searcher) isDraw() bool {
	// 50-move rule
	if s.pos.HalfMoveClock >= 100 {
		return true
	}

	// Insufficient material
	if s.pos.IsInsufficientMaterial() {
		return true
	}

	// Threefold repetition
	// Current position hash is the last one in posHistory
	// Count how many times it appears
	if len(s.posHistory) > 0 {
		currentHash := s.pos.Hash
		count := 0
		for _, h := range s.posHistory {
			if h == currentHash {
				count++
				if count >= 2 {
					// Current position + 2 in history = 3 total
					return true
				}
			}
		}
	}

	return false
}

// singularSearch performs a search excluding a specific move.
// Used for singular extension verification.
func (s *Searcher) singularSearch(depth, ply int, alpha, beta int, prevMove, excludedMove board.Move) int {
	moves := s.pos.GenerateLegalMoves()

	bestScore := -Infinity

	for i := 0; i < moves.Len(); i++ {
		move := moves.Get(i)

		// Skip the excluded move
		if move == excludedMove {
			continue
		}

		// Make move
		s.undoStack[ply] = s.pos.MakeMove(move)
		if !s.undoStack[ply].Valid {
			continue
		}

		// Track position hash for repetition detection
		s.posHistory = append(s.posHistory, s.pos.Hash)

		// Search with null window
		score := -s.negamax(depth-1, ply+1, -beta, -alpha, move)

		// Pop position hash before unmake
		s.posHistory = s.posHistory[:len(s.posHistory)-1]

		s.pos.UnmakeMove(move, s.undoStack[ply])

		if score > bestScore {
			bestScore = score
		}

		// Beta cutoff
		if score >= beta {
			return score
		}
	}

	if bestScore == -Infinity {
		return alpha // No moves searched
	}

	return bestScore
}

// GetPV returns the principal variation from the last search.
func (s *Searcher) GetPV() []board.Move {
	pv := make([]board.Move, s.pv.length[0])
	for i := 0; i < s.pv.length[0]; i++ {
		pv[i] = s.pv.moves[0][i]
	}
	return pv
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// isExcludedRootMove checks if a move is in the excluded list (for Multi-PV).
func (s *Searcher) isExcludedRootMove(move board.Move) bool {
	for _, excluded := range s.excludedRootMoves {
		if move == excluded {
			return true
		}
	}
	return false
}

// detectSeriousThreats checks if opponent has serious threats against our pieces.
// Returns true if high-value pieces are attacked and cannot be adequately defended.
func (s *Searcher) detectSeriousThreats() bool {
	pos := s.pos
	us := pos.SideToMove
	them := us.Other()
	occupied := pos.AllOccupied

	// Compute enemy attack map
	enemyPawnAttacks := computePawnAttacksBB(pos, them)
	enemyKnightAttacks := computeKnightAttacksBB(pos, them)
	enemyBishopAttacks := computeBishopAttacksBB(pos, them, occupied)
	enemyRookAttacks := computeRookAttacksBB(pos, them, occupied)
	enemyQueenAttacks := computeQueenAttacksBB(pos, them, occupied)

	enemyAttacks := enemyPawnAttacks | enemyKnightAttacks | enemyBishopAttacks |
		enemyRookAttacks | enemyQueenAttacks

	// Compute our defense map
	ourPawnAttacks := computePawnAttacksBB(pos, us)
	ourKnightAttacks := computeKnightAttacksBB(pos, us)
	ourBishopAttacks := computeBishopAttacksBB(pos, us, occupied)
	ourRookAttacks := computeRookAttacksBB(pos, us, occupied)
	ourQueenAttacks := computeQueenAttacksBB(pos, us, occupied)
	ourKingAttacks := board.KingAttacks(pos.KingSquare[us])

	ourDefenses := ourPawnAttacks | ourKnightAttacks | ourBishopAttacks |
		ourRookAttacks | ourQueenAttacks | ourKingAttacks

	// Check for attacked pieces (excluding king)
	ourPieces := pos.Occupied[us] &^ board.SquareBB(pos.KingSquare[us])

	// Hanging pieces: attacked but not defended
	hangingPieces := ourPieces & enemyAttacks & ^ourDefenses

	// Check if any hanging piece is worth extending for
	for hangingPieces != 0 {
		sq := hangingPieces.PopLSB()
		piece := pos.PieceAt(sq)
		if piece != board.NoPiece && pieceValues[piece.Type()] >= threatExtensionThreshold {
			return true
		}
	}

	// Also check for pieces attacked by lower-value attackers (even if defended)
	// e.g., queen attacked by bishop
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
