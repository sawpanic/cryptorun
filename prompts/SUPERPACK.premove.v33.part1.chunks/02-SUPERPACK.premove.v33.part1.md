OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
PROMPT_ID=SUPERPACK.premove.v33.part1.PART_2_OF_3

CONTINUATION RULES
- This is part 2/3.
- If prior parts referenced WRITE-SCOPE/PATCH-ONLY, keep them identical here.
- Begin where the last part ended; do not repeat previous content.
E-FLIGHT
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
  - Expos