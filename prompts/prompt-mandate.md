HOUSE RULES — CRYPTORUN (UPDATED FOR NEW STATUS)
- OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS. If output nears truncation, stop and print:
  >>> [PAUSE - OUTPUT NEAR LIMIT] Continue with: CONTINUE <EPIC>.<PHASE>.<STEP>
- DOCS MANDATE — UPDATE MD ON EVERY PROMPT. For each change, update relevant docs (SCORING.md, PREMOVE.md, DATA_SOURCES.md, REGIME.md, GATES.md, SCHEDULER.md, PROVIDERS.md, SECURITY.md, UI.md, CHANGELOG.md).
- ALREADY IMPLEMENTED PRE-FLIGHT — Before writing code for ANY phase:
  1) Search the repo for an existing implementation + tests. 
  2) If ≥90% complete and matches spec, DO NOT re-implement. 
  3) Output a clear “ALREADY IMPLEMENTED” report with file paths + line refs + a tiny diff plan (if any) and STOP that phase.
  4) If partial, propose the SMALLEST diffs necessary; avoid rewrites.
- WRITE-SCOPE — PATCH ONLY unless a phase explicitly adds a new file. Preserve public APIs unless explicitly allowed by the phase.
- NON-NEGOTIABLE PRODUCT CONSTRAINTS
  * MomentumCore is protected (never residualized). Factor order: MomentumCore → TechnicalResidual → VolumeResidual → QualityResidual → SocialResidual (cap +10 applied LAST).
  * Regime-adaptive weights: 4h cadence (TRENDING/CHOPPY/HIGH-VOL) via majority vote (realized vol 7d, %>20MA breadth, breadth thrust).
  * Microstructure from exchange-native L1/L2 only (Binance/OKX/Coinbase/Kraken); NO aggregators for depth/spread. Depth ≥ $100k within ±2%; spread < 50 bps; VADR ≥ 1.75× with stability (≥20 bars) and ADV sanity.
  * USD pairs preferred; Kraken prioritized in execution examples.
  * Free/keyless APIs by default; provider-aware rate limits & circuit breakers; budget guards for free tiers. Respect robots.txt and TOS.
  * Entry guards: Freshness (≤2 bars & ≤1.2×ATR(1h) from trigger), Fatigue (24h>+12% & RSI4h>70 unless renewed accel), Late-Fill (<30s). (Already implemented individually per new status; integration is now required.)
  * Pre-Movement: 2-of-3 gate (Funding/Supply/Whales); hourly sweep on Top-50 ADV; artifacts persisted; detector exists per new status; replace mocks with live data.

UNIVERSAL IMPLEMENTATION PROTOCOL (APPLIES TO EVERY PHASE)
A) PRE-FLIGHT CHECK — List files/functions found; paste exact paths + line spans; declare ALREADY IMPLEMENTED if ≥90% complete → STOP.
B) DESIGN DELTA — If needed, show minimal patch plan (files, functions, interfaces, config).
C) PATCH — Apply smallest diffs. Keep public APIs stable unless phase allows change (note migration).
D) TESTS — Add/extend unit/table-driven tests and fixtures. `go test ./...` must pass. Include sample outputs.
E) DOCS & CHANGELOG — Update relevant docs; include acceptance notes & examples. Append CHANGELOG.md bullets with file paths.
F) ACCEPTANCE REPORT — Print PASS/FAIL per acceptance item. If FAIL, print actionable diff plan and STOP (do not advance).

GLOBAL CONFIG REQUIREMENTS (IF NOT PRESENT, ADD THEM IN PHASES THAT TOUCH THEM)
- `config/providers.yaml`: provider endpoints, QPS weights, budgets, backoff, fallback chain (free-first), and health checks.
- `config/scheduler.yaml`: job names, intervals (cron or duration), enable flags, concurrency, jitter.
- `internal/infrastructure/providers/*`: live connectors (REST/WS), rate limiter, circuits, fallback registry.
- `internal/application/scheduler/*`: job dispatcher, loop runners, graceful shutdown, metrics hooks.
- `internal/domain/gates/*`: EvaluateAllGates integration of Fatigue/Freshness/LateFill (+ microstructure if required).
- `internal/domain/premove/*`: analyzers wired to live providers with fixtures for offline tests.

