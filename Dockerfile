# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary with no CGO
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /bin/harness \
    ./cmd/main.go

# Install Goose for migrations
ARG GOOSE_VERSION=v3.29.0
RUN CGO_ENABLED=0 go install -tags='no_postgres no_mysql no_ydb no_mssql no_clickhouse no_azuresql no_vertica no_libsql' github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}

# Final stage
FROM alpine:latest
WORKDIR /app

# Copy binary and goose from builder
COPY --from=builder /bin/harness /usr/local/bin/harness
COPY --from=builder /go/bin/goose /usr/local/bin/goose

# Copy migrations
COPY db/migrations /app/db/migrations
RUN mkdir -p /db && ln -sf /app/db/migrations /db/migrations

# Persistent data directory for SQLite
VOLUME ["/data"]

EXPOSE 9090

ENV HARNESS_DB_PATH=/data/harness.db
ENV HARNESS_PORT=9090

CMD ["harness"]
