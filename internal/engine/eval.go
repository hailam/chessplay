// Package engine implements the chess AI search engine.
package engine

import (
	"github.com/hailam/chessplay/internal/board"
)

// Evaluation constants
const (
	PawnValue   = 100
	KnightValue = 320
	BishopValue = 330
	RookValue   = 500
	QueenValue  = 900
	KingValue   = 20000
)

// Piece values array for quick lookup
var pieceValues = [7]int{PawnValue, KnightValue, BishopValue, RookValue, QueenValue, KingValue, 0}

// Passed pawn bonuses by rank (from pawn's perspective)
// Index 0 = rank 2, Index 6 = rank 8 (about to promote)
var passedPawnBonus = [8]int{0, 10, 20, 40, 70, 120, 200, 0}

const (
	passedPawnConnectedBonus = 20 // Connected passed pawns
	passedPawnProtectedBonus = 15 // Protected by own pawn
	passedPawnFreePathBonus  = 30 // No blockers in front
)

// Mobility weights per piece type
var mobilityMgWeight = [6]int{0, 4, 5, 2, 1, 0} // Pawn, Knight, Bishop, Rook, Queen, King
var mobilityEgWeight = [6]int{0, 3, 4, 4, 2, 0}

// King safety weights per attacker type
var attackerWeight = [6]int{0, 20, 20, 40, 80, 0} // Pawn, Knight, Bishop, Rook, Queen, King

const (
	pawnShieldBonus      = 10  // Bonus per pawn in front of king
	pawnShieldMissing    = -15 // Penalty per missing shield pawn
	openFileNearKing     = -20 // Penalty for open file near king
	semiOpenFileNearKing = -10 // Penalty for semi-open file
)

// Bishop pair bonus (having two bishops)
const (
	bishopPairMgBonus = 25
	bishopPairEgBonus = 50
)

// Rook on open/semi-open file bonuses
const (
	rookOpenFileMg     = 20
	rookOpenFileEg     = 25
	rookSemiOpenFileMg = 10
	rookSemiOpenFileEg = 15
)

// Pawn structure penalties
const (
	doubledPawnMgPenalty  = -15
	doubledPawnEgPenalty  = -20
	isolatedPawnMgPenalty = -20
	isolatedPawnEgPenalty = -25
	backwardPawnMgPenalty = -15
	backwardPawnEgPenalty = -10
)

// Outpost bonuses
const (
	knightOutpostMg          = 25
	knightOutpostEg          = 15
	knightOutpostProtectedMg = 15
	knightOutpostProtectedEg = 10
	bishopOutpostMg          = 15
	bishopOutpostEg          = 10
)

// Tempo bonus - small advantage for having the move
const tempoBonus = 10

// Threat evaluation constants
const (
	hangingPiecePenalty = -40 // Undefended piece attacked by enemy
	threatByPawnBonus   = 25  // Attacking enemy piece with pawn
	threatByMinorBonus  = 20  // Attacking enemy major with minor
	loosePiecePenalty   = -10 // Undefended piece (potential target)
)

// Piece-Square Tables (PST) for positional evaluation
// Values are from White's perspective; mirrored for Black

// Pawn PST - encourages central control and advancement
var pawnPST = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	50, 50, 50, 50, 50, 50, 50, 50,
	10, 10, 20, 30, 30, 20, 10, 10,
	5, 5, 10, 25, 25, 10, 5, 5,
	0, 0, 0, 20, 20, 0, 0, 0,
	5, -5, -10, 0, 0, -10, -5, 5,
	5, 10, 10, -20, -20, 10, 10, 5,
	0, 0, 0, 0, 0, 0, 0, 0,
}

// Knight PST - encourages central positioning
var knightPST = [64]int{
	-50, -40, -30, -30, -30, -30, -40, -50,
	-40, -20, 0, 0, 0, 0, -20, -40,
	-30, 0, 10, 15, 15, 10, 0, -30,
	-30, 5, 15, 20, 20, 15, 5, -30,
	-30, 0, 15, 20, 20, 15, 0, -30,
	-30, 5, 10, 15, 15, 10, 5, -30,
	-40, -20, 0, 5, 5, 0, -20, -40,
	-50, -40, -30, -30, -30, -30, -40, -50,
}

// Bishop PST - encourages central diagonals
var bishopPST = [64]int{
	-20, -10, -10, -10, -10, -10, -10, -20,
	-10, 0, 0, 0, 0, 0, 0, -10,
	-10, 0, 5, 10, 10, 5, 0, -10,
	-10, 5, 5, 10, 10, 5, 5, -10,
	-10, 0, 10, 10, 10, 10, 0, -10,
	-10, 10, 10, 10, 10, 10, 10, -10,
	-10, 5, 0, 0, 0, 0, 5, -10,
	-20, -10, -10, -10, -10, -10, -10, -20,
}

