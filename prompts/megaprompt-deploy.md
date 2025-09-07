Deploy/Secrets/Providers/Monitoring/Docs/Perf/Research)
HOUSE RULES (READ FIRST)

OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS.
DOCS MANDATE — UPDATE MD ON EVERY PROMPT.
ALREADY IMPLEMENTED PRE-FLIGHT — If a feature already exists, STOP and emit ALREADY IMPLEMENTED + exact file paths/lines; do not re-implement.
WRITE-SCOPE — Patch-only; list exact files to add/edit; no broad rewrites.
CITATIONS IN PR — Quote repo paths/lines or spec docs in the PR description Evidence Ledger.

SPEC ANCHORS (for acceptance)

Build/Test/Lint, layering, envs: REDIS_ADDR, PG_DSN, METRICS_ADDR, KRAKEN_API_BASE, KRAKEN_WS_URL (CLAUDE.md).

Scanner modes, scoring bands, gates & exits (PRD v3.0).

Free-first, provider-aware circuit breakers, exchange-native microstructure only, regime model, latency targets (v3.2.1).

Source catalog, TTLs, rate limits, cascades, venue health (Playbook v1.0).

Outstanding deployment/observability/DB to-dos (Status 9-7).

PRE-FLIGHT (DO THIS BEFORE ANY CODING)

Repo scan for prior implementations: Dockerfile, docker-compose.yml, deploy/k8s/*, docs/DEPLOYMENT.md, SECURITY.md, MONITORING.md, Trivy CI, commit-lint, provider adapters (Kraken L1/L2/trades/ticker), derivatives provider (funding/OI/basis), DeFi metrics provider, Prometheus metrics, Grafana json, alerting, cryptorun health CLI, perf tests (k6/Locust), regression diffs, factor updates for on-chain/derivs.

If found and sufficient: STOP and output ALREADY IMPLEMENTED with paths/lines + short diff plan only.

Otherwise proceed, one phase at a time in order below.

PHASE DKR-1 — Multi-Stage Dockerfile (+ .dockerignore)

Goal: production-grade, reproducible image.
Write-Scope:

Dockerfile (multi-stage: builder w/ Go 1.21+, go test ./... && golangci-lint run ; runtime: distroless/ubi-micro, non-root).

.dockerignore (trim build context: src, go.mod, go.sum, config, docs, exclude tests/artifacts, .git, **/*.csv).

Respect envs from CLAUDE.md in ENTRY/CMD/env pass-through (REDIS_ADDR, PG_DSN, METRICS_ADDR, KRAKEN_API_BASE, KRAKEN_WS_URL).
Acceptance:

docker build succeeds, runs unit tests and lints in builder stage.

docker run … cryptorun monitor serves /health and /metrics (per CLAUDE.md run targets).
Commit Checklist: Update CHANGELOG; add Docker build/run doc stubs in DEPLOYMENT.md.

PHASE DKR-2 — docker-compose (CryptoRun + Redis + Postgres/TimescaleDB)

Goal: local/dev orchestration with health checks.
Write-Scope:

deploy/compose/docker-compose.yml

Services: cryptorun, redis, postgres (TimescaleDB image), named volumes, single user-defined network.

Healthchecks: Redis redis-cli ping; Postgres pg_isready; CryptoRun /health.

deploy/compose/.env.example (document required envs from CLAUDE.md).
Acceptance: docker compose up -d → all healthy; cryptorun monitor exposes /metrics for Prometheus scrape. v3.2.1 and Playbook mandate free-first + metrics exposure & latency targets; keep it lean.
Commit Checklist: DEPLOYMENT.md section “Local (Compose)” + env table.

PHASE K8S-1 — Kubernetes Manifests (Staging & Prod)

Goal: Namespaced, secrets-safe, observable deploys.
Write-Scope:

deploy/k8s/base/ — namespace.yaml, serviceaccount.yaml, configmap.yaml (non-secret config), secret.yaml (K8s Secret; staged via CI), deployment.yaml (readiness /health, liveness /health, envFrom), service.yaml (ClusterIP), Prometheus scrape annotations on /metrics.

