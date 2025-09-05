# CProtocol — Handoff Context and Changelog

Location: `C:\\wallet\\CProtocol`

Updated: 2025‑09‑04 (banner uses Jerusalem timestamp via Go `-ldflags`)

## Mission Statement
- Purpose: Detect, rank, and present high‑quality, tradeable crypto opportunities in real time using a rigorous, orthogonal factor framework that avoids double counting, preserves momentum when appropriate, and stays transparent about why each asset is ranked.
- Driving Need: Prior versions routinely surfaced flat assets and produced near‑identical results across scanners due to filtering oversights, conservative momentum handling, and opaque factor contributions. The system must prioritize tradeable momentum (when regime supports it), enforce liquidity, and make factor math visible.
- Core Why: Orthogonalization + gates separation keeps the math honest (no inflated Sharpe from collinearity) while momentum‑preserving design and regime‑aware thresholds ensure we capture real movers without abandoning risk discipline.

## What We Applied (What/Why)
- Orthogonal factor system (What): Quality residual, Volume+Liquidity fused, Technical residual, On‑chain residual, Social residual — weighted to 100% with no overlap.
  - Why: Remove factor collinearity and double counting; keep weights meaningful and auditable.
- Gates separated from alpha (What): Liquidity/volume/trend gates are multiplicative 0‑1, not additive into the score.
  - Why: Avoid “sneaking” risk/filters into alpha; score reflects pure factor conviction.
- Regime‑aware momentum handling (What): Movement thresholds and percentile fallback that adapt to bull/neutral‑bullish vs. other regimes.
  - Why: Capture real momentum in semi‑bull markets; avoid flat selections in quiet times.
- Early volume floor (What): Enforce VOL(USD) ≥ $250k before scoring/display.
  - Why: Eliminate illiquid names before they pollute rankings; reduce reject noise.
- Transparent factor breakdown (What): Print MOMO_z, TECH_orth, VOL+LIQ, QUAL_res, SOC_res, VOL(USD), COMPOSITE for each pick.
  - Why: Make ranking rationale explicit for triage and debugging.

## Executive Summary
- Scans Kraken pairs with daily volume ≥ $250k (early filter).
- Regime‑aware momentum filter with percentile fallback to avoid flat picks.
- Orthogonal factor breakdown table enabled (MOMO_z, TECH_orth, VOL+LIQ, QUAL_res, SOC_res, VOL(USD), COMPOSITE).
- Acceptance thresholds tuned to avoid blanket rejections (Ultra 35, Balanced 30).
- Go‑only build with injected Jerusalem timestamp.

## Build & Run (Go‑only)
- Build timestamp: `go run ./tools/buildstamp` → copy output
- Build exe: `go build -ldflags "-X main.BuildStamp=<STAMP>" -o cprotocol.exe .`
- Run menu‑only: `cprotocol.exe` (CLI args ignored by design)
- Optional API key: `setx COINMARKETCAP_API_KEY "YOUR_KEY"`

## Live‑Data QA Checklist
- Pair filtering prints non‑zero “Tradable pairs after filters” and positive pass ratio.
- No warnings: “CRITICAL: … using emergency static data”.
- Momentum filter summary prints: `MOMENTUM FILTER: X flat assets excluded, Y moving assets remain`.
- Factor breakdown table is printed under the main results table.

## Mode Behavior (Current)
- Ultra‑Alpha
  - MinCompositeScore: 35; momentum classification > 3.0%.
- Balanced
  - MinCompositeScore: 30; regime‑aware gates (2.5% bull/neutral‑bullish, 3.0% otherwise) + 70th percentile fallback.
  - Early volume filter (≥ $250k) before scoring and display.
- Sweet‑Spot
  - Range‑biased; benefits from improved filtering and classification.

## Changelog (Latest First)
- 2025‑09‑04 09:50 Jerusalem
  - main.go: BuildStamp support; banner shows Jerusalem time when injected.
  - internal/comprehensive/live_comprehensive.go: early VOL(USD) ≥ 250k filter; regime‑aware momentum gates; 70th percentile fallback; import `sort`.
  - internal/ui/comprehensive_display.go: enable factor breakdown table.
  - internal/comprehensive/comprehensive.go: momentum classification eased to > 3.0%.
  - internal/models/orthogonal_weights.go: Ultra MinCompositeScore=35; Balanced MinCompositeScore=30.
  - internal/api/filtering.go: allow USDT/USDC/DAI/BUSD as quotes (base stablecoins still excluded).
  - internal/ui/momentum_display.go: momentum summary/table helpers.
  - main.go: ignore CLI args; force interactive menu.

## Files Changed (Summary)
- main.go — BuildStamp; CLI arg guard.
- internal/comprehensive/live_comprehensive.go — volume pre‑filter; regime momentum gates; percentile fallback.
- internal/ui/comprehensive_display.go — factor breakdown enabled.
- internal/models/orthogonal_weights.go — threshold tuning.
- internal/comprehensive/comprehensive.go — momentum classification threshold.
- internal/api/filtering.go — quotes allowed (USDT/USDC/DAI/BUSD).
- internal/ui/momentum_display.go — momentum UI helpers.
- tools/buildstamp/main.go — Go helper to print Jerusalem stamp.

## Planned Next Tasks
- Mode‑specific tie‑breakers:
  - Ultra: MOMO_z → VOL_SURGE → TECH_orth
  - Balanced: QUAL_res → VOL+LIQ → TECH_orth
  - Sweet‑Spot: TECH_orth(range) → VOL+LIQ → QUAL_res
- STRICT_LIVE env toggle to abort (no static fallback) for clean QA.
- Overlap monitor (Jaccard) between top‑10 lists per mode.
- CSV export of daily opps with factor columns.

## Quick Start (New Laptop/Terminal)
- Install Go 1.21+
- `cd C:\\wallet\\CProtocol`
- `go run ./tools/buildstamp` → copy stamp
- `go build -ldflags "-X main.BuildStamp=<STAMP>" -o cprotocol.exe .`
- `cprotocol.exe`
- (Optional) `setx COINMARKETCAP_API_KEY "YOUR_KEY"`

## General Information & Soft Requirements
- Project structure (Go):
  - App entry: `main.go`; live scanning: `internal/comprehensive/`; APIs: `internal/api/`; models: `internal/models/`; UI: `internal/ui/`; docs: `CProtocol/docs/`; tools: `CProtocol/tools/`.
  - Outputs: console tables; optional CSV export planned (`csv/`).
- Build & run:
  - Go‑only. Use `go run ./tools/buildstamp` then `go build -ldflags "-X main.BuildStamp=<STAMP>" -o cprotocol.exe .`.
  - Menu‑only runtime; CLI args are ignored by design.
- Coding style (Go):
  - idiomatic Go, `gofmt` formatting, clear package boundaries; no cross‑package side effects; explicit imports.
  - Naming: exported `PascalCase`, internal `camelCase`, constants `UPPER_CASE` where appropriate.
- Testing guidelines:
  - Prefer deterministic, offline unit tests for factor math and gates; mock APIs where needed; place test fixtures under a dedicated folder; avoid network in unit tests.
- Commit/PR guidelines:
  - Concise, imperative commits (e.g., “Implement regime‑aware momentum gates”).
  - PRs explain what/why, visible impact on tables, and any latency/throughput changes.
- Security/config tips:
  - Never commit secrets. Use env vars (e.g., `COINMARKETCAP_API_KEY`).
  - Start with limited scan scope if network or API quotas are constrained.
  - Prefer market data sources with rate limits you can respect.

