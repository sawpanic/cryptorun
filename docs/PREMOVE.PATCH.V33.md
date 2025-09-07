---
name: PREMOVE.PATCH.V33
description: Patch config + add portfolio/alerts/execution/backtest layers and SSE refresh throttle
---
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT

SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=PREMOVE.PATCH.V33

GOAL
1) Fix freshness hard_fail_s to 90.
2) Add portfolio/alerts/execution/backtest modules & wire them.
3) Throttle UI refresh & add SSE transitions.

SCOPE (Atomic)
- Edit `config/premove.yaml`: set freshness.hard_fail_s=90; add portfolio/alerts/execution_quality/learning blocks (values from docs/PREMOVE.md).
- Create files:
  - `src/domain/premove/portfolio.go`  // correlation matrix (1h/4h), sector & beta caps, prune stage
  - `src/application/premove/execution.go`  // intended vs actual, slippage_bps, time_to_fill_ms, recovery
  - `src/application/premove/alerts.go`     // rate limits 3/hour,10/day (+vol allowance), manual override alert-only
  - `src/application/premove/backtest.go`   // PIT replay + hit-rates + isotonic calibration stub
- Wire portfolio pruning in `application/premove/runner.go` (apply **post-gates, pre-alerts**).
- Wire execution metrics to `/metrics` and write minimal artifacts on fills.
- UI: `interfaces/ui/menu/page_premove_board.go` → throttle ≤1 Hz, send state changes via SSE.
- Update docs: `docs/PREMOVE.md`, `docs/MENUS_PREMOVE_SECTION.md`, `docs/DATA_SOURCES_PREMOVE_SECTION.md`, `docs/CHANGELOG_PREMOVE_V3.3.md`.

GUARDS
- Follow v3.3 precedence: portfolio after scoring/gates; manual override → alert_only; microstructure is exchange-native only.
- No network in unit tests; deterministic fakes only.

ACCEPTANCE
- `go build ./... && go test ./...` green.
- New metrics visible: `premove_slippage_bps`, `premove_alerts_rate_limited_total`, `premove_portfolio_pruned_total`.
- Menu renders with throttle and SSE hooks; docs updated.

### GIT COMMIT CHECKLIST (run exactly)
1) `git add config/premove.yaml docs/PREMOVE.md docs/MENUS_PREMOVE_SECTION.md docs/DATA_SOURCES_PREMOVE_SECTION.md`  
   `git add docs/CHANGELOG_PREMOVE_V3.3.md interfaces/ui/menu/*.go src/domain/premove/*.go src/application/premove/*.go cmd/cryptorun/menu_premove.go`
2) Verify build/tests locally: `go build ./... && go test ./...`
3) Update CHANGELOG if generator edited files: `type docs\CHANGELOG_PREMOVE_V3.3.md`
4) Commit with conventional message:
   `git commit -m "feat(premove): add Pre-Movement Detector v3.3 (menu-only); freshness=90s; portfolio/alerts/exec/backtest; docs"`
5) Push: `git push -u origin HEAD`

