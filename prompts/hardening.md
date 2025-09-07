HOUSE RULES
––––––––––––––––––––––––––––––––––––––––––––
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT
ALREADY IMPLEMENTED PRE-FLIGHT — STOP if Docker/K8s/monitor already implemented; output ALREADY IMPLEMENTED + path/lines
WRITE-SCOPE — Patch-only, list exact files/lines; no rewrites
CITATIONS — Must cite spec refs:contentReference[oaicite:1]{index=1}:contentReference[oaicite:2]{index=2}

TASK: Finalize **Deployment Hardening & Monitoring**.

PHASES:

1. **Docker/K8s Hardening**
   - Add resource requests/limits (CPU, memory) to K8s manifests.
   - Implement readiness/liveness probes for `cryptorun` HTTP `/health` endpoint:contentReference[oaicite:3]{index=3}.
   - Add staging & prod overlays (`deploy/k8s/overlays/staging`, `prod`).

2. **Monitoring & Observability**
   - Expose Prometheus metrics: rate limit consumption, cache hit ratios, regime flips, exit distribution.
   - Add Grafana alerts (latency >300ms P99, API error >5%, cache hit <85%).
   - Deploy dashboards into `deploy/grafana/*`.

3. **CI/CD Integration**
   - Add load test stage (`go test ./... -tags=load`) verifying <300ms P99 latency.
   - Add Trivy security scans.

4. **Docs**
   - Update `docs/DEPLOYMENT.md` with staging/prod rollout instructions.
   - Document dashboards in `docs/MONITORING.md`.

ACCEPTANCE
–––––––––––––––––––––––––
- `kubectl apply -k deploy/k8s/overlays/staging` runs cleanly.
- `/metrics` endpoint live and scraped by Prometheus.
- Grafana dashboards/alerts functional.
- CI pipeline runs security + load test stages.
Git Commit Checklist for Prompt 7

 Patch deploy/k8s/* with resources, probes, overlays

 Add deploy/grafana/*

 Update CI workflow for load test + Trivy

 Update docs: docs/DEPLOYMENT.md, docs/MONITORING.md

 Update CHANGELOG.md

 PROGRESS.yaml milestone bump (Deployment Hardening)

Prompt 8 — CONFORMANCE.SELF.AUDIT
markdown
Copy code
HOUSE RULES …
TASK: Implement **Conformance Self-Audit Suite**:contentReference[oaicite:4]{index=4}:contentReference[oaicite:5]{index=5}.

PHASES:

1. **Conformance Tests**
   - Factor order test: MomentumCore first, Gram–Schmidt ordering preserved.
   - Guards test: Fatigue, Freshness, Late-fill activate per thresholds.
   - Social cap ≤10 enforced after momentum/volume.
   - Aggregator ban test: no depth/spread from DEXScreener/Coingecko:contentReference[oaicite:6]{index=6}.
   - Regime switch toggles weights every 4h.

2. **Self-Audit Tool**
   - Implement CLI `cryptorun audit` in `cmd/cryptorun/audit.go`.
   - Audit run produces JSON receipt: factors, guards, regime, sources, timestamps.
   - Store receipts in `/audit/receipts`.

3. **CI Enforcement**
   - Fail build if conformance suite fails.
   - Add daily cron job to run `audit` and publish artifact.

4. **Docs**
   - Write `docs/CONFORMANCE.md` explaining tests, receipts, CI hooks.

ACCEPTANCE
–––––––––––––––––––––––––
- `cryptorun audit` generates receipts with attribution.
- CI fails on guard/social/aggregator/regime violations.
- Docs updated, CHANGELOG updated.
Git Commit Checklist for Prompt 8

 Add cmd/cryptorun/audit.go, internal/audit/*

 Add tests: tests/unit/conformance/*

 Update docs: docs/CONFORMANCE.md

 Update CHANGELOG.md

 PROGRESS.yaml milestone bump (Conformance)

Prompt 9 — DOCS.GOVERNANCE.SECURITY
markdown
Copy code
HOUSE RULES …
TASK: Finalize **Docs, Governance, and Security**:contentReference[oaicite:7]{index=7}.

PHASES:

1. **Docs Creation**
   - Write missing docs:  
     • `docs/PREMOVE.md` (scoring, states, gates)  
     • `docs/PREMOVE_CONFIG.md` (YAML configs, thresholds)  
     • `docs/UI_GUIDE.md` (CLI/menu, SSE refresh, coil board)  
     • `docs/DATA_SOURCES_PREMOVE_SECTION.md`.

2. **Governance Protocol**
   - Automate changelog stubs: pre-push hook adds skeleton entry.
   - Enforce WRITE-SCOPE guardrails: `.crun_write_lock` respected in CI.

3. **Security Guidelines**
   - Add `docs/SECURITY_GUIDE.md`: secret handling, encryption keys, access control.
   - Reference in `SECURITY.md`.

4. **Docs Consistency**
   - Update `product.md`, `mission.md`, `engineering_transparency_log.md` to align with new modules.

ACCEPTANCE
–––––––––––––––––––––––––
- Missing docs exist and are populated with spec content.
- Changelog stub auto-generation works.
- CI enforces write-scope locks.
- Security docs cover secrets/encryption explicitly.
Git Commit Checklist for Prompt 9

 Add docs: docs/PREMOVE.md, docs/PREMOVE_CONFIG.md, docs/UI_GUIDE.md, docs/DATA_SOURCES_PREMOVE_SECTION.md

 Add docs/SECURITY_GUIDE.md

 Update product.md, mission.md, ENGINEERING_TRANSPARENCY_LOG.md

 Add pre-push hook for CHANGELOG stubs

 Update CHANGELOG.md

 PROGRESS.yaml milestone bump (Docs & Governance)