deploy/k8s/overlays/{staging,prod}/kustomization.yaml (image tag, replicas, resources, env diffs, HPA optional).

Secrets: use K8s Secret refs + doc the no-secrets-in-code rule (CLAUDE.md) and v3.2.1 free-first APIs.
Acceptance:

kubectl apply -k deploy/k8s/overlays/staging → pods Ready; /metrics scraped; ConfigMap rollouts update cleanly.
Commit Checklist: DEPLOYMENT.md “Kubernetes” w/ kubectl and Kustomize steps.

PHASE SEC-1 — Secret Management & Security Hardening

Goal: eliminate secrets-in-code; active scanning.
Write-Scope:

Switch all sensitive config to env/secret lookups (ensure code paths only read from env). CLAUDE.md requires no secrets in code.

docs/SECURITY.md — secret handling, access control, code review checklists, don’t log secrets, dependency policy.

CI: add Trivy vulnerability scan job; add commit-lint / conventional commits and docs-first gate (fail if code changes reference undocumented features).
Acceptance: Secrets only via env or K8s Secret; CI fails on critical vulns; PR template includes Evidence Ledger.
Commit Checklist: SECURITY.md + CI YAML; CHANGELOG entry.

PHASE PRV-1 — Provider & Data Source Expansion

Goal: Kraken adapter + derivatives/DeFi providers with guards.
Constraints: Exchange-native only for microstructure (L1/L2 depth/spread) — no aggregators for depth/spread; use aggregators only for warm/cold context per Playbook.
Write-Scope:

src/infrastructure/providers/kraken/

WS L2 order book (snap + diffs), trades, ticker; retries/backoff; Prometheus metrics (RPS, error_rate, latency).

src/infrastructure/providers/derivatives/

Funding rate, Open Interest, Basis (Binance/OKX/Bybit/BitMEX REST; Deribit options context). Playbook catalogs these sources.

src/infrastructure/providers/defi/

TVL + AMM volumes (The Graph / DefiLlama where free).

Compile-time guard (Go build tag or feature flag) to disable aggregator calls for microstructure; unit tests assert banned paths are not invoked when guard enabled.

Factor update: incorporate on-chain/derivs metrics into appropriate residual layers (don’t double-count MomentumCore per PRD).
Tests:

Mocks for WS/REST; PIT integrity; no aggregator depth/spread; rate-limit & circuit-breaker behavior per Playbook.
Acceptance: Kraken L1/L2/trades/ticker live in dev; derivatives/DeFi endpoints return normalized structs; guard tests pass.
Commit Checklist: Update docs: DATA_SOURCES, SCORING notes; CHANGELOG.

PHASE OBS-1 — Monitoring & Observability

Goal: metrics, dashboards, alerts, health CLI.
Write-Scope:

Prometheus metrics export: rate-limit consumption, cache hit ratio, latency percentiles, SSE throughput (Playbook metrics & provider health).

Grafana dashboards: data tiers, scheduler, Premove detector, regime tuner; JSON in observability/grafana/.

Alerting: Slack/email hooks for provider health changes, job failures, KPI breaches; heartbeats from each microservice; alerts for missed beats.

Structured logging fields: request_id, provider, duration, symbol, error_code.

cryptorun health CLI command printing current health + last error (CLAUDE.md mentions monitor endpoints & health).
Acceptance: Dashboards load; alerts fire on induced failures; cryptorun health returns green w/ fields.
Commit Checklist: MONITORING.md usage + screenshots; CHANGELOG.

PHASE DOC-1 — Documentation & Governance

Goal: close doc gaps & governance.
Write-Scope:

Expand docs/DEPLOYMENT.md (end-to-end steps, env vars, sample configs).

Update docs/MONITORING.md (metrics, dashboards, alert thresholds).

Create/expand docs/SECURITY.md (secret mgmt, dependency rules, code review).

Changelog automation: pre-commit or pre-push hook to write/update CHANGELOG by filenames or commit tags; enforce commit style <scope>: <subject> with commit-lint; CI docs-first check (fail on undocumented features).
Acceptance: CI blocks non-conforming commits; docs updated automatically alongside patches (HOUSE RULES).
Commit Checklist: CHANGELOG hook added; README pointers updated.