// Rook PST - encourages 7th rank and open files
var rookPST = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	5, 10, 10, 10, 10, 10, 10, 5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	0, 0, 0, 5, 5, 0, 0, 0,
}

// Queen PST - slight central preference
var queenPST = [64]int{
	-20, -10, -10, -5, -5, -10, -10, -20,
	-10, 0, 0, 0, 0, 0, 0, -10,
	-10, 0, 5, 5, 5, 5, 0, -10,
	-5, 0, 5, 5, 5, 5, 0, -5,
	0, 0, 5, 5, 5, 5, 0, -5,
	-10, 5, 5, 5, 5, 5, 0, -10,
	-10, 0, 5, 0, 0, 0, 0, -10,
	-20, -10, -10, -5, -5, -10, -10, -20,
}

// King PST (middlegame) - encourages castling
var kingMidgamePST = [64]int{
	-30, -40, -40, -50, -50, -40, -40, -30,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-20, -30, -30, -40, -40, -30, -30, -20,
	-10, -20, -20, -20, -20, -20, -20, -10,
	20, 20, 0, 0, 0, 0, 20, 20,
	20, 30, 10, 0, 0, 10, 30, 20,
}

// King PST (endgame) - king should be active
var kingEndgamePST = [64]int{
	-50, -40, -30, -20, -20, -30, -40, -50,
	-30, -20, -10, 0, 0, -10, -20, -30,
	-30, -10, 20, 30, 30, 20, -10, -30,
	-30, -10, 30, 40, 40, 30, -10, -30,
	-30, -10, 30, 40, 40, 30, -10, -30,
	-30, -10, 20, 30, 30, 20, -10, -30,
	-30, -30, 0, 0, 0, 0, -30, -30,
	-50, -30, -30, -30, -30, -30, -30, -50,
}

// All PSTs combined for easy lookup
var psts = [...][64]int{
	pawnPST, knightPST, bishopPST, rookPST, queenPST, kingMidgamePST,
}

// Evaluate returns the static evaluation of the position from White's perspective.
func Evaluate(pos *board.Position) int {
	var mgScore, egScore int // Middlegame and endgame scores
	var phase int             // Game phase (for tapered eval)

	// Evaluate material and positional factors for both sides
	for c := board.White; c <= board.Black; c++ {
		sign := 1
		if c == board.Black {
			sign = -1
		}

		for pt := board.Pawn; pt <= board.King; pt++ {
			bb := pos.Pieces[c][pt]
			for bb != 0 {
				sq := bb.PopLSB()

				// Material
				mgScore += sign * pieceValues[pt]
				egScore += sign * pieceValues[pt]

				// Piece-square tables
				pstSq := sq
				if c == board.Black {
					pstSq = sq.Mirror() // Mirror for black
				}

				if pt == board.King {
					mgScore += sign * kingMidgamePST[pstSq]
					egScore += sign * kingEndgamePST[pstSq]
				} else {
					pstValue := psts[pt][pstSq]
					mgScore += sign * pstValue
					egScore += sign * pstValue
				}

				// Phase calculation (for tapered eval)
				switch pt {
				case board.Knight, board.Bishop:
					phase += 1
				case board.Rook:
					phase += 2
				case board.Queen:
					phase += 4
				}
			}
		}
	}

	// Passed pawn evaluation
	ppMg, ppEg := evaluatePassedPawns(pos)
	mgScore += ppMg
	egScore += ppEg

	// Mobility evaluation
	mobMg, mobEg := evaluateMobility(pos)
	mgScore += mobMg
	egScore += mobEg

	// King safety evaluation (middlegame focused)
	kingSafety := evaluateKingSafety(pos)
	mgScore += kingSafety

	// Bishop pair bonus
	bpMg, bpEg := evaluateBishopPair(pos)
	mgScore += bpMg
	egScore += bpEg

	// Rook on open files
	rfMg, rfEg := evaluateRooksOnFiles(pos)
	mgScore += rfMg
	egScore += rfEg

	// Pawn structure (doubled, isolated, backward)
	psMg, psEg := evaluatePawnStructure(pos)
	mgScore += psMg
	egScore += psEg

	// Outposts
	opMg, opEg := evaluateOutposts(pos)
	mgScore += opMg
	egScore += opEg

	// Threat evaluation (hanging/loose pieces)
	thrMg, thrEg := evaluateThreats(pos)
	mgScore += thrMg
	egScore += thrEg

	// Tapered evaluation (interpolate between middlegame and endgame)
	// Maximum phase = 2*4 + 2*1 + 2*1 + 2*2 = 16 per side = 32 total
	const maxPhase = 24
	if phase > maxPhase {
		phase = maxPhase
	}

	score := (mgScore*phase + egScore*(maxPhase-phase)) / maxPhase

	// Tempo bonus: side to move has slight initiative advantage
	score += tempoBonus

	// Return score from side to move's perspective
	if pos.SideToMove == board.Black {
		return -score
	}
	return score
}

