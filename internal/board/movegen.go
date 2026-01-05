package board

import (
	"fmt"
	"log"
)

// GenerateLegalMoves generates all legal moves for the position.
func (p *Position) GenerateLegalMoves() *MoveList {
	ml := NewMoveList()
	p.generateAllMoves(ml)
	return p.filterLegalMoves(ml)
}

// GeneratePseudoLegalMoves generates all pseudo-legal moves (may leave king in check).
func (p *Position) GeneratePseudoLegalMoves() *MoveList {
	ml := NewMoveList()
	p.generateAllMoves(ml)
	return ml
}

// GenerateCaptures generates all capture moves.
func (p *Position) GenerateCaptures() *MoveList {
	ml := NewMoveList()
	p.generateCaptures(ml)
	return p.filterLegalMoves(ml)
}

// generateAllMoves generates all pseudo-legal moves.
func (p *Position) generateAllMoves(ml *MoveList) {
	us := p.SideToMove
	them := us.Other()
	occupied := p.AllOccupied
	enemies := p.Occupied[them]

	// Validate King position consistency
	if DebugMoveValidation {
		kingBB := p.Pieces[us][King]
		if kingBB == 0 {
			log.Printf("MOVEGEN FATAL: %v King bitboard empty! KingSquare=%v AllOcc=%x Hash=%x",
				us, p.KingSquare[us], uint64(p.AllOccupied), p.Hash)
		} else if p.KingSquare[us] != kingBB.LSB() {
			log.Printf("MOVEGEN FATAL: %v KingSquare=%v but King bitboard says %v! Hash=%x",
				us, p.KingSquare[us], kingBB.LSB(), p.Hash)
		}
	}

	// Pawn moves
	p.generatePawnMoves(ml, us, enemies, occupied)

	// Knight moves
	knights := p.Pieces[us][Knight]
	for knights != 0 {
		from := knights.PopLSB()
		attacks := KnightAttacks(from) & ^p.Occupied[us]
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}

	// Bishop moves
	bishops := p.Pieces[us][Bishop]
	for bishops != 0 {
		from := bishops.PopLSB()
		attacks := BishopAttacks(from, occupied) & ^p.Occupied[us]
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}

	// Rook moves
	rooks := p.Pieces[us][Rook]
	for rooks != 0 {
		from := rooks.PopLSB()
		attacks := RookAttacks(from, occupied) & ^p.Occupied[us]
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}

	// Queen moves
	queens := p.Pieces[us][Queen]
	for queens != 0 {
		from := queens.PopLSB()
		attacks := QueenAttacks(from, occupied) & ^p.Occupied[us]
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}

	// King moves
	p.generateKingMoves(ml, us)

	// Castling
	p.generateCastlingMoves(ml, us)
}

// generatePawnMoves generates all pawn moves.
func (p *Position) generatePawnMoves(ml *MoveList, us Color, enemies, occupied Bitboard) {
	pawns := p.Pieces[us][Pawn]
	empty := ^occupied

	var push1, push2, attackL, attackR Bitboard
	var promotionRank Bitboard
	var pushDir int

	if us == White {
		push1 = pawns.North() & empty
		push2 = (push1 & Rank3).North() & empty
		attackL = pawns.NorthWest() & enemies
		attackR = pawns.NorthEast() & enemies
		promotionRank = Rank8
		pushDir = 8
	} else {
		push1 = pawns.South() & empty
		push2 = (push1 & Rank6).South() & empty
		attackL = pawns.SouthWest() & enemies
		attackR = pawns.SouthEast() & enemies
		promotionRank = Rank1
		pushDir = -8
	}

	// Single pushes (non-promotion)
	nonPromo := push1 & ^promotionRank
	for nonPromo != 0 {
		to := nonPromo.PopLSB()
		from := Square(int(to) - pushDir)
		ml.Add(NewMove(from, to))
	}

	// Double pushes
	for push2 != 0 {
		to := push2.PopLSB()
		from := Square(int(to) - 2*pushDir)
		ml.Add(NewMove(from, to))
	}

	// Captures (non-promotion)
	nonPromoL := attackL & ^promotionRank
	for nonPromoL != 0 {
		to := nonPromoL.PopLSB()
		from := Square(int(to) - pushDir + 1)
		ml.Add(NewMove(from, to))
	}

	nonPromoR := attackR & ^promotionRank
	for nonPromoR != 0 {
		to := nonPromoR.PopLSB()
		from := Square(int(to) - pushDir - 1)
		ml.Add(NewMove(from, to))
	}

	// Promotions
	promoPush := push1 & promotionRank
	for promoPush != 0 {
		to := promoPush.PopLSB()
		from := Square(int(to) - pushDir)
		addPromotions(ml, from, to)
	}

	promoL := attackL & promotionRank
	for promoL != 0 {
		to := promoL.PopLSB()
		from := Square(int(to) - pushDir + 1)
		addPromotions(ml, from, to)
	}

	promoR := attackR & promotionRank
	for promoR != 0 {
		to := promoR.PopLSB()
		from := Square(int(to) - pushDir - 1)
		addPromotions(ml, from, to)
	}

	// En passant
	if p.EnPassant != NoSquare {
		epBB := SquareBB(p.EnPassant)
		var epAttackers Bitboard
		if us == White {
			epAttackers = (epBB.SouthWest() | epBB.SouthEast()) & pawns
		} else {
			epAttackers = (epBB.NorthWest() | epBB.NorthEast()) & pawns
		}
		for epAttackers != 0 {
			from := epAttackers.PopLSB()
			ml.Add(NewEnPassant(from, p.EnPassant))
		}
	}
}