================================================================================
EPIC G0 — GUARDS INTEGRATION (CRITICAL PATH)
================================================================================

PHASE G0.0 — Implement Unified EvaluateAllGates() Integration
Context (from new status): Individual guards COMPLETE (Fatigue/Freshness/Late-Fill). `EvaluateAllGates` is a STUB returning allow=true.
Goal: Integrate all existing guards into a single orchestration with reason codes and CLI “why/why-not”.
Pre-flight:
  - Search: internal/domain/fatigue.go, freshness.go, latefill.go, any existing EvaluateAllGates() stubs.
  - If EvaluateAllGates is already integrated and tested, ALREADY IMPLEMENTED → STOP.
Tasks:
  1) Implement `internal/domain/gates/evaluate.go` with:
     - `type GateReason struct{ Name string; Passed bool; Message string; Metrics map[string]float64 }`
     - `func EvaluateAllGates(ctx, inputs) (passed bool, reasons []GateReason)`
     - Invoke: Freshness, Fatigue, Late-Fill in that order; short-circuit on hard-fail but still collect reasons.
     - Optional hook for microstructure gate bundle if available (spread/depth/VADR from live).
  2) Add CLI command: `cryptorun gates explain --symbol BTCUSD --at <timestamp>` prints reasons and metrics.
  3) Add table-driven tests covering boundary conditions (2 bars vs 3; 1.19×ATR vs 1.21×; 29s vs 31s; 24h +11.9% vs +12.1% with RSI>70).
Docs:
  - GATES.md: flow diagram, guard order, reason codes, example CLI output.
  - CHANGELOG.md: add “guards: integrate EvaluateAllGates orchestration.”
Acceptance:
  - All guard unit tests pass; new EvaluateAllGates tests pass.
  - CLI “explain” shows deterministic reasons with metrics.
Git Commit Checklist:
  - Message: “guards: integrate EvaluateAllGates orchestration + CLI explain; add boundary tests”
  - Paths touched listed; CHANGELOG updated; GATES.md updated with examples.

================================================================================
EPIC G1 — LIVE INFRASTRUCTURE (CONNECTORS, RATE LIMITS, CIRCUITS)
================================================================================

PHASE G1.0 — Provider Interfaces & Live Connector Skeletons (Free-First)
Goal: Replace mocks with live-capable providers behind interfaces and a fallback chain.
Pre-flight:
  - Search for existing provider interfaces, REST/WS clients, mocks.
  - If skeleton exists with runtime-selectable providers and config, ALREADY IMPLEMENTED → STOP.
Tasks:
  1) Define `ProviderRegistry` with named providers and capabilities:
     - Funding (perpetuals funding history/current), Spot trades, Order book L2 diffs, Klinedata/aggregate candles.
     - Supply/Reserves (venue or on-chain proxy), Whale/large-print detection (from trades), CVD (computed locally).
  2) Implement minimal live connectors (REST/WS) for Binance/OKX/Coinbase/Kraken using free/keyless endpoints where available.
  3) Config-driven selection: `config/providers.yaml` defines primary → fallback order per capability; disable aggregators for depth/spread.
  4) Add provenance to all returned data: `{venue, endpoint, window, latency_ms}`.
Docs:
  - DATA_SOURCES.md (capabilities matrix, provenance fields).
  - PROVIDERS.md (endpoints, rate-limit notes).
  - CHANGELOG.md.
Acceptance:
  - Unit tests using recorded fixtures (golden files) for each capability.
  - Live smoke command `cryptorun providers probe` prints capability table and provenance.

PHASE G1.1 — Weighted Rate Limiting (QPS & Budgets)
Goal: Provider-aware QPS weights per key, sliding-window counters, daily/monthly budget guards.
Tasks:
  1) Add rate limiter with weighted tokens per provider from `providers.yaml`.
  2) Respect headers (e.g., X-* used-weight) when available; otherwise use conservative defaults.
  3) Expose metrics: tokens available, requests blocked, cooldowns.
  4) Tests simulate bursts and verify throttling.
