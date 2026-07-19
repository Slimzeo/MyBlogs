.PHONY: run build fmt test

run:
	go run ./cmd/blog

build:
	go build -o blog ./cmd/blog

fmt:
	gofmt -w ./cmd ./config ./internal

test:
	go test ./...