// addPromotions adds all four promotion moves.
func addPromotions(ml *MoveList, from, to Square) {
	ml.Add(NewPromotion(from, to, Queen))
	ml.Add(NewPromotion(from, to, Rook))
	ml.Add(NewPromotion(from, to, Bishop))
	ml.Add(NewPromotion(from, to, Knight))
}

// generateKingMoves generates king moves (non-castling).
func (p *Position) generateKingMoves(ml *MoveList, us Color) {
	// Use actual King bitboard to find King position (defensive against desync)
	kingBB := p.Pieces[us][King]
	if kingBB == 0 {
		// No King on board - skip (this is a corrupted position)
		return
	}
	from := kingBB.LSB()
	attacks := KingAttacks(from) & ^p.Occupied[us]

	for attacks != 0 {
		to := attacks.PopLSB()
		ml.Add(NewMove(from, to))
	}
}

// generateCastlingMoves generates castling moves.
func (p *Position) generateCastlingMoves(ml *MoveList, us Color) {
	them := us.Other()

	if us == White {
		// Kingside (O-O)
		if p.CastlingRights&WhiteKingSideCastle != 0 {
			// Check squares are empty (f1, g1)
			if p.AllOccupied&((1<<F1)|(1<<G1)) == 0 {
				// Check king doesn't pass through check (e1, f1, g1)
				if !p.IsSquareAttacked(E1, them) && !p.IsSquareAttacked(F1, them) && !p.IsSquareAttacked(G1, them) {
					ml.Add(NewCastling(E1, G1))
				}
			}
		}

		// Queenside (O-O-O)
		if p.CastlingRights&WhiteQueenSideCastle != 0 {
			// Check squares are empty (b1, c1, d1)
			if p.AllOccupied&((1<<B1)|(1<<C1)|(1<<D1)) == 0 {
				// Check king doesn't pass through check (c1, d1, e1)
				if !p.IsSquareAttacked(E1, them) && !p.IsSquareAttacked(D1, them) && !p.IsSquareAttacked(C1, them) {
					ml.Add(NewCastling(E1, C1))
				}
			}
		}
	} else {
		// Kingside (O-O)
		if p.CastlingRights&BlackKingSideCastle != 0 {
			// Check squares are empty (f8, g8)
			if p.AllOccupied&((1<<F8)|(1<<G8)) == 0 {
				// Check king doesn't pass through check
				if !p.IsSquareAttacked(E8, them) && !p.IsSquareAttacked(F8, them) && !p.IsSquareAttacked(G8, them) {
					ml.Add(NewCastling(E8, G8))
				}
			}
		}

		// Queenside (O-O-O)
		if p.CastlingRights&BlackQueenSideCastle != 0 {
			// Check squares are empty (b8, c8, d8)
			if p.AllOccupied&((1<<B8)|(1<<C8)|(1<<D8)) == 0 {
				// Check king doesn't pass through check
				if !p.IsSquareAttacked(E8, them) && !p.IsSquareAttacked(D8, them) && !p.IsSquareAttacked(C8, them) {
					ml.Add(NewCastling(E8, C8))
				}
			}
		}
	}
}

