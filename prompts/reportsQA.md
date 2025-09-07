REPORTING.MONITORING.METRICS
HOUSE RULES
––––––––––––––––––––––––––––––––––––––––––––
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT
ALREADY IMPLEMENTED PRE-FLIGHT — STOP if repo already has metrics/reports; output ALREADY IMPLEMENTED + file/lines
WRITE-SCOPE — Patch-only, list exact files/lines; no rewrites
CITATIONS — Must cite spec refs:contentReference[oaicite:1]{index=1}:contentReference[oaicite:2]{index=2}

TASK: Implement **Reporting & Monitoring Framework** per roadmap:contentReference[oaicite:3]{index=3}.

PHASES:

1. **Performance Reports**
   - Add system-wide performance metrics: hit rate, P&L, Sharpe ratio.
   - Add position-level risk metrics: max drawdown, stop exit counts, time-limit exits.
   - Write report generators in `internal/reports/` outputting Markdown + CSV.

2. **Portfolio Monitoring**
   - Track correlation caps, beta budget, sector exposure in live reports.
   - Emit alerts if caps breached (warn only, not hard gate here).

3. **Observability Dashboards**
   - Instrument Prometheus metrics for:  
     • scan latency P99,  
     • cache hit rate,  
     • API errors & circuit breaker triggers,  
     • regime flips,  
     • alert emission counts.
   - Add Grafana dashboards (`deploy/grafana/*`) with charts and thresholds.

4. **Docs**
   - Write `docs/REPORTING.md` covering reports, metrics, dashboards.
   - Update `SECURITY.md` to include dependency monitoring scanners (Trivy, Snyk optional).

ACCEPTANCE
–––––––––––––––––––––––––
- Reports run after scans, output Markdown + CSV in `/reports`.
- Prometheus `/metrics` endpoint exposes data:contentReference[oaicite:4]{index=4}.
- Grafana dashboards importable and functional.
- Docs + CHANGELOG updated.


Git Commit Checklist for Prompt 4

 Add internal/reports/*.go, deploy/grafana/*

 Update docs: docs/REPORTING.md, SECURITY.md

 Update CHANGELOG.md

 Add tests: tests/integration/reports/*

 PROGRESS.yaml milestone bump (Reporting)

Prompt 5 — SCHEDULER.JOBS.CROSSPLATFORM
HOUSE RULES …
TASK: Replace brittle OS-specific timers with cross-platform Scheduler/Job Queue:contentReference[oaicite:5]{index=5}.

PHASES:

1. **Scheduler Implementation**
   - Implement YAML-driven job definitions in `config/jobs.yaml`:
     • S1 — Hot scans every 15m
     • S2 — Warm scans every 2h
     • S3 — Regime refresh every 4h
     • S4 — Reports daily
   - Implement Go scheduler in `internal/scheduler/loop.go` reading jobs.yaml.

2. **Job Queue & CLI**
   - Add CLI commands: `jobs list`, `jobs run`, `jobs pause`, `jobs resume`, `jobs next`.
   - Add persistence of job history to DB (optional, otherwise local log file).

3. **Cross-Platform Guarantee**
   - Replace Windows `timeout` artifacts with portable cron/Go timers.
   - Ensure jobs can be dynamically rescheduled without rebuild.

4. **Docs**
   - Write `docs/SCHEDULER.md` describing cadence, job definitions, and CLI usage.
   - Update `CLAUDE.md` build/run instructions.

ACCEPTANCE
–––––––––––––––––––––––––
- `cryptorun jobs list` shows all jobs with next run times.
- Jobs run on schedule in Linux, Windows, macOS (tested).
- No external shell dependencies (`timeout`, `cron`); all in Go.


Git Commit Checklist for Prompt 5

 Add internal/scheduler/loop.go, config/jobs.yaml

 Update docs: docs/SCHEDULER.md, CLAUDE.md

 Add tests: tests/unit/scheduler/*

 Update CHANGELOG.md

 PROGRESS.yaml milestone bump (Scheduler)

Prompt 6 — QA.CONFORMANCE.TESTS
HOUSE RULES …
TASK: Implement **QA & Conformance Suite** per specs:contentReference[oaicite:6]{index=6}:contentReference[oaicite:7]{index=7}.

PHASES:

1. **Unit/Integration Tests**
   - Add tests for:  
     • Orthogonalization order (Momentum protected),  
     • VADR stability (≥20 bars),  
     • Entry gate sequencing (movement → volume → liquidity → freshness → microstructure),  
     • Regime transitions every 4h,  
     • Premove R² guard (CVD regression fallback).

2. **QA Runner Enforcement**
   - Enhance QA runner to enforce:  
     • No TODO/FIXME stubs,  
     • Banned tokens not present,  
     • PROGRESS.yaml milestones strictly increasing.

3. **Backtest Hooks**
   - Integrate Premove backtest harness into CI; run decile lift analysis & edge calibration.
   - Fail CI if conformance tests fail (factor order, guards, aggregator ban, social cap).

4. **Docs**
   - Write `docs/QA_TESTS.md` listing all QA/conformance tests and how to run them.
   - Update `CHANGELOG.md` with QA improvements.

ACCEPTANCE
–––––––––––––––––––––––––
- `go test ./...` runs full suite with new QA tests.
- QA runner fails on stub/banned token presence.
- Conformance suite enforces spec: Momentum protected, 2-of-3 gates, social cap ≤10.


Git Commit Checklist for Prompt 6

 Add tests: tests/unit/factors/*, tests/unit/gates/*, tests/unit/regime/*, tests/unit/premove/*

 Update QA runner scripts

 Add docs: docs/QA_TESTS.md

 Update CHANGELOG.md

 PROGRESS.yaml milestone bump (QA)