# CryptoRun Scanner System: Technical Specification & Business Blueprint (v3.0)

Version: 3.0 — 6–48 Hour Multi‑Timeframe Momentum & Reversal Detection
Date: 2025‑09‑04

## Executive Summary

CryptoRun is a real‑time crypto scanner for 6–48 hour trades across 500+ assets. It evaluates multi‑timeframe momentum (1h/4h/12h/24h) to capture explosive moves with strict risk controls.

Revenue Impact: 50–150 tradeable signals/month, 55–65% hit rate, 8–12% average gain per winner.

## Core Scanner Modes

- Momentum: Continuation probability on moving assets
- Reversal: Oversold bounces near support
- Exit: Actionable exit guidance for open positions

## Scoring System (0–100)

- 85–100: Exceptional (full entry)
- 70–84: Strong (standard)
- 60–69: Building (starter)
- 40–59: Neutral (monitor)
- <40: Weakening (consider exits)

## Multi‑Timeframe Architecture

Timeframe | Weight | Purpose
1h | 20% | Entry precision & early exits
4h | 35% | Primary signal
12h | 30% | Trend confirmation
24h | 15% | Context & exhaustion

## Legend (Displayed in UI)

- SCORING LEGEND: 85–100 STRONG BUY | 70–84 BUY | 60–69 ACCUMULATE | 50–59 WATCH | <50 EXIT
- Factor Stars: ★★★★★(80–100) ★★★★(60–79) ★★★(40–59) ★★(20–39) ★(0–19)
- Active Weights (balanced example): Momentum(40%) Technical(25%) Volume(20%) Quality(10%) Social(5%)
- Gates: Movement >2.5% | Volume >1.75x | Liquidity >$500k | ADX >25

## Exit Hierarchy (Ordered)

1) Risk: HARD_STOP (loss > 1.5× ATR) → FULL_EXIT
2) Time: ≥ 48h → FULL_EXIT
3) Momentum Decay: acceleration reversal → SCALE_50%; momentum dead (1h<0 & 4h<0) → FULL_EXIT
4) Profit Taking: +8% → SCALE_25%; +15% → SCALE_50%

## Regime Detection & Adaptation

- TRENDING_BULL: emphasize momentum, require movement ≥ 2.5%, volume surge ≥ 1.5x
- CHOPPY: emphasize mean reversion, allow RSI extremes
- TRENDING_BEAR: tighter gates, higher quality weighting

## Success Metrics & Accountability

Weekly Report
- Hit Rate (target 55–65%)
- Avg winner/loser ratio
- Exit reasons breakdown (% time vs momentum vs profit)
- Regime accuracy & weight adjustments
- False positive rate (<20%)

Risk Controls
- Max 15 concurrent positions
- Single asset cap: 10% of portfolio
- Daily drawdown breaker: −8%
- Forced exit: 48 hours max

## Implementation Priority (Roadmap)

Week 1: Core Scanner — multi‑timeframe momentum, orthogonal factors, gates & scoring
Week 2: Risk & Exits — exit hierarchy, ATR sizing, time management
Week 3: Regime & Refinement — regime detection, dynamic weights, reversal mode
Week 4: Production — backtesting, perf monitoring, alert integration

## Alignment with Code

- Momentum core protected; technical/volume residualized vs momentum (internal/models/clean_orthogonal_system.go)
- Hard gates implemented, including movement ≥ 2.5%, liquidity ≥ $500k, optional volume surge ≥ 1.75x, ADX/Hurst proxy
- Trader‑facing outputs: Momentum Breakouts, Reversal Candidates, Exit Signals with stars and action tags
- New scanners: Balanced (40/30/30) and Acceleration

Updates tracked in docs/ENGINEERING_TRANSPARENCY_LOG.md