// generateCaptures generates capture moves only.
func (p *Position) generateCaptures(ml *MoveList) {
	us := p.SideToMove
	them := us.Other()
	enemies := p.Occupied[them]
	occupied := p.AllOccupied

	// Pawn captures
	pawns := p.Pieces[us][Pawn]
	var attackL, attackR Bitboard
	var promotionRank Bitboard
	var pushDir int

	if us == White {
		attackL = pawns.NorthWest() & enemies
		attackR = pawns.NorthEast() & enemies
		promotionRank = Rank8
		pushDir = 8
	} else {
		attackL = pawns.SouthWest() & enemies
		attackR = pawns.SouthEast() & enemies
		promotionRank = Rank1
		pushDir = -8
	}

	// Non-promotion captures
	nonPromoL := attackL & ^promotionRank
	for nonPromoL != 0 {
		to := nonPromoL.PopLSB()
		from := Square(int(to) - pushDir + 1)
		ml.Add(NewMove(from, to))
	}

	nonPromoR := attackR & ^promotionRank
	for nonPromoR != 0 {
		to := nonPromoR.PopLSB()
		from := Square(int(to) - pushDir - 1)
		ml.Add(NewMove(from, to))
	}

	// Promotion captures
	promoL := attackL & promotionRank
	for promoL != 0 {
		to := promoL.PopLSB()
		from := Square(int(to) - pushDir + 1)
		addPromotions(ml, from, to)
	}

	promoR := attackR & promotionRank
	for promoR != 0 {
		to := promoR.PopLSB()
		from := Square(int(to) - pushDir - 1)
		addPromotions(ml, from, to)
	}

	// Pawn push promotions (technically not captures but important for quiescence)
	empty := ^occupied
	var push1 Bitboard
	if us == White {
		push1 = pawns.North() & empty & Rank8
	} else {
		push1 = pawns.South() & empty & Rank1
	}
	for push1 != 0 {
		to := push1.PopLSB()
		from := Square(int(to) - pushDir)
		addPromotions(ml, from, to)
	}

	// En passant
	if p.EnPassant != NoSquare {
		epBB := SquareBB(p.EnPassant)
		var epAttackers Bitboard
		if us == White {
			epAttackers = (epBB.SouthWest() | epBB.SouthEast()) & pawns
		} else {
			epAttackers = (epBB.NorthWest() | epBB.NorthEast()) & pawns
		}
		for epAttackers != 0 {
			from := epAttackers.PopLSB()
			ml.Add(NewEnPassant(from, p.EnPassant))
		}
	}

	// Knight captures
	knights := p.Pieces[us][Knight]
	for knights != 0 {
		from := knights.PopLSB()
		attacks := KnightAttacks(from) & enemies
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}

	// Bishop captures
	bishops := p.Pieces[us][Bishop]
	for bishops != 0 {
		from := bishops.PopLSB()
		attacks := BishopAttacks(from, occupied) & enemies
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}

	// Rook captures
	rooks := p.Pieces[us][Rook]
	for rooks != 0 {
		from := rooks.PopLSB()
		attacks := RookAttacks(from, occupied) & enemies
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}

	// Queen captures
	queens := p.Pieces[us][Queen]
	for queens != 0 {
		from := queens.PopLSB()
		attacks := QueenAttacks(from, occupied) & enemies
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}

	// King captures
	from := p.KingSquare[us]
	attacks := KingAttacks(from) & enemies
	for attacks != 0 {
		to := attacks.PopLSB()
		ml.Add(NewMove(from, to))
	}
}

// DebugLegalMoveVerification enables dual-path verification in filterLegalMoves.
// Set to true during development to catch any fast path bugs.
var DebugLegalMoveVerification = false

