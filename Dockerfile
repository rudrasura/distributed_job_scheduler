# Build Stage
FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build argument to specify which service to build
# Defaults to ingestion
ARG SERVICE=ingestion

# Build the binary
# CGO_ENABLED=1 is required for confluent-kafka-go
RUN CGO_ENABLED=1 go build -ldflags="-w -s" -o /app/server ./cmd/${SERVICE}

# Runtime Stage
FROM debian:bookworm-slim

WORKDIR /app

# Install runtime dependencies (e.g. ca-certificates)
RUN apt-get update && apt-get install -y ca-certificates curl && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/server /app/server

# Expose ports (can be overridden by compose)
EXPOSE 8080

CMD ["/app/server"]
