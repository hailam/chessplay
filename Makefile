BINARY=./bin/chessplay
PROFILE=./cpu.pprof

run:
	echo "No $(PROFILE) found. Generating..."; \
	$(MAKE) gen-pprof; \
	go run -pgo=$(PROFILE) .

build:
	@mkdir -p ./bin
	echo "No $(PROFILE) found. Generating..."; \
	$(MAKE) gen-pprof; \
	go build -pgo=$(PROFILE) -o $(BINARY) .

build-amd64:
	@mkdir -p ./bin
	echo "No $(PROFILE) found. Generating..."; \
	$(MAKE) gen-pprof; \
	GOOS=linux GOARCH=amd64 GOEXPERIMENT=simd go build -pgo=$(PROFILE) -o $(BINARY)-linux-amd64 .

gen-pprof:
	go test -bench=. -cpuprofile=$(PROFILE) ./internal/engine -o /dev/null