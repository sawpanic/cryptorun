HOUSE RULES — CRYPTORUN
- OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS. If output nears truncation, stop and print: 
  >>> [PAUSE - OUTPUT NEAR LIMIT] Tell me to continue with: CONTINUE <EPIC>.<PHASE>.<STEP>
- DOCS MANDATE — UPDATE MD ON EVERY PROMPT. For each change, update relevant docs (SCORING.md, PREMOVE.md, DATA_SOURCES.md, REGIME.md, GATES.md, UI.md, SECURITY.md, CHANGELOG.md).
- ALREADY IMPLEMENTED PRE-FLIGHT — Before writing code for ANY phase:
  1) Search repo for existing implementation and tests. 2) If ≥90% functionality exists and matches spec, DO NOT re-implement. 
  3) Output a clear “ALREADY IMPLEMENTED” report with file paths + line refs, diff summary (what tiny patch remains), and STOP that phase. 
  4) If partial, propose smallest diffs necessary; avoid rewrites.
- WRITE-SCOPE — PATCH ONLY unless a phase explicitly calls for a new file. Preserve public API unless specified.
- NON-NEGOTIABLE PRODUCT CONSTRAINTS
  * MomentumCore is protected (never residualized). Factor hierarchy: MomentumCore → TechnicalResidual → VolumeResidual → QualityResidual → SocialResidual (cap).
  * Regime-adaptive weights: 4h cadence (TRENDING/CHOPPY/HIGH-VOL) via majority vote (realized vol 7d, %>20MA, breadth thrust).
  * Social contribution hard cap +10, applied LAST (after Momentum/Volume residuals).
  * Microstructure gates via exchange-native L1/L2 only (Binance/OKX/Coinbase/Kraken); NO depth/spread from aggregators. Depth ≥ $100k within ±2%; spread < 50 bps; VADR ≥ 1.75× with stability/ADV rules.
  * USD pairs preferred. Kraken prioritized for execution examples.
  * Free/keyless APIs by default; provider-aware rate limits & circuit breakers; budget guards for free tiers.
  * Entry gates & exits as specified; late-fill < 30s; freshness ≤ 2 bars; fatigue guard (24h > +12% & RSI4h > 70 unless renewed accel).

QUEUE-RUNNER MEGABATCH — EPICS & PHASES
Run epics in order. Within each epic, run phases in order. After every phase: run ACCEPTANCE and only advance on PASS. If FAIL, print minimal diff plan and re-attempt.

================================================================================
EPIC P0 — PRODUCTION GATERS (SCORING, GUARDS, REGIME, REALTIME)
================================================================================

PHASE P0.0 — UNIFY SCORING TO SINGLE MOMENTUMCORE-PROTECTED PIPELINE
Goal: One composite scorer; MomentumCore protected; residualization order fixed; weight sum = 100%; remove parallel/duplicate pipelines.
Pre-flight:
  - Search for multiple composite paths (e.g., internal/domain/scoring/composite.go, internal/score/composite/, optimized/weights paths, legacy scorer).
  - If unified already and documented, emit ALREADY IMPLEMENTED with evidence and STOP.
Tasks:
  1) Ensure single entry point (e.g., internal/domain/scoring/composite.go). Remove or deprecate alternate pipelines.
  2) Enforce factor order: MomentumCore → TechnicalResidual → VolumeResidual → QualityResidual → SocialResidual.
  3) Normalize weights to 100%; relocate portfolio/risk controls out of alpha into constraints.
  4) Add unit tests: (a) factor order invariants; (b) MomentumCore never residualized; (c) identical scores from all UI/code paths.
Docs:
  - Update SCORING.md (diagram + equations), CHANGELOG.md (breaking changes if any).
Acceptance:
  - `go test ./...` passes; unit tests confirm order & weight normalization.
Git Commit Checklist:
  - Commit message: "scoring: unify to single MomentumCore-protected composite; normalize weights; tests"
  - CHANGELOG.md entry with scope, paths, tests added
  - Cross-link to SCORING.md section refs

PHASE P0.1 — SOCIAL SCORE HARD CAP (+10) APPLIED LAST
Goal: Cap SocialResidual contribution to +10 and apply LAST.
Pre-flight: search for any social cap logic; if exists post-residual and last, ALREADY IMPLEMENTED → STOP.
Tasks:
  1) Implement `ApplySocialCap()` in scoring layer; clamp social delta to +10.
  2) Wire cap AFTER Technical/Volume/Quality.
  3) Add unit tests: prove cap cannot exceed +10; verify order of application via delta snapshots.
Docs: SCORING.md (cap semantics), CHANGELOG.md.
Acceptance: tests show cap enforced; composite delta order verified.

