.PHONY: run build fmt test

run:
	if [ -f .env ]; then set -a; . ./.env; set +a; fi; go run ./cmd/blog

build:
	go build -o blog ./cmd/blog

fmt:
	gofmt -w ./cmd ./config ./internal

test:
	go test ./...
