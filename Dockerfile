FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/blog ./cmd/blog

FROM alpine:3.22

RUN addgroup -S blog && adduser -S -G blog blog
WORKDIR /app
COPY --from=builder /out/blog /app/blog
COPY templates /app/templates
COPY static /app/static
RUN mkdir -p /app/data/upload && chown -R blog:blog /app

USER blog
ENV PORT=8081 \
    DB_DRIVER=sqlite \
    DB_DSN="data/blog.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)" \
    UPLOAD_DIR=data/upload \
    GIN_MODE=release
EXPOSE 8081
VOLUME ["/app/data"]
ENTRYPOINT ["/app/blog"]
