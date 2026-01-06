# --- ChessPlay Makefile ---
# Core paths
CMD_UCI=./cmd/chessplay-uci/main.go
BINARY_UCI=./bin/chessplay-uci
BINARY_CORE=./bin/chess-core
PROFILE=./cpu.pprof

# Benchmarking tools
CHESS_CLI=./bin/c-chess-cli
STOCKFISH=$(shell which stockfish 2>/dev/null || echo "/opt/homebrew/bin/stockfish")

# c-chess-cli build config
CLI_CFLAGS=-I./src -std=gnu11 -mpopcnt -DNDEBUG -Os -ffast-math -flto -s
CLI_LFLAGS=-lpthread -lm
CLI_SOURCES=src/bitboard.c src/gen.c src/position.c src/str.c src/util.c src/vec.c \
            src/engine.c src/game.c src/jobs.c src/main.c src/openings.c src/options.c \
            src/seqwriter.c src/sprt.c src/workers.c

.PHONY: deps build uci build-amd64-uci gen-pprof test-elo profile-elo clean

# 1. Dependency Management
deps:
	@mkdir -p ./bin
	@if [ ! -f $(CHESS_CLI) ]; then \
		echo "Downloading and building c-chess-cli..."; \
		curl -fL https://github.com/lucasart/c-chess-cli/archive/refs/heads/master.zip -o ./bin/cli.zip; \
		unzip -q ./bin/cli.zip -d ./bin; \
		VER_STR=$$(date +%Y-%m-%d); \
		(cd ./bin/c-chess-cli-master && $(CC) $(CLI_CFLAGS) -DVERSION=\"$$VER_STR\" $(CLI_SOURCES) -o ../c-chess-cli $(CLI_LFLAGS)); \
		chmod +x $(CHESS_CLI); \
		rm -rf ./bin/cli.zip ./bin/c-chess-cli-master; \
	fi

run: build
	@echo "Running ChessPlay Core Engine..."
	go run -pgo=$(PROFILE) .

# 2. Core Engine Build (Library/Standalone)
build:
	@mkdir -p ./bin
	go build -pgo=$(PROFILE) -o $(BINARY_CORE) .

# 3. UCI Protocol Build (Mac ARM64)
# Uses PGO if profile exists
uci: deps
	@if [ ! -f $(PROFILE) ]; then $(MAKE) gen-pprof; fi
	go build -pgo=$(PROFILE) -o $(BINARY_UCI) $(CMD_UCI)

# 4. Optimized Linux/AMD64 UCI Build (Deployment)
# Enables Go 1.26 SIMD experiments
build-amd64-uci: deps
	@if [ ! -f $(PROFILE) ]; then $(MAKE) gen-pprof; fi
	@echo "Building optimized Linux/AMD64 UCI binary..."
	GOOS=linux GOARCH=amd64 GOEXPERIMENT=simd go build -pgo=$(PROFILE) -o $(BINARY_UCI)-amd64 $(CMD_UCI)

# 5. PGO Profile Generation
# Runs benchmarks in the internal engine to capture hot paths
gen-pprof:
	@echo "Generating PGO profile from engine benchmarks..."
	go test -bench=. -cpuprofile=$(PROFILE) ./internal/engine

# 6. Elo Testing (Targets the UCI binary)
# Multi-threaded Lazy SMP search enabled
test-elo: uci
	rm -f results.pgn
	@echo "Starting Elo benchmark: $(BINARY_UCI) vs Stockfish (multi-threaded)"
	$(CHESS_CLI) \
		-each tc=13+0.1 \
		-engine cmd=$(BINARY_UCI) name=ChessPlay-Go \
		-engine cmd=$(STOCKFISH) name=Stockfish option.Skill\ Level=3 \
		-concurrency 2 \
		-rounds 100 \
		-pgn results.pgn

# 7. Profile during Elo Testing
# Runs with profiling enabled via UCI option to identify hot paths
PROFILE_OUTPUT=$(shell pwd)/elo_profile.pprof
profile-elo: uci
	rm -f results.pgn $(PROFILE_OUTPUT)
	@echo "Starting profiled Elo benchmark (concurrency=1 for accurate profiling)..."
	@echo "Profile will be saved to: $(PROFILE_OUTPUT)"
	$(CHESS_CLI) \
		-each tc=10+0.1 \
		-engine cmd=$(BINARY_UCI) name=ChessPlay-Go option.cpuprofile=$(PROFILE_OUTPUT) \
		-engine cmd=$(STOCKFISH) name=Stockfish option.Skill\ Level=4 \
		-concurrency 1 \
		-rounds 4 \
		-pgn results.pgn
	@echo ""
	@echo "=== CPU Profile Summary ==="
	@if [ -f $(PROFILE_OUTPUT) ]; then \
		go tool pprof -top -cum $(PROFILE_OUTPUT) 2>/dev/null | head -40; \
		echo ""; \
		echo "For interactive analysis: go tool pprof -http=:8080 $(PROFILE_OUTPUT)"; \
	else \
		echo "Profile not generated - engine may not have exited cleanly"; \
	fi

clean:
	rm -rf ./bin $(PROFILE) $(PROFILE_OUTPUT) results.pgn