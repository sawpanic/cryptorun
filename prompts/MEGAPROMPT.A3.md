MEGAPROMPT — EPIC A3 (Multi-Region Replication & Validation) — with Git Update & Strict Pre-Flight
==================================================================================================

HOUSE RULES
-----------
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS.
DOCS MANDATE — UPDATE MD ON EVERY PROMPT (list exact files/anchors).
ALREADY IMPLEMENTED PRE-FLIGHT — Before edits, scan the repo. If acceptance criteria already satisfied, STOP and emit **ALREADY IMPLEMENTED** with exact file:line quotes and acceptance mapping. If tiny gaps remain, emit a minimal diff plan (patch-only) and execute only the gaps.
WRITE-SCOPE — Patch-only unless explicitly adding a new file.
EVIDENCE — Every material claim must include a direct repo quote (path:line). Maintain an Evidence Ledger.
CONSTRAINTS — Exchange-native microstructure only (no aggregators for depth/spread), USD pairs default, Kraken prioritized; MomentumCore protected; Gram–Schmidt orthogonalization; Social contribution cap (+10) after momentum/volume; regime-adaptive weights (4h cadence); point-in-time integrity; hot/warm/cold tiers; source authority & cascades; VADR/freshness/late-fill/liquidity gates.

AUTO-CONTINUE MODE
------------------
Do not pause for user approval. If output nears token limits, split automatically into subphases (A3.0, A3.1, …) and continue until acceptance passes.

0) GIT UPDATE & REPO FRESHNESS CHECK (MANDATORY)
------------------------------------------------
• Print working dir + current branch.
• Run:
  - git status -sb
  - git fetch --all --prune
  - git log --oneline -n 5
• If remote has new commits, run git pull --rebase --autostash and show result.
• Emit a short “Repo Freshness” summary: HEAD commit, author, date; files changed; merge/rebase notes.
• If repo is dirty with uncommitted changes that this phase will touch, STOP and emit a safety plan (stash/commit list).

1) PHASE SCOPE — EPIC A3 (Multi-Region & Validation)
----------------------------------------------------
Goal: Design and implement **multi-region replication** across hot/warm/cold tiers with **validation**, **anomaly detection**, and **Prometheus metrics**, plus **automated failover tests** and docs.

Primary files to create/modify (patch-only unless NEW):
- docs/DATA_MULTI_REGION.md (NEW design doc)
- internal/replication/ (NEW package: rules, planner, executors)
  - rules.go, planner.go, executors_hot.go, executors_warm_cold.go
- internal/data/validate/ (NEW package: schema+staleness+anomaly checks)
  - schema.go, staleness.go, anomaly.go
- internal/metrics/data.go (extend Prometheus metrics)
- cmd/cryptorun/cmd_replication.go (NEW CLI for simulate/failover/status)
- tests/integration/multiregion_failover_test.go (NEW)
- tests/unit/validate_*.go (NEW unit tests)
- CHANGELOG.md, PROGRESS.yaml
- docs/DATA_FACADE.md (update links/sections if this doc exists)

2) ALREADY IMPLEMENTED PRE-FLIGHT (STRICT)
------------------------------------------
Search for existing implementations:
• grep/find for: replication, active-active, active-passive, failover, warm, cold, validate, anomaly, staleness, MAD, replication_lag, consistency_errors, schema registry, quarantine.
• If acceptance (see §10) is already met, STOP and emit:
  - **ALREADY IMPLEMENTED**
  - Evidence Ledger mapping each acceptance item → file:line quote.
  - If minor gaps only, emit minimal diff plan and execute only gaps.

Evidence Ledger (pre-flight):
Feature | Status | Evidence Quote | File:Line | Spec Ref

3) DESIGN DOC — Multi-Region Rules (A3.0)
-----------------------------------------
Create docs/DATA_MULTI_REGION.md with:
• Topology: regions (e.g., eu-central, us-east), network assumptions, clock skew (NTP).
• Tiers:
  - **Hot**: active-active, WebSocket mirrors per region; local ingestion authoritative for local region; anti-entropy reconcilers.
  - **Warm/Cold**: active-passive default; async replication with SLOs; backfill jobs.
• Source authority & cascades:
  - Order of truth selection by venue/tier; “worst-feed-wins” freshness penalty recorded.
• Failure classes: regional loss, partial provider outage, split-brain, clock drift.
• Recovery playbook: promote passive → active; reconcile deltas; verify PIT.
• Metrics & SLOs table: replication lag, staleness limits, error budgets.