Docs: DATA_SOURCES.md (rate-limits), SECURITY.md (budget policy), CHANGELOG.md.
Acceptance:
  - Simulated bursts trigger throttling; metrics counters increment.

PHASE G1.2 — Circuit Breakers & Fallbacks
Goal: Resilient failure handling with restore-probe.
Tasks:
  1) For 429/418/5xx/timeouts: trip circuit; backoff (exponential + jitter); switch to next provider in chain.
  2) Periodic “restore probe” attempts to re-enable tripped provider.
  3) Expose circuit state in metrics and CLI `cryptorun providers state`.
  4) Tests simulate error cascades and restoration.
Docs: DATA_SOURCES.md (circuits & backoff), SECURITY.md (error handling), CHANGELOG.md.
Acceptance:
  - Tests pass; live demo shows failover and recovery logs.

================================================================================
EPIC G2 — SCHEDULER JOB DISPATCHER & LOOPS
================================================================================

PHASE G2.0 — Job Dispatcher Core
Context: Jobs defined in prompts/specs but no dispatcher exists.
Goal: Deterministic job runner with per-job config, concurrency, jitter, and graceful shutdown.
Tasks:
  1) Add `internal/application/scheduler/dispatcher.go`:
     - `Job` interface: `Name() string`, `Run(ctx)`; 
     - `Runner` with registry, start/stop, on-error policy (retry/backoff/skip), and metrics.
  2) Config file `config/scheduler.yaml` with jobs:
     - hot_scan (15m), warm_scan (2h), regime_refresh (4h), provider_health (5m), premove_hourly (60m).
     - enable flags, jitter %, max_concurrency, error backoff.
  3) CLI: `cryptorun sched list|start|stop|status`.
Docs: SCHEDULER.md (job lifecycle), CHANGELOG.md.
Acceptance:
  - Unit tests for registry, start/stop, and jitter.
  - `cryptorun sched list` shows configured jobs; `status` reflects running jobs.

PHASE G2.1 — Implement Loops from Spec
Goal: Implement/verify each job loop; wire to dispatcher.
Tasks:
  - hot_scan (15m): evaluate top signals using live data & EvaluateAllGates().
  - warm_scan (2h): refresh universe/metadata.
  - regime_refresh (4h): recompute regime, switch weights (already implemented); ensure it’s scheduled.
  - provider_health (5m): ping providers, record health metrics; optionally warm caches.
  - premove_hourly (60m): already COMPLETE—wire to dispatcher and ensure artifacts path & retention.
Docs: SCHEDULER.md updated with intervals & acceptance; CHANGELOG.md.
Acceptance:
  - Logs prove each job runs on schedule (with jitter) and completes within SLA.
  - Backpressure/reschedules visible if jobs overrun.

================================================================================
EPIC G3 — PRE-MOVEMENT: REPLACE MOCKS WITH LIVE DATA
================================================================================

PHASE G3.0 — Gate A (Funding) Live Data
Goal: Real funding inputs → funding_z; spot≥VWAP with CVD confirmation.
Tasks:
  1) Implement Funding provider: fetch recent funding rates (lookback adequate for z-score), normalize, compute z.
  2) Spot/VWAP: compute VWAP from trades or klines; add local CVD calculation.
  3) Replace mock analyzer functions; keep fixture playback for tests.
  4) Add provenance & latency in analyzer outputs.
Docs: PREMOVE.md (Gate A formulas & data flow), PROVIDERS.md (endpoints), CHANGELOG.md.
Acceptance:
  - Unit tests with golden fixtures; live smoke prints Gate A verdict and metrics.

PHASE G3.1 — Gate B (Supply/Reserves) Live Data or Proxy
Goal: 7d reserves change ≤ −5% across ≥3 venues (or documented, audited proxies if direct reserves unavailable via free endpoints).
Tasks:
  1) Implement provider chain for reserves/inflow/outflow proxies; compute 7d delta per venue.
  2) Majority logic: require condition across ≥3 venues; handle missing data gracefully.
  3) Replace mock Gate B with live-backed computation; keep fixtures.
  4) Document exact proxies if used (free-first), with confidence tags.
