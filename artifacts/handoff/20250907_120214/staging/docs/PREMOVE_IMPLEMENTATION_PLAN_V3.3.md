# Pre-Movement Detector v3.3 — Implementation Plan (Menu-Only, Top-100)

> **Status:** Ready for implementation  
> **Scope:** A standalone, menu-driven module that detects “coiled-spring” setups before breakouts using a **100‑point composite** (Structural 45, Behavioral 30, Catalyst/Compression 25) with **2‑of‑3 gates**, regime/correlation haircuts, and data‑health decay.  
> **Non-goals:** No CLI exposure; no aggregator books for spread/depth (venue‑native only); Coinbase derivatives (perps/funding) shown as N/A.  
> **Source spec:** *Pre‑Movement Detector v3.3 — Final Production Specification* (PDF), plus project playbooks.

---

## 1) Navigation (Menu-only)

```
📊 Dashboard
📈 Momentum Scanner
⚡ Pre‑Movement Detector  ← (new section)
  ├─ 🎯 Coil Board (Top‑100)
  ├─ 🔍 Asset Deep Dive
  ├─ 📚 Pattern Casebook (replay from PIT artifacts)
  ├─ 🔥 Catalyst Heat
  ├─ 🌍 Market Regime
  └─ 💚 System Health
```

- **No CLI command.** The `cmd/cryptorun/menu_premove.go` router only attaches pages into the app menu.

---

## 2) Go Architecture & File Map

```
src/
  domain/
    premove/
      detector.go          // interface + state machine
      score_model.go       // 100-pt v3.3 buckets + modifiers
      gates.go             // 2-of-3 gates (+ volume-confirm in certain regimes)
      residuals.go         // CVD residual (robust), OI residual vs price
      decay_regime.go      // half-life, freshness penalties (worst-feed wins)
      percentile.go        // rolling winsorized percentiles
      whale.go             // composite whale detector (2-of-3)
      microstructure.go    // depth@±2%, spread pctile, OB imbalance, spoof/iceberg
      portfolio.go         // NEW: corr ≤0.65, 2-per-sector, beta budget
      types.go             // Inputs/TierScores/Reasons/Result
  application/
    premove/
      runner.go            // Top-100 orchestration; hot(WS)/warm(REST) cadence
      universe.go          // ADV-based reconstitution (USD quote), tiering
      explain.go           // reasons (metric/value/pctl), penalties, provenance
      artifacts.go         // PIT JSONL writer for replay/casebook
      alerts.go            // NEW: governance (3/hour,10/day), manual override
      execution.go         // NEW: slippage/time-to-fill tracking & recovery
      backtest.go          // NEW: PIT replay + hit-rates + isotonic calibration stub
  infrastructure/
    providers/
      cex/{binance,okx,coinbase,kraken}   // venue-native WS/REST
      derivs/{deribit,bybit,bitmex}       // funding, OI, basis, options (Deribit)
      agg/{coingecko,paprika}             // warm context (not books)
      catalysts/{cmc,unlocks}             // event ladder + decay
  interfaces/ui/menu/
    page_premove_board.go   // Coil Board
    page_premove_asset.go   // Deep Dive tabs
    page_premove_health.go  // Data health & breakers
cmd/cryptorun/
  menu_premove.go           // mount menu pages (no CLI command)
config/
  premove.yaml              // defaults (v3.3)
docs/
  PREMOVE.md, DATA_SOURCES.md, MENUS.md, CHANGELOG.md, CLAUDE.md
tests/
  unit/*, integration/*, system/*
```

**Design notes**
- Venue‑native microstructure is required for spread/depth/OB; aggregators are for warm context only. Coinbase perps/funding/basis = **N/A**.
- Precedence snippets in code:  
  - `VADR gate = max(p80_24h, tier_min)`  
  - Supply squeeze prefers **primary reserves**; else proxy.  
  - Freshness applies **worst feed multiplier** across funding/trades/depth/basis.

---

## 3) Scoring Model (100 points, v3.3)

