OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT
SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=SUPERPACK.PREMOVE.V33.PART1

WRITE-SCOPE — ALLOW ONLY:
  - src/domain/premove/ports/**
  - src/infrastructure/percentiles/**
  - src/domain/premove/cvd/**
  - src/domain/premove/proxy/**
  - src/application/premove/runner.go
  - internal/testdata/premove/**
  - tests/unit/premove/**
  - docs/PREMOVE.md
  - docs/*CHANGELOG*
PATCH-ONLY — Emit unified diffs or full file bodies only; no prose.

SUPERPACK: Premove v3.3 — Percentiles + CVD Residuals + Supply-Squeeze Proxy + Runner Wiring + Tests
INDEX
  [S1] PRE-FLIGHT
  [S2] Percentile Engine
  [S3] CVD Residuals
  [S4] Supply-Squeeze Proxy
  [S5] Runner Wiring
  [S6] Tests & Fixtures
  [S7] Docs + CHANGELOG
  [S8] POST-FLIGHT

CONVENTIONS
- STOP-ON-FAIL if any step fails.
- Deterministic fakes only; no network.
- GREEN-ONLY MERGE: must end with go fmt/vet/test green.

[S1] PRE-FLIGHT
- List files to touch (must be subset of WRITE-SCOPE).
- Verify `internal/testdata/premove/` exists, else create.
- Print "Pre-flight OK".

[S2] Percentile Engine
- New interface `src/domain/premove/ports/percentiles.go` with PercentileEngine (14d/30d windows, winsorize ±3σ).
- Implement `src/infrastructure/percentiles/engine.go` with `NewPercentileEngine()`.

[S3] CVD Residuals
- Interface `src/domain/premove/ports/cvd.go`: Residualize(cvdNorm, volNorm).
- Impl `src/domain/premove/cvd/residuals.go`: robust regression with winsorization, fallback if <200 obs or R²<0.30.

[S4] Supply-Squeeze Proxy
- Interface `src/domain/premove/ports/supply_proxy.go` with ProxyInputs and Evaluate().
- Impl `src/domain/premove/proxy/supply.go`: gates A–C, conditional volume confirm in risk_off/btc_driven.

[S5] Runner Wiring
- Edit `src/application/premove/runner.go`:
  - Inject deps: perc := NewPercentileEngine(), cvd := NewCVDResiduals(), ssp := NewSupplyProxy().
  - Expose via RunnerDeps struct for testability.
  - Use perc for p80 lookups (VADR gate).
  - Use cvd residuals when available; degrade confidence if fallback.
  - Use ssp to decide gate count and if volume confirm required.

[S6] Tests & Fixtures
- Add CSV fixtures under `internal/testdata/premove/`:
  - percentiles_small.csv
  - cvd_norm.csv
- Unit tests in `tests/unit/premove/`:
  - percentiles_test.go
  - cvd_resid_test.go
  - supply_proxy_test.go
  - runner_wiring_test.go

[S7] Docs + CHANGELOG
- docs/PREMOVE.md: sections for Percentile Engine, CVD Residuals, Supply-Squeeze Proxy.
- Append CHANGELOG entry noting new modules and runner wiring.

[S8] POST-FLIGHT
- Run: go fmt ./... ; go vet ./... ; go test ./... -count=1
- Print compact PASS summary (files changed, tests run, PASS/FAIL).
