package board

import (
	"strings"
)

// ToSAN converts a move to Standard Algebraic Notation.
func (m Move) ToSAN(pos *Position) string {
	if m == NoMove {
		return "-"
	}

	from := m.From()
	to := m.To()
	piece := pos.PieceAt(from)

	if piece == NoPiece {
		return m.String() // Fallback to UCI
	}

	var sb strings.Builder

	// Castling
	if m.IsCastling() {
		if to > from {
			return "O-O" // Kingside
		}
		return "O-O-O" // Queenside
	}

	pt := piece.Type()

	// Piece letter (not for pawns)
	if pt != Pawn {
		sb.WriteByte("PNBRQK"[pt])
	}

	// Disambiguation
	if pt != Pawn {
		disambig := getDisambiguation(pos, m, pt)
		sb.WriteString(disambig)
	}

	// Capture marker
	isCapture := m.IsCapture(pos)
	if isCapture {
		if pt == Pawn {
			// Pawn captures include the file of origin
			sb.WriteByte('a' + byte(from.File()))
		}
		sb.WriteByte('x')
	}

	// Destination square
	sb.WriteString(to.String())

	// Promotion
	if m.IsPromotion() {
		sb.WriteByte('=')
		sb.WriteByte("PNBRQK"[m.Promotion()])
	}

	// Check/checkmate marker
	// Make the move temporarily to check
	newPos := pos.Copy()
	newPos.MakeMove(m)
	if newPos.IsCheckmate() {
		sb.WriteByte('#')
	} else if newPos.InCheck() {
		sb.WriteByte('+')
	}

	return sb.String()
}

// getDisambiguation returns the disambiguation string needed for a move.
func getDisambiguation(pos *Position, m Move, pt PieceType) string {
	from := m.From()
	to := m.To()
	us := pos.SideToMove

	// Find all pieces of the same type that can move to the same square
	var candidates []Square

	// Get all pieces of this type
	pieces := pos.Pieces[us][pt]

	// Generate legal moves for each piece
	allMoves := pos.GenerateLegalMoves()
	for i := 0; i < allMoves.Len(); i++ {
		move := allMoves.Get(i)
		if move.To() != to {
			continue
		}

		moveFrom := move.From()
		if moveFrom == from {
			continue // Skip the move itself
		}

		// Check if this piece is the same type
		if pieces.IsSet(moveFrom) {
			candidates = append(candidates, moveFrom)
		}
	}

	// No ambiguity
	if len(candidates) == 0 {
		return ""
	}

	// Check if file is sufficient
	sameFile := false
	sameRank := false
	for _, sq := range candidates {
		if sq.File() == from.File() {
			sameFile = true
		}
		if sq.Rank() == from.Rank() {
			sameRank = true
		}
	}

	// Determine disambiguation
	if !sameFile {
		// File is sufficient
		return string('a' + byte(from.File()))
	}
	if !sameRank {
		// Rank is sufficient
		return string('1' + byte(from.Rank()))
	}
	// Need both file and rank
	return from.String()
}

// ParseSAN parses a SAN string and returns the corresponding move.
func ParseSAN(s string, pos *Position) (Move, error) {
	s = strings.TrimSpace(s)

	// Handle castling
	if s == "O-O" || s == "0-0" {
		if pos.SideToMove == White {
			return NewCastling(E1, G1), nil
		}
		return NewCastling(E8, G8), nil
	}
	if s == "O-O-O" || s == "0-0-0" {
		if pos.SideToMove == White {
			return NewCastling(E1, C1), nil
		}
		return NewCastling(E8, C8), nil
	}

	// Remove check/checkmate markers
	s = strings.TrimSuffix(s, "+")
	s = strings.TrimSuffix(s, "#")

	// Parse promotion
	var promoPiece PieceType = NoPieceType
	if idx := strings.Index(s, "="); idx >= 0 {
		promoChar := s[idx+1]
		switch promoChar {
		case 'N':
			promoPiece = Knight
		case 'B':
			promoPiece = Bishop
		case 'R':
			promoPiece = Rook
		case 'Q':
			promoPiece = Queen
		}
		s = s[:idx]
	}

	// Remove capture marker
	isCapture := strings.Contains(s, "x")
	s = strings.Replace(s, "x", "", -1)

	// Determine piece type
	pt := Pawn
	if len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z' {
		switch s[0] {
		case 'N':
			pt = Knight
		case 'B':
			pt = Bishop
		case 'R':
			pt = Rook
		case 'Q':
			pt = Queen
		case 'K':
			pt = King
		}
		s = s[1:]
	}

	// Parse destination (last 2 characters)
	if len(s) < 2 {
		return NoMove, nil
	}
	destStr := s[len(s)-2:]
	dest, err := ParseSquare(destStr)
	if err != nil {
		return NoMove, err
	}
	s = s[:len(s)-2]

	// Parse disambiguation (file, rank, or both)
	var disambigFile, disambigRank int = -1, -1
	for _, c := range s {
		if c >= 'a' && c <= 'h' {
			disambigFile = int(c - 'a')
		} else if c >= '1' && c <= '8' {
			disambigRank = int(c - '1')
		}
	}

	// Find the matching move
	moves := pos.GenerateLegalMoves()
	for i := 0; i < moves.Len(); i++ {
		m := moves.Get(i)
		if m.To() != dest {
			continue
		}

		from := m.From()
		piece := pos.PieceAt(from)
		if piece.Type() != pt {
			continue
		}

		// Check disambiguation
		if disambigFile >= 0 && from.File() != disambigFile {
			continue
		}
		if disambigRank >= 0 && from.Rank() != disambigRank {
			continue
		}

		// Check capture
		if isCapture && !m.IsCapture(pos) {
			continue
		}

		// Check promotion
		if promoPiece != NoPieceType {
			if !m.IsPromotion() || m.Promotion() != promoPiece {
				continue
			}
		}

		return m, nil
	}

	return NoMove, nil
}

// MovesToSAN converts a slice of moves to SAN notation.
func MovesToSAN(pos *Position, moves []Move) []string {
	result := make([]string, len(moves))
	p := pos.Copy()

	for i, m := range moves {
		result[i] = m.ToSAN(p)
		p.MakeMove(m)
	}

	return result
}
