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

// PVTable stores the principal variation.
type PVTable struct {
	length [MaxPly]int
	moves  [MaxPly][MaxPly]board.Move
}

// Searcher performs the alpha-beta search.
type Searcher struct {
	pos     *board.Position
	tt      *TranspositionTable
	orderer *MoveOrderer

	// Search state
	nodes    uint64
	stopFlag atomic.Bool

	// PV tracking
	pv PVTable

	// Undo stack
	undoStack [MaxPly]board.UndoInfo
}

// NewSearcher creates a new searcher.
func NewSearcher(tt *TranspositionTable) *Searcher {
	return &Searcher{
		tt:      tt,
		orderer: NewMoveOrderer(),
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

// Search performs the search at the given depth.
func (s *Searcher) Search(pos *board.Position, depth int) (board.Move, int) {
	s.pos = pos.Copy()
	s.Reset()

	score := s.negamax(depth, 0, -Infinity, Infinity)

	var bestMove board.Move
	if s.pv.length[0] > 0 {
		bestMove = s.pv.moves[0][0]
	}

	return bestMove, score
}

// negamax implements the negamax algorithm with alpha-beta pruning.
func (s *Searcher) negamax(depth, ply int, alpha, beta int) int {
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
		if int(ttEntry.Depth) >= depth {
			score := AdjustScoreFromTT(int(ttEntry.Score), ply)
			switch ttEntry.Flag {
			case TTExact:
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
				return score
			}
		}
	}

	// Quiescence search at depth 0
	if depth <= 0 {
		return s.quiescence(ply, alpha, beta)
	}

	// Check if in check
	inCheck := s.pos.InCheck()

	// Generate moves
	moves := s.pos.GenerateLegalMoves()

	// Check for checkmate or stalemate
	if moves.Len() == 0 {
		if inCheck {
			return -MateScore + ply // Checkmate
		}
		return 0 // Stalemate
	}

	// Score and sort moves
	scores := s.orderer.ScoreMoves(s.pos, moves, ply, ttMove)

	bestScore := -Infinity
	bestMove := board.NoMove
	flag := TTUpperBound

	for i := 0; i < moves.Len(); i++ {
		// Pick the best remaining move
		PickMove(moves, scores, i)
		move := moves.Get(i)

		// Make move
		s.undoStack[ply] = s.pos.MakeMove(move)

		// Skip if move was invalid (no piece at from square)
		if !s.undoStack[ply].Valid {
			continue
		}

		// Recursive search
		score := -s.negamax(depth-1, ply+1, -beta, -alpha)

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
			// Store in TT
			s.tt.Store(s.pos.Hash, depth, AdjustScoreToTT(score, ply), TTLowerBound, bestMove)

			// Update killer and history for quiet moves
			if !move.IsCapture(s.pos) {
				s.orderer.UpdateKillers(move, ply)
				s.orderer.UpdateHistory(move, depth, true)
			}

			return score
		}
	}

	// Store in TT
	s.tt.Store(s.pos.Hash, depth, AdjustScoreToTT(bestScore, ply), flag, bestMove)

	return bestScore
}

// quiescence searches only captures to avoid horizon effect.
func (s *Searcher) quiescence(ply int, alpha, beta int) int {
	// Depth limit to prevent infinite recursion
	const maxQuiescencePly = 32
	if ply >= MaxPly || ply > maxQuiescencePly {
		return Evaluate(s.pos)
	}

	// Check for stop
	if s.stopFlag.Load() {
		return 0
	}

	s.nodes++

	// Stand pat (evaluate current position)
	standPat := Evaluate(s.pos)

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
		score := -s.quiescence(ply+1, -beta, -alpha)

		// Unmake move
		s.pos.UnmakeMove(move, undo)

		if score >= beta {
			return beta
		}

		if score > alpha {
			alpha = score
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

	// Note: Repetition detection would require storing position history
	// For now, we rely on the game-level repetition check

	return false
}

// GetPV returns the principal variation from the last search.
func (s *Searcher) GetPV() []board.Move {
	pv := make([]board.Move, s.pv.length[0])
	for i := 0; i < s.pv.length[0]; i++ {
		pv[i] = s.pv.moves[0][i]
	}
	return pv
}