- **Structural (45):** derivatives/basis/liquidations; exchange flows & stablecoin dynamics; microstructure (depth, spread, OB imbalance).
- **Behavioral (30):** composite whale detector (large‑print clustering + CVD residual + maker‑pull/hot‑wallet optional); CVD/volume‑profile/absorption.
- **Catalyst & Compression (25):** BB width/ATR/squeeze/failed‑break logic; event tiers with time‑decay multipliers; social = binary +3, capped separately later.
- **Post‑score modifiers:** regime multiplier, correlation haircut, liquidity‑gradient penalty, **freshness penalty** (worst‑feed across feeds).

**States (UI ladder):** QUIET <60, WATCH 60–79, PREPARE 80–99, PRIME 100–119, EXECUTE ≥120 (UI only; decisions still respect gates).

---

## 4) Gates & Overrides

- **Critical gates (2‑of‑3 required):**  
  (A) Funding divergence with price hold,  
  (B) Supply squeeze (primary reserves if available, else proxy),  
  (C) Accumulation (CVD residual + iceberg/pull ≥ p80).  
  **Volume‑confirm** is **additive** in risk_off/BTC‑driven regimes (not a replacement).
- **Manual override path:** when `Score > 90` but `< 2 gates`, emit **alert‑only** (no “execute” affordances). System tests must check this.
- **VADR precedence:** gating values use `max(p80_24h, tier_min)` per asset tier.

---

## 5) Decay, Freshness & Regime

- **Regime detector** is **shared** with the Scanner; updated hourly; provides half‑life and weight multipliers (risk‑on, selective, BTC‑driven, risk‑off).
- **Freshness config (fixed):** soft penalty start **8s**; τ=**30s**; **hard‑fail 90s**; apply **worst feed** across funding/trades/depth/basis.

```yaml
decay:
  regime_half_life_h: { risk_on: 8, selective: 6, btc_driven: 5, risk_off: 4 }
  freshness:
    soft_start_s: 8
    tau_s: 30
    hard_fail_s: 90        # fixed (was 900)
    precedence: "worst_feed"
```

---

## 6) Portfolio & Risk Controls (NEW)

- **Apply after scoring/gates, before alerts** on every scan cycle. Enforce: pairwise correlation **≤ 0.65**, sector cap **≤ 2**, total beta to BTC **≤ 2.0**, single position **≤ 5%**, total exposure **≤ 20%**; break ties by ADV then symbol.

```yaml
portfolio:
  pairwise_corr_max: 0.65
  sector_caps: { L1: 2, DeFi: 2, Infrastructure: 2, Gaming: 2 }
  beta_budget_to_btc: 2.0
  max_single_position_pct: 5
  max_total_exposure_pct: 20
  apply_stage: post_gates_pre_alerts
```

---

## 7) Execution Quality (NEW)

- Track **intended vs actual** fills, **slippage_bps**, and **time_to_fill_ms**. If slippage exceeds **30 bps**, tighten size/spread requirements; recovery after a run of good trades or a time‑based reset.

```yaml
execution_quality:
  slippage_bps_tighten_threshold: 30
  recovery: { good_trades: 20, hours: 48 }
```

---

## 8) Alerts & Governance (NEW)

- Rate limits: **3/hour, 10/day**, with a volatility‑aware allowance (e.g., up to 6/hour when realized vol > p90).
- Governance tracks “operator fatigue” and quality; **alert‑only** mode for manual‑override cases (score>90, gates<2).

```yaml
alerts:
  per_hour: 3
  per_day: 10
  high_vol_per_hour: 6
  manual_override:
    condition: "score>90 && gates<2"
    mode: "alert_only"
```

---

## 9) Backtest & Calibration (NEW)

- **PIT replay** from artifacts (`artifacts/premove/*.jsonl`) to compute hit‑rates by state and regime, plus daily log of CVD regression **R²**.
- Fit an **isotonic calibration** (score → P(move>5%/48h)) with monthly refresh & immutability during a freeze window.