4) REPLICATION ENGINE — Rules & Planner (A3.1)
----------------------------------------------
Create internal/replication/rules.go:
```go
type Tier string
const (TierHot Tier="hot"; TierWarm Tier="warm"; TierCold Tier="cold")

type Mode string
const (ActiveActive Mode="active-active"; ActivePassive Mode="active-passive")

type Region string

type Rule struct{
  Tier  Tier
  Mode  Mode
  From  Region
  To    []Region
  LagSLO time.Duration // e.g., warm<=60s, cold<=5m
}

type Plan struct{
  Steps []Step
}
type Step struct{
  Tier Tier
  From, To Region
  Window TimeRange
  Validator []ValidateFn
}
Create internal/replication/planner.go:
• Build a Plan from Rules and the observed state (lag metrics, health, windows to sync).
• Enforce PIT windows (no reading beyond To).
• Choose incremental windows for warm/cold; hot tier uses WS mirror semantics.

Executors:

internal/replication/executors_hot.go — subscribe/pipe WS streams across regions with replay buffer; detect seq gaps.

internal/replication/executors_warm_cold.go — copy parquet/csv partitions and indexes with integrity checks, resumable offsets, and idempotent writes.

VALIDATION LAYER — Schema, Staleness, Anomalies (A3.2)

Create internal/data/validate/:
• schema.go — Verify required fields/types. Integrate with existing schema/registry if present.
• staleness.go — Check ts skew vs ingest clock; flag if > configurable threshold (e.g., 5s hot, 60s warm).
• anomaly.go — Robust outlier checks using MAD-based z-score for price/volume and spike detection; “quarantine” flag to downstream.

Public API:

go
Copy code
type Record map[string]any
type ValidateFn func(Record) error

func SchemaCheck(schema Schema) ValidateFn
func StalenessCheck(maxSkew time.Duration) ValidateFn
func AnomalyCheck(cfg AnomalyCfg) ValidateFn
Behavior:
• On validation error: mark record/file with a structured error; do not stop the entire pipeline unless strict=true.
• Maintain counts by {check, tier, region}.

METRICS — Prometheus Counters/Gauges (A3.3)

Extend internal/metrics/data.go:
• cryptorun_replication_lag_seconds{tier,region,source}
• cryptorun_replication_plan_steps_total{tier,from,to}
• cryptorun_replication_step_failures_total{tier,from,to,reason}
• cryptorun_data_consistency_errors_total{check} // schema|staleness|anomaly|corrupt
• cryptorun_quarantine_total{tier,region,kind}
• (Optional) histograms: cryptorun_replication_step_seconds_bucket{tier}

Ensure /metrics exposes these; add unit tests for label cardinality sanity where feasible.

CLI — Replication Simulate/Failover (A3.4)

Create cmd/cryptorun/cmd_replication.go with subcommands:

pgsql
Copy code
cryptorun replication simulate \
  --from eu-central --to us-east --tier warm \
  --window 2025-09-01T00:00:00Z/2025-09-01T06:00:00Z \
  --strict=false

cryptorun replication failover \
  --tier warm --promote us-east --demote eu-central \
  --dry-run=false

cryptorun replication status --tier warm --region us-east
• simulate prints a proposed Plan and a dry-run summary of steps + validators.
• failover promotes region for tier; writes a small state file or uses ConfigMap/DB flag.
• status prints lag, last success time, quarantine counts.

FAILOVER TESTS — Automated Integration (A3.5)

Create tests/integration/multiregion_failover_test.go:
• Use temp dirs to emulate warm/cold partitions in two “regions”.
• Seed region A with day D data; region B is behind by N hours.
• Run simulate then execute plan (direct API) to sync; assert:

Files/counters match.

cryptorun_replication_lag_seconds drops under SLO.
• Kill region A (simulate); run failover to promote region B; write to B; bring A back and ensure delta reconciliation back to SLO.
• Inject schema error and staleness skew in a partition; assert consistency_errors_total and quarantine_total increment with correct labels.

UNIT TESTS — Validation (A3.6)

Create tests/unit/validate_schema_test.go, validate_staleness_test.go, validate_anomaly_test.go:
• Schema: missing field, wrong type → error.
• Staleness: inject skew beyond threshold → error.
• Anomaly: craft spikes; MAD z-score flags expected rows; verify false-positive guard.

DOCS — Operator & SRE Playbook (A3.7)

Update/create:
• docs/DATA_MULTI_REGION.md — finalized with:

Replication diagrams (hot vs warm/cold).

Promotion/demotion flows.

SLO tables and alert thresholds.

Troubleshooting (clock drift, partial outage, split-brain).
• Link from docs/DATA_FACADE.md (if present) and product.md.
• Include CLI examples for simulate, failover, and status.

ACCEPTANCE CRITERIA (must all pass)

• Build & tests green: go build ./... && go test ./...
• Replication Plan produced for warm/cold; executors run with PIT safety.
• Validation layer catches schema/staleness/anomalies; quarantine path increments metrics.
• Prometheus metrics exposed: lag, consistency errors, quarantine, plan/step counters.
• simulate, failover, status CLI subcommands function and show informative output.
• Integration failover test demonstrates lag decrease to within SLO and correct promotion/demotion.
• docs/DATA_MULTI_REGION.md exists and is cross-linked; CHANGELOG updated (today); PROGRESS.yaml incremented with phase code A3.

EVIDENCE LEDGER (fill as you go)

Feature | Status | Evidence Quote | File:Line | Spec Ref

GIT COMMIT CHECKLIST (emit in final output)

 CHANGELOG.md updated (today) with feature/fix summary, file paths, spec refs

 Docs updated (paths)

 Tests added/updated (paths + assertions)

 Breaking changes: [none|describe]

 PROGRESS.yaml incremented; test summary attached

EXECUTION NOTES
• Auto-split into subphases A3.0 … A3.7 as needed.
• Maintain PIT discipline; never read beyond To.
• Choose minimal dependencies (no heavy frameworks); provide go.mod patch rationale if needed.
• Keep aggregator ban in microstructure firm; this phase must not bleed into gates logic.
"@

diff
Copy code

---

### macOS / Linux (POSIX shells)

```bash
claude -p "
MEGAPROMPT — EPIC A3 (Multi-Region Replication & Validation) — with Git Update & Strict Pre-Flight
==================================================================================================

