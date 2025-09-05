# Repository Guidelines

## Project Structure & Module Organization
- Source: `CProtocol/src/` with layered packages:
  - `domain/` (scoring, gates, orthogonalization, regime logic)
  - `application/` (universe/factors builders, snapshot store, config loaders)
  - `infrastructure/` (Kraken APIs, cache, circuit breaker, rate limit, DB)
  - `interfaces/` (HTTP: `/health`, `/metrics`, `/decile`)
  - `cmd/cprotocol/` (CLI entry: scan/backtest/monitor/health)
- Docs: `CProtocol/docs/` (build, usage, monitoring, integrations).
- Tests: `CProtocol/tests/` (`unit/`, `integration/`, `load/`).
- Config: `CProtocol/config/` (`apis.yaml`, `cache.yaml`, `circuits.yaml`, `regimes.yaml`, `pairs.yaml`).
- Artifacts & logs: `C:\wallet\artifacts\` (audit/logs when DB disabled).

## Build, Test, and Development Commands
- Build (dev): `go build ./src/cmd/cprotocol`
- Build (release):
  - `go run ./tools/buildstamp`
  - `go build -ldflags "-X main.BuildStamp=<STAMP>" -o cprotocol.exe ./src/cmd/cprotocol`
- Run: `./cprotocol scan --exchange kraken --pairs USD-only --dry-run`
- Tests: `go test ./...` (unit covers orthogonalization, VADR, gates; integration uses httptest; load includes P99 latency).

## Coding Style & Naming Conventions
- Language: Go 1.21+; format with `go fmt`.
- Packages: lower_snake for folders; exported `PascalCase` types; unexported `camelCase`.
- Logging/metrics: use structured logging; expose health/metrics via `interfaces/http`.
- Keep modules single-purpose; respect layer boundaries; explicit imports only.

## Testing Guidelines
- Place new tests under `tests/unit` or `tests/integration`; name `*_test.go`.
- Keep tests deterministic; network-dependent cases use fakes/mocks or be skipped.
- Run full suite: `go test ./... -count=1` before PRs.

## Commit & Pull Request Guidelines
- Commits: imperative, concise, capitalized (e.g., "Implement Regime Weights", "Fix circuit backoff jitter").
- PRs: include what/why, linked issues, performance impact, sample output, and paths to generated artifacts under `artifacts/`.
- Documentation: update relevant docs (`docs/`, `product.md`), and include a `changelog.log` excerpt per `docs/DOCUMENTATION_PROTOCOL.md`.

## Security & Configuration Tips
- Secrets: never commit; use env vars: `REDIS_ADDR`, `PG_DSN`, `METRICS_ADDR`, `KRAKEN_API_BASE`, `KRAKEN_WS_URL`.
- Venue/data: Kraken USD pairs only; respect rate limits with backoff and provider-aware circuit breakers; do not use aggregators for depth/spread.
- Caching: tune TTLs in `config/cache.yaml`; file audit enabled when DB is off.

