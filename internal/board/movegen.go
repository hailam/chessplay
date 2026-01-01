package board

import "fmt"

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
	from := p.KingSquare[us]
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

// filterLegalMoves filters out illegal moves (those that leave king in check).
func (p *Position) filterLegalMoves(ml *MoveList) *MoveList {
	result := NewMoveList()

	for i := 0; i < ml.Len(); i++ {
		m := ml.Get(i)
		if p.IsLegal(m) {
			result.Add(m)
		}
	}

	return result
}

// IsLegal returns true if the move is legal (doesn't leave king in check).
// Uses make/unmake for guaranteed correctness.
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

	// DEBUG: Log rejected moves
	if attacked {
		fmt.Printf("DEBUG: Move %v rejected - king on %v attacked by %v after move\n",
			m, ksq, them)
		// Show what's attacking the king
		attackers := p.AttackersByColor(ksq, them, p.AllOccupied)
		fmt.Printf("DEBUG: Attackers bitboard:\n%s\n", attackers.String())
	}

	p.UnmakeMove(m, undo)

	return !attacked
}

// MakeMove applies a move to the position and returns undo information.
func (p *Position) MakeMove(m Move) UndoInfo {
	undo := UndoInfo{
		CapturedPiece:  NoPiece,
		CastlingRights: p.CastlingRights,
		EnPassant:      p.EnPassant,
		HalfMoveClock:  p.HalfMoveClock,
		Hash:           p.Hash,
		Checkers:       p.Checkers,
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
	} else if captured := p.PieceAt(to); captured != NoPiece {
		// Normal capture
		undo.CapturedPiece = captured
		p.removePiece(to)
		p.Hash ^= zobristPiece[them][captured.Type()][to]
	}

	// Move the piece
	p.movePiece(from, to)
	p.Hash ^= zobristPiece[us][pt][from]
	p.Hash ^= zobristPiece[us][pt][to]

	// Handle promotion
	if m.IsPromotion() {
		promoPt := m.Promotion()
		// Remove pawn, add promoted piece
		p.Pieces[us][Pawn] &^= SquareBB(to)
		p.Pieces[us][promoPt] |= SquareBB(to)
		p.Hash ^= zobristPiece[us][Pawn][to]
		p.Hash ^= zobristPiece[us][promoPt][to]
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

	// Update checkers
	p.UpdateCheckers()

	return undo
}

// UnmakeMove undoes a move using the stored undo information.
func (p *Position) UnmakeMove(m Move, undo UndoInfo) {
	them := p.SideToMove
	us := them.Other()
	from := m.From()
	to := m.To()

	// Restore state
	p.CastlingRights = undo.CastlingRights
	p.EnPassant = undo.EnPassant
	p.HalfMoveClock = undo.HalfMoveClock
	p.Hash = undo.Hash
	p.Checkers = undo.Checkers
	p.SideToMove = us

	if us == Black {
		p.FullMoveNumber--
	}

	// Handle promotion first (before moving piece back)
	if m.IsPromotion() {
		promoPt := m.Promotion()
		// Remove promoted piece, restore pawn
		p.Pieces[us][promoPt] &^= SquareBB(to)
		p.Pieces[us][Pawn] |= SquareBB(to)
	}

	// Move piece back
	p.movePiece(to, from)

	// Handle castling rook
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
		p.movePiece(rookTo, rookFrom)
	}

	// Restore captured piece
	if undo.CapturedPiece != NoPiece {
		if m.IsEnPassant() {
			var capturedSq Square
			if us == White {
				capturedSq = to - 8
			} else {
				capturedSq = to + 8
			}
			p.setPiece(undo.CapturedPiece, capturedSq)
		} else {
			p.setPiece(undo.CapturedPiece, to)
		}
	}
}

// HasLegalMoves returns true if the side to move has any legal moves.
func (p *Position) HasLegalMoves() bool {
	ml := p.GeneratePseudoLegalMoves()
	for i := 0; i < ml.Len(); i++ {
		if p.IsLegal(ml.Get(i)) {
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
