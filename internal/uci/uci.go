package uci

import (
	"bufio"
	"fmt"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hailam/chessplay/internal/board"
	"github.com/hailam/chessplay/internal/engine"
	"github.com/hailam/chessplay/internal/tablebase"
)

// UCI implements the Universal Chess Interface protocol.
type UCI struct {
	engine   *engine.Engine
	position *board.Position

	// Position history for repetition detection
	positionHashes []uint64

	// NNUE configuration
	nnueBigPath   string
	nnueSmallPath string

	// Syzygy tablebase configuration
	syzygyPath       string
	syzygyProbeDepth int
	syzygyProber     *tablebase.SyzygyProber

	// Search state
	searching     bool
	searchDone    chan struct{}
	stopRequested atomic.Bool

	// CPU profiling
	profileFile *os.File
}

// New creates a new UCI protocol handler.
func New(eng *engine.Engine) *UCI {
	return &UCI{
		engine:   eng,
		position: board.NewPosition(),
	}
}

// Run starts the UCI main loop.
func (u *UCI) Run() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := parts[0]
		args := parts[1:]

		switch cmd {
		case "uci":
			u.handleUCI()
		case "isready":
			fmt.Println("readyok")
		case "ucinewgame":
			u.handleNewGame()
		case "position":
			if board.DebugMoveValidation {
				fmt.Fprintf(os.Stderr, "info string DEBUG: position %s\n", strings.Join(args, " "))
			}
			u.handlePosition(args)
		case "go":
			u.handleGo(args)
		case "stop":
			u.handleStop()
		case "quit":
			u.handleQuit()
		case "setoption":
			u.handleSetOption(args)
		// Debug commands
		case "d":
			fmt.Println(u.position.String())
		case "perft":
			u.handlePerft(args)
		}
	}
}

// handleUCI responds to the "uci" command.
func (u *UCI) handleUCI() {
	fmt.Println("id name ChessPlay")
	fmt.Println("id author ChessPlay Team")
	fmt.Println()
	fmt.Println("option name Hash type spin default 64 min 1 max 4096")
	fmt.Println("option name UseNNUE type check default false")
	fmt.Println("option name EvalFile type string default <empty>")
	fmt.Println("option name EvalFileSmall type string default <empty>")
	fmt.Println("option name SyzygyPath type string default <empty>")
	fmt.Println("option name SyzygyProbeDepth type spin default 1 min 1 max 100")
	fmt.Println("uciok")
}

// handleNewGame resets the engine for a new game.
func (u *UCI) handleNewGame() {
	u.engine.Clear()
	u.position = board.NewPosition()
	u.positionHashes = []uint64{u.position.Hash}
}

// handlePosition parses and sets up a position.
// Formats:
//   - position startpos
//   - position startpos moves e2e4 e7e5
//   - position fen <fen>
//   - position fen <fen> moves e2e4
func (u *UCI) handlePosition(args []string) {
	if len(args) == 0 {
		return
	}

	u.positionHashes = nil
	var moveStart int

	if args[0] == "startpos" {
		u.position = board.NewPosition()
		moveStart = 1
		// Find "moves" keyword
		for i, arg := range args {
			if arg == "moves" {
				moveStart = i + 1
				break
			}
		}
	} else if args[0] == "fen" {
		// Find where FEN ends (at "moves" or end of args)
		fenEnd := len(args)
		for i, arg := range args[1:] {
			if arg == "moves" {
				fenEnd = i + 1
				break
			}
		}

		fenStr := strings.Join(args[1:fenEnd], " ")
		pos, err := board.ParseFEN(fenStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "info string Invalid FEN: %v\n", err)
			return
		}
		u.position = pos

		// Find "moves" keyword
		moveStart = len(args)
		for i, arg := range args {
			if arg == "moves" {
				moveStart = i + 1
				break
			}
		}
	} else {
		return
	}

	// Record initial position hash
	u.positionHashes = append(u.positionHashes, u.position.Hash)

	// Apply moves
	if moveStart < len(args) {
		for _, moveStr := range args[moveStart:] {
			move := u.parseMove(moveStr)
			if move == board.NoMove {
				fmt.Fprintf(os.Stderr, "info string Invalid move: %s\n", moveStr)
				return
			}
			u.position.MakeMove(move)
			u.position.UpdateCheckers()
			u.positionHashes = append(u.positionHashes, u.position.Hash)
		}
	}

	// Debug: log position state after setup
	if board.DebugMoveValidation {
		legal := u.position.GenerateLegalMoves()
		var legalStrs []string
		for i := 0; i < legal.Len() && i < 8; i++ {
			legalStrs = append(legalStrs, legal.Get(i).String())
		}
		fmt.Fprintf(os.Stderr, "info string DEBUG: After position setup - hash=%016x inCheck=%v legal=%v...\n",
			u.position.Hash, u.position.InCheck(), legalStrs)
	}
}