Docs: PREMOVE.md (Gate B specifics & proxy policy), PROVIDERS.md (sources), CHANGELOG.md.
Acceptance:
  - Tests cover mixed data presence; live smoke prints venue-wise deltas and decision.

PHASE G3.2 — Gate C (Whales) Live Trades → Clustered Large Prints + CVD Residual
Goal: Detect large-print clustering vs ADV and confirm with CVD residual.
Tasks:
  1) Consume live trades stream (or REST aggregate) and define “large” vs instrument ADV and recent percentile.
  2) Cluster by time/price buckets; compute signal strength.
  3) Compute CVD residual (winsorized regression vs volume); already in detector—ensure live inputs and R² fallback.
Docs: PREMOVE.md; ANALYTICS.md; CHANGELOG.md.
Acceptance:
  - Tests verify clustering thresholds and stability; live smoke prints clusters and residual stats.

================================================================================
EPIC G4 — REPORTS & ARTIFACTS: WIRE TO LIVE PIPELINE
================================================================================

PHASE G4.0 — EOD/Weekly Reports Against Live Artifacts
Context: Reports are COMPLETE. Ensure they consume scheduler outputs/artifacts paths and reflect live runs.
Tasks:
  1) Parameterize input sources (artifact directories from scheduler).
  2) Add provenance section to each report (regime profile, provider states, % mock usage = 0 target).
  3) CLI `cryptorun reports run --type eod|weekly --at <date>` uses latest artifacts.
Docs: REPORTS.md; CHANGELOG.md.
Acceptance:
  - Running after a live session produces populated reports with zero mock flags.

================================================================================
EPIC G5 — SECURITY/OPS POLISH
================================================================================

PHASE G5.0 — Secrets, Budgets, Metrics, Health
Goal: Keep costs at $0, reveal provider health, and enforce budgets.
Tasks:
  1) Ensure all providers use keyless endpoints by default; if optional keys exist, move to env vars and document.
  2) Budget guards per provider (daily/monthly); logs when approaching caps; disable non-critical features if budget low.
  3) Metrics endpoints include: provider health, circuits, rate-limit counters, scheduler job states.
Docs: SECURITY.md; DATA_SOURCES.md; SCHEDULER.md; CHANGELOG.md.
Acceptance:
  - Metrics show budget counters and circuit states; load test triggers non-invasive throttling without crashes.

================================================================================
ACCEPTANCE MATRIX (REBASED TO NEW STATUS)
================================================================================
- Guards Integration: EvaluateAllGates returns deterministic reasons and honors existing guard logic (freshness/fatigue/late-fill). CLI “explain” works.
- Live Connectors: Providers support Funding/Trades/Book/Klines (+ Supply/Reserves proxy policy), with provenance and rate limits.
- Rate Limits & Circuits: Weighted QPS throttling; circuit breakers with restore probes; fallback chain works.
- Scheduler: Dispatcher runs hot/warm/regime/health/premove on schedule with jitter, metrics, and graceful shutdown.
- Pre-Movement Live: Gates A/B/C run on live data; fixtures available for offline deterministic unit tests.
- Reports: EOD/Weekly consume live artifacts and include provenance; mock usage = 0 in production runs.
- Docs & CHANGELOG: Updated for every change with examples and acceptance notes.

================================================================================
ANTI-TIMEOUT SUBPHASING
================================================================================
If output length risks truncation:
  1) Finish the current sub-step and stop.
  2) Print: >>> [PAUSE - OUTPUT NEAR LIMIT] Continue with: CONTINUE <EPIC>.<PHASE>.<STEP>
On resume, reprint a one-line recap of the last completed step, then proceed.

================================================================================
START RUN
================================================================================
Begin with EPIC G0, PHASE G0.0 pre-flight. Remember: STOP immediately if ALREADY IMPLEMENTED and provide evidence & a tiny diff plan.