// EvaluateMaterial returns just the material balance (for quick evaluation).
func EvaluateMaterial(pos *board.Position) int {
	score := 0
	for pt := board.Pawn; pt < board.King; pt++ {
		score += pos.Pieces[board.White][pt].PopCount() * pieceValues[pt]
		score -= pos.Pieces[board.Black][pt].PopCount() * pieceValues[pt]
	}
	if pos.SideToMove == board.Black {
		return -score
	}
	return score
}

// IsEndgame returns true if the position is in the endgame phase.
func IsEndgame(pos *board.Position) bool {
	// Simple heuristic: endgame if both sides have no queens
	// or total material (excluding kings) is low
	whiteQueens := pos.Pieces[board.White][board.Queen].PopCount()
	blackQueens := pos.Pieces[board.Black][board.Queen].PopCount()

	if whiteQueens == 0 && blackQueens == 0 {
		return true
	}

	// Count total non-pawn, non-king material
	whitePieces := pos.Pieces[board.White][board.Knight].PopCount() +
		pos.Pieces[board.White][board.Bishop].PopCount() +
		pos.Pieces[board.White][board.Rook].PopCount()
	blackPieces := pos.Pieces[board.Black][board.Knight].PopCount() +
		pos.Pieces[board.Black][board.Bishop].PopCount() +
		pos.Pieces[board.Black][board.Rook].PopCount()

	return whiteQueens+blackQueens <= 1 && whitePieces+blackPieces <= 4
}

// isPassedPawn checks if a pawn at the given square is a passed pawn.
// A passed pawn has no enemy pawns blocking or attacking its path to promotion.
func isPassedPawn(pos *board.Position, sq board.Square, color board.Color) bool {
	file := sq.File()
	enemyPawns := pos.Pieces[color.Other()][board.Pawn]

	// Create a mask for the files that matter (same file and adjacent files)
	fileMask := board.FileMask[file]
	if file > 0 {
		fileMask |= board.FileMask[file-1]
	}
	if file < 7 {
		fileMask |= board.FileMask[file+1]
	}

	// Create a mask for ranks in front of the pawn
	var frontMask board.Bitboard
	if color == board.White {
		frontMask = board.SquareBB(sq).NorthFill() &^ board.SquareBB(sq)
	} else {
		frontMask = board.SquareBB(sq).SouthFill() &^ board.SquareBB(sq)
	}

	// Check if any enemy pawns are in the blocking zone
	blockingZone := fileMask & frontMask
	return (enemyPawns & blockingZone) == 0
}

// evaluatePassedPawns returns the passed pawn evaluation bonus.
func evaluatePassedPawns(pos *board.Position) (mgBonus, egBonus int) {
	for color := board.White; color <= board.Black; color++ {
		sign := 1
		if color == board.Black {
			sign = -1
		}

		pawns := pos.Pieces[color][board.Pawn]
		friendlyPawns := pawns

		for pawns != 0 {
			sq := pawns.PopLSB()

			if !isPassedPawn(pos, sq, color) {
				continue
			}

			// Get relative rank (0-7 from pawn's perspective)
			relRank := sq.RelativeRank(color)

			// Base bonus by rank
			bonus := passedPawnBonus[relRank]

			// Check if protected by own pawn
			pawnAttackers := board.PawnAttacks(sq, color.Other()) & friendlyPawns
			if pawnAttackers != 0 {
				bonus += passedPawnProtectedBonus
			}

			// Check for connected passed pawns (adjacent file)
			file := sq.File()
			var adjacentFiles board.Bitboard
			if file > 0 {
				adjacentFiles |= board.FileMask[file-1]
			}
			if file < 7 {
				adjacentFiles |= board.FileMask[file+1]
			}
			connectedPawns := friendlyPawns & adjacentFiles
			for temp := connectedPawns; temp != 0; {
				connSq := temp.PopLSB()
				if isPassedPawn(pos, connSq, color) {
					bonus += passedPawnConnectedBonus
					break
				}
			}

			// Check if path is free (no pieces blocking)
			var frontSquares board.Bitboard
			if color == board.White {
				frontSquares = board.SquareBB(sq).NorthFill() &^ board.SquareBB(sq)
			} else {
				frontSquares = board.SquareBB(sq).SouthFill() &^ board.SquareBB(sq)
			}
			frontSquares &= board.FileMask[file] // Only check same file
			if (frontSquares & pos.AllOccupied) == 0 {
				bonus += passedPawnFreePathBonus
			}

			// Endgame bonus is higher (passed pawns more valuable in endgame)
			mgBonus += sign * bonus
			egBonus += sign * (bonus * 3 / 2) // 50% higher in endgame
		}
	}

	return mgBonus, egBonus
}