HOUSE RULES
-----------
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS.
DOCS MANDATE — UPDATE MD ON EVERY PROMPT (list exact files/anchors).
ALREADY IMPLEMENTED PRE-FLIGHT — Before edits, scan the repo. If acceptance criteria already satisfied, STOP and emit **ALREADY IMPLEMENTED** with exact file:line quotes and acceptance mapping. If tiny gaps remain, emit a minimal diff plan (patch-only) and execute only the gaps.
WRITE-SCOPE — Patch-only unless explicitly adding a new file.
EVIDENCE — Every material claim must include a direct repo quote (path:line). Maintain an Evidence Ledger.
CONSTRAINTS — Exchange-native microstructure only (no aggregators for depth/spread), USD pairs default, Kraken prioritized; MomentumCore protected; Gram–Schmidt orthogonalization; Social contribution cap (+10) after momentum/volume; regime-adaptive weights (4h cadence); point-in-time integrity; hot/warm/cold tiers; source authority & cascades; VADR/freshness/late-fill/liquidity gates.

AUTO-CONTINUE MODE
------------------
Do not pause for user approval. If output nears token limits, split automatically into subphases (A3.0, A3.1, …) and continue until acceptance passes.

0) GIT UPDATE & REPO FRESHNESS CHECK (MANDATORY)
------------------------------------------------
• Print working dir + current branch.
• Run:
  - git status -sb
  - git fetch --all --prune
  - git log --oneline -n 5
• If remote has new commits, run git pull --rebase --autostash and show result.
• Emit a short “Repo Freshness” summary: HEAD commit, author, date; files changed; merge/rebase notes.
• If repo is dirty with uncommitted changes that this phase will touch, STOP and emit a safety plan (stash/commit list).

1) PHASE SCOPE — EPIC A3 (Multi-Region & Validation)
----------------------------------------------------
Goal: Design and implement multi-region replication across hot/warm/cold tiers with validation, anomaly detection, and Prometheus metrics, plus automated failover tests and docs.

Primary files to create/modify (patch-only unless NEW):
- docs/DATA_MULTI_REGION.md
- internal/replication/ (rules, planner, executors)
- internal/data/validate/ (schema, staleness, anomaly)
- internal/metrics/data.go
- cmd/cryptorun/cmd_replication.go
- tests/integration/multiregion_failover_test.go
- tests/unit/validate_*.go
- CHANGELOG.md, PROGRESS.yaml
- docs/DATA_FACADE.md (link updates)

2) ALREADY IMPLEMENTED PRE-FLIGHT (STRICT)
------------------------------------------
# (same as PowerShell block)

3) DESIGN DOC — Multi-Region Rules (A3.0)
-----------------------------------------
# (same content as PowerShell block)

4) REPLICATION ENGINE — Rules & Planner (A3.1)
----------------------------------------------
# (same content as PowerShell block)

5) VALIDATION LAYER — Schema, Staleness, Anomalies (A3.2)
---------------------------------------------------------
# (same content as PowerShell block)

6) METRICS — Prometheus Counters/Gauges (A3.3)
----------------------------------------------
# (same content as PowerShell block)

7) CLI — Replication Simulate/Failover (A3.4)
---------------------------------------------
# (same content as PowerShell block)

8) FAILOVER TESTS — Automated Integration (A3.5)
------------------------------------------------
# (same content as PowerShell block)

9) UNIT TESTS — Validation (A3.6)
---------------------------------
# (same content as PowerShell block)

10) DOCS — Operator & SRE Playbook (A3.7)
-----------------------------------------
# (same content as PowerShell block)

11) ACCEPTANCE CRITERIA (must all pass)
---------------------------------------
# (same content as PowerShell block)

12) EVIDENCE LEDGER (fill as you go)
------------------------------------
Feature | Status | Evidence Quote | File:Line | Spec Ref

13) GIT COMMIT CHECKLIST (emit in final output)
-----------------------------------------------
- [ ] CHANGELOG.md updated (today) with feature/fix summary, file paths, spec refs
- [ ] Docs updated (paths)
- [ ] Tests added/updated (paths + assertions)
- [ ] Breaking changes: [none|describe]
- [ ] PROGRESS.yaml incremented; test summary attached

EXECUTION NOTES
---------------
• Auto-split into subphases A3.0 … A3.7 as needed.
• Maintain PIT discipline; never read beyond To.
• Minimal new deps; provide go.mod patch rationale if needed.
• No aggregator leakage into any microstructure gates.
"