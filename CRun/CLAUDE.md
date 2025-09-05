# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Building
- **Development build**: `go build ./src/cmd/cryptorun`
- **Release build**: 
  ```bash
  go run ./tools/buildstamp
  go build -ldflags "-X main.BuildStamp=<STAMP>" -o cryptorun.exe ./src/cmd/cryptorun
  ```
- **Build from root**: `cd src && go build ./cmd/cryptorun`

### Testing
- **Run all tests**: `go test ./...`
- **Run with count**: `go test ./... -count=1` (recommended before PRs)
- **Test structure**: `tests/unit`, `tests/integration`, `tests/load`

### Linting
- **Lint**: `golangci-lint run ./...`
- **Format**: `go fmt`

### Running
- **Scan**: `./cryptorun scan --exchange kraken --pairs USD-only --dry-run`
- **Monitor**: `./cryptorun monitor` (serves `/health`, `/metrics`, `/decile`)
- **Health check**: `./cryptorun health`

## Project Architecture

This is a cryptocurrency momentum scanner built in Go with a layered architecture in the `src/` directory:

### Core Layers
- **`domain/`**: Business logic (scoring, gates, orthogonalization, regime detection)
- **`application/`**: Use cases (universe builders, factor builders, snapshot store, config loaders)  
- **`infrastructure/`**: External integrations (Kraken APIs, cache, circuit breakers, rate limiting, DB)
- **`interfaces/`**: HTTP endpoints (`/health`, `/metrics`, `/decile`)
- **`cmd/cryptorun/`**: CLI entry point with commands: scan, backtest, monitor, health

### Key Concepts
- **6-48 hour momentum scanner**: Not HFT, not buy-and-hold
- **Exchange-native only**: Never use aggregators for depth/spread data
- **Kraken USD pairs only**: Primary data source with rate limiting
- **Regime-adaptive**: Weights change based on market conditions (trending/choppy/volatile)
- **Orthogonal factors**: Gram-Schmidt orthogonalization to avoid correlation
- **Circuit breakers**: Provider-aware fallbacks and rate limit handling

### Configuration
- **Config files**: `config/*.yaml` (apis, cache, circuits, regimes, pairs)
- **Environment vars**: `REDIS_ADDR`, `PG_DSN`, `METRICS_ADDR`, `KRAKEN_API_BASE`, `KRAKEN_WS_URL`

## Important Technical Details

### Data Sources & Rate Limits
- **Primary**: Kraken WebSocket and REST APIs (free tier)
- **Rate limiting**: Weight-based system with exponential backoff
- **No aggregators**: DEXScreener, CoinGecko etc. banned for microstructure data
- **Cache strategy**: Redis with TTLs defined in `config/cache.yaml`

### Testing Strategy  
- **Unit tests**: Orthogonalization, VADR calculations, gates logic
- **Integration tests**: HTTP endpoints with httptest
- **Load tests**: P99 latency validation (<300ms target)

### Performance Requirements
- **Scanner latency**: <300ms P99 (stretch goal)
- **Data freshness**: ≤60s for hot pairs
- **Cache hit rate**: >85% target

### Code Style
- **Go 1.21+** with standard formatting
- **Package naming**: lower_snake for folders, PascalCase exports, camelCase unexported
- **Layer boundaries**: Respect dependency direction, explicit imports only
- **Structured logging**: Use throughout with health/metrics exposure