// filterLegalMoves filters out illegal moves using Stockfish's optimization.
// Non-pinned, non-king, non-en-passant moves are automatically legal (when not in check).
func (p *Position) filterLegalMoves(ml *MoveList) *MoveList {
	result := NewMoveList()
	pinned := p.ComputePinned() // Compute once for all moves
	ksq := p.KingSquare[p.SideToMove]
	inCheck := p.Checkers != 0

	for i := 0; i < ml.Len(); i++ {
		m := ml.Get(i)
		from := m.From()

		// When in check, only king moves can use the fast path
		// (other pieces must block or capture, which requires validation)
		if inCheck {
			if p.IsLegalFast(m, pinned) {
				result.Add(m)
			}
			continue
		}

		// Fast path: non-pinned, non-king, non-EP moves are automatically legal
		if from != ksq && !m.IsEnPassant() && pinned&SquareBB(from) == 0 {
			if DebugLegalMoveVerification {
				// Verify fast path against slow path
				slowResult := p.IsLegal(m)
				if !slowResult {
					fmt.Printf("DEBUG MISMATCH: Fast path accepted move %v but slow path rejected it\n", m)
					fmt.Printf("DEBUG: pinned=%v from=%v ksq=%v\n", pinned, from, ksq)
					continue // Trust slow path in debug mode
				}
			}
			result.Add(m)
			continue
		}

		// Slow path: pinned pieces, king moves, or en passant
		if p.IsLegalFast(m, pinned) {
			if DebugLegalMoveVerification {
				// Verify against original slow path
				slowResult := p.IsLegal(m)
				if !slowResult {
					fmt.Printf("DEBUG MISMATCH: IsLegalFast accepted move %v but IsLegal rejected it\n", m)
					continue
				}
			}
			result.Add(m)
		} else if DebugLegalMoveVerification {
			// Check if slow path would have accepted it
			if p.IsLegal(m) {
				fmt.Printf("DEBUG MISMATCH: IsLegalFast rejected move %v but IsLegal accepted it\n", m)
				result.Add(m)
			}
		}
	}

	return result
}

// IsLegalFast returns true if the move is legal using Stockfish's optimization.
// Key insight: non-pinned, non-king, non-en-passant moves are automatically legal.
// This avoids expensive make/unmake for ~90% of moves.
func (p *Position) IsLegalFast(m Move, pinned Bitboard) bool {
	from := m.From()
	to := m.To()
	us := p.SideToMove
	them := us.Other()
	ksq := p.KingSquare[us]
	checkers := p.Checkers

	// King moves: check destination not attacked (with king removed from occupancy)
	if from == ksq {
		if m.IsCastling() {
			// Castling is not allowed when in check (and was validated during generation)
			return checkers == 0
		}
		occ := p.AllOccupied &^ SquareBB(from)
		return p.AttackersByColor(to, them, occ) == 0
	}

	// When in check, non-king moves must block or capture the checker
	if checkers != 0 {
		// Double check: only king can move
		if checkers.PopCount() > 1 {
			return false
		}

		// Single check: must capture checker or block
		checker := checkers.LSB()
		// Valid targets: the checker square OR squares between checker and king
		validTargets := SquareBB(checker) | Between(checker, ksq)

		// En passant special case: the captured pawn might be the checker
		if m.IsEnPassant() {
			var capturedSq Square
			if us == White {
				capturedSq = to - 8
			} else {
				capturedSq = to + 8
			}
			// If en passant captures the checker, it's potentially valid
			// (still need to verify horizontal pin, use slow path)
			if capturedSq == checker {
				return p.isLegalEnPassant(m)
			}
			// Otherwise can't block with en passant
			return false
		}

		// Move must go to a valid target (block or capture)
		if validTargets&SquareBB(to) == 0 {
			return false
		}

		// Also check pin constraint
		if pinned&SquareBB(from) != 0 && !Aligned(from, to, ksq) {
			return false
		}

		return true
	}

	// Not in check - use normal logic

	// En passant: use slow path (horizontal pin edge case where two pawns are removed)
	if m.IsEnPassant() {
		return p.isLegalEnPassant(m)
	}

	// Non-pinned pieces: automatically legal (cannot expose king)
	if pinned&SquareBB(from) == 0 {
		return true
	}

	// Pinned pieces: legal only if moving along the pin ray
	return Aligned(from, to, ksq)
}

// isLegalEnPassant validates en passant moves using make/unmake.
// En passant is special because it removes two pawns, which can expose
// horizontal attacks on the king that aren't detected by the normal pin logic.
func (p *Position) isLegalEnPassant(m Move) bool {
	us := p.SideToMove
	them := us.Other()
	ksq := p.KingSquare[us]

	undo := p.MakeMove(m)
	if !undo.Valid {
		return false
	}

	attacked := p.IsSquareAttacked(ksq, them)
	p.UnmakeMove(m, undo)

	return !attacked
}

