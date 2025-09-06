OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
PROMPT_ID=SUPERPACK.premove.v33.part1.PART_3_OF_3

CONTINUATION RULES
- This is part 3/3.
- If prior parts referenced WRITE-SCOPE/PATCH-ONLY, keep them identical here.
- Begin where the last part ended; do not repeat previous content.
e via RunnerDeps struct for testability.
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
