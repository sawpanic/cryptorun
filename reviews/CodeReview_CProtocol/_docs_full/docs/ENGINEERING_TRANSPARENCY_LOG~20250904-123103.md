# CProtocol Scanner — Engineering Transparency Log (2025-09-04)

Author: Assistant Engineer (Noam’s brief)

Mission: Momentum capture with guardrails. Catch rockets, not statues.

## Summary

- Reoriented the system to treat Momentum as the base signal, not noise.
- Added momentum‑protected orthogonal scoring so momentum isn’t residualized away.
- Introduced two new scanners: Balanced (40/30/30) and Acceleration.
- Implemented hard gates (multiplicative mindset) to block flat/illiquid names.
- Prepared hooks for regime adaptation and ATR‑scaled sizing.

## Changes Implemented (Code)

- Protected Momentum Core (base vector)
  - File: `internal/models/clean_orthogonal_system.go`
  - Added `computeMomentumCore()` and exported `ComputeMomentumCore()` (0–100).
  - Technical channel now: `combinedTech = 0.6*momentumCore + 0.4*technicalResidual`.
  - Volume+Liquidity residualized vs momentum to avoid confirmation double‑counting.

- Mean Reversion + Acceleration Signals (0–100)
  - File: `internal/models/clean_orthogonal_system.go`
  - Added `ComputeMeanReversionScore()` (oversold, depth of dip).
  - Added `ComputeAccelerationScore()` (trend strength, tech−quality gap, 24h vs 7d slope, volume boost).

- Hard Gates (pre‑filters; multiplicative mindset)
  - File: `internal/models/clean_orthogonal_system.go`
  - Added `PassesHardGates(opp)` approximating:
    - Momentum threshold: `abs(24h_change) >= 3%` (proxy for 4h/24h).
    - Liquidity: `VolumeUSD >= $500k` AND `LiquidityScore >= 60`.
    - Market cap: `>= $10M` when known.
    - Anti‑manipulation: require on‑chain/whale activity signal.
    - Trend quality: TrendStrength ≥55 OR PatternQuality ≥60.

- Momentum‑First Orthogonal Weights for Ultra‑Alpha (Momentum Mode)
  - File: `internal/models/clean_orthogonal_system.go`
  - Added `GetMomentumOrthogonalWeights()` = Tech 35% + Social 20% + Vol/Liq 20% + Quality 15% + On‑Chain 10% (sum 100%).
  - File: `main.go`
  - Ultra‑Alpha Orthogonal now uses `models.GetMomentumOrthogonalWeights()`.
  - Updated UI headings to reflect momentum‑first split.

- Balanced Scanner (Varied Market Conditions)
  - File: `main.go`
  - Added `runBalancedVariedConditions()` and `applyBalancedVariedRescore()`.
  - Composite: `0.40*Momentum + 0.30*MeanRev + 0.30*Quality` with hard gates.

- Acceleration Scanner (Momentum of Momentum)
  - File: `main.go`
  - Added `runAccelerationScanner()` and `applyAccelerationRescore()`.
  - Composite: `0.60*Acceleration + 0.20*Momentum + 0.20*Volume` + volume/micro‑timeframe guards.

- Cleanup
  - Removed backup files with `~` suffix under `internal/models` to avoid duplicate symbols.

## Mapping to Brief — Do’s / Don’ts

Do’s
- Preserve momentum as primary signal: Implemented “momentum core” and protected it in scoring; no residualization of momentum itself.
- Use multiplicative gates: Orthogonal path continues to use multiplicative gates; added pre‑filter gates to prevent flats.
- Implement regime detection: Current regime stub exists; planned dynamic weights/gates (see Roadmap).
- Add acceleration detection: Implemented `ComputeAccelerationScore()` and Acceleration scanner.
- Scale positions by volatility: Planned (ATR proxy) in Roadmap.

Don’ts
- Don’t over‑weight quality/safety: Reduced quality to 15% in Momentum Mode; removed bias toward majors in Ultra‑Alpha.
- Don’t use fixed thresholds only: Added gate proxies; percentile fallbacks planned in Roadmap.
- Don’t treat all timeframes equally: Momentum core biases recency; Acceleration uses recent slope proxy.
- Don’t ignore microstructure: Using LiquidityScore as a proxy today; VWAP/spread/depth integration planned.
- Don’t residualize momentum: Fixed — momentum is now the base vector.