```text
Backtest outputs:
- hit_rates_by_state_and_regime.json
- isotonic_calibration_curve.json
- cvd_resid_r2_daily.csv
```

---

## 10) Data Map (free-first; venue-native for microstructure)

| Signal family          | Primary                                    | Fallback              | TTL        | Notes                                    |
|------------------------|--------------------------------------------|----------------------|------------|------------------------------------------|
| Price/klines           | Venue REST (Binance/OKX/Coinbase/Kraken)   | CoinGecko (context)  | 30–60s     | PIT cache                                 |
| L2 depth & spread      | Venue WS/REST                              | —                    | stream/15s | **No aggregators** for depth/spread       |
| Trades & CVD           | Venue WS                                   | Venue REST (short)   | stream/30s | Split spot vs perp CVD                    |
| Funding                | Venue REST                                 | Alt venue endpoint   | 5–10m      | Keep last 8 periods                       |
| Open interest          | Venue REST                                 | —                    | 5–10m      | ΔOI & residual vs ΔPrice                  |
| Basis                  | Venue REST                                 | —                    | 5–10m      | Perp/spot lead/lag                        |
| Options (skew/IV)      | Deribit public                             | —                    | 10m        | Large caps                                |
| Catalysts              | CoinMarketCal; DefiLlama (unlocks)         | TokenUnlocks (paid)  | 30–60m     | Decay buckets                             |
| Social (capped)        | Optional free feeds                         | —                    | 30–60m     | Applied **last**, cap +10 overall         |
| On-chain (optional)    | Public explorers/DefiLlama                 | —                    | 60m        | NA→0 weight for thin alts                 |

Budget guards: rate‑limit backoff; TTL doubling on degradation; staleness penalties applied to scores.

---

## 11) UI Performance & Refresh

- **Client throttle ≤ 1 Hz** for tile refreshes; state transitions (WATCH→…→EXECUTE) delivered via SSE to avoid polling storms.
- Pagination remains **25 tiles/page** for Top‑100.

---

## 12) Config Defaults (`config/premove.yaml`)

```yaml
version: 3.3
cadence: { hot_minutes: 15, warm_minutes: 60 }
universe: { size: 100, quote: USD }
weights: { structural: 45, behavioral: 30, catalyst: 25 }  # 100-pt model (v3.3)

gates:
  two_of_three: true
  volume_confirm:
    enabled_in: [risk_off, btc_driven]   # additive, not replacement

decay:
  regime_half_life_h: { risk_on: 8, selective: 6, btc_driven: 5, risk_off: 4 }
  freshness:
    soft_start_s: 8
    tau_s: 30
    hard_fail_s: 90
    precedence: worst_feed

portfolio:
  pairwise_corr_max: 0.65
  sector_caps: { L1: 2, DeFi: 2, Infrastructure: 2, Gaming: 2 }
  beta_budget_to_btc: 2.0
  max_single_position_pct: 5
  max_total_exposure_pct: 20
  apply_stage: post_gates_pre_alerts

alerts:
  per_hour: 3
  per_day: 10
  high_vol_per_hour: 6
  manual_override:
    condition: "score>90 && gates<2"
    mode: "alert_only"

execution_quality:
  slippage_bps_tighten_threshold: 30
  recovery: { good_trades: 20, hours: 48 }

learning:
  pattern_exhaustion:
    degrade_confidence_when_7d_lt_0_7_of_30d: true

sources:
  microstructure: exchange_native_only
  warm: { coingecko_ttl_s: 300, paprika_ttl_s: 300 }
```

---

## 13) Metrics & Artifacts

- Prometheus: `premove_score{symbol}`, `premove_state{symbol}`, `premove_gate_count{symbol}`, `premove_data_staleness_seconds{feed}`, `premove_slippage_bps`, `premove_alerts_rate_limited_total`, `premove_portfolio_pruned_total`.
- **Artifacts (PIT JSONL):** `symbol, ts, score, state, sub_scores, passed_gates, penalties, top_reasons, sources` (feeds the Casebook & backtests).

