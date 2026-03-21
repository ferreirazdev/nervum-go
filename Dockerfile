# Build stage — CGO required: gorm.io/driver/sqlite (mattn/go-sqlite3) is linked by internal/database.
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/server ./cmd/api

# Runtime — Alpine matches musl-linked binary from builder.
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -H -u 65532 app

WORKDIR /app

COPY --from=builder --chown=65532:65532 /app/server /app/server

USER app

# Cloud Run injects PORT; Gin reads the same env in config.Load().
ENV PORT=8080
ENV GIN_MODE=release

EXPOSE 8080

CMD ["/app/server"]
