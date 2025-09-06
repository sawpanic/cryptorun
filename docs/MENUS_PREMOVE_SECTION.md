# Menus — Pre‑Movement Detector (UI spec)

## Coil Board (Top‑100)
- 25 tiles/page; sticky filters (State, Liquidity Tier, Venue Health, Regime), sorts (Score, Freshness, Liquidity, ETA)
- Tile shows: Symbol • PreMoveScore • State • Freshness dot • Top‑3 reasons • Gates meter • Decay clock • Catalyst badge • Venue health
- Color coding: WATCH (yellow), PREPARE (orange), PRIME (red border), EXECUTE (pulsing red)
- Client refresh ≤1 Hz; state transitions via SSE

## Asset Deep Dive (tabs)
- Derivatives • Flows • Microstructure • Compression • Smart‑Money • Catalyst Timeline • Factor Breakdown • Risk & Correlation • Data Health • Trigger Tape
- Vendor capability flags: Coinbase perps/funding/basis = N/A; microstructure is exchange‑native only

## System Health
- Feed age, latency, error‑rate, breaker state; coverage & quality heatmaps
