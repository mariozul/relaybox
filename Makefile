.PHONY: test lint build tidy run

test:
	go test -race -timeout 90s ./...

lint:
	golangci-lint run ./...

build:
	go build -o bin/relaybox ./cmd/relaybox

tidy:
	go mod tidy

run:
	go run ./cmd/relaybox
