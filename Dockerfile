# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build deps for CGO (needed by mattn/go-sqlite3)
RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /app/server ./cmd/api

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/server .

# Cloud Run sets PORT automatically; default to 8080
ENV PORT=8080

EXPOSE 8080

CMD ["./server"]
