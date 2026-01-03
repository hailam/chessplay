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

.PHONY: deps build uci build-amd64-uci gen-pprof test-elo clean

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

# 2. Core Engine Build (Library/Standalone)
build:
	@mkdir -p ./bin
	go build -o $(BINARY_CORE) .

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
test-elo: uci
	@echo "Starting Elo benchmark: $(BINARY_UCI) vs Stockfish"
	$(CHESS_CLI) \
		-each tc=10+0.1 \
		-engine cmd=$(BINARY_UCI) name=ChessPlay-Go \
		-engine cmd=$(STOCKFISH) name=Stockfish \
		-concurrency 2 \
		-rounds 5 \
		-pgn results.pgn

clean:
	rm -rf ./bin $(PROFILE) results.pgn