// IsLegal returns true if the move is legal (doesn't leave king in check).
// Uses make/unmake for guaranteed correctness. Kept for debugging/validation.
func (p *Position) IsLegal(m Move) bool {
	us := p.SideToMove
	them := us.Other()
	from := m.From()
	ksq := p.KingSquare[us]

	// For king moves, check if destination is attacked
	if from == ksq {
		if m.IsCastling() {
			return true // Already validated in generation
		}
		// King moves: temporarily remove king and check destination
		occ := p.AllOccupied &^ SquareBB(from)
		return p.AttackersByColor(m.To(), them, occ) == 0
	}

	// For all other moves: actually make the move and check
	undo := p.MakeMove(m)
	if !undo.Valid {
		return false
	}

	// Check if OUR king is now attacked
	// After MakeMove, SideToMove is flipped, so "them" is now "us"
	attacked := p.IsSquareAttacked(ksq, them)

	p.UnmakeMove(m, undo)

	return !attacked
}

// GenerateChecks generates non-capture moves that give check.
// Used in quiescence search to find forcing moves beyond captures.
func (p *Position) GenerateChecks() *MoveList {
	ml := NewMoveList()
	p.generateChecks(ml)
	return p.filterLegalMoves(ml)
}

// generateChecks generates pseudo-legal non-capture check-giving moves.
func (p *Position) generateChecks(ml *MoveList) {
	us := p.SideToMove
	them := us.Other()
	enemyKing := p.KingSquare[them]
	occupied := p.AllOccupied
	empty := ^occupied

	// Knight checks: find squares that attack enemy king and move knights there
	knightCheckSquares := KnightAttacks(enemyKing) & empty
	knights := p.Pieces[us][Knight]
	for knights != 0 {
		from := knights.PopLSB()
		attacks := KnightAttacks(from) & knightCheckSquares
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}

	// Bishop checks: find squares on diagonals to enemy king
	bishopCheckSquares := BishopAttacks(enemyKing, occupied) & empty
	bishops := p.Pieces[us][Bishop]
	for bishops != 0 {
		from := bishops.PopLSB()
		attacks := BishopAttacks(from, occupied) & bishopCheckSquares
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}

	// Rook checks: find squares on files/ranks to enemy king
	rookCheckSquares := RookAttacks(enemyKing, occupied) & empty
	rooks := p.Pieces[us][Rook]
	for rooks != 0 {
		from := rooks.PopLSB()
		attacks := RookAttacks(from, occupied) & rookCheckSquares
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}

	// Queen checks: both diagonal and straight
	queenCheckSquares := bishopCheckSquares | rookCheckSquares
	queens := p.Pieces[us][Queen]
	for queens != 0 {
		from := queens.PopLSB()
		attacks := QueenAttacks(from, occupied) & queenCheckSquares
		for attacks != 0 {
			to := attacks.PopLSB()
			ml.Add(NewMove(from, to))
		}
	}
}