// parseMove converts a UCI move string to a board.Move.
func (u *UCI) parseMove(moveStr string) board.Move {
	if len(moveStr) < 4 {
		return board.NoMove
	}

	fromFile := int(moveStr[0] - 'a')
	fromRank := int(moveStr[1] - '1')
	toFile := int(moveStr[2] - 'a')
	toRank := int(moveStr[3] - '1')

	if fromFile < 0 || fromFile > 7 || fromRank < 0 || fromRank > 7 ||
		toFile < 0 || toFile > 7 || toRank < 0 || toRank > 7 {
		return board.NoMove
	}

	from := board.NewSquare(fromFile, fromRank)
	to := board.NewSquare(toFile, toRank)

	// Check for promotion
	var promo board.PieceType
	if len(moveStr) == 5 {
		switch moveStr[4] {
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
	moves := u.position.GenerateLegalMoves()
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

// GoOptions holds parsed "go" command options.
type GoOptions struct {
	Depth     int
	Nodes     uint64
	MoveTime  time.Duration
	Infinite  bool
	WTime     time.Duration
	BTime     time.Duration
	WInc      time.Duration
	BInc      time.Duration
	MovesToGo int
}

// handleGo starts a search with the given parameters.
func (u *UCI) handleGo(args []string) {
	opts := u.parseGoOptions(args)

	// Set up position history for repetition detection
	u.engine.SetPositionHistory(u.positionHashes)

	// Configure info callback
	u.engine.OnInfo = func(info engine.SearchInfo) {
		u.sendInfo(info)
	}

	// Calculate search limits
	limits := u.calculateLimits(opts)

	// Start search in goroutine
	u.searching = true
	u.stopRequested.Store(false)
	u.searchDone = make(chan struct{})

	pos := u.position.Copy()

	go func() {
		defer close(u.searchDone)

		bestMove := u.engine.SearchWithLimits(pos, limits)

		u.searching = false

		// Validate move is legal before sending
		// Use fresh copy of original position for validation (search may have corrupted pos)
		validationPos := u.position.Copy()
		if bestMove != board.NoMove {
			legal := validationPos.GenerateLegalMoves()
			found := false
			for i := 0; i < legal.Len(); i++ {
				if legal.Get(i) == bestMove {
					found = true
					break
				}
			}
			if found {
				if board.DebugMoveValidation {
					fmt.Fprintf(os.Stderr, "info string DEBUG: Sending bestmove %s (hash=%016x)\n", bestMove.String(), validationPos.Hash)
				}
				fmt.Printf("bestmove %s\n", bestMove.String())
				return
			}
			// Move not legal - log detailed warning
			fmt.Fprintf(os.Stderr, "info string CRITICAL: Search returned illegal move %s (not in %d legal moves)\n", bestMove.String(), legal.Len())
			// Log all legal moves for debugging
			var legalStrs []string
			for i := 0; i < legal.Len() && i < 10; i++ {
				legalStrs = append(legalStrs, legal.Get(i).String())
			}
			fmt.Fprintf(os.Stderr, "info string Legal moves (first 10): %v\n", legalStrs)
		} else {
			fmt.Fprintf(os.Stderr, "info string WARNING: Search returned NoMove, using fallback\n")
		}

		// Fallback: return first legal move if available
		legal := validationPos.GenerateLegalMoves()
		if legal.Len() > 0 {
			fmt.Printf("bestmove %s\n", legal.Get(0).String())
		} else {
			// Only send 0000 for checkmate/stalemate (no legal moves)
			fmt.Println("bestmove 0000")
		}
	}()
}

// parseGoOptions parses "go" command arguments.
func (u *UCI) parseGoOptions(args []string) GoOptions {
	opts := GoOptions{}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "depth":
			if i+1 < len(args) {
				opts.Depth, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "nodes":
			if i+1 < len(args) {
				n, _ := strconv.ParseUint(args[i+1], 10, 64)
				opts.Nodes = n
				i++
			}
		case "movetime":
			if i+1 < len(args) {
				ms, _ := strconv.Atoi(args[i+1])
				opts.MoveTime = time.Duration(ms) * time.Millisecond
				i++
			}
		case "infinite":
			opts.Infinite = true
		case "wtime":
			if i+1 < len(args) {
				ms, _ := strconv.Atoi(args[i+1])
				opts.WTime = time.Duration(ms) * time.Millisecond
				i++
			}
		case "btime":
			if i+1 < len(args) {
				ms, _ := strconv.Atoi(args[i+1])
				opts.BTime = time.Duration(ms) * time.Millisecond
				i++
			}
		case "winc":
			if i+1 < len(args) {
				ms, _ := strconv.Atoi(args[i+1])
				opts.WInc = time.Duration(ms) * time.Millisecond
				i++
			}
		case "binc":
			if i+1 < len(args) {
				ms, _ := strconv.Atoi(args[i+1])
				opts.BInc = time.Duration(ms) * time.Millisecond
				i++
			}
		case "movestogo":
			if i+1 < len(args) {
				opts.MovesToGo, _ = strconv.Atoi(args[i+1])
				i++
			}
		}
	}

	return opts
}

// calculateLimits converts GoOptions to engine.SearchLimits.
func (u *UCI) calculateLimits(opts GoOptions) engine.SearchLimits {
	limits := engine.SearchLimits{}

	if opts.Infinite {
		limits.Infinite = true
		return limits
	}

	if opts.Depth > 0 {
		limits.Depth = opts.Depth
	}

	if opts.Nodes > 0 {
		limits.Nodes = opts.Nodes
	}

	if opts.MoveTime > 0 {
		limits.MoveTime = opts.MoveTime
	} else if opts.WTime > 0 || opts.BTime > 0 {
		// Time control - calculate time for this move
		limits.MoveTime = u.calculateTimeForMove(opts)
	}

	return limits
}

// calculateTimeForMove determines how much time to spend on this move.
func (u *UCI) calculateTimeForMove(opts GoOptions) time.Duration {
	var ourTime, ourInc time.Duration

	if u.position.SideToMove == board.White {
		ourTime = opts.WTime
		ourInc = opts.WInc
	} else {
		ourTime = opts.BTime
		ourInc = opts.BInc
	}

	// Estimate moves remaining
	movesRemaining := opts.MovesToGo
	if movesRemaining == 0 {
		movesRemaining = u.estimateMovesRemaining()
	}

	// Base time allocation
	baseTime := ourTime / time.Duration(movesRemaining)

	// Add increment (use 90%)
	moveTime := baseTime + (ourInc * 90 / 100)

	// Safety: never use more than 90% of remaining time
	maxTime := ourTime * 90 / 100
	if moveTime > maxTime {
		moveTime = maxTime
	}

	// Minimum time
	if moveTime < 10*time.Millisecond {
		moveTime = 10 * time.Millisecond
	}

	// Debug logging for time management diagnosis
	fmt.Printf("info string time_allocated=%dms moves_remaining=%d our_time=%dms our_inc=%dms\n",
		moveTime.Milliseconds(), movesRemaining, ourTime.Milliseconds(), ourInc.Milliseconds())

	return moveTime
}

// estimateMovesRemaining estimates remaining moves based on piece count.
func (u *UCI) estimateMovesRemaining() int {
	totalPieces := u.position.AllOccupied.PopCount()

	if totalPieces > 24 {
		return 40 // Opening/early middlegame
	} else if totalPieces > 12 {
		return 30 // Middlegame
	}
	return 20 // Endgame
}

// sendInfo outputs search info in UCI format.
func (u *UCI) sendInfo(info engine.SearchInfo) {
	var parts []string

	parts = append(parts, fmt.Sprintf("depth %d", info.Depth))

	// Score
	if info.Score > engine.MateScore-100 {
		mateIn := (engine.MateScore - info.Score + 1) / 2
		parts = append(parts, fmt.Sprintf("score mate %d", mateIn))
	} else if info.Score < -engine.MateScore+100 {
		mateIn := -(engine.MateScore + info.Score + 1) / 2
		parts = append(parts, fmt.Sprintf("score mate %d", mateIn))
	} else {
		parts = append(parts, fmt.Sprintf("score cp %d", info.Score))
	}

	parts = append(parts, fmt.Sprintf("nodes %d", info.Nodes))
	parts = append(parts, fmt.Sprintf("time %d", info.Time.Milliseconds()))

	// NPS
	if info.Time > 0 {
		nps := uint64(float64(info.Nodes) / info.Time.Seconds())
		parts = append(parts, fmt.Sprintf("nps %d", nps))
	}

	// Hash fullness
	if info.HashFull > 0 {
		parts = append(parts, fmt.Sprintf("hashfull %d", info.HashFull))
	}

	// PV - validate moves to prevent outputting illegal sequences
	if len(info.PV) > 0 {
		validPV := make([]string, 0, len(info.PV))
		testPos := u.position.Copy()
		for _, move := range info.PV {
			// Validate move is legal in current test position
			legal := testPos.GenerateLegalMoves()
			isLegal := false
			for i := 0; i < legal.Len(); i++ {
				if legal.Get(i) == move {
					isLegal = true
					break
				}
			}
			if !isLegal {
				break // Stop at first illegal move
			}
			validPV = append(validPV, move.String())
			testPos.MakeMove(move)
		}
		if len(validPV) > 0 {
			parts = append(parts, "pv "+strings.Join(validPV, " "))
		}
	}

	fmt.Printf("info %s\n", strings.Join(parts, " "))
}

// handleStop stops the current search.
func (u *UCI) handleStop() {
	if u.searching {
		u.stopRequested.Store(true)
		u.engine.Stop()
		<-u.searchDone // Wait for search to finish
	}
}

// handleQuit exits the program.
func (u *UCI) handleQuit() {
	u.handleStop()
	// Stop profiling if active
	if u.profileFile != nil {
		pprof.StopCPUProfile()
		u.profileFile.Close()
		fmt.Fprintf(os.Stderr, "info string CPU profile saved\n")
	}
	os.Exit(0)
}

// handleSetOption processes "setoption" commands.
func (u *UCI) handleSetOption(args []string) {
	// Format: setoption name <name> value <value>
	var name, value string
	readingName := false
	readingValue := false

	for _, arg := range args {
		switch arg {
		case "name":
			readingName = true
			readingValue = false
		case "value":
			readingName = false
			readingValue = true
		default:
			if readingName {
				if name != "" {
					name += " "
				}
				name += arg
			} else if readingValue {
				if value != "" {
					value += " "
				}
				value += arg
			}
		}
	}

	// Handle options
	switch strings.ToLower(name) {
	case "hash":
		// TODO: Resize hash table
		// For now, ignore - would need engine support
	case "usennue":
		useNNUE := strings.ToLower(value) == "true"
		if useNNUE && u.nnueBigPath != "" && u.nnueSmallPath != "" {
			// Load networks if not already loaded
			if !u.engine.HasNNUE() {
				if err := u.engine.LoadNNUE(u.nnueBigPath, u.nnueSmallPath); err != nil {
					fmt.Fprintf(os.Stderr, "info string Failed to load NNUE: %v\n", err)
					return
				}
			}
		}
		u.engine.SetUseNNUE(useNNUE)
	case "evalfile":
		u.nnueBigPath = value
		u.tryLoadNNUE()
	case "evalfilesmall":
		u.nnueSmallPath = value
		u.tryLoadNNUE()
	case "syzygypath":
		u.syzygyPath = value
		u.initSyzygy()
	case "syzygyprobedepth":
		depth, err := strconv.Atoi(value)
		if err == nil && depth >= 1 {
			u.syzygyProbeDepth = depth
			u.engine.SetSyzygyProbeDepth(depth)
		}
	case "debug":
		enabled := strings.ToLower(value) == "true"
		board.DebugMoveValidation = enabled
		if enabled {
			fmt.Fprintf(os.Stderr, "info string Debug mode enabled\n")
		}
	case "cpuprofile":
		// Stop existing profile if any
		if u.profileFile != nil {
			pprof.StopCPUProfile()
			u.profileFile.Close()
			fmt.Fprintf(os.Stderr, "info string CPU profile stopped\n")
			u.profileFile = nil
		}
		// Start new profile if path provided
		if value != "" && value != "stop" {
			f, err := os.Create(value)
			if err != nil {
				fmt.Fprintf(os.Stderr, "info string Failed to create profile: %v\n", err)
				return
			}
			if err := pprof.StartCPUProfile(f); err != nil {
				f.Close()
				fmt.Fprintf(os.Stderr, "info string Failed to start profile: %v\n", err)
				return
			}
			u.profileFile = f
			fmt.Fprintf(os.Stderr, "info string CPU profiling to %s\n", value)
		}
	}
}

// tryLoadNNUE attempts to load NNUE networks if both paths are set.
func (u *UCI) tryLoadNNUE() {
	if u.nnueBigPath != "" && u.nnueSmallPath != "" {
		if err := u.engine.LoadNNUE(u.nnueBigPath, u.nnueSmallPath); err != nil {
			fmt.Fprintf(os.Stderr, "info string Failed to load NNUE: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "info string NNUE networks loaded\n")
		}
	}
}

// initSyzygy initializes Syzygy tablebase probing.
func (u *UCI) initSyzygy() {
	if u.syzygyPath == "" {
		return
	}

	u.syzygyProber = tablebase.NewSyzygyProber(u.syzygyPath)
	u.engine.SetTablebase(u.syzygyProber)

	probeDepth := u.syzygyProbeDepth
	if probeDepth < 1 {
		probeDepth = 1
	}
	u.engine.SetSyzygyProbeDepth(probeDepth)

	fmt.Fprintf(os.Stderr, "info string Syzygy tablebase initialized at %s\n", u.syzygyPath)
}

// handlePerft runs a perft test.
func (u *UCI) handlePerft(args []string) {
	depth := 5
	if len(args) > 0 {
		depth, _ = strconv.Atoi(args[0])
	}

	start := time.Now()
	nodes := u.engine.Perft(u.position, depth)
	elapsed := time.Since(start)

	fmt.Printf("Nodes: %d\n", nodes)
	fmt.Printf("Time: %v\n", elapsed)
	if elapsed > 0 {
		nps := float64(nodes) / elapsed.Seconds()
		fmt.Printf("NPS: %.0f\n", nps)
	}
}