---

## 14) Acceptance Criteria (mapped to v3.3)

**Core (Phase 1)**  
- Winsorized percentile engine; **2‑of‑3 gate system**; CVD residual with **R² fallback**; freshness penalties (worst‑feed wins); base **100‑pt** scoring.

**Safety (Phase 2)**  
- MM withdrawal detection (30‑min pause), portfolio pruning, contamination cooldown (15‑min), liquidity‑gradient filter (≥0.70), venue health degradation handling.

**Intelligence (Phase 3)**  
- Composite whale detection (2‑of‑3), supply‑squeeze proxy (2‑of‑4), bid‑refill asymmetry, temporal learning/pattern exhaustion tracking.

**Polish (Phase 4)**  
- Execution quality tracking; **isotonic calibration**; alert governance (volatility‑aware).

**System tests (selected):**  
- Volume‑confirm sequencing: **additive** to 2‑of‑3; venue degradation rules (≥1 for gates, ≥2 for cross‑checks); portfolio pruning stage; regime transitions re‑evaluate positions; clock drift handled in freshness; **pattern exhaustion** reduces confidence; **slippage>30bps** tightens; **manual override = alert‑only**; new‑asset graduation (≥90 days and ≥10 signals).

---

## 15) Ops & Governance

- **Launch protocol & version control:** Freeze windows, 30‑day minimum between changes; change process = **propose_with_backtest → paper_trade_30_days → gradual_rollout**; emergency‑only exceptions for critical bugs/abort rules.
- **Read‑only burn‑in:** Run in parallel for 30 days (no trading) while capturing alert fatigue, CVD R² stability, and pattern‑exhaustion telemetry.

---

## 16) Claude Code Prompts (delta-safe)

### PX — Patch config & add risk/alerts/exec/backtest + SSE/refresh throttle
```
SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=PREMOVE.PATCH.V33

GOAL
1) Fix freshness hard_fail_s to 90.
2) Add portfolio/alerts/execution/backtest modules & wire them.
3) Throttle UI refresh & add SSE transitions.

SCOPE (Atomic)
- Edit config/premove.yaml: freshness.hard_fail_s=90; add portfolio/alerts/execution_quality/learning blocks (values from spec).
- Create:
  src/domain/premove/portfolio.go
  src/application/premove/{execution.go,alerts.go,backtest.go}
- Wire portfolio pruning: application/premove/runner.go applies portfolio after gates, before alerts.
- Wire execution tracking to /metrics and artifacts.
- UI: interfaces/ui/menu/page_premove_board.go → client refresh ≤1Hz; SSE push on state transitions.

GUARDS
- Follow v3.3 precedence: portfolio after scoring/gates; alerts honor rate limits; manual override → alert_only.
- Keep microstructure venue-native; no aggregators for depth/spread.

ACCEPTANCE
- go build ./... && go test ./... green (see CLAUDE.md build/test flow).
- /metrics exposes: premove_slippage_bps, premove_alerts_rate_limited_total, premove_portfolio_pruned_total.
```

### PY — Backtest & isotonic calibration
```
SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=PREMOVE.BACKTEST.V33

GOAL
Add PIT replay from artifacts to compute hit rates by state (WATCH/PREPARE/PRIME/EXECUTE), regime-sliced,
and fit an isotonic calibration curve (score → P(move>5% in 48h)).

SCOPE
- Implement src/application/premove/backtest.go:
  - Load artifacts/premove/*.jsonl (point-in-time only).
  - Compute per-state, per-regime hit-rates; log R² for CVD residual daily fits.
  - Isotonic calibration stub: monotone mapping with monthly refresh and immutability guarantees.
- docs/PREMOVE.md: add “Calibration & Governance” with monthly refresh and freeze process.

ACCEPTANCE
- Backtest unit tests with deterministic fixtures.
- CLI-free invocation via internal test harness; no network in tests.
```
