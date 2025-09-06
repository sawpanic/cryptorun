## [2025-09-06 16:18:00Z] Pre‑Movement Detector v3.3 — Added (Menu‑only)

### Added
- New **Pre‑Movement Detector** module (menu‑only, Top‑100) with 100‑pt composite and **2‑of‑3 gates**
- Portfolio pruning (corr≤0.65, 2‑per‑sector, beta budget=15.0), execution quality tracking, alert governance (3/hour,10/day)
- **Backtest & calibration harness** with PIT replay from artifacts, hit-rate computation by state/regime
- **Isotonic calibration curve** (score → P(move>5% in 48h)) with monthly refresh and freeze governance
- **CVD residual R² tracking** for daily signal quality monitoring
- CLI-free backtest invocation via internal test harness with deterministic unit tests
- **SSE-Throttled Live Dashboard** (≤1 Hz) accessible via Monitor Menu > PreMove Detection Board
- **Interactive Console Interface** with real-time candidate display and manual refresh capabilities
- **Execution Quality Monitoring** with slippage BPS tracking, fill time metrics, and recovery mode
- **Alert Rate Limiting** with manual override support and volatility allowance

### Fixed
- Freshness **hard_fail_s** corrected to **90s** (worst‑feed precedence)

### Config
- Added **portfolio** config block with correlation limits, sector caps, beta budget, and position sizing
- Added **alerts** config block with rate limiting, high volatility allowances, and manual override
- Added **execution_quality** config block with slippage thresholds and recovery criteria  
- Added **learning** config block with pattern exhaustion monitoring

### Docs
- `docs/PREMOVE.md` (feature spec), `docs/MENUS_PREMOVE_SECTION.md`, `docs/DATA_SOURCES_PREMOVE_SECTION.md`
- `config/premove.yaml` created and version‑locked
