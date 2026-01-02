package board

import (
	"fmt"
	"strconv"
	"strings"
)

// StartFEN is the FEN string for the starting position.
const StartFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

// ParseFEN parses a FEN string and returns a Position.
func ParseFEN(fen string) (*Position, error) {
	parts := strings.Fields(fen)
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid FEN: need at least 4 fields, got %d", len(parts))
	}

	pos := &Position{
		EnPassant:      NoSquare,
		FullMoveNumber: 1,
	}
	pos.KingSquare[White] = NoSquare
	pos.KingSquare[Black] = NoSquare

	// Parse piece placement (field 0)
	if err := parsePiecePlacement(pos, parts[0]); err != nil {
		return nil, err
	}

	// Parse side to move (field 1)
	switch parts[1] {
	case "w":
		pos.SideToMove = White
	case "b":
		pos.SideToMove = Black
	default:
		return nil, fmt.Errorf("invalid side to move: %s", parts[1])
	}

	// Parse castling rights (field 2)
	if err := parseCastlingRights(pos, parts[2]); err != nil {
		return nil, err
	}

	// Parse en passant square (field 3)
	if parts[3] != "-" {
		sq, err := ParseSquare(parts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid en passant square: %s", parts[3])
		}
		pos.EnPassant = sq
	}

	// Parse half-move clock (field 4, optional)
	if len(parts) > 4 {
		hmc, err := strconv.Atoi(parts[4])
		if err != nil {
			return nil, fmt.Errorf("invalid half-move clock: %s", parts[4])
		}
		pos.HalfMoveClock = hmc
	}

	// Parse full-move number (field 5, optional)
	if len(parts) > 5 {
		fmn, err := strconv.Atoi(parts[5])
		if err != nil {
			return nil, fmt.Errorf("invalid full-move number: %s", parts[5])
		}
		pos.FullMoveNumber = fmn
	}

	// Update derived state
	pos.updateOccupied()
	pos.findKings()
	pos.Hash = pos.ComputeHash()
	pos.PawnKey = pos.ComputePawnKey()

	return pos, nil
}

// parsePiecePlacement parses the piece placement section of a FEN string.
func parsePiecePlacement(pos *Position, placement string) error {
	ranks := strings.Split(placement, "/")
	if len(ranks) != 8 {
		return fmt.Errorf("invalid piece placement: need 8 ranks, got %d", len(ranks))
	}

	for i, rankStr := range ranks {
		rank := 7 - i // FEN starts from rank 8
		file := 0

		for _, c := range rankStr {
			if file > 7 {
				return fmt.Errorf("too many squares in rank %d", rank+1)
			}

			if c >= '1' && c <= '8' {
				// Skip empty squares
				file += int(c - '0')
			} else {
				// Place a piece
				piece := PieceFromChar(byte(c))
				if piece == NoPiece {
					return fmt.Errorf("invalid piece character: %c", c)
				}
				sq := NewSquare(file, rank)
				pos.setPiece(piece, sq)
				file++
			}
		}

		if file != 8 {
			return fmt.Errorf("invalid number of squares in rank %d: got %d", rank+1, file)
		}
	}

	return nil
}

// parseCastlingRights parses the castling rights section of a FEN string.
func parseCastlingRights(pos *Position, castling string) error {
	if castling == "-" {
		pos.CastlingRights = NoCastling
		return nil
	}

	for _, c := range castling {
		switch c {
		case 'K':
			pos.CastlingRights |= WhiteKingSideCastle
		case 'Q':
			pos.CastlingRights |= WhiteQueenSideCastle
		case 'k':
			pos.CastlingRights |= BlackKingSideCastle
		case 'q':
			pos.CastlingRights |= BlackQueenSideCastle
		default:
			return fmt.Errorf("invalid castling character: %c", c)
		}
	}

	return nil
}

// ToFEN returns the FEN representation of the position.
func (p *Position) ToFEN() string {
	var sb strings.Builder

	// Piece placement
	for rank := 7; rank >= 0; rank-- {
		empty := 0
		for file := 0; file < 8; file++ {
			sq := NewSquare(file, rank)
			piece := p.PieceAt(sq)
			if piece == NoPiece {
				empty++
			} else {
				if empty > 0 {
					sb.WriteString(strconv.Itoa(empty))
					empty = 0
				}
				sb.WriteString(piece.String())
			}
		}
		if empty > 0 {
			sb.WriteString(strconv.Itoa(empty))
		}
		if rank > 0 {
			sb.WriteByte('/')
		}
	}

	// Side to move
	sb.WriteByte(' ')
	if p.SideToMove == White {
		sb.WriteByte('w')
	} else {
		sb.WriteByte('b')
	}

	// Castling rights
	sb.WriteByte(' ')
	sb.WriteString(p.CastlingRights.String())

	// En passant
	sb.WriteByte(' ')
	sb.WriteString(p.EnPassant.String())

	// Half-move clock and full-move number
	sb.WriteByte(' ')
	sb.WriteString(strconv.Itoa(p.HalfMoveClock))
	sb.WriteByte(' ')
	sb.WriteString(strconv.Itoa(p.FullMoveNumber))

	return sb.String()
}

// ComputeHash computes the Zobrist hash for the position from scratch.
// This is a placeholder that will be fully implemented in zobrist.go.
func (p *Position) ComputeHash() uint64 {
	var hash uint64

	// Hash pieces
	for c := White; c <= Black; c++ {
		for pt := Pawn; pt <= King; pt++ {
			bb := p.Pieces[c][pt]
			for bb != 0 {
				sq := bb.PopLSB()
				hash ^= zobristPiece[c][pt][sq]
			}
		}
	}

	// Hash side to move
	if p.SideToMove == Black {
		hash ^= zobristSideToMove
	}

	// Hash castling rights
	hash ^= zobristCastling[p.CastlingRights]

	// Hash en passant
	if p.EnPassant != NoSquare {
		hash ^= zobristEnPassant[p.EnPassant.File()]
	}

	return hash
}

// ComputePawnKey computes the pawn hash key from scratch.
// Only includes pawn positions for pawn structure caching.
func (p *Position) ComputePawnKey() uint64 {
	var key uint64

	for c := White; c <= Black; c++ {
		bb := p.Pieces[c][Pawn]
		for bb != 0 {
			sq := bb.PopLSB()
			key ^= zobristPiece[c][Pawn][sq]
		}
	}

	return key
}