### Security & Best Practices
- **No secrets in code**: Use environment variables
- **Venue-specific**: Kraken USD pairs only, respect rate limits
- **Circuit breakers**: Handle API degradation gracefully
- **File audit**: Logs to `C:\wallet\artifacts\` when DB disabled

## Common Development Patterns

### Adding New Factors
1. Implement in `domain/` package
2. Add to factor builder in `application/`
3. Update orthogonalization sequence
4. Add unit tests in `tests/unit`

### API Integration
1. Add circuit breaker configuration in `config/circuits.yaml`
2. Implement rate limiting in `infrastructure/`
3. Add fallback chains for reliability
4. Monitor via `/metrics` endpoint

### Testing Changes
1. Run full test suite before commits
2. Check P99 latencies don't degrade
3. Verify cache hit rates remain high
4. Validate regime detection accuracy

---

# 🏃‍♂️ CryptoRun Baseline Document

## 🌐 Strategic Mission

**CryptoRun**
Real-time **6–48h cryptocurrency momentum scanner** powered by free, keyless exchange-native APIs. Designed to deliver **explainable trading signals** with strong safeguards: freshness, fatigue, and late-fill guards, microstructure validation, regime awareness, and strict conformance tests.

**Promise:**

* *Never chase late entries* → freshness & late-fill guards.
* *Never size what can't be exited* → depth/spread/VADR enforcement.
* *Never let hype outrank price/volume* → capped social factor.
* *Never break under load* → provider-aware rate limits + circuit breakers.
* *Always transparent* → attribution fields in outputs.

---

## 🧭 Product Requirements (v3.2.1)

**Factor System**

* Multi-timeframe momentum:
  * 1h (20%)
  * 4h (35%)
  * 12h (30%)
  * 24h (10–15%)
  * Weekly 7d (5–10%) regime-dependent
* Protected MomentumCore in Gram–Schmidt hierarchy.
* Social/Brand contribution: max +10, applied *after* momentum & volume.

**Guards**

* **Fatigue Guard:** block if 24h > +12% and RSI4h > 70 unless accel ↑.
* **Freshness Guard:** ≤2 bars old & within 1.2×ATR(1h).
* **Late-Fill Guard:** reject fills >30s after signal bar close.

**Microstructure**

* Depth ≥ $100k within ±2%.
* Spread < 50bps.
* VADR ≥ 1.75×.
* Must be **exchange-native** only (Binance/OKX/Coinbase/Kraken).

**Regime System**

* Detector uses realized vol 7d, % above 20MA, breadth thrust.
* Majority vote every 4h → bull, chop, high-vol.
* Adaptive weight sets per regime.

**Entry/Exit**

* Movement thresholds by regime.
* Volume surge (≥1.75×).
* Liquidity gates (spread, depth, ADV).
* ADX > 25 or Hurst > 0.55.
* Exit: hard stop, venue health, 48h limit, accel reversal, fade, trailing, profit targets.

**Data Architecture**

* **Hot**: WebSockets.
* **Warm**: REST + caches.
* **Cold**: historical for backtests/regimes.
* Provider-aware RPS, Retry-After handling, circuit breakers.

---

## 📊 Current Progress (after P0 bootstrap)

✅ Repo initialized as Go module (`go.mod`, `go.sum`).
✅ Added `main.go` + Cobra `root.go`.
✅ Stubs for `internal/api`, `internal/config`, `internal/cobra`.
✅ Makefile + CI workflow.
✅ `go build ./... && go test ./...` passes with stubs.

⚠️ Everything else is missing: indicators, guards, factor engine, microstructure, regime, data facade, CLI scan pipeline, providers, conformance tests.

**Completion:** ~5–10%.

---

## 🚧 Bottlenecks

* No factor hierarchy or weights.
* No guardrail implementations.
* No microstructure metrics or ban enforcement.
* No regime detector.
* No scan pipeline or outputs.
* No providers or caching.
* No conformance tests.

---

## ✅ Do's & ❌ Don'ts for Devs

**Do**

* Follow prompts step by step — one PR per prompt.
* Respect config defaults — never hardcode thresholds.
* Add deterministic fakes + unit tests before merge.
* Use feature flags for experimental logic.
* Always attribute outputs (sources, timestamps, cache hits).
* Run CI (`tidy`, `vet`, `test`) before merge.

**Don't**

* Skip prompts or reorder tasks.
* Hardcode live API keys, weights, thresholds.
* Fetch real data in unit tests (fakes only).
* Remove TTL caches for hot/warm layers.
* Touch files outside declared prompt ownership.
* Merge red CI.

---

## 📂 Tactical Roadmap — Parallel Prompts

### Lane A — Math Core

* **A1 Config:** extend config with Weights/Gates/Limits/Flags + tests.
* **A2 Indicators + Guards:** RSI, ATR, ADX, Hurst + Fatigue/Freshness/Late-fill guards.
* **A3 Factors + Gram–Schmidt:** BuildFactorRow + Orthogonalize (MomentumCore protected) + Social cap.

### Lane B — Structure Safety

* **B1 Microstructure:** SpreadBps, DepthUSDWithinPct, VADR, aggregator ban guard.
* **B2 Regime + Composite:** regime detector + CompositeScore (ApplySocialCap).

### Lane C — I/O Shell

* **C1 Data facade:** TTL caches + deterministic fakes.
* **C2 CLI Scan (offline):** JSON/CSV output with attribution.

### Lane D — Serialized Later

* **D1 Providers:** Binance & Kraken REST (keyless).
* **D2 Rate limits + circuit breakers.**
* **D3 Entry gates integration.**
* **D4 Scale to Top-100 universe.**
* **D5 Conformance suite (factor order, guards, social cap, aggregator ban, regime).**
* **D6 P-VERIFY self-audit.**

---

## 🧪 Conformance Suite (must always pass)

* Factor hierarchy enforced.
* Guards implemented & tested.
* Social cap ≤ +10.
* Aggregator ban enforced.
* Regime switch toggles weights.

---

## 📝 Executive Summary

CryptoRun is **strategically clear** but **tactically early**. Only the bootstrap module is done. Next steps are parallelizable (A1–C2) to establish a full offline skeleton with fakes. After that, serialize providers & gating (D1–D4), then enforce spec tests (D5) and self-audit (D6).

**High-leverage milestones:**

1. **Top-20 online scan** with attribution.
2. **Conformance suite** failing CI on drift.