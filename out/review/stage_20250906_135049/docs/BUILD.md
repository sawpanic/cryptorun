# Build and Run (Go)

This project builds with Go 1.21+ only. Use the CLI flags; there is no interactive menu.

## Environment
- Install Go 1.21+ and ensure `go` is on PATH.
- Network: outbound HTTPS to `api.kraken.com`.
- Optional env: `REDIS_ADDR`, `PG_DSN`, `METRICS_ADDR`.

## Build
- From `CryptoRun/src`: `go build ./cmd/cryptorun`
- Release with timestamp:
  - Generate: `go run ./tools/buildstamp`
  - Build: `go build -ldflags "-X main.BuildStamp=<STAMP>" -o cryptorun.exe ./cmd/cryptorun`

## Run
- Scan: `./cryptorun scan --exchange kraken --pairs USD-only --dry-run`
- Monitor: `./cryptorun monitor` (visit `/health`, `/metrics`, `/decile`)
- Health: `./cryptorun health`
- Backtest: stub log only

## Quick Checks
- Scan logs show universe count and Top 10 ranked pairs.
- No aggregator usage for depth/spread; Kraken-only endpoints.
- Metrics update: ingest/normalize/score/serve latencies.

## Rebuild
- Re-run the build command after changes. For stamped builds, regenerate the stamp each release.