PHASE P0.2 — GUARDS: FRESHNESS, FATIGUE, LATE-FILL
Goal: Implement three hard guards with reason codes and CLI "why/why-not".
Pre-flight: search internal/guards/*; if present & tested, ALREADY IMPLEMENTED → STOP.
Tasks:
  1) Freshness Guard: block if signal age > 2 bars OR |px - trigger| > 1.2×ATR(1h).
  2) Fatigue Guard: block if 24h return > +12% AND RSI(4h) > 70 unless 4h accel just flipped positive.
  3) Late-Fill Guard: reject if time from signal to fill > 30s.
  4) Implement reason codes; add CLI report listing which guard tripped.
  5) Unit tests with synthetic fixtures for each guard boundary.
Docs: GUARDS.md; CLI.md updated; CHANGELOG.md.
Acceptance: guard unit tests pass; CLI shows reasons deterministically.

PHASE P0.3 — AUTO REGIME SWITCHING (4H CADENCE)
Goal: Connect detector to weight profiles, switching automatically with logs.
Pre-flight: search internal/domain/regime; if auto-switch exists with 4h cadence & logs, ALREADY IMPLEMENTED → STOP.
Tasks:
  1) Detector majority vote of (realized vol 7d, %>20MA breadth, breadth thrust).
  2) Map TRENDING/CHOPPY/HIGH-VOL → weight sets; rescore every 4h.
  3) Emit log lines on regime changes with indicators and weights used.
  4) Unit tests: canned regimes → expected profiles; rescoring deltas verified.
Docs: REGIME.md; SCORING.md note on dynamic weights; CHANGELOG.md.
Acceptance: tests pass; manual run shows regime swap logs.

PHASE P0.4 — EXCHANGE-NATIVE WEBSOCKET STREAMING (HOT SET)
Goal: WS consumers for Binance/OKX/Coinbase/Kraken; compute spread, depth (±2%), VADR, movement, freshness; NO aggregators for depth/spread.
Pre-flight: search internal/streaming; if live WS with these derived metrics exists, ALREADY IMPLEMENTED → STOP.
Tasks:
  1) Implement per-venue WS clients (trades, book diffs, klines where available).
  2) Derive L2 depth within ±2%, best bid/ask spread bps; VADR (1h vs 7d per-hour); movement/freshness.
  3) Anti-rate-limit & reconnect backoff; provenance tags per metric (venue, window).
  4) Metrics/health endpoints for WS freshness (≤60s).
  5) Tests: simulated WS frames; calc correctness on fixtures.
Docs: DATA_SOURCES.md (WS), GATES.md (metric provenance), CHANGELOG.md.
Acceptance: live demo shows metrics updating; tests green.

PHASE P0.5 — MICROSTRUCTURE GATE LOCKS
Goal: Enforce spec thresholds and provenance in gate reports.
Tasks:
  1) Ensure checks: spread < 50 bps; depth ≥ $100k @ ±2%; VADR ≥ 1.75× (with stability ≥ 20 bars, ADV guards).
  2) Add provenance fields (venue, window, sample size) to GateReport.
  3) Unit tests on borderline cases (49 vs 51 bps, $99,999 vs $100,000).
Docs: GATES.md with examples; CHANGELOG.md.
Acceptance: tests pass; gate report displays provenance.

PHASE P0.6 — PORTFOLIO CORRELATION CONTROL
Goal: Prune post-gate list to enforce exposure hygiene.
Rules:
  - Pairwise corr ≤ 0.65; sector/ecosystem caps; beta-to-BTC ≤ 2; single position ≤ 5%; total exposure ≤ 20%.
Tasks:
  1) Implement correlation matrix from rolling returns; apply pruner.
  2) Deterministic pruning order (keep highest calibrated score, drop others).
  3) Tests with synthetic correlated sets.
Docs: RISK.md; CHANGELOG.md.
Acceptance: tests show caps enforced and deterministic.

PHASE P0.7 — SSE THROTTLING ≤ 1 HZ
Goal: Throttle UI event streams to 1 event/sec per client; token bucket with logging.
Tasks:
  1) Shared throttle for all SSE endpoints.
  2) Metrics on drops and active clients.
  3) Load test to verify cap.
Docs: UI.md; CHANGELOG.md.
Acceptance: load test passes; metrics confirm ≤1 Hz.

================================================================================
EPIC P1 — QUALITY & RESILIENCE (VOLUME CONFIRM, CVD, GATES, CIRCUITS, EXITS)
================================================================================

PHASE P1.0 — REGIME-AWARE VOLUME CONFIRM (VADR)
Goal: Enforce VADR ≥ 1.75× base with regime tint; stability ≥ 20 bars; ADV sanity.
Tasks:
  1) Compute per-venue VADR (1h vs 7d per-hour); regime multipliers if needed.
  2) Block entries failing volume confirm under risk-off/BTC-driven as required by PREMOVE.
  3) Tests covering regime variants and insufficient history.
Docs: PREMOVE.md; GATES.md; CHANGELOG.md.
Acceptance: tests demonstrate correct toggles by regime and data sufficiency.

PHASE P1.1 — CVD RESIDUALIZATION + R² FALLBACK
Goal: `cvd_residual = cvd_norm − β×vol_norm` with robust regression; if R² < 0.30 or n < 200, halve weight.
Tasks:
  1) Implement regression fit with winsorization (±3σ); nightly refit; log β, R², n.
  2) Tests for fit and fallback path.
Docs: PREMOVE.md; ANALYTICS.md; CHANGELOG.md.
Acceptance: unit tests pass; nightly logs show model stats.

PHASE P1.2 — 2-OF-3 GATE PRECEDENCE + RISK-OFF VOLUME-CONFIRM
Goal: Encode precedence (Funding ∨ Supply ∨ Whales) require 2-of-3; in risk-off/BTC-driven add volume-confirm.
Tasks:
  1) Gate A: funding divergence; Gate B: supply squeeze/2-of-4 proxy; Gate C: whale composite.
  2) Table-driven tests for all combos/regimes.
Docs: PREMOVE.md updated truth table; CHANGELOG.md.
Acceptance: tests pass; detector explains “why/why-not”.

PHASE P1.3 — PROVIDER RATE LIMITS & CIRCUIT BREAKERS (FREE-FIRST)
Goal: Respect provider budgets; graceful backoff/restore probes.
Tasks:
  1) Parse headers (e.g., X-MBX-USED-WEIGHT); exponential backoff on 429/418.
  2) Budget guards (e.g., CoinGecko daily cap); cooldown + restore probes before re-enable.
  3) Tests simulating overuse; metrics counters.
Docs: DATA_SOURCES.md; SECURITY.md; CHANGELOG.md.
Acceptance: simulated overload triggers circuit & successful restore.

PHASE P1.4 — EXIT ORDERING (FIRST-TRIGGER-WINS)
Goal: Implement fixed exit hierarchy: Hard stop (1.5×ATR) → Venue health tighten → 48h time-stop → Accel reversal → Momentum fade (1h & 4h) → Trailing tighten → Profit targets.
Tasks:
  1) Deterministic state machine; cause-of-exit tagging.
  2) Backtest harness that reports exit distribution.
Docs: EXITS.md; CHANGELOG.md.
Acceptance: harness shows ordering invariants and reproducible cause-of-exit stats.

================================================================================
EPIC P2 — CALIBRATION, ALERTS, UI
================================================================================

PHASE P2.0 — ISOTONIC CALIBRATION (SCORE → P(move>5% in 48h))
Goal: Fit monotonic isotonic per regime; monthly refresh; report decile lift.
Tasks:
  1) Build calibration dataset (label move>5% in 48h); fit per-regime.
  2) Emit curves, residuals; freeze windows.
  3) Tests for monotonicity and refresh cadence.
Docs: CALIBRATION.md; CHANGELOG.md.
Acceptance: scanner can print calibrated probabilities; report exists.

PHASE P2.1 — ALERT GOVERNANCE
Goal: Volatility-aware alert limits (e.g., 3/hr, 10/day; 6/hr in high-vol); temporal multipliers; dampen underperforming windows.
Tasks:
  1) Rate counters per user/channel; schedule-aware multipliers.
  2) Learning dampener for bad windows.
Docs: ALERTS.md; CHANGELOG.md.
Acceptance: counters enforced; logs show multipliers/dampening.

PHASE P2.2 — COIL BOARD & DEEP-DIVE PANELS (≤1 HZ)
Goal: Trader-facing “state machine” views (QUIET/WATCH/PREPARE/PRIME/EXECUTE), gate badges, regime banner, microstructure badges, and “why/why-not.”
Tasks:
  1) UI components reading SSE with ≤1 Hz throttle; accessibility and attribution.
Docs: UI.md; CHANGELOG.md.
Acceptance: demo with example assets; refresh rate verified.

================================================================================
UNIVERSAL IMPLEMENTATION PROTOCOL (APPLIES TO EVERY PHASE)
================================================================================
A) PRE-FLIGHT IMPLEMENTATION CHECK
   - List files and functions you found; paste exact paths and line spans; declare ALREADY IMPLEMENTED if ≥90% complete, then STOP.

B) DESIGN DELTA (IF NEEDED)
   - Show minimal patch plan (functions touched, new files, interfaces).

C) PATCH
   - Apply smallest diffs. Avoid changing public APIs unless required; if so, provide migration note.

D) TESTS
   - Add/extend unit tests, table-driven tests, or fixtures. `go test ./...` must pass. Provide sample test outputs.

E) DOCS & CHANGELOG
   - Update relevant docs; include minimal reproducible examples and acceptance notes.
   - Append CHANGELOG.md with neat bullet points referencing file paths.

F) ACCEPTANCE REPORT
   - Print PASS/FAIL per acceptance item. If FAIL, print actionable diff plan and STOP (do not proceed to next phase).

================================================================================
ANTI-TIMEOUT SUBPHASING
================================================================================
- If output length risks truncation: 
  1) Finish current sub-step and stop.
  2) Print: >>> [PAUSE - OUTPUT NEAR LIMIT] Continue with: CONTINUE <EPIC>.<PHASE>.<STEP>
- On resume, reprint a one-line recap of last completed step and proceed.

================================================================================
START RUN
================================================================================
Begin with EPIC P0, PHASE P0.0 pre-flight. Remember: STOP immediately if ALREADY IMPLEMENTED and provide the evidence & tiny diff plan. 
