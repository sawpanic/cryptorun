# Menus — Pre‑Movement Detector (UI spec)

## ✅ IMPLEMENTED v3.3 — Console Interface 

### PreMove Detection Board (Monitor Menu > Option 4)
- **SSE-Throttled Updates:** ≤1 Hz refresh rate with real-time state management
- **Interactive Console:** Manual refresh ('r') and quit ('q') commands
- **Live Candidate Display:** Symbol, score, gates passed (2-of-3), beta, sector
- **Portfolio Status:** Execution metrics, success rates, slippage tracking, recovery mode
- **Alert History:** Recent alerts with severity, status, and timestamp
- **System Health:** Component status, pipeline health, SSE client count
- **Demo Mode:** Realistic simulated data with BTCUSD/ETHUSD candidates

## Coil Board (Top‑100) — Planned Future Enhancement
- 25 tiles/page; sticky filters (State, Liquidity Tier, Venue Health, Regime), sorts (Score, Freshness, Liquidity, ETA)
- Tile shows: Symbol • PreMoveScore • State • Freshness dot • Top‑3 reasons • Gates meter • Decay clock • Catalyst badge • Venue health
- Color coding: WATCH (yellow), PREPARE (orange), PRIME (red border), EXECUTE (pulsing red)
- Client refresh ≤1 Hz; state transitions via SSE

## Asset Deep Dive (tabs)
- Derivatives • Flows • Microstructure • Compression • Smart‑Money • Catalyst Timeline • Factor Breakdown • Risk & Correlation • Data Health • Trigger Tape
- Vendor capability flags: Coinbase perps/funding/basis = N/A; microstructure is exchange‑native only

## System Health
- Feed age, latency, error‑rate, breaker state; coverage & quality heatmaps
