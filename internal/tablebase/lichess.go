package tablebase

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hailam/chessplay/internal/board"
)

// LichessProber uses the Lichess tablebase API for online lookups.
// Note: This requires network access and has rate limits.
// For production use, consider local Syzygy files with CGO bindings.
type LichessProber struct {
	client    *http.Client
	maxPieces int
}

// NewLichessProber creates a new Lichess-based tablebase prober.
func NewLichessProber() *LichessProber {
	return &LichessProber{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		maxPieces: 7, // Lichess supports up to 7-piece tablebases
	}
}

// Lichess API response structure
type lichessResponse struct {
	Category string `json:"category"` // "win", "draw", "maybe-win", "maybe-draw", "loss"
	DTZ      int    `json:"dtz"`
	Moves    []struct {
		UCI      string `json:"uci"`
		Category string `json:"category"`
		DTZ      int    `json:"dtz"`
	} `json:"moves"`
}

func (lp *LichessProber) Probe(pos *board.Position) ProbeResult {
	if CountPieces(pos) > lp.maxPieces {
		return ProbeResult{Found: false}
	}

	fen := pos.ToFEN()
	// URL encode the FEN (spaces become underscores for Lichess)
	fen = strings.ReplaceAll(fen, " ", "_")

	url := fmt.Sprintf("https://tablebase.lichess.ovh/standard?fen=%s", fen)
	resp, err := lp.client.Get(url)
	if err != nil {
		return ProbeResult{Found: false}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ProbeResult{Found: false}
	}

	var result lichessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ProbeResult{Found: false}
	}

	return ProbeResult{
		Found: true,
		WDL:   categoryToWDL(result.Category),
		DTZ:   result.DTZ,
	}
}

func (lp *LichessProber) ProbeRoot(pos *board.Position) RootResult {
	if CountPieces(pos) > lp.maxPieces {
		return RootResult{Found: false}
	}

	fen := pos.ToFEN()
	fen = strings.ReplaceAll(fen, " ", "_")

	url := fmt.Sprintf("https://tablebase.lichess.ovh/standard?fen=%s", fen)
	resp, err := lp.client.Get(url)
	if err != nil {
		return RootResult{Found: false}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return RootResult{Found: false}
	}

	var result lichessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return RootResult{Found: false}
	}

	if len(result.Moves) == 0 {
		return RootResult{Found: false}
	}

	// Find the best move
	bestMove := result.Moves[0]
	move := parseUCIMove(pos, bestMove.UCI)
	if move == board.NoMove {
		return RootResult{Found: false}
	}

	return RootResult{
		Found: true,
		Move:  move,
		WDL:   categoryToWDL(bestMove.Category),
		DTZ:   bestMove.DTZ,
	}
}

func (lp *LichessProber) MaxPieces() int {
	return lp.maxPieces
}

func (lp *LichessProber) Available() bool {
	return true // Always available if network is up
}

func categoryToWDL(category string) WDL {
	switch category {
	case "win":
		return WDLWin
	case "maybe-win":
		return WDLCursedWin
	case "draw":
		return WDLDraw
	case "maybe-draw", "cursed-win", "blessed-loss":
		return WDLDraw // Treat ambiguous as draw for safety
	case "loss":
		return WDLLoss
	default:
		return WDLDraw
	}
}

// parseUCIMove converts a UCI move string to a board.Move.
func parseUCIMove(pos *board.Position, uci string) board.Move {
	if len(uci) < 4 {
		return board.NoMove
	}

	fromFile := int(uci[0] - 'a')
	fromRank := int(uci[1] - '1')
	toFile := int(uci[2] - 'a')
	toRank := int(uci[3] - '1')

	if fromFile < 0 || fromFile > 7 || fromRank < 0 || fromRank > 7 ||
		toFile < 0 || toFile > 7 || toRank < 0 || toRank > 7 {
		return board.NoMove
	}

	from := board.NewSquare(fromFile, fromRank)
	to := board.NewSquare(toFile, toRank)

	// Check for promotion
	var promo board.PieceType
	if len(uci) == 5 {
		switch uci[4] {
		case 'q':
			promo = board.Queen
		case 'r':
			promo = board.Rook
		case 'b':
			promo = board.Bishop
		case 'n':
			promo = board.Knight
		}
	}

	// Find matching legal move
	moves := pos.GenerateLegalMoves()
	for i := 0; i < moves.Len(); i++ {
		m := moves.Get(i)
		if m.From() == from && m.To() == to {
			if promo != 0 {
				if m.IsPromotion() && m.Promotion() == promo {
					return m
				}
			} else if !m.IsPromotion() {
				return m
			}
		}
	}

	return board.NoMove
}
