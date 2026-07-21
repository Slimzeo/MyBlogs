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
COPY notes /app/notes
RUN mkdir -p /app/data/upload && chown -R blog:blog /app

USER blog
ENV BLOG_PORT=8081 \
    BLOG_BIND_ADDRESS=0.0.0.0 \
    BLOG_GIN_MODE=release \
    BLOG_DB_DRIVER=sqlite \
    BLOG_DB_DSN="data/blog.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)" \
    BLOG_NOTES_DIR=notes \
    BLOG_UPLOAD_DIR=data/upload \
    BLOG_COOKIE_SECURE=true
EXPOSE 8081
VOLUME ["/app/data"]
ENTRYPOINT ["/app/blog"]