// MakeMove applies a move to the position and returns undo information.
func (p *Position) MakeMove(m Move) UndoInfo {
	// DEBUG: Check position BEFORE saving undo
	if DebugMoveValidation {
		us := p.SideToMove
		them := us.Other()
		// Check our King exists
		if p.Pieces[us][King] == 0 {
			log.Printf("MAKEMOVE ENTRY: %v King bitboard empty! move=%v hash=%x", us, m, p.Hash)
		}
		// Check opponent King exists (should never capture King)
		if p.Pieces[them][King] == 0 {
			log.Printf("MAKEMOVE ENTRY: %v (opponent) King bitboard empty! move=%v hash=%x", them, m, p.Hash)
		}
		// Check if move is trying to capture King
		to := m.To()
		capturedPiece := p.PieceAt(to)
		if capturedPiece != NoPiece && capturedPiece.Type() == King {
			log.Printf("MAKEMOVE ILLEGAL: Trying to capture %v King at %v! move=%v hash=%x",
				capturedPiece.Color(), to, m, p.Hash)
		}
	}

	undo := UndoInfo{
		CapturedPiece:  NoPiece,
		CastlingRights: p.CastlingRights,
		EnPassant:      p.EnPassant,
		HalfMoveClock:  p.HalfMoveClock,
		Hash:           p.Hash,
		PawnKey:        p.PawnKey,
		Checkers:       p.Checkers,
		KingSquare:     p.KingSquare,   // Save King positions
		Pieces:         p.Pieces,       // Save all piece bitboards
		Occupied:       p.Occupied,     // Save occupancy bitboards
		AllOccupied:    p.AllOccupied,  // Save all occupied
		Valid:          false,
	}

	us := p.SideToMove
	them := us.Other()
	from := m.From()
	to := m.To()
	piece := p.PieceAt(from)

	// Safety check - if no piece at from square, return without modifying position
	if piece == NoPiece {
		return undo
	}

	// Validate piece belongs to side to move - catches hash collisions and bugs
	if piece.Color() != us {
		if DebugMoveValidation {
			log.Printf("DEBUG: MakeMove - trying to move %v piece when %v to move! Move: %v (from=%v to=%v)",
				piece.Color(), us, m, from, to)
		}
		return undo
	}

	// Mark as valid since we have a piece and will apply the move
	undo.Valid = true
	pt := piece.Type()

	// Update hash for side to move
	p.Hash ^= zobristSideToMove

	// Update hash for castling rights (will be updated again below if they change)
	p.Hash ^= zobristCastling[p.CastlingRights]

	// Update hash for en passant
	if p.EnPassant != NoSquare {
		p.Hash ^= zobristEnPassant[p.EnPassant.File()]
	}

	// Clear en passant
	p.EnPassant = NoSquare

	// Handle captures
	if m.IsEnPassant() {
		// En passant capture
		var capturedSq Square
		if us == White {
			capturedSq = to - 8
		} else {
			capturedSq = to + 8
		}
		undo.CapturedPiece = p.removePiece(capturedSq)
		p.Hash ^= zobristPiece[them][Pawn][capturedSq]
		p.PawnKey ^= zobristPiece[them][Pawn][capturedSq] // Pawn captured
	} else if captured := p.PieceAt(to); captured != NoPiece {
		// Normal capture
		undo.CapturedPiece = captured
		p.removePiece(to)
		p.Hash ^= zobristPiece[them][captured.Type()][to]
		if captured.Type() == Pawn {
			p.PawnKey ^= zobristPiece[them][Pawn][to] // Pawn captured
		}
	}

	// Move the piece
	p.movePiece(from, to)
	p.Hash ^= zobristPiece[us][pt][from]
	p.Hash ^= zobristPiece[us][pt][to]

	// Update pawn key for pawn moves
	if pt == Pawn {
		p.PawnKey ^= zobristPiece[us][Pawn][from]
		p.PawnKey ^= zobristPiece[us][Pawn][to]
	}

	// Handle promotion
	if m.IsPromotion() {
		promoPt := m.Promotion()
		// Remove pawn, add promoted piece
		p.Pieces[us][Pawn] &^= SquareBB(to)
		p.Pieces[us][promoPt] |= SquareBB(to)
		p.Hash ^= zobristPiece[us][Pawn][to]
		p.Hash ^= zobristPiece[us][promoPt][to]
		// Pawn is removed from board (promoted), so remove from pawn key
		p.PawnKey ^= zobristPiece[us][Pawn][to]
	}

	// Handle castling
	if m.IsCastling() {
		var rookFrom, rookTo Square
		if to > from {
			// Kingside
			rookFrom = NewSquare(7, from.Rank())
			rookTo = NewSquare(5, from.Rank())
		} else {
			// Queenside
			rookFrom = NewSquare(0, from.Rank())
			rookTo = NewSquare(3, from.Rank())
		}
		p.movePiece(rookFrom, rookTo)
		p.Hash ^= zobristPiece[us][Rook][rookFrom]
		p.Hash ^= zobristPiece[us][Rook][rookTo]
	}

	// Update castling rights
	if pt == King {
		if us == White {
			p.CastlingRights &^= WhiteKingSideCastle | WhiteQueenSideCastle
		} else {
			p.CastlingRights &^= BlackKingSideCastle | BlackQueenSideCastle
		}
	}

	// Rook moves or captures affect castling
	if from == A1 || to == A1 {
		p.CastlingRights &^= WhiteQueenSideCastle
	}
	if from == H1 || to == H1 {
		p.CastlingRights &^= WhiteKingSideCastle
	}
	if from == A8 || to == A8 {
		p.CastlingRights &^= BlackQueenSideCastle
	}
	if from == H8 || to == H8 {
		p.CastlingRights &^= BlackKingSideCastle
	}

	// Update hash for new castling rights
	p.Hash ^= zobristCastling[p.CastlingRights]

	// Set en passant square for double pawn push
	if pt == Pawn && abs(int(to)-int(from)) == 16 {
		epSquare := Square((int(from) + int(to)) / 2)
		p.EnPassant = epSquare
		p.Hash ^= zobristEnPassant[epSquare.File()]
	}

	// Update half-move clock
	if pt == Pawn || undo.CapturedPiece != NoPiece {
		p.HalfMoveClock = 0
	} else {
		p.HalfMoveClock++
	}

	// Update full-move number
	if us == Black {
		p.FullMoveNumber++
	}

	// Switch side to move
	p.SideToMove = them

	// Update checkers (for the side now to move)
	p.UpdateCheckers()

	// CRITICAL: Verify the side that just moved didn't leave their King in check
	// This catches illegal moves that slipped through move generation
	usKingSq := p.KingSquare[us]
	if p.IsSquareAttacked(usKingSq, them) {
		// Move is illegal - leaves own King in check
		if DebugMoveValidation {
			log.Printf("MAKEMOVE ILLEGAL: %v left King at %v in check! move=%v hash=%x",
				us, usKingSq, m, p.Hash)
		}
		undo.Valid = false
	}

	return undo
}