## Output Format — Trader‑Focused Alerts (Planned Wiring)

Target display sections (to replace “statue tables” where applicable):

BREAKOUT ALERTS (Last 15 mins)
1. SYMBOL  +XX.X%  Volume: Y.Yx  Signal: STRONG BUY    [ENTER NOW]
2. SYMBOL  +XX.X%  Volume: Y.Yx  Signal: ACCUMULATE    [BUILDING]

REVERSAL WATCH (Oversold Bounces)
1. SYMBOL  -X.X%   RSI: NN       Signal: BOUNCE SETUP  [WAIT FOR TURN]

⚠️ EXITING (Time/ATR/Degrade triggers)
1. SYMBOL  +X.X%   Time: 14h     Signal: TAKE PROFIT   [EXIT 75%]

Notes: Until 1h/4h baselines are live, “Volume: Y.Yx” will be an honest proxy derived from recent volume score; we’ll swap in true 1h vs 24h ratios once adapters are in.

## Regime Adaptation (Design)

Logic outline (to be wired):
- TRENDING: weights `{momentum: 0.45, technical: 0.30, volume: 0.15, ...}`; gates `{min_move: 3%, min_volume_surge: 1.5x}`.
- CHOPPY: weights `{momentum: 0.25, technical: 0.25, quality: 0.25, ...}`; gates `{min_move: 2%, RSI_extremes: True}`.
- VOLATILE: weights `{quality: 0.35, momentum: 0.30, ...}`; gates `{min_liquidity: 2x_normal, max_positions: 6}`.

We’ll compute market breadth from live adapters and pivot weights/gates at scan time.

## Position Sizing (ATR‑Scaled)

Design (to implement):
- Base size `$10k`, max `$50k`, scaling factor `size = base / ATR_normalized`.
- Stops: initial `entry − 1.5*ATR`, trailing `high − 1.2*ATR` after `> 2*ATR` profit, time stop at 18h.
- Portfolio limits: `max_positions=12`, `max_per_sector=2`, `max_corr=0.7`.

We’ll add ATR proxy from volatility or integrate true ATR from OHLC once adapters are live.

## Data Adapters — Fallback Ladder

Added in brief as a reference (not yet wired). Proposed order:
1) Aggregators: CoinPaprika → CoinCap → CMC → CryptoCompare → LiveCoinWatch
2) CEX REST/WS: Binance → Coinbase → OKX → Bitstamp → Gemini → Bybit
3) DeFi: DEXScreener → Pyth (Hermes) → Jupiter (SOL)
4) Vendors: CoinAPI / Messari / Kaiko / Amberdata

Behavior: fail‑fast cascade, 429‑aware backoff, trimmed‑median reconciliation across sources, symbol normalization, micro‑cache for burst control, WS preferred where possible.

## Roadmap — Next Iterations

- Wire new scanners into the menu and add alert‑style outputs.
- Implement regime‑aware weight/gate switching based on breadth.
- Integrate ATR and output position sizes + stops per pick.
- Add percentile‑based fallbacks for thresholds (market‑adaptive).
- Add microstructure metrics (VWAP, bid‑ask, L2 depth ≥ $50k @ 2%).
- Implement the adapter layer with the API ladder above.

## Acceptance Criteria (KPIs)

- Momentum capture rate: ≥70% of >5% moves in universe.
- Avg selected movement: 6–10%.
- Win rate: 55–60%.
- Sharpe: >1.4.
- False positives: <15%, momentum persistence ≥60% over 4h, max portfolio DD <8%.

## Known Gaps (Honest Accounting)

- True 1h/4h momentum and volume ratios not yet live; proxies in place.
- ATR and microstructure (VWAP/spread/depth) not yet implemented.
- Regime adapter currently stubbed; next iteration will connect.

---

Questions/approvals: Want me to wire the menu and add the alert‑style output next, or prioritize ATR sizing + regime switching first?

