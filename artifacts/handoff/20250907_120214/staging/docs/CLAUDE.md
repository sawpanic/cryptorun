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

This is a cryptocurrency momentum scanner built in Go with a **unified composite scoring system** that provides a single, consistent scoring path.

### Unified Composite Architecture
All scoring routes through a single composite system:
- **`internal/score/composite/`**: Unified composite scorer with protected MomentumCore
- **`internal/gates/entry.go`**: Hard entry gates (Score≥75 + VADR≥1.8 + funding divergence)
- **`internal/explain/explainer.go`**: Comprehensive scoring explanations with attribution
- **`internal/data/derivs/`**: New measurements (funding z-score, OI residuals, ETF flows)

### Core Layers
- **`domain/`**: Business logic (scoring, gates, orthogonalization, regime detection)
- **`application/`**: Use cases with **single pipeline entry points per action**
- **`infrastructure/`**: External integrations (Kraken APIs, cache, circuit breakers, rate limiting, DB)
- **`interfaces/`**: HTTP endpoints (`/health`, `/metrics`, `/decile`)
- **`cmd/cryptorun/`**: CLI commands that call unified pipelines
- **Menu system**: Interactive interface that calls the SAME unified pipelines

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
5. **Run conformance tests**: `go test ./tests/conformance` - ensures single pipeline architecture compliance

### Single Pipeline Enforcement
The codebase enforces single implementation per action through:
- **Conformance tests**: `tests/conformance/no_duplicate_paths_test.go` validates CMD and menu route to same functions
- **Architecture constraint**: One exported function per action in `internal/application/`
- **CI enforcement**: Conformance suite runs in CI to prevent architectural drift

## Documentation & Quality Gates

### Documentation UX Verification
- **UX Guard Check**: `go run scripts/check_docs_ux.go` (or `pwsh -File scripts/check_docs_ux.ps1`)
- **Branding Guard**: `go test -v ./tests/branding -run TestBrandConsistency`
- **Combined Check**: Run both guards to validate documentation consistency

### Pre-Commit Hooks (Recommended)
Enable automated checks before each commit:

```bash
# Enable git hooks (one-time setup)
git config core.hooksPath .githooks

# Make hooks executable (Linux/Mac)
chmod +x .githooks/pre-commit

# Test hook manually
./.githooks/pre-commit
```

**Windows PowerShell:**
```powershell
# Test hook manually
pwsh -File .githooks/pre-commit.ps1
```

### Documentation Requirements
All markdown files must include:
```markdown
## UX MUST — Live Progress & Explainability
```

### Branding Rules
- **Allowed**: "CryptoRun" only
- **Forbidden**: "CryptoEdge", "Crypto Edge" (except in `_codereview/**` for historic references)
- **Enforcement**: Automated via branding guard test and CI pipeline

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

**Unified Composite Scoring System**

* **MomentumCore (Protected)**: Multi-timeframe momentum (1h/4h/12h/24h) that is NEVER orthogonalized
* **Gram-Schmidt Residualization**: Technical → Volume → Quality → Social (in sequence)
* **Regime-Adaptive Weights**: Three profiles (calm/normal/volatile) with automatic 4h switching
* **Social Cap**: Strictly limited to +10 points, applied OUTSIDE the 100% weight allocation
* **Entry Gates**: Hard requirements (Score≥75 + VADR≥1.8 + funding divergence≥2σ)
* **New Measurements**: Cross-venue funding z-score, OI residuals, ETF flows (free sources only)

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

## 📊 Current Progress (after UNIFIED MODEL v1)

✅ **Unified Composite Scoring System**: Single scoring path with protected MomentumCore
✅ **Gram-Schmidt Orthogonalization**: Residualized factors with momentum protection  
✅ **Regime-Adaptive Weights**: Three weight profiles (calm/normal/volatile)
✅ **Hard Entry Gates**: Score≥75 + VADR≥1.8 + funding divergence enforcement
✅ **New Measurements**: Funding z-score, OI residuals, ETF flows (free sources only)
✅ **Explainability System**: Comprehensive scoring explanations with attribution
✅ **Exchange-Native Microstructure**: L1/L2 validation with proof generation
✅ **Menu & CLI Integration**: Unified interface with real-time testing capabilities
✅ **Test Suite**: Unit and integration tests for unified system validation

❌ **Legacy FactorWeights**: Removed dual-path system - SINGLE PATH ONLY
❌ **Aggregator Dependencies**: Banned for microstructure data - venue-native only  

**Completion:** ~85% core system, ~60% overall project

---

## 🚧 Current Limitations

* No live data connections (uses mocks for testing)
* No regime detector implementation (manual regime setting)
* No persistence layer or database integration
* No production deployment configuration
* No performance monitoring beyond basic metrics

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