package nnue

import "github.com/hailam/chessplay/internal/board"

// PieceIndex maps (PieceType, Color) to a 0-9 index for HalfKP.
// White: P=0, N=1, B=2, R=3, Q=4
// Black: p=5, n=6, b=7, r=8, q=9
func PieceIndex(pt board.PieceType, c board.Color) int {
	if pt == board.King || pt > board.Queen {
		return -1 // Kings not included in features
	}
	base := int(pt) // Pawn=0, Knight=1, Bishop=2, Rook=3, Queen=4
	if c == board.Black {
		base += 5
	}
	return base
}

// HalfKPIndex computes the feature index for a piece from a perspective.
// perspective: the side whose perspective we're computing (their king square matters)
// kingSquare: where the perspective's king is located
// pieceType: type of the non-king piece
// pieceColor: color of the non-king piece
// pieceSquare: where the non-king piece is located
func HalfKPIndex(perspective board.Color, kingSquare board.Square,
	pieceType board.PieceType, pieceColor board.Color,
	pieceSquare board.Square) int {

	// Mirror squares for black's perspective (so black sees the board the same way)
	kingSq := int(kingSquare)
	pieceSq := int(pieceSquare)
	pc := pieceColor

	if perspective == board.Black {
		kingSq = int(kingSquare.Mirror())
		pieceSq = int(pieceSquare.Mirror())
		// Also flip piece color for black's perspective
		pc = pieceColor.Other()
	}

	// Piece index (0-9)
	pi := PieceIndex(pieceType, pc)
	if pi < 0 {
		return -1 // Invalid (king or invalid piece type)
	}

	// Feature index: kingSquare * (pieceTypes * squares) + pieceIndex * squares + pieceSquare
	return kingSq*(NumPieceTypes*NumPieceSquares) + pi*NumPieceSquares + pieceSq
}

// GetActiveFeatures returns all active feature indices for a position from both perspectives.
func GetActiveFeatures(pos *board.Position) (white, black []int) {
	white = make([]int, 0, 32) // Typical piece count
	black = make([]int, 0, 32)

	whiteKingSq := pos.KingSquare[board.White]
	blackKingSq := pos.KingSquare[board.Black]

	// Iterate all pieces except kings
	for color := board.White; color <= board.Black; color++ {
		for pt := board.Pawn; pt < board.King; pt++ {
			pieces := pos.Pieces[color][pt]
			for pieces != 0 {
				sq := pieces.PopLSB()

				// White's perspective
				idx := HalfKPIndex(board.White, whiteKingSq, pt, color, sq)
				if idx >= 0 && idx < HalfKPSize {
					white = append(white, idx)
				}

				// Black's perspective
				idx = HalfKPIndex(board.Black, blackKingSq, pt, color, sq)
				if idx >= 0 && idx < HalfKPSize {
					black = append(black, idx)
				}
			}
		}
	}

	return white, black
}

// GetChangedFeatures returns features that need to be added/removed for a move.
// This is used for incremental accumulator updates.
// Returns: (whiteAdded, whiteRemoved, blackAdded, blackRemoved)
func GetChangedFeatures(pos *board.Position, m board.Move, captured board.Piece) (
	whiteAdd, whiteRem, blackAdd, blackRem []int) {

	whiteKingSq := pos.KingSquare[board.White]
	blackKingSq := pos.KingSquare[board.Black]

	from := m.From()
	to := m.To()
	movedPiece := pos.PieceAt(to) // Piece after move was made

	if movedPiece == board.NoPiece {
		return // Invalid state
	}

	movingPT := movedPiece.Type()
	movingColor := movedPiece.Color()

	// If king moved, we can't do incremental update
	if movingPT == board.King {
		return // Caller should do full refresh
	}

	// Remove feature for piece at old square (from)
	idxW := HalfKPIndex(board.White, whiteKingSq, movingPT, movingColor, from)
	idxB := HalfKPIndex(board.Black, blackKingSq, movingPT, movingColor, from)
	if idxW >= 0 && idxW < HalfKPSize {
		whiteRem = append(whiteRem, idxW)
	}
	if idxB >= 0 && idxB < HalfKPSize {
		blackRem = append(blackRem, idxB)
	}

	// Add feature for piece at new square (to)
	// Handle promotion: use promoted piece type
	addPT := movingPT
	if m.IsPromotion() {
		addPT = m.Promotion()
	}

	idxW = HalfKPIndex(board.White, whiteKingSq, addPT, movingColor, to)
	idxB = HalfKPIndex(board.Black, blackKingSq, addPT, movingColor, to)
	if idxW >= 0 && idxW < HalfKPSize {
		whiteAdd = append(whiteAdd, idxW)
	}
	if idxB >= 0 && idxB < HalfKPSize {
		blackAdd = append(blackAdd, idxB)
	}

	// Handle capture
	if captured != board.NoPiece && captured.Type() != board.King {
		capturedPT := captured.Type()
		capturedColor := captured.Color()
		capturedSq := to // Normal capture

		// En passant: captured pawn is on different square
		if m.IsEnPassant() {
			if movingColor == board.White {
				capturedSq = to - 8
			} else {
				capturedSq = to + 8
			}
		}

		idxW = HalfKPIndex(board.White, whiteKingSq, capturedPT, capturedColor, capturedSq)
		idxB = HalfKPIndex(board.Black, blackKingSq, capturedPT, capturedColor, capturedSq)
		if idxW >= 0 && idxW < HalfKPSize {
			whiteRem = append(whiteRem, idxW)
		}
		if idxB >= 0 && idxB < HalfKPSize {
			blackRem = append(blackRem, idxB)
		}
	}

	return
}