PHASE PERF-1 — Performance & Regression

Goal: prove we meet PRD/v3.2.1 latency/SLOs and detect regressions.
Write-Scope:

k6 or Locust scripts to simulate high-throughput scans/backtests; capture CPU/mem/I/O/net.

Profiling on data facade & scheduler (pprof); identify bottlenecks.

Regression tests comparing hit-rate, correlation, latency between versions; integrate Premove backtest curves and calibration diffs; HTML/Markdown diff reports per release.
Acceptance: Report shows P99 latency <300ms (target) for hot set; cache hit >85% for warm (per CLAUDE.md / v3.2.1).
Commit Checklist: PERF.md with thresholds and how to run.

PHASE RSR-1 — Additional Data & Factor Research

Goal: prototype new factors + macro/sentiment drivers.
Write-Scope:

Propose & backtest event-driven factors (protocol upgrades, listings) and social velocity (rate-of-change) with decile-lift and significance; document hypothesis, formula, params, expected behavior.

Macro ingestion (Fed, CPI, GDP) + sentiment indices; integrate as gating/weighting adjusters; normalize frequency/scales to factor stack (v3.2.1 regime & catalyst heat provide patterns).
Acceptance: Research notes with lift tables; wiring plan that preserves MomentumCore protection & social cap rules. (PRD orthogonalization + social cap).
Commit Checklist: docs/RESEARCH_*.md + CHANGELOG.

FINAL TURN-IN

Evidence Ledger in PR (paths/quotes or spec refs) per HOUSE RULES.

Update CHANGELOG.md and touch docs/* for every phase.

Bonus: PowerShell helper scripts (Windows-friendly)

You asked to prioritize “output/solution in prompt” first, PowerShell second. Here are ready-to-use helpers.

1) Build, test, lint, Docker, Trivy (local)
# tools\build_and_scan.ps1
param(
  [string]$ImageName = "cryptorun:dev",
  [string]$ComposeFile = "deploy/compose/docker-compose.yml"
)

Write-Host "==> Go test + lint"
go test ./... -count=1
if ($LASTEXITCODE -ne 0) { throw "Tests failed" }
golangci-lint run ./...
if ($LASTEXITCODE -ne 0) { throw "Lint failed" }

Write-Host "==> Docker build (multi-stage)"
docker build -t $ImageName .

Write-Host "==> Trivy image scan"
trivy image --severity CRITICAL,HIGH --exit-code 1 $ImageName
if ($LASTEXITCODE -ne 0) { throw "Trivy found high/critical vulnerabilities" }

Write-Host "==> Compose up (dev stack)"
docker compose -f $ComposeFile --env-file deploy/compose/.env.example up -d

2) K8s apply overlays + create secrets from .env
# tools\k8s_apply.ps1
param([ValidateSet("staging","prod")][string]$Env = "staging")

$Overlay = "deploy/k8s/overlays/$Env"
kubectl apply -k $Overlay

# Optional: load .env into K8s Secret (non-prod)
$envPath = "deploy/compose/.env.example"
if (Test-Path $envPath) {
  $data = Get-Content $envPath | Where-Object { $_ -and $_ -notmatch '^#' }
  $kv = @{}
  foreach ($line in $data) {
    $parts = $line.Split('=',2)
    if ($parts.Length -eq 2) { $kv[$parts[0]] = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($parts[1])) }
  }
  $secret = @{
    apiVersion = "v1"; kind="Secret"; metadata=@{name="cryptorun-secrets"; namespace="cryptorun"}
    type="Opaque"; data=$kv
  } | ConvertTo-Json -Depth 5
  $secret | kubectl apply -f -
}

3) Commit-lint & CHANGELOG stubber (git hook)
# tools\install_hooks.ps1
# Install pre-commit to block bad secrets and generate CHANGELOG stubs
Copy-Item tools\hooks\pre-commit .git\hooks\pre-commit -Force
Copy-Item tools\hooks\commit-msg .git\hooks\commit-msg -Force
