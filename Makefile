run:
	go run .

build:
	go build -o ./bin/chessplay .

build-amd64:
	GOOS=linux GOARCH=amd64 GOEXPERIMENT=simd go build -o ./bin/chessplay-linux-amd64 .