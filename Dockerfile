# Multi-stage Dockerfile for CryptoRun v3.2.1
# Optimized for production deployment with minimal attack surface

# Stage 1: Build stage with Go toolchain
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata make

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags '-w -s -extldflags "-static"' \
    -o cryptorun \
    ./cmd/cryptorun

# Stage 2: Runtime stage with minimal base
FROM gcr.io/distroless/static:nonroot

# Copy timezone data and CA certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder stage
COPY --from=builder /app/cryptorun /usr/local/bin/cryptorun

# Copy configuration files
COPY --from=builder /app/config /etc/cryptorun/config

# Create necessary directories with proper permissions
USER 65532:65532

# Expose ports for HTTP endpoints
EXPOSE 8080 8081 8082

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/cryptorun", "health"]

# Default command
ENTRYPOINT ["/usr/local/bin/cryptorun"]
CMD ["monitor"]

# Metadata labels following OCI spec
LABEL org.opencontainers.image.title="CryptoRun"
LABEL org.opencontainers.image.description="6-48h cryptocurrency momentum scanner with exchange-native APIs"
LABEL org.opencontainers.image.version="3.2.1"
LABEL org.opencontainers.image.vendor="CryptoRun"
LABEL org.opencontainers.image.licenses="Proprietary"
LABEL org.opencontainers.image.source="https://github.com/sawpanic/cryptorun"
LABEL org.opencontainers.image.documentation="https://docs.cryptorun.io"

# Security labels
LABEL security.scan-policy="strict"
LABEL security.non-root="true"
LABEL security.readonly-rootfs="true"