// evaluateMobility calculates mobility scores for all pieces.
// Returns middlegame and endgame bonuses.
func evaluateMobility(pos *board.Position) (mgBonus, egBonus int) {
	occupied := pos.AllOccupied

	for color := board.White; color <= board.Black; color++ {
		sign := 1
		if color == board.Black {
			sign = -1
		}

		// Calculate squares attacked by enemy pawns (unsafe squares)
		enemyPawns := pos.Pieces[color.Other()][board.Pawn]
		var unsafeSquares board.Bitboard
		if color == board.White {
			// Black pawn attacks (southeast and southwest)
			unsafeSquares = enemyPawns.SouthEast() | enemyPawns.SouthWest()
		} else {
			// White pawn attacks (northeast and northwest)
			unsafeSquares = enemyPawns.NorthEast() | enemyPawns.NorthWest()
		}

		// Also exclude squares occupied by own pieces
		ownPieces := pos.Occupied[color]
		blockedSquares := unsafeSquares | ownPieces

		// Knights
		knights := pos.Pieces[color][board.Knight]
		for knights != 0 {
			sq := knights.PopLSB()
			attacks := board.KnightAttacks(sq)
			safeSquares := attacks &^ blockedSquares
			count := safeSquares.PopCount()
			mgBonus += sign * mobilityMgWeight[board.Knight] * count
			egBonus += sign * mobilityEgWeight[board.Knight] * count
		}

		// Bishops
		bishops := pos.Pieces[color][board.Bishop]
		for bishops != 0 {
			sq := bishops.PopLSB()
			attacks := board.BishopAttacks(sq, occupied)
			safeSquares := attacks &^ blockedSquares
			count := safeSquares.PopCount()
			mgBonus += sign * mobilityMgWeight[board.Bishop] * count
			egBonus += sign * mobilityEgWeight[board.Bishop] * count
		}

		// Rooks
		rooks := pos.Pieces[color][board.Rook]
		for rooks != 0 {
			sq := rooks.PopLSB()
			attacks := board.RookAttacks(sq, occupied)
			safeSquares := attacks &^ blockedSquares
			count := safeSquares.PopCount()
			mgBonus += sign * mobilityMgWeight[board.Rook] * count
			egBonus += sign * mobilityEgWeight[board.Rook] * count
		}

		// Queens
		queens := pos.Pieces[color][board.Queen]
		for queens != 0 {
			sq := queens.PopLSB()
			attacks := board.QueenAttacks(sq, occupied)
			safeSquares := attacks &^ blockedSquares
			count := safeSquares.PopCount()
			mgBonus += sign * mobilityMgWeight[board.Queen] * count
			egBonus += sign * mobilityEgWeight[board.Queen] * count
		}
	}

	return mgBonus, egBonus
}

// evaluateKingSafety evaluates king safety for both sides.
// Returns middlegame score (king safety matters less in endgame).
func evaluateKingSafety(pos *board.Position) int {
	var score int
	occupied := pos.AllOccupied

	for color := board.White; color <= board.Black; color++ {
		sign := 1
		if color == board.Black {
			sign = -1
		}

		kingSq := pos.KingSquare[color]
		kingFile := kingSq.File()

		// Define king zone (3x3 area around king, extended forward)
		kingZone := board.KingAttacks(kingSq) | board.SquareBB(kingSq)

		// Extend zone forward (toward enemy)
		if color == board.White {
			kingZone |= kingZone.North()
		} else {
			kingZone |= kingZone.South()
		}

		enemy := color.Other()

		// Count attackers to king zone
		attackerCount := 0
		attackWeight := 0

		// Enemy knights attacking king zone
		enemyKnights := pos.Pieces[enemy][board.Knight]
		for temp := enemyKnights; temp != 0; {
			sq := temp.PopLSB()
			attacks := board.KnightAttacks(sq)
			if attacks&kingZone != 0 {
				attackerCount++
				attackWeight += attackerWeight[board.Knight]
			}
		}

		// Enemy bishops attacking king zone
		enemyBishops := pos.Pieces[enemy][board.Bishop]
		for temp := enemyBishops; temp != 0; {
			sq := temp.PopLSB()
			attacks := board.BishopAttacks(sq, occupied)
			if attacks&kingZone != 0 {
				attackerCount++
				attackWeight += attackerWeight[board.Bishop]
			}
		}

		// Enemy rooks attacking king zone
		enemyRooks := pos.Pieces[enemy][board.Rook]
		for temp := enemyRooks; temp != 0; {
			sq := temp.PopLSB()
			attacks := board.RookAttacks(sq, occupied)
			if attacks&kingZone != 0 {
				attackerCount++
				attackWeight += attackerWeight[board.Rook]
			}
		}

		// Enemy queens attacking king zone
		enemyQueens := pos.Pieces[enemy][board.Queen]
		for temp := enemyQueens; temp != 0; {
			sq := temp.PopLSB()
			attacks := board.QueenAttacks(sq, occupied)
			if attacks&kingZone != 0 {
				attackerCount++
				attackWeight += attackerWeight[board.Queen]
			}
		}

		// Scale attack weight by number of attackers (more attackers = exponentially worse)
		if attackerCount >= 2 {
			attackWeight = attackWeight * attackerCount / 2
		}
		score -= sign * attackWeight

		// Pawn shield evaluation
		ownPawns := pos.Pieces[color][board.Pawn]
		enemyFilePawns := pos.Pieces[enemy][board.Pawn]

		// Define pawn shield area (files around king)
		for f := kingFile - 1; f <= kingFile+1; f++ {
			if f < 0 || f > 7 {
				continue
			}

			filePawns := ownPawns & board.FileMask[f]
			enemyOnFile := enemyFilePawns & board.FileMask[f]

			// Check for shield pawn on second rank
			var shieldRank int
			if color == board.White {
				shieldRank = 1 // Rank 2
			} else {
				shieldRank = 6 // Rank 7
			}

			shieldMask := board.FileMask[f] & board.RankMask[shieldRank]
			if ownPawns&shieldMask != 0 {
				score += sign * pawnShieldBonus
			} else if filePawns == 0 {
				score += sign * pawnShieldMissing
			}

			// Check for open/semi-open files toward king
			if filePawns == 0 && enemyOnFile == 0 {
				score += sign * openFileNearKing
			} else if filePawns == 0 {
				score += sign * semiOpenFileNearKing
			}
		}
	}

	return score
}