// UnmakeMove undoes a move using the stored undo information.
// Uses full position restoration to avoid issues with movePiece failures.
func (p *Position) UnmakeMove(m Move, undo UndoInfo) {
	us := p.SideToMove.Other()

	// Directly restore all position state from undo
	p.CastlingRights = undo.CastlingRights
	p.EnPassant = undo.EnPassant
	p.HalfMoveClock = undo.HalfMoveClock
	p.Hash = undo.Hash
	p.PawnKey = undo.PawnKey
	p.Checkers = undo.Checkers
	p.KingSquare = undo.KingSquare
	p.Pieces = undo.Pieces
	p.Occupied = undo.Occupied
	p.AllOccupied = undo.AllOccupied
	p.SideToMove = us

	if us == Black {
		p.FullMoveNumber--
	}
}

// HasLegalMoves returns true if the side to move has any legal moves.
func (p *Position) HasLegalMoves() bool {
	ml := p.GeneratePseudoLegalMoves()
	pinned := p.ComputePinned()
	for i := 0; i < ml.Len(); i++ {
		if p.IsLegalFast(ml.Get(i), pinned) {
			return true
		}
	}
	return false
}

// IsCheckmate returns true if the position is checkmate.
func (p *Position) IsCheckmate() bool {
	return p.InCheck() && !p.HasLegalMoves()
}

// IsStalemate returns true if the position is stalemate.
func (p *Position) IsStalemate() bool {
	return !p.InCheck() && !p.HasLegalMoves()
}

// IsDraw returns true if the position is a draw (stalemate, 50-move, insufficient material).
func (p *Position) IsDraw() bool {
	if p.IsStalemate() {
		return true
	}
	if p.HalfMoveClock >= 100 {
		return true
	}
	return p.IsInsufficientMaterial()
}

// IsInsufficientMaterial returns true if neither side can checkmate.
func (p *Position) IsInsufficientMaterial() bool {
	// If there are any pawns, rooks, or queens, sufficient material
	if p.Pieces[White][Pawn]|p.Pieces[Black][Pawn] != 0 ||
		p.Pieces[White][Rook]|p.Pieces[Black][Rook] != 0 ||
		p.Pieces[White][Queen]|p.Pieces[Black][Queen] != 0 {
		return false
	}

	// Count minor pieces
	wKnights := p.Pieces[White][Knight].PopCount()
	wBishops := p.Pieces[White][Bishop].PopCount()
	bKnights := p.Pieces[Black][Knight].PopCount()
	bBishops := p.Pieces[Black][Bishop].PopCount()

	// K vs K
	if wKnights+wBishops+bKnights+bBishops == 0 {
		return true
	}

	// K+minor vs K
	if wKnights+wBishops <= 1 && bKnights+bBishops == 0 {
		return true
	}
	if bKnights+bBishops <= 1 && wKnights+wBishops == 0 {
		return true
	}

	return false
}
