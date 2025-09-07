# Build Documentation

This document describes how to build CryptoRun from source.

## Prerequisites

- **Go 1.23+**: Download from [golang.org](https://golang.org/dl/)
- **Git**: For version control operations
- **Network**: Outbound HTTPS to `api.kraken.com`
- **Optional**: `REDIS_ADDR`, `PG_DSN`, `METRICS_ADDR` environment variables

## Clean Build Recipe

To perform a clean build from scratch:

```bash
# 1. Clean any cached modules (optional but recommended for reproducible builds)
go clean -modcache

# 2. Install/update dependencies
go mod tidy

# 3. Verify all packages
go vet ./...

# 4. Run tests
go test ./...

# 5. Build all packages
go build ./...

# 6. Build the main binary
go build -o cryptorun ./cmd/cryptorun
```

## Development Build

For development with faster iteration:

```bash
# From CryptoRun/src directory
go build ./cmd/cryptorun

# Or from root directory  
go build ./src/cmd/cryptorun
```

## Release Build

For production releases with build metadata:

```bash
# Generate build timestamp
go run ./tools/buildstamp

# Build with build info
go build -ldflags "-X main.BuildStamp=<STAMP>" -o cryptorun.exe ./src/cmd/cryptorun
```

## Testing

### Run All Tests
```bash
go test ./...
```

### Run with Count (recommended before PRs)
```bash
go test ./... -count=1
```

### Test Categories
- `tests/unit/`: Unit tests
- `tests/integration/`: Integration tests  
- `tests/load/`: Load tests

## Running

### Basic Commands
- **Scan**: `./cryptorun scan --exchange kraken --pairs USD-only --dry-run`
- **Monitor**: `./cryptorun monitor` (serves `/health`, `/metrics`, `/decile`)
- **Health check**: `./cryptorun health`

### Quick Verification
- Scan logs show universe count and Top 10 ranked pairs
- No aggregator usage for depth/spread; Kraken-only endpoints
- Metrics update: ingest/normalize/score/serve latencies

## Quality Assurance

### Linting
```bash
# Format code
go fmt ./...

# Lint with golangci-lint (if installed)
golangci-lint run ./...
```

### QA Gates

The project includes automated quality gates that run during CI:

#### No-TODO Gate
Prevents builds with TODO/FIXME/STUB markers:

```bash
# Run locally
scripts/qa/no_todo.sh

# Or use Go version
go run scripts/qa/scanner.go
```

Add patterns to `scripts/qa/no_todo.allow` to exempt specific files.

## Troubleshooting

### Common Issues

1. **Import path errors**: Ensure you're in the correct module root
2. **Missing dependencies**: Run `go mod tidy`  
3. **Version conflicts**: Check `go.mod` for version constraints

### Module Information
- **Module path**: `github.com/sawpanic/cryptorun`
- **Go version**: 1.23+

### Cache Issues
If you encounter caching issues:
```bash
go clean -cache -modcache -i -r
```

## CI/CD Integration

The build process is automated via GitHub Actions:
1. **QA Gates**: No-TODO scanner runs first
2. **Build**: Only proceeds if QA passes
3. **Test**: Full test suite execution
4. **Artifacts**: Build reports and binaries uploaded

Build fails fast if quality gates don't pass, ensuring code quality standards.

## Performance Expectations

Typical build times on 4-core machine:
- **Full clean build**: ~60-90 seconds
- **Development build**: ~5-15 seconds  
- **Test suite**: ~30-60 seconds

## Docker Build (Optional)

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY . .
RUN go mod tidy && go build -o cryptorun ./cmd/cryptorun

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /src/cryptorun /usr/local/bin/
CMD ["cryptorun"]
```

