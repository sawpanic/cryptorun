---
name: PREMOVE.BACKTEST.V33
description: Backtest harness + isotonic calibration
---
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT

SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=PREMOVE.BACKTEST.V33

GOAL
Add PIT replay from artifacts to compute hit rates by state (WATCH/PREPARE/PRIME/EXECUTE), sliced by regime,
and fit an isotonic calibration curve (score → P(move>5% in 48h)).

SCOPE
- Implement `src/application/premove/backtest.go`:
  - Load `artifacts/premove/*.jsonl` (point-in-time only).
  - Compute per-state, per-regime hit-rates; log daily CVD residual fit **R²**.
  - Implement an isotonic calibration stub (monotone mapping), monthly refresh with freeze governance.
- Update docs: add “Calibration & Governance” to `docs/PREMOVE.md` and append to `docs/CHANGELOG_PREMOVE_V3.3.md`.

GUARDS
- CLI-free invocation via internal test harness.
- No external network calls in tests.

ACCEPTANCE
- Deterministic unit tests pass for replay and isotonic fit.
- Calibration artifacts produced: `hit_rates_by_state_and_regime.json`, `isotonic_calibration_curve.json`, `cvd_resid_r2_daily.csv`.

### GIT COMMIT CHECKLIST (run exactly)
1) `git add config/premove.yaml docs/PREMOVE.md docs/MENUS_PREMOVE_SECTION.md docs/DATA_SOURCES_PREMOVE_SECTION.md`  
   `git add docs/CHANGELOG_PREMOVE_V3.3.md interfaces/ui/menu/*.go src/domain/premove/*.go src/application/premove/*.go cmd/cryptorun/menu_premove.go`
2) Verify build/tests locally: `go build ./... && go test ./...`
3) Update CHANGELOG if generator edited files: `type docs\CHANGELOG_PREMOVE_V3.3.md`
4) Commit with conventional message:
   `git commit -m "feat(premove): add Pre-Movement Detector v3.3 (menu-only); freshness=90s; portfolio/alerts/exec/backtest; docs"`
5) Push: `git push -u origin HEAD`