// SEE (Static Exchange Evaluation) estimates the result of a capture sequence.
// Returns the estimated material gain/loss from the perspective of the moving side.
// This is a proper implementation that simulates the entire capture sequence.
func SEE(pos *board.Position, m board.Move) int {
	from := m.From()
	to := m.To()

	attacker := pos.PieceAt(from)
	if attacker == board.NoPiece {
		return 0
	}

	// Get initial capture value
	var capturedValue int
	if m.IsEnPassant() {
		capturedValue = PawnValue
	} else {
		victim := pos.PieceAt(to)
		if victim == board.NoPiece {
			return 0 // Not a capture
		}
		capturedValue = pieceValues[victim.Type()]
	}

	// Add promotion bonus if applicable
	if m.IsPromotion() {
		capturedValue += pieceValues[m.Promotion()] - PawnValue
	}

	// Use the swap algorithm for SEE
	// This simulates captures alternating between sides
	return seeSwap(pos, to, from, attacker, capturedValue)
}

// seeSwap performs the SEE swap algorithm.
// It simulates alternating captures on the target square.
func seeSwap(pos *board.Position, target, excludeFrom board.Square, firstAttacker board.Piece, initialGain int) int {
	// Gain array for the swap algorithm
	var gain [32]int
	d := 0 // Depth in swap sequence

	// Start with initial capture gain
	gain[d] = initialGain

	// Occupied bitboard, excluding the initial attacker
	occupied := pos.AllOccupied &^ board.SquareBB(excludeFrom)

	// Current attacker info
	attackerValue := pieceValues[firstAttacker.Type()]
	side := firstAttacker.Color().Other() // Next side to capture

	// Find all attackers and simulate capture sequence
	for {
		d++

		// Gain at this depth is the attacker value minus what opponent gains after
		gain[d] = attackerValue - gain[d-1]

		// If we're clearly winning, we can stop (opponent won't recapture)
		if max(-gain[d-1], gain[d]) < 0 {
			break
		}

		// Find least valuable attacker for this side
		attackerSq, attackerPiece := getLeastValuableAttacker(pos, target, side, occupied)
		if attackerSq == board.NoSquare {
			break // No more attackers
		}

		// Remove attacker from occupied
		occupied &^= board.SquareBB(attackerSq)

		// Update attacker value and switch sides
		attackerValue = pieceValues[attackerPiece.Type()]
		side = side.Other()

		// Check for x-ray attackers revealed
		// (handled implicitly by getLeastValuableAttacker using updated occupied)
	}

	// Negamax the gain array to get final result
	for d--; d > 0; d-- {
		gain[d-1] = -max(-gain[d-1], gain[d])
	}

	return gain[0]
}

// getLeastValuableAttacker finds the least valuable piece attacking a square.
// Returns NoSquare if no attacker found.
func getLeastValuableAttacker(pos *board.Position, target board.Square, side board.Color, occupied board.Bitboard) (board.Square, board.Piece) {
	// Check attackers in order of value (pawn first, king last)

	// Pawns
	pawns := pos.Pieces[side][board.Pawn]
	pawnAttacks := board.PawnAttacks(target, side.Other()) // Squares that attack target
	attackers := pawns & pawnAttacks & occupied
	if attackers != 0 {
		sq := attackers.LSB()
		return sq, board.NewPiece(board.Pawn, side)
	}

	// Knights
	knights := pos.Pieces[side][board.Knight]
	knightAttacks := board.KnightAttacks(target)
	attackers = knights & knightAttacks & occupied
	if attackers != 0 {
		sq := attackers.LSB()
		return sq, board.NewPiece(board.Knight, side)
	}

	// Bishops (and diagonal queen attacks)
	bishops := pos.Pieces[side][board.Bishop]
	bishopAttacks := board.BishopAttacks(target, occupied)
	attackers = bishops & bishopAttacks & occupied
	if attackers != 0 {
		sq := attackers.LSB()
		return sq, board.NewPiece(board.Bishop, side)
	}

	// Rooks (and straight queen attacks)
	rooks := pos.Pieces[side][board.Rook]
	rookAttacks := board.RookAttacks(target, occupied)
	attackers = rooks & rookAttacks & occupied
	if attackers != 0 {
		sq := attackers.LSB()
		return sq, board.NewPiece(board.Rook, side)
	}

	// Queens (check both diagonal and straight)
	queens := pos.Pieces[side][board.Queen]
	attackers = queens & (bishopAttacks | rookAttacks) & occupied
	if attackers != 0 {
		sq := attackers.LSB()
		return sq, board.NewPiece(board.Queen, side)
	}

	// King (only if no other attackers, king captures last)
	kingBB := pos.Pieces[side][board.King]
	kingAttacks := board.KingAttacks(target)
	attackers = kingBB & kingAttacks & occupied
	if attackers != 0 {
		sq := attackers.LSB()
		return sq, board.NewPiece(board.King, side)
	}

	return board.NoSquare, board.NoPiece
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// evaluateBishopPair returns bonus for having the bishop pair.
func evaluateBishopPair(pos *board.Position) (mgBonus, egBonus int) {
	for color := board.White; color <= board.Black; color++ {
		sign := 1
		if color == board.Black {
			sign = -1
		}

		bishops := pos.Pieces[color][board.Bishop]
		if bishops.PopCount() >= 2 {
			mgBonus += sign * bishopPairMgBonus
			egBonus += sign * bishopPairEgBonus
		}
	}
	return mgBonus, egBonus
}

// evaluateRooksOnFiles returns bonus for rooks on open/semi-open files.
func evaluateRooksOnFiles(pos *board.Position) (mgBonus, egBonus int) {
	for color := board.White; color <= board.Black; color++ {
		sign := 1
		if color == board.Black {
			sign = -1
		}

		ownPawns := pos.Pieces[color][board.Pawn]
		enemyPawns := pos.Pieces[color.Other()][board.Pawn]

		rooks := pos.Pieces[color][board.Rook]
		for rooks != 0 {
			sq := rooks.PopLSB()
			file := sq.File()
			fileMask := board.FileMask[file]

			hasOwnPawn := (ownPawns & fileMask) != 0
			hasEnemyPawn := (enemyPawns & fileMask) != 0

			if !hasOwnPawn {
				if !hasEnemyPawn {
					// Open file
					mgBonus += sign * rookOpenFileMg
					egBonus += sign * rookOpenFileEg
				} else {
					// Semi-open file
					mgBonus += sign * rookSemiOpenFileMg
					egBonus += sign * rookSemiOpenFileEg
				}
			}
		}
	}
	return mgBonus, egBonus
}

// evaluatePawnStructure evaluates pawn structure defects.
func evaluatePawnStructure(pos *board.Position) (mgPenalty, egPenalty int) {
	for color := board.White; color <= board.Black; color++ {
		sign := 1
		if color == board.Black {
			sign = -1
		}

		pawns := pos.Pieces[color][board.Pawn]
		allPawns := pawns

		for pawns != 0 {
			sq := pawns.PopLSB()
			file := sq.File()
			fileMask := board.FileMask[file]

			// Doubled pawns: more than one pawn on same file
			pawnsOnFile := allPawns & fileMask
			if pawnsOnFile.PopCount() > 1 {
				// Only count penalty once per doubled pair
				// Check if this is the forward pawn
				var forwardPawn board.Square
				if color == board.White {
					// White's forward pawn has higher rank
					forwardPawn = pawnsOnFile.MSB()
				} else {
					// Black's forward pawn has lower rank
					forwardPawn = pawnsOnFile.LSB()
				}
				if sq == forwardPawn {
					mgPenalty += sign * doubledPawnMgPenalty
					egPenalty += sign * doubledPawnEgPenalty
				}
			}

			// Isolated pawns: no friendly pawns on adjacent files
			var adjacentFiles board.Bitboard
			if file > 0 {
				adjacentFiles |= board.FileMask[file-1]
			}
			if file < 7 {
				adjacentFiles |= board.FileMask[file+1]
			}
			if (allPawns & adjacentFiles) == 0 {
				mgPenalty += sign * isolatedPawnMgPenalty
				egPenalty += sign * isolatedPawnEgPenalty
				continue // Isolated pawns can't be backward
			}

			// Backward pawns: behind adjacent pawns and can't safely advance
			relRank := sq.RelativeRank(color)
			if relRank > 1 { // Not on starting rank
				// Check if we're behind adjacent pawns
				var behindMask board.Bitboard
				if color == board.White {
					// Ranks below this pawn's rank
					for r := 0; r < sq.Rank(); r++ {
						behindMask |= board.RankMask[r]
					}
				} else {
					// Ranks above this pawn's rank
					for r := sq.Rank() + 1; r < 8; r++ {
						behindMask |= board.RankMask[r]
					}
				}

				adjacentPawns := allPawns & adjacentFiles
				if adjacentPawns != 0 && (adjacentPawns&behindMask) == adjacentPawns {
					// All adjacent pawns are behind us - this pawn is not backward
					continue
				}

				// Check if the stop square is attacked by enemy pawns
				var stopSq board.Square
				if color == board.White {
					stopSq = sq + 8
				} else {
					stopSq = sq - 8
				}
				if stopSq.IsValid() {
					enemyPawnAttacks := board.PawnAttacks(stopSq, color)
					enemyPawns := pos.Pieces[color.Other()][board.Pawn]
					if (enemyPawns & enemyPawnAttacks) != 0 {
						// Stop square is controlled by enemy pawn - this is backward
						mgPenalty += sign * backwardPawnMgPenalty
						egPenalty += sign * backwardPawnEgPenalty
					}
				}
			}
		}
	}
	return mgPenalty, egPenalty
}

// evaluateOutposts evaluates knight and bishop outposts.
func evaluateOutposts(pos *board.Position) (mgBonus, egBonus int) {
	for color := board.White; color <= board.Black; color++ {
		sign := 1
		if color == board.Black {
			sign = -1
		}

		ownPawns := pos.Pieces[color][board.Pawn]
		enemyPawns := pos.Pieces[color.Other()][board.Pawn]

		// Define outpost ranks (4-6 for white, 3-5 for black)
		var outpostRanks board.Bitboard
		if color == board.White {
			outpostRanks = board.RankMask[3] | board.RankMask[4] | board.RankMask[5] // Ranks 4, 5, 6
		} else {
			outpostRanks = board.RankMask[2] | board.RankMask[3] | board.RankMask[4] // Ranks 3, 4, 5
		}

		// Calculate squares that can't be attacked by enemy pawns
		// A square is safe from enemy pawns if there are no enemy pawns on adjacent files
		// that can advance to attack it

		// Knights on outposts
		knights := pos.Pieces[color][board.Knight] & outpostRanks
		for knights != 0 {
			sq := knights.PopLSB()
			file := sq.File()

			// Check if this square can be attacked by enemy pawns
			// Look at adjacent files for enemy pawns that could attack this square
			var attackers board.Bitboard
			if file > 0 {
				attackers |= board.FileMask[file-1]
			}
			if file < 7 {
				attackers |= board.FileMask[file+1]
			}

			// Enemy pawns that could potentially attack this square
			// (must be on ranks behind this square from enemy's perspective)
			var potentialAttackers board.Bitboard
			if color == board.White {
				// Enemy pawns below this rank can advance to attack
				for r := 0; r <= sq.Rank(); r++ {
					potentialAttackers |= board.RankMask[r]
				}
			} else {
				// Enemy pawns above this rank can advance to attack
				for r := sq.Rank(); r < 8; r++ {
					potentialAttackers |= board.RankMask[r]
				}
			}

			if (enemyPawns & attackers & potentialAttackers) == 0 {
				// This is an outpost - no enemy pawns can attack it
				mgBonus += sign * knightOutpostMg
				egBonus += sign * knightOutpostEg

				// Extra bonus if protected by own pawn
				pawnDefenders := board.PawnAttacks(sq, color.Other()) & ownPawns
				if pawnDefenders != 0 {
					mgBonus += sign * knightOutpostProtectedMg
					egBonus += sign * knightOutpostProtectedEg
				}
			}
		}

		// Bishops on outposts (smaller bonus)
		bishops := pos.Pieces[color][board.Bishop] & outpostRanks
		for bishops != 0 {
			sq := bishops.PopLSB()
			file := sq.File()

			var attackers board.Bitboard
			if file > 0 {
				attackers |= board.FileMask[file-1]
			}
			if file < 7 {
				attackers |= board.FileMask[file+1]
			}

			var potentialAttackers board.Bitboard
			if color == board.White {
				for r := 0; r <= sq.Rank(); r++ {
					potentialAttackers |= board.RankMask[r]
				}
			} else {
				for r := sq.Rank(); r < 8; r++ {
					potentialAttackers |= board.RankMask[r]
				}
			}

			if (enemyPawns & attackers & potentialAttackers) == 0 {
				mgBonus += sign * bishopOutpostMg
				egBonus += sign * bishopOutpostEg
			}
		}
	}
	return mgBonus, egBonus
}

// evaluateThreats evaluates threats and hanging pieces.
func evaluateThreats(pos *board.Position) (mgBonus, egBonus int) {
	occupied := pos.AllOccupied

	for color := board.White; color <= board.Black; color++ {
		sign := 1
		if color == board.Black {
			sign = -1
		}

		enemy := color.Other()

		// Compute attack maps for our side
		ourPawnAttacks := computePawnAttacksBB(pos, color)
		ourKnightAttacks := computeKnightAttacksBB(pos, color)
		ourBishopAttacks := computeBishopAttacksBB(pos, color, occupied)
		ourRookAttacks := computeRookAttacksBB(pos, color, occupied)
		ourQueenAttacks := computeQueenAttacksBB(pos, color, occupied)
		ourKingAttacks := board.KingAttacks(pos.KingSquare[color])

		ourAttacks := ourPawnAttacks | ourKnightAttacks | ourBishopAttacks |
			ourRookAttacks | ourQueenAttacks | ourKingAttacks

		// Compute attack maps for enemy side
		enemyPawnAttacks := computePawnAttacksBB(pos, enemy)
		enemyKnightAttacks := computeKnightAttacksBB(pos, enemy)
		enemyBishopAttacks := computeBishopAttacksBB(pos, enemy, occupied)
		enemyRookAttacks := computeRookAttacksBB(pos, enemy, occupied)
		enemyQueenAttacks := computeQueenAttacksBB(pos, enemy, occupied)
		enemyKingAttacks := board.KingAttacks(pos.KingSquare[enemy])

		enemyAttacks := enemyPawnAttacks | enemyKnightAttacks | enemyBishopAttacks |
			enemyRookAttacks | enemyQueenAttacks | enemyKingAttacks

		// Evaluate threats TO us (penalties)
		ourPieces := pos.Occupied[color] &^ board.SquareBB(pos.KingSquare[color])

		// Hanging pieces: our pieces attacked by enemy but not defended by us
		hangingPieces := ourPieces & enemyAttacks & ^ourAttacks
		hangingCount := hangingPieces.PopCount()
		mgBonus += sign * hangingCount * hangingPiecePenalty
		egBonus += sign * hangingCount * (hangingPiecePenalty * 3 / 2) // Worse in endgame

		// Loose pieces: our pieces not defended (potential future targets)
		loosePieces := ourPieces & ^ourAttacks
		looseCount := loosePieces.PopCount()
		mgBonus += sign * looseCount * loosePiecePenalty

		// Evaluate threats BY us (bonuses)
		enemyPieces := pos.Occupied[enemy] &^ board.SquareBB(pos.KingSquare[enemy])

		// Pawn threats to enemy pieces (very strong)
		pawnThreats := enemyPieces & ourPawnAttacks & ^pos.Pieces[enemy][board.Pawn]
		threatCount := pawnThreats.PopCount()
		mgBonus += sign * threatCount * threatByPawnBonus
		egBonus += sign * threatCount * threatByPawnBonus

		// Minor piece threats to enemy major pieces (rooks/queens)
		minorAttacks := ourKnightAttacks | ourBishopAttacks
		majorPieces := pos.Pieces[enemy][board.Rook] | pos.Pieces[enemy][board.Queen]
		minorThreats := majorPieces & minorAttacks
		threatCount = minorThreats.PopCount()
		mgBonus += sign * threatCount * threatByMinorBonus
		egBonus += sign * threatCount * threatByMinorBonus
	}

	return mgBonus, egBonus
}

// Helper functions for computing attack bitboards

func computePawnAttacksBB(pos *board.Position, color board.Color) board.Bitboard {
	pawns := pos.Pieces[color][board.Pawn]
	if color == board.White {
		return pawns.NorthEast() | pawns.NorthWest()
	}
	return pawns.SouthEast() | pawns.SouthWest()
}

func computeKnightAttacksBB(pos *board.Position, color board.Color) board.Bitboard {
	knights := pos.Pieces[color][board.Knight]
	var attacks board.Bitboard
	for knights != 0 {
		sq := knights.PopLSB()
		attacks |= board.KnightAttacks(sq)
	}
	return attacks
}

func computeBishopAttacksBB(pos *board.Position, color board.Color, occupied board.Bitboard) board.Bitboard {
	bishops := pos.Pieces[color][board.Bishop]
	var attacks board.Bitboard
	for bishops != 0 {
		sq := bishops.PopLSB()
		attacks |= board.BishopAttacks(sq, occupied)
	}
	return attacks
}

func computeRookAttacksBB(pos *board.Position, color board.Color, occupied board.Bitboard) board.Bitboard {
	rooks := pos.Pieces[color][board.Rook]
	var attacks board.Bitboard
	for rooks != 0 {
		sq := rooks.PopLSB()
		attacks |= board.RookAttacks(sq, occupied)
	}
	return attacks
}

func computeQueenAttacksBB(pos *board.Position, color board.Color, occupied board.Bitboard) board.Bitboard {
	queens := pos.Pieces[color][board.Queen]
	var attacks board.Bitboard
	for queens != 0 {
		sq := queens.PopLSB()
		attacks |= board.QueenAttacks(sq, occupied)
	}
	return attacks
}
