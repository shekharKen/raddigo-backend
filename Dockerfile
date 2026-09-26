# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source
COPY . .

# Build the static binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOMAXPROCS=1 \
    CGO_ENABLED=0 \
    GOOS=linux \
    go build -p 1 -ldflags="-s -w" -o /raddigo ./cmd/server


# ---- Runtime stage ----
FROM alpine:3.20

# Install CA certificates and create non-root user
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 10001 appuser

WORKDIR /app

# Copy static public assets (terms page, images); uploads are runtime data, not baked into the image
COPY --from=builder /app/public/assets ./public/assets
COPY --from=builder /app/public/terms.html ./public/terms.html

# Uploads are created at runtime; give appuser ownership of the whole public tree
RUN mkdir -p /app/public/uploads && chown -R appuser:appuser /app/public

# Copy application binary
COPY --from=builder /raddigo /usr/local/bin/raddigo

# Run as non-root user
USER appuser

EXPOSE 8080

ENTRYPOINT ["raddigo"]