HOUSE RULES — CRYPTORUN (OPS & QUALITY MEGABATCH)
- OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS. If output nears truncation, stop and print:
  >>> [PAUSE - OUTPUT NEAR LIMIT] Continue with: CONTINUE <EPIC>.<PHASE>.<STEP>
- DOCS MANDATE — UPDATE MD ON EVERY PROMPT. For each change, update relevant docs (DATA_SOURCES.md, PREMOVE.md, SCORING.md, REGIME.md, GATES.md, SCHEDULER.md, DB.md, DEPLOY.md, OBSERVABILITY.md, QA.md, ALERTS.md, SECURITY.md, UI.md, CHANGELOG.md).
- ALREADY IMPLEMENTED PRE-FLIGHT — Before writing code for ANY phase:
  1) Search the repo for existing implementation + tests.
  2) If ≥90% complete and matches spec, DO NOT re-implement.
  3) Output “ALREADY IMPLEMENTED” with file paths + line refs + tiny patch plan (if any) and STOP that phase.
  4) If partial, propose SMALLEST diffs; avoid rewrites.
- WRITE-SCOPE — PATCH ONLY unless a phase explicitly adds a new file. Keep public APIs stable; if changed, include migration notes.
- NON-NEGOTIABLES
  * MomentumCore protected; factor order: MomentumCore → TechnicalResidual → VolumeResidual → QualityResidual → SocialResidual (+10 cap LAST).
  * Regime-adaptive weights: 4h cadence via majority vote (realized vol 7d, %>20MA breadth, breadth thrust).
  * Exchange-native L1/L2 only for microstructure; NO aggregators for depth/spread; Depth ≥ $100k @ ±2%; Spread < 50 bps; VADR ≥ 1.75× with ≥20 bars stability.
  * USD pairs preferred; Kraken execution examples; free/keyless APIs first; provider rate limits & circuit breakers with budget guards.
  * Guards (Freshness, Fatigue, Late-Fill) are COMPLETE individually; their EvaluateAllGates integration should already be done from your last megabatch — respect that.

UNIVERSAL IMPLEMENTATION PROTOCOL (EACH PHASE)
A) PRE-FLIGHT CHECK — List files & tests; declare ALREADY IMPLEMENTED if ≥90% → STOP.
B) DESIGN DELTA — Minimal patch plan (files/functions/interfaces/config).
C) PATCH — Apply smallest diffs; preserve public APIs where possible.
D) TESTS — Add/extend unit + table-driven tests/fixtures. `go test ./...` must pass. Show sample output.
E) DOCS & CHANGELOG — Update relevant MDs; add acceptance notes & examples. CHANGELOG bullets with file paths.
F) ACCEPTANCE REPORT — PASS/FAIL per acceptance item. If FAIL → actionable diff plan and STOP.

================================================================================
EPIC DPL — DEPLOYMENT & PERSISTENCE LAYER (Postgres/Timescale + Containers)
================================================================================

PHASE DPL.0 — DB Schemas & Migrations
Goal: Postgres/Timescale schemas for trades (hot snapshots), regime snapshots, pre-move artifacts, and job audit logs; Goose migrations.
Tasks:
  1) New files: db/migrations/*.sql (idempotent, reversible), internal/db/models.go, internal/db/repo.go (interfaces).
  2) Schemas: trades_rt (venue, symbol, ts, bid, ask, spread_bps, depth_$, vadr, provenance), regime_snap (ts, regime, indicators), premove_artifacts (ts, symbol, gates, reasons, scores), jobs_audit (job, start_ts, end_ts, status, latency_ms).
  3) Timescale hypertables on high-volume tables; indices on (symbol, ts DESC).
Acceptance:
  - `goose up` works locally; `go test ./internal/db/...` green; insertion/scan round-trips.

Git Commit Checklist:
  - "db: add schemas + goose migrations for rt metrics, regime, premove artifacts, job audit"
  - Paths listed; DB.md updated; CHANGELOG updated.

PHASE DPL.1 — Repository Interfaces + Data Facade
Goal: Read/write DB alongside existing file artifacts; point-in-time reads.
Tasks:
  1) `internal/data/facade` adds Postgres-backed readers/writers; PIT reads for reports & calibration.
  2) Config: `config/db.yaml` (dsn, max_conns, timeouts, retries).
Acceptance:
  - Dual-path read/write works; fallback to files if DB disabled; PIT queries verified in tests.

Git Commit Checklist:
  - "data: add Postgres repositories + PIT reads; config plumbing"
  - DB.md, DATA_SOURCES.md, CHANGELOG.

PHASE DPL.2 — Docker & Compose
Goal: Multi-stage Dockerfile, docker-compose for app + Postgres + Redis (if used).
Tasks:
  1) Dockerfile: builder + slim runtime; non-root; healthcheck.
  2) docker-compose.yml: services (cryptorun, postgres, grafana/prometheus optional), volumes, env, depends_on.
Acceptance:
  - `docker compose up` launches all; app connects to DB; health endpoint green.

Git Commit Checklist:
  - "deploy: multi-stage image + docker-compose; healthchecks"
  - DEPLOY.md, SECURITY.md (image hardening), CHANGELOG.

PHASE DPL.3 — Kubernetes Manifests (staging/prod)
Goal: Deployments, Services, ConfigMaps (providers/scheduler/db), Secrets, HPA, PDB, liveness/readiness.
Tasks:
  1) k8s/*.yaml: deployment with resource requests/limits; `readinessProbe` on /health; `livenessProbe` on /live.
  2) Secrets for DSN/keys; ConfigMaps for scheduler/providers; HPA (CPU or custom metrics).
Acceptance:
  - `kubectl apply` works in staging; pods become Ready; HPA scales from 1→N in a load test.

Git Commit Checklist:
  - "k8s: manifests for staging/prod + HPA/PDB + probes"
  - DEPLOY.md (kubectl steps), SECURITY.md, CHANGELOG.

PHASE DPL.4 — Secret Management & Image Security
Goal: Move any optional API keys to env/Secrets; integrate Trivy scan in CI.
Tasks:
  1) `SECURITY.md` checklist; `.github/workflows/ci.yml` adds Trivy.
  2) Audit logs for accidental secret prints (regex guard).
Acceptance:
  - CI fails on critical vulns; secrets absent from logs; allow-listed deps documented.

Git Commit Checklist:
  - "security: secrets flow + trivy + secret-scan guard"
  - SECURITY.md, CI config, CHANGELOG.

================================================================================
EPIC OBS — OBSERVABILITY, METRICS, LOGGING, SLOs
================================================================================

PHASE OBS.0 — Prometheus Metrics & JSON Logs
Goal: Instrument core paths; structured logs.
Tasks:
  1) `/metrics` with: WS freshness (per venue/symbol), rate-limit tokens, circuit states, scheduler job latencies, guard trip counts, EvaluateAllGates pass rate, alert counts.
  2) Zerolog/zap JSON logs: include `symbol, venue, ts, regime, score, gate, reason`.
Acceptance:
  - Metrics scrapeable; logs parseable; unit tests for counters.

Git Commit Checklist:
  - "obs: prometheus metrics + structured logs"
  - OBSERVABILITY.md, CHANGELOG.

PHASE OBS.1 — Grafana Dashboards
Goal: Dashboards for Operators.
Tasks:
  1) Dash JSONs under `ops/grafana/`: WS Health, Guards & Gates, Scheduler, Provider Circuits, Alerts.
  2) Provisioning docs.
Acceptance:
  - Dashboards import + render with real metrics.

Git Commit Checklist:
  - "ops: grafana dashboards for health, gates, scheduler"
  - OBSERVABILITY.md screenshots, CHANGELOG.

PHASE OBS.2 — SLOs & Alerts
Goal: Define SLOs + Alerting rules.
Tasks:
  1) SLOs: WS freshness ≤60s 99th; Guard eval latency p95 ≤200ms; job on-time rate ≥98%; alert rate ≤3/hr/user (6/hr high-vol).
  2) Alert rules + runbooks link.
Acceptance:
  - Alert rules loaded; test fires demo alarms; runbooks referenced.

Git Commit Checklist:
  - "ops: SLOs + alert rules + runbooks linkage"
  - RUNBOOKS.md, OBSERVABILITY.md, CHANGELOG.

================================================================================
EPIC QA — DETERMINISTIC REPLAY & RED-TEAM TESTING
================================================================================

PHASE QA.0 — Recorders & Golden Fixtures
Goal: Deterministic inputs for CI.
Tasks:
  1) WS/REST recorders create `fixtures/<date>/<venue>/<symbol>/...` (trades, book, klines, funding).
  2) Golden files with checksums; provenance captured.
Acceptance:
  - `make record DAY=2025-09-01` yields fixtures; tests consume them.

Git Commit Checklist:
  - "qa: live data recorders + golden fixtures"
  - QA.md, CHANGELOG.

PHASE QA.1 — Replay Harness
Goal: Full pipeline replay with fixed time.
Tasks:
  1) Harness feeds fixtures → metrics/gates/detector → artifacts; freeze time via clock interface.
  2) Deterministic outputs (hash-checked).
Acceptance:
  - CI step runs replay on sample day; acceptance gates PASS.

Git Commit Checklist:
  - "qa: deterministic replay harness + acceptance checks"
  - QA.md, CHANGELOG.

PHASE QA.2 — Property/Fuzz Tests & Boundary Tables
Goal: Hardening.
Tasks:
  1) Property tests for guards and gates (e.g., monotonicity around thresholds).
  2) Fuzz parsers for provider payloads.
Acceptance:
  - Fuzz finds no panics; boundary tables pass.

Git Commit Checklist:
  - "qa: property & fuzz tests for guards/providers"
  - QA.md, CHANGELOG.

================================================================================
EPIC NOTIFY — OPERATOR NOTIFICATIONS (Slack/Telegram)
================================================================================

PHASE NOTIFY.0 — Connectors & Templates
Goal: Alerting to Slack/Telegram with provenance.
Tasks:
  1) Connectors (`internal/notify/*`), markdown/text templates with symbol/regime/score/gates/microstructure snapshot.
  2) Per-channel rate limits (3/hr user; 10/day; high-vol 6/hr).
Acceptance:
  - Dry-run sends sample; limits enforced in tests.

Git Commit Checklist:
  - "notify: slack/telegram connectors + templates + limits"
  - ALERTS.md, CHANGELOG.

PHASE NOTIFY.1 — Manual Override & Muting
Goal: Ops control.
Tasks:
  1) CLI/HTTP to mute/unmute symbols/channels; timeboxed mutes; audit log.
Acceptance:
  - Muting prevents sends; audit entries created.

Git Commit Checklist:
  - "notify: mute/unmute + audit"
  - ALERTS.md, SECURITY.md, CHANGELOG.

================================================================================
EPIC PERF — PERFORMANCE & LOAD
================================================================================

PHASE PERF.0 — Profiling & Benchmarks
Goal: pprof, targeted benches.
Tasks:
  1) CPU/heap profiles on scoring/gates/detector; `bench_test.go` for hot paths.
Acceptance:
  - Bench baselines recorded; guidance for regressions.

Git Commit Checklist:
  - "perf: pprof integration + benches for hot paths"
  - PERF.md, CHANGELOG.

PHASE PERF.1 — WS Throughput & Backpressure
Goal: Sustain N symbols × 4 venues at 1Hz derived metrics.
Tasks:
  1) Bounded queues; drop policies; backpressure logs/metrics.
  2) Load test simulating N× throughput.
Acceptance:
  - Target throughput hit; no OOM; bounded latency.

Git Commit Checklist:
  - "perf: WS throughput/backpressure + load test"
  - PERF.md, OBSERVABILITY.md, CHANGELOG.

================================================================================
EPIC COMPLY — COMPILE/RUNTIME GUARDS & ROBOTS/TOS
================================================================================

PHASE COMPLY.0 — “No Aggregators” Compile-Time Guard
Goal: Enforce venue-native microstructure.
Tasks:
  1) Build tag `no-aggregators`; CI step greps for banned endpoints; tests fail if used.
Acceptance:
  - CI fails on banned imports/calls; allowed venues pass.

Git Commit Checklist:
  - "comply: build tag + CI guard against aggregators"
  - SECURITY.md, DATA_SOURCES.md, CHANGELOG.

PHASE COMPLY.1 — Robots/TOS Gate & Attribution
Goal: Respect providers.
Tasks:
  1) Robots/TOS notes per provider; attribution strings in UI/CLI.
Acceptance:
  - Audit shows compliant usage; attribution visible.

Git Commit Checklist:
  - "comply: robots/TOS guardrails + attribution"
  - SECURITY.md, UI.md, CHANGELOG.

================================================================================
EPIC ASSET — UNIVERSE & HYGIENE (WHITELISTS, LOW-LIQUIDITY, SCHEDULES)
================================================================================

PHASE ASSET.0 — Universe Builder & Filters
Goal: USD pairs only; liquidity floors; venue schedules.
Tasks:
  1) Builder from providers: ADV floors, spread/depth/VADR requirements; exclude stablecoins unless flagged.
  2) Venue trading schedules (maintenance windows) in config.
Acceptance:
  - Universe file produced nightly; scheduler uses it; exclusions logged.

Git Commit Checklist:
  - "asset: universe builder + liquidity/schedule filters"
  - DATA_SOURCES.md, SCHEDULER.md, CHANGELOG.

PHASE ASSET.1 — Blacklists/Whitelists & Escalations
Goal: Operator control.
Tasks:
  1) Configured allow/deny lists; CLI to add/remove with TTL.
Acceptance:
  - Lists applied at scan time; audit trail kept.

Git Commit Checklist:
  - "asset: allow/deny lists + TTL + audit"
  - RUNBOOKS.md, CHANGELOG.

================================================================================
EPIC RUNBOOK — OPERATOR PLAYBOOKS & ONBOARDING
================================================================================

PHASE RUNBOOK.0 — Incident, Recovery, DR
Goal: Practical docs + scripts.
Tasks:
  1) RUNBOOKS.md: “WS outage,” “Provider 429 storm,” “DB failover,” “Calibration drift,” “Excess alerts.”
  2) Scripts: quick probe, provider state dump, replay last hour.
Acceptance:
  - Tabletop exercise documented; scripts run.

Git Commit Checklist:
  - "ops: incident runbooks + ops scripts"
  - RUNBOOKS.md, OBSERVABILITY.md, CHANGELOG.

PHASE RUNBOOK.1 — Developer Onboarding
Goal: Zero-to-first-scan.
Tasks:
  1) ONBOARDING.md: prerequisites, `make up`, local fixtures replay, first report, first alert (dry run).
Acceptance:
  - New env reaches first scan in <30 minutes following doc.

Git Commit Checklist:
  - "docs: dev onboarding with fixtures + first-scan guide"
  - ONBOARDING.md, CHANGELOG.

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
Begin with EPIC DPL, PHASE DPL.0 pre-flight. Remember: STOP immediately if ALREADY IMPLEMENTED and provide evidence & a tiny diff plan.
