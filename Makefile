.PHONY: run build build-cli install-cli fmt test

run:
	if [ -f .env ]; then set -a; . ./.env; set +a; fi; go run ./cmd/blog

build:
	go build -o blog ./cmd/blog

build-cli:
	mkdir -p bin
	go build -trimpath -o bin/blogctl ./cmd/blogctl

install-cli:
	go install ./cmd/blogctl

fmt:
	gofmt -w ./cmd ./config ./internal

test:
	go test ./...
