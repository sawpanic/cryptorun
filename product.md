# CryptoRun Scanner System — Product Requirements Document (PRD)

Version: 3.0 (Approved) — Final Product Vision & Requirements
Date: 2025-09-04
Owner: CryptoRun Product & Engineering

## Executive Summary
CryptoRun is a real-time crypto scanner optimized for 6–48 hour trades across 500+ assets. It captures explosive, short-term momentum moves with institutional guardrails. Target output: 50–150 tradeable signals/month, 55–65% hit rate, 8–12% average gain per winner.

## Problem & Opportunity
- Existing scanning biases (quality/majors) miss fast movers and surface flat assets.
- Traders need a momentum capture engine with transparent risk controls, not a conservative selector.

## Goals & Non‑Goals
- Goals:
  - Capture momentum and early acceleration with clear, actionable outputs.
  - Maintain strict guardrails (liquidity, microstructure, time stops, ATR sizing).
  - Provide regime‑adaptive, orthogonal factor scoring with momentum as the base vector.
- Non‑Goals:
  - Buy‑and‑hold, low‑volatility portfolio construction.
  - Value investing or long‑term fundamentals.

## Core Scanner Modes
1) Momentum Scanner — Finds continuation opportunities on already moving assets.
2) Reversal Scanner — Catches oversold bounces near support.
3) Exit Scanner — Actionable exit guidance for open positions.

## Scoring System (0–100)
- 85–100: STRONG BUY
- 70–84: BUY
- 60–69: ACCUMULATE
- 50–59: WATCH
- <50: EXIT ZONE

## Multi‑Timeframe Architecture (6–48h bias)
- 1h (20%): Entry timing / early exit cues
- 4h (35%): Primary momentum signal
- 12h (30%): Trend confirmation
- 24h (15%): Context / exhaustion detection

## Factor Model — Momentum‑Protected Orthogonalization
- Momentum is the protected base vector (not residualized).
- Technical residual = Technical − (overlap with Quality, Momentum)
- Volume/Liquidity residualized vs Momentum to avoid double counting confirmation.
- Quality residualized vs Technical/Volume/Momentum; Social residualized last.

## Momentum Core (Concept)
- Multi‑timeframe returns with recency weighting (1h/4h/12h/24h = 20/35/30/15)
- Acceleration term (Δ 4h momentum)
- Volatility normalization (ATR‑adjusted)
- Optional volume confirmation (1h vs 7‑day per‑hour)

## Regime Detection & Adaptation
- TRENDING_BULL: emphasize momentum; stricter movement and volume surge gates
- CHOPPY: emphasize mean reversion; allow RSI extremes
- TRENDING_BEAR: tighter gates; increase quality weighting

## Guardrails (Hard Gates)
- Movement ≥ 2.5% (prefer 4h; fallback 24h)
- Volume surge ≥ 1.75× (1h vs 7d per‑hour) when available
- Liquidity ≥ $500k 24h volume
- Trend quality: ADX ≥ 25 or Hurst ≥ 0.55 (or technical proxies)
- Microstructure: spread ≤ 50 bps, depth ≥ $50k @ 2% where available
- Anti‑manipulation: whale/on‑chain activity presence

## Position‑Level Controls
- Sizing: Size = Base / ATR (Base $10k; cap $50k)
- Stops: initial stop = entry − 1.5× ATR; trailing after >2× ATR in profit = high − 1.2× ATR
- Time stop: max 48h (6–48h holding window)
- Portfolio: max 15 positions; 10% single‑asset cap; correlation cap 0.7

## Exit Hierarchy (Ordered)
1) Risk: HARD_STOP (loss > 1.5× ATR) → FULL_EXIT
2) Time: ≥ 48h → FULL_EXIT
3) Momentum decay: acceleration reversal → SCALE 50%; momentum dead (1h<0 & 4h<0) → EXIT ALL
4) Profit taking: +8% → TAKE PROFIT 25%; +15% → SCALE OUT 50%

## Output — Trader‑Facing Sections
- BREAKOUT ALERTS (Last 15 mins): rank, % change, volume multiple, action tag (ENTER NOW / BUILDING)
- REVERSAL WATCH (Oversold Bounces): rank, % change, RSI, action tag (WAIT FOR TURN / SCALING IN)
- ⚠️ EXIT SIGNALS: symbol, entry, score prev→now, factors, held hours, P&L%, cause → action
- Momentum Breakouts & Reversal Candidates tables: two‑line rows with factor strengths (0–100), star ratings, and signals

## Scoring Legend (Displayed)
- 85–100 STRONG BUY | 70–84 BUY | 60–69 ACCUMULATE | 50–59 WATCH | <50 EXIT ZONE
- Factor Stars: ★★★★★ (80–100) | ★★★★ (60–79) | ★★★ (40–59) | ★★ (20–39) | ★ (0–19)
- Example Active Weights (balanced): Momentum(40%) Technical(25%) Volume(20%) Quality(10%) Social(5%)
- Gates Applied: Movement >2.5% | Volume >1.75× | Liquidity >$500k | ADX >25

## Success Metrics & Accountability
- Weekly: Hit rate (55–65%), Avg win/loss, exit reasons breakdown, regime accuracy, false positives (<20%)
- Risk: Max 15 concurrent positions; 10% single asset cap; daily drawdown breaker −8%; forced exit at 48h

## Data Strategy & Fallbacks (Ops)
- Aggregators: CoinPaprika → CoinCap → CMC → CryptoCompare → LiveCoinWatch
- CEX WS/REST: Binance, Coinbase, OKX (+ Bitstamp, Gemini, Bybit)
- DeFi: DEXScreener, Pyth (Hermes), Jupiter (SOL)
- Vendors: CoinAPI, Messari, Kaiko, Amberdata
- Behavior: fail‑fast cascade, 429‑aware backoff, trimmed‑median reconciliation, symbol/quote normalization

## Timeline (Implementation Priority)
- Week 1: Core scanner (multi‑timeframe momentum, orthogonalization, gates & scoring)
- Week 2: Risk & exits (exit hierarchy, ATR sizing, time management)
- Week 3: Regime & refinement (regime detection, dynamic weights, reversal mode)
- Week 4: Production (backtesting, performance monitoring, alert integration)

## Acceptance Criteria
- Momentum capture rate ≥ 70% of >5% moves in universe
- Avg selected movement 6–10%
- 55–65% win rate; Sharpe > 1.5
- False positive rate < 20%; max portfolio drawdown < 15%

## Out of Scope
- Long‑term value scoring, fundamental valuation, tax optimization

## Appendix
- Engineering Transparency Log: docs/ENGINEERING_TRANSPARENCY_LOG.md
- Technical Blueprint v3.0: docs/V3_TECH_BUSINESS_BLUEPRINT.md

---

# Final Product Vision Addendum (Implemented Roadmap Alignment)

Mission: Real‑time momentum scanner combining price momentum, catalyst timing, brand power, and microstructure into actionable, transparent signals.

New core requirements captured and acted upon:
- Multi‑Timeframe Momentum Engine (1h/4h/12h/24h; acceleration on 4h)
- Catalyst‑Heat scoring with time decay buckets (imminent → distant)
- Brand Power & Social Momentum (narrative strength 0–10)
- Market Microstructure (spread, depth, volume surge VADR)
- Regime‑Adaptive Weighting including Catalyst
- Entry/Exit Gates with regime dependency
- Momentum Signals display including Momentum, Catalyst, Volume (VADR), Change%, and Action
