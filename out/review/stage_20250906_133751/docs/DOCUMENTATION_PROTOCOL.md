# CryptoRun Documentation Protocol

Effective: 2025-09-04

## Principle
Every change, addition, or improvement must be documented and logged. Documentation is part of the product and engineering definition of done.

## Requirements
- Append a timestamped entry to `changelog.log` for:
  - App startup and version
  - Menu selections and executed scanners
  - Major output sections rendered (e.g., Momentum Signals)
  - Configuration changes (weights, gates, regimes)
- Update relevant docs for feature work:
  - `product.md` for product requirements
  - `docs/ENGINEERING_TRANSPARENCY_LOG.md` for engineering‑level changes
  - `docs/V3_TECH_BUSINESS_BLUEPRINT.md` for architecture alignment
- Include a brief rationale and impacted files where practical.

## Format
```
[YYYY-MM-DD HH:MM:SS UTC] <summary>
```
Examples:
- `[2025-09-04 12:30:12 UTC] Menu selection: Ultra-Alpha Orthogonal`
- `[2025-09-04 12:31:44 UTC] Render Momentum Signals: 10 rows, regime=TRENDING_BULL`

## Enforcement
- The app emits entries to `changelog.log` automatically (see `main.go: logChange`).
- PRs must include doc updates and a `changelog.log` excerpt.

## Notes
- Source attribution and freshness badges will be included in outputs; reconciliation stats to follow.
- Keep `changelog.log` in repo root; rotate as needed.
