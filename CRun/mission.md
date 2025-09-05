# CProtocol Product Vision & Requirements v3.2.1
*Real-Time 6-48 Hour Cryptocurrency Momentum Scanner with Fact-Checked Free API Implementation*

---

## Executive Summary

CProtocol identifies and ranks cryptocurrency trading opportunities optimized for 6-48 hour holding periods by combining price momentum, catalyst timing, brand power, and market microstructure into transparent, actionable signals. The system prevents late entries through freshness gates, avoids exhausted moves via fatigue guards, and maintains score calibration across market regimes. **V3.2.1 provides fact-checked, production-ready free API implementation with proper rate limits and provider-aware circuit breakers.**

---

## Core Product Requirements

### 1. Multi-Timeframe Momentum Engine with Context Guards

#### Signal Architecture
- **Primary Signal** (4-hour): 35% weight - optimal for 6-48h positions
- **Entry Precision** (1-hour): 20% weight - timing entries and early exits  
- **Trend Confirmation** (12-hour): 30% weight - validating momentum quality
- **Daily Context** (24-hour): 10-15% weight - exhaustion detection
- **Weekly Carry** (7-day): 5-10% weight in trending markets, 0-2% in chop

#### Momentum Protection Features
- **Acceleration Detection**: Second derivative of 4h momentum (d²/dt²)
- **Fatigue Guard**: Block entries when 24h > +12% AND RSI(4h) > 70 unless pullback or renewed acceleration
- **Entry Freshness**: Entries must occur within 2 bars of signal AND within 1.2×ATR(1h) of trigger price
- **Late Fill Guard** (V3.2.1): Invalidate entry if signal-to-fill > 30 seconds

### 2. Catalyst-Heat Framework with Time Decay

#### Event Scoring
- **Imminent** (0-4 weeks): 1.2x multiplier
- **Near-term** (4-8 weeks): 1.0x multiplier
- **Medium-term** (8-16 weeks): 0.8x multiplier
- **Distant** (16+ weeks): 0.6x multiplier

#### Event Categories
- Token unlocks and vesting schedules
- Exchange listings (with 30-60s polling of announcement pages, respecting robots.txt)
- Protocol upgrades and governance proposals
- Partnership announcements
- Major conference presentations

### 3. Brand Power & Social Momentum (Capped Contribution)

#### Metrics (0-10 Scale)
- Social volume velocity (LunarCrush Galaxy Score)
- Influencer engagement rates
- Media coverage frequency
- Community growth acceleration
- Sentiment divergence from price

#### Contribution Limits
- Maximum positive contribution: +10 score points
- Applied as residual after momentum and volume factors
- Prevents hype from overwhelming price/volume reality

### 4. Market Microstructure & Execution Feasibility

#### Depth Requirements
- **Measurement**: Cumulative depth on bid (sells) and ask (buys) within ±2% on primary execution venue
- **Minimum**: $100k at 2% slippage for full position
- **Source**: **EXCHANGE-NATIVE ONLY** (Binance/OKX/Coinbase L1/L2 books)
- **Aggregator Ban**: DEXScreener, CoinGecko, CoinMarketCap **MUST NOT** be used for depth/spread
- **Venue-Specific Sizing**: Calculate max size per venue based on that venue's specific depth

#### Liquidity Gates
- Bid-ask spread: <50 basis points
- Daily volume: >$500,000 USD
- Volume surge: VADR >1.75x average (minimum 20 bars for stability)
- ADV limits: Soft cap at 0.25%, hard cap at 0.5%

#### Venue Health Definition (V3.2.1)
```python
VENUE_UNHEALTHY = {
    "reject_rate": > 0.05,      # 5% order rejects
    "heartbeat_gap": > 10,      # seconds
    "error_rate": > 0.03,       # 3% API errors
    "latency_p99": > 2000,      # milliseconds
    "action": "halve_position_size"
}
```

### 5. Orthogonal Factor System

#### Factor Hierarchy (Gram-Schmidt Order)
1. **MomentumCore** - Protected base vector (never residualized)
2. **TechnicalResidual** - Orthogonal to momentum
3. **VolumeResidual** - Orthogonal to momentum + technical
4. **QualityResidual** - Orthogonal to all above
5. **SocialResidual** - Fully residualized (capped at +10 points)

### 6. Regime-Adaptive Weight System

#### Regime Detection (V3.2.1)
```python
REGIME_DETECTOR = {
    "indicators": {
        "realized_vol_7d": weight(0.4),
        "percent_above_20ma": weight(0.3),
        "breadth_thrust": weight(0.3)
    },
    "thresholds": {
        "trending_bull": {"vol": <0.3, "above_ma": >0.6, "thrust": >0.4},
        "choppy": {"vol": 0.3-0.5, "above_ma": 0.4-0.6, "thrust": 0.2-0.4},
        "high_volatility": {"vol": >0.5, "above_ma": any, "thrust": any}
    },
    "update_frequency": "4h",
    "majority_vote": True
}
```

#### Trending Bull Market
- Momentum: 40-45% (24h: 10-15%, 7d: 5-10% within)
- Catalyst: 12-15%
- Technical: 18-22%
- Volume: 15-20%
- Quality: 5-10%

#### Choppy/Ranging Market
- Momentum: 25-30% (24h: 5-8%, 7d: ≤2% within)
- Catalyst: 18-22%
- Technical: 22-28%
- Volume: 15-20%
- Quality: 10-15%

#### High Volatility
- Momentum: 28-35%
- Quality: 30-35% (elevated for safety)
- Technical: 20-25%
- Movement gates: Tightened to 3.0-4.0%

### 7. Entry & Exit Signal System

#### Entry Gates (All Must Pass)
- **Movement Threshold**: |4h%| ≥ 2.5% (bull), 3.0% (chop), 4.0% (bear)
- **Volume Surge**: ≥1.75x average (or top 35th percentile)
- **Liquidity Minimum**: $500k daily volume
- **Trend Quality**: ADX > 25 OR Hurst > 0.55
- **Freshness Window**: Within 2 bars of trigger
- **Late Fill Guard**: Signal-to-fill < 30 seconds
- **Symbol Cool-off**: N hours after exit in choppy markets

#### Exit Hierarchy (First Trigger Wins)
1. **Hard Stop**: -1.5 × ATR
2. **Venue Health Exit**: Tighten by +0.3×ATR if venue degrades
3. **Time Limit**: 48 hours maximum
4. **Acceleration Reversal**: d²/dt² < 0
5. **Momentum Fade**: Both 1h and 4h negative
6. **Trailing Adjustment**: After 12h, tighten to ATR×1.8 unless accelerating
7. **Profit Targets**: 8% (25% position), 15% (50% position), 25% (75% position)

#### Exit Distribution KPIs
- Time-limit exits: ≤40%
- Hard stops: ≤20%
- Momentum/profit exits: ≥40%
- Self-tuning: If time-limits >40% for 2 weeks, auto-tighten gates by +0.5pp

---

## Data Architecture & Quality Requirements

### Three-Ring Data Mesh

#### Hot Set (Top 30 by Volume)
- **Sources**: Exchange WebSockets (Binance, Coinbase, OKX)
- **Latency Budget**: 
  - Ingest: 80ms
  - Normalize: 40ms
  - Score: 80ms
  - Serve: 80ms
  - Total: <300ms P99 (stretch goal)
- **Freshness**: Bar close + ≤60s (skip entries if late)

#### Warm Set (Remaining Universe)
- **Sources**: REST APIs with 5-minute caching
- **Cascade**: DEXScreener → CoinGecko → CoinPaprika
- **Reconciliation**: Trimmed median, discard >1% deviations

#### Cold Set (Historical/Context)
- **Sources**: Daily aggregator pulls within monthly budgets
- **Purpose**: 7-day returns, regime detection, backtesting
- **Backfill Switch**: Provision for paid historical data when needed

### Canonical Source Authority (V3.2.1)
```python
CANONICAL_SOURCES = {
    "microstructure": "exchange_native_only",  # Binance/OKX/Coinbase
    "market_cap": "coingecko",                 # Single source
    "circulating_supply": "coingecko",         # Avoid drift
    "price_cex": "binance",                    # Primary CEX
    "price_dex": "dexscreener",                # Primary DEX
    "volume_24h": "coingecko",                 # Aggregated
    "holders": "moralis",                      # On-chain
}
```

### Data Source Cascade

#### Price & Volume
1. **CEX**: Binance → Coinbase → OKX (exchange-native only)
2. **DEX**: DEXScreener → Moralis
3. **Aggregated**: CoinGecko → CoinPaprika → CoinMarketCap (cached)

#### Microstructure (CRITICAL)
- **MUST** use exchange-native L1/L2 books
- **BANNED**: All aggregators for depth/spread
- **Per-Venue**: Calculate metrics per exchange, not globally

#### Catalysts
- CoinMarketCal (events) → TokenUnlocks (vesting) → Messari (governance)
- Exchange announcement pages (respect robots.txt, use cache)

#### Social/Brand
- LunarCrush (primary) → Santiment (fallback)
- Cache for 1 hour minimum

### Data Quality Controls
- **Staleness Detection**: Flag data >interval old
- **Source Attribution**: Track contributing APIs
- **Reconciliation Stats**: Show red dot when sources disagree
- **Point-in-Time Integrity**: Immutable snapshots, no retro edits
- **VADR Stability**: Freeze scores if <20 bars available

---

## 🆕 V3.2.1: Production-Ready Free API Implementation

### Fact-Checked API Limits & Implementation

#### **DEXScreener** 
- **Cost**: FREE, no API key required
- **Limits**: **Endpoint-specific** (e.g., 60 rpm for token-profiles/latest)
- **Implementation**: Per-endpoint throttles with exponential backoff
- **Use For**: Real-time DEX discovery, NOT for microstructure
- **Cache**: 0 for prices, 60s for metadata

#### **Binance**
- **Cost**: FREE
- **Limits**: **Weight-based** across multiple windows
- **Headers**: Monitor `X-MBX-USED-WEIGHT-*` 
- **Errors**: Handle 429 (rate limit) and 418 (IP ban) with backoff
- **WebSocket**: Has connection limits and reconnection rules
- **Use For**: CEX microstructure, order books, execution

#### **CoinGecko (Demo Plan)**
- **Cost**: FREE with registration
- **Limits**: 30 calls/min, 10,000 calls/month
- **Cache**: 5-10 minutes minimum
- **Use For**: Market caps, aggregated volumes

#### **Moralis**
- **Cost**: FREE tier available
- **Limits**: 40,000 CU/day, ~1,000 CU/s throughput
- **CU Costs**: Vary by endpoint (check latest table)
- **Cache**: 60-300 seconds
- **Use For**: On-chain data, wallet analysis

#### **CoinMarketCap**
- **Cost**: FREE tier available
- **Limits**: ~10,000 calls/month, ~30 rpm
- **Cache**: 10 minutes minimum
- **Use For**: Backup market data only

#### **Etherscan**
- **Cost**: FREE with API key
- **Limits**: 5 requests/sec, 100,000/day
- **Cache**: 1+ hour for static data
- **Use For**: Contract verification, holder counts

#### **CoinPaprika**
- **Cost**: FREE unauthenticated
- **Limits**: ~1,000 requests/day without key
- **Cache**: 5 minutes
- **Use For**: Tertiary fallback only

### Rate Limit Configuration (Production-Safe)

```python
RATE_LIMITS_V321 = {
    "dexscreener": {
        "rpm": 60,  # Conservative per-endpoint
        "burst_rps": 2,
        "per_endpoint": True,
        "backoff_base": 2
    },
    "binance": {
        "mode": "weight_based",
        "respect_headers": True,
        "handle_429": True,
        "handle_418": True,
        "cooldown_418": 120  # 2 min for IP bans
    },
    "coingecko": {
        "rpm": 30,
        "monthly": 10_000,
        "cache_ttl": 300,
        "budget_guard": 1_000  # Switch providers at 1k remaining
    },
    "moralis": {
        "daily_cu": 40_000,
        "cps": 1_000,
        "cache_ttl": 120,
        "cu_guard": 5_000  # Daily reserve
    },
    "coinmarketcap": {
        "monthly": 10_000,
        "rpm": 30,
        "cache_ttl": 600,
        "use": "backup_only"
    },
    "etherscan": {
        "rps": 5,
        "daily": 100_000,
        "cache_ttl": 3_600
    },
    "coinpaprika": {
        "daily": 1_000,
        "cache_ttl": 300,
        "use": "tertiary_fallback"
    }
}
```

### Provider-Aware Circuit Breakers

```python
CIRCUIT_BREAKERS = {
    "triggers": {
        "monthly_remaining": 1_000,     # CoinGecko/CMC
        "daily_cu_remaining": 5_000,    # Moralis
        "error_rate": 0.05,            # 5% errors
        "latency_p99": 1_000,          # 1 second
        "consecutive_failures": 3
    },
    "actions": {
        "primary_degraded": "switch_to_secondary",
        "secondary_degraded": "switch_to_tertiary",
        "cache_ttl_multiplier": 2.0,   # Double cache times
        "restore_probes": 5,            # Successful before restore
        "probe_interval": 60            # Seconds between probes
    }
}
```

### Monthly Budget Management

```python
MONTHLY_BUDGET_SCENARIO = {
    "hot_set_30_tokens": {
        "binance_ws": "unlimited",
        "dexscreener": 2_592_000,  # 60rpm * 30d * 24h * 60m
        "api_calls": 0  # All via WebSocket
    },
    "warm_set_970_tokens": {
        "coingecko": 8_000,  # With 5-min cache
        "coinpaprika": 15_000,  # 500/day avg
        "cache_hit_rate": 0.85  # Target
    },
    "cold_set_historical": {
        "coingecko": 1_000,  # Daily pulls
        "cache_ttl": 86_400  # 24 hours
    },
    "reserves": {
        "coingecko": 1_000,  # Emergency buffer
        "moralis": 10_000   # CU reserve
    }
}
```

### Optimized Cache Strategy

```python
CACHE_CONFIG_V321 = {
    # Real-time (no cache)
    "order_books": 0,
    "trades": 0,
    "websocket_data": 0,
    
    # Near real-time
    "prices_hot": 5,        # 5s for hot pairs
    "prices_warm": 30,      # 30s for warm
    "new_pairs": 60,
    
    # Market data
    "market_caps": 300,     # 5 minutes
    "volumes": 120,         # 2 minutes for VADR
    "trending": 300,
    
    # Static data
    "token_metadata": 86_400,
    "contract_info": 86_400,
    "historical": 86_400
}
```

### API Fallback Chains (Explicit)

```python
API_FALLBACK_V321 = {
    "price_data_cex": [
        "binance",         # Primary (native)
        "coingecko",       # Secondary (cached)
        "coinpaprika"      # Tertiary
    ],
    "price_data_dex": [
        "dexscreener",     # Primary
        "moralis"          # Secondary (if CU available)
    ],
    "market_cap": [
        "coingecko",       # Canonical source
        "coinmarketcap",   # Backup
        "coinpaprika"      # Emergency
    ],
    "microstructure_cex": [
        "binance"          # ONLY exchange-native
        # NO FALLBACK - better to skip than use aggregator
    ],
    "liquidity_dex": [
        "dexscreener"      # Volume/trades only
        # NOT for depth/spread
    ],
    "holder_data": [
        "moralis",         # If CU budget allows
        "etherscan"        # Fallback
    ]
}
```

### Cross-Correlation Matrix (V3.2.1)

```python
CORRELATION_CONTROLS = {
    "calculation": {
        "windows": ["1h", "4h"],
        "method": "rolling_pearson",
        "threshold": 0.8
    },
    "limits": {
        "max_correlated_positions": 2,
        "sector_cap": 2,
        "ecosystem_cap": 3  # e.g., max 3 SOL ecosystem
    },
    "override": "manual_only"  # Human can override
}
```

### Emergency Controls

```python
EMERGENCY_CONTROLS = {
    "symbol_blacklist": {
        "duration": "24h",
        "trigger": "manual",
        "reason": ["exchange_halt", "exploit", "news_bomb"]
    },
    "global_pause": {
        "trigger": ["drawdown > 8%", "api_failures > 50%"],
        "duration": "until_manual_resume"
    },
    "degraded_mode": {
        "reduce_position_sizes": 0.5,
        "increase_score_threshold": 10,
        "tighten_stops": 0.3
    }
}
```

---

## Output & Display Requirements

### Scanner Display Format
```
MOMENTUM SIGNALS (6-48h Opportunities) | Regime: CHOPPY | APIs: 4/6 Healthy
═══════════════════════════════════════════════════════════════════════════════════════════
Rank | Symbol | Score | Momentum | Catalyst | Volume | Change%              | Action
     |        | 0-100 | Core     | Heat     | VADR   | 1h/4h/12h/24h/7d*    |
───────────────────────────────────────────────────────────────────────────────────────────
1    | CAMP   | 94.2  | 89.5 ⭐⭐⭐⭐⭐ | 7.5 ⭐⭐⭐⭐  | 3.2x   | +1.8/+5.2/+12.1/+15.3/+8.5 | STRONG BUY
     |        |       | [Fresh ●] [Depth ✓] [Venue: BIN] [Sources: 3] [Latency: 142ms]       |

*7d shown in Trending Bull only
```

### Score Calibration & Interpretation
- **85-100**: STRONG BUY - Immediate full position
- **70-84**: BUY - Standard entry with normal size
- **60-69**: ACCUMULATE - Scale in 50% position
- **50-59**: WATCH - Monitor only, no entry
- **<50**: EXIT/AVOID - Reduce or avoid entirely

### Signal Transparency Indicators
- **Freshness**: 🟢 ≤60s | 🟡 ≤180s | 🔴 >180s
- **Data Quality**: ✓ if ≥2 sources agree
- **Venue Health**: Per-exchange status
- **API Health**: Shows degraded providers
- **Latency**: Actual processing time

---

## Success Metrics & Monitoring

### Performance KPIs
- **Hit Rate**: 55-65% profitable trades
- **Average Winner**: 8-12% gain
- **Average Loser**: 2-3% loss  
- **Sharpe Ratio**: >1.5
- **Monthly Signals**: 50-150
- **Exit Distribution**: ≤40% time-limits, ≤20% stops

### Operational KPIs
- **Scanner Latency**: <300ms P99 (stretch goal)
- **Data Freshness**: 1-minute maximum lag for hot set
- **API Efficiency**: <1000 calls/hour AND <10k/mo on CG/CMC
- **False Positive Rate**: <20%
- **Reconciliation Success**: >80% multi-source agreement
- **Cache Hit Rate**: >85% for warm set
- **API Cost**: $0/month

### API Health Dashboard (V3.2.1)
```
API USAGE & HEALTH (UTC 2024-03-15 14:30:00)
═══════════════════════════════════════════════════════════════════════════════
Provider     | Today    | Month Used | Limit    | Health | Latency | Cost
─────────────────────────────────────────────────────────────────────────────
Kraken       | 523,421  | N/A        | 15/sec   | 🟢 99% | 32ms    | $0
DEXScreener  | 43,200   | N/A        | 60/min*  | 🟢 99% | 89ms    | $0
Binance      | 12.3k W  | N/A        | Weight   | 🟢 98% | 42ms    | $0
CoinGecko    | 312      | 8,234      | 10,000   | 🟢 98% | 234ms   | $0
Moralis      | 12k CU   | N/A        | 40k/day  | 🟢 97% | 156ms   | $0
CoinMarketCap| 89       | 3,421      | 10,000   | 🟡 94% | 412ms   | $0
CoinPaprika  | 234      | N/A        | 1k/day   | 🟢 96% | 203ms   | $0
─────────────────────────────────────────────────────────────────────────────
Circuit Status: [CG Budget Guard Active - 1,766 remaining]
*Per endpoint | W=Weight | Primary: KRAKEN
```

### Weekly Reports
- Decile lift analysis (score vs forward returns)
- Exit distribution breakdown  
- Source reliability metrics
- Regime detection accuracy
- API usage vs budget
- Cache hit rates by tier
- Venue health statistics

---

## Risk Controls & Governance

### Position & Portfolio Limits
- Maximum 15 concurrent positions
- Single asset cap: 10% of portfolio
- Sector concentration: Max 2 per category
- Ecosystem correlation: Max 3 per ecosystem (e.g., SOL)
- Daily drawdown circuit breaker: -8%

### Universe & Symbol Management
- Daily liquidity-based universe rebuild
- No auto-exit on constituent changes (exit via rules only)
- Symbol cool-off period after exits in choppy markets
- Stablecoin filter to avoid peg anomalies
- Cross-correlation monitoring (>0.8 = same bet)

### Change Management
- All factor/weight changes require A/B testing
- 7-day shadow period before production
- Feature flags for rollback capability
- Immutable audit trail of all scoring changes

### Exchange & API Health Monitoring
- Per-venue latency and reject tracking
- Halve position sizes on unhealthy venues (>5% rejects, >2s latency)
- Automatic API source switching on degradation
- Monthly budget guards with auto-degradation
- Circuit breakers with probe-based recovery

---

## What This Product Is NOT
- Not a high-frequency trading system (6-48 hour holds)
- Not a fundamental analysis tool (momentum-focused)
- Not a buy-and-hold screener (active trading)
- Not a backtesting platform (though it feeds one)
- Not chasing exhausted moves (fatigue guards)
- Not entering late (freshness requirements)
- Not using aggregators for microstructure (exchange-native only)

---

## Core Product Promise

**CProtocol delivers disciplined 6-48 hour momentum signals that:**
- Won't chase late entries (freshness gates + late fill guard)
- Won't size what can't be exited (venue-specific depth requirements)  
- Won't let hype outrank price/volume reality (capped social contribution)
- Won't break under load (circuit breakers + fallbacks)
- Maintains consistent score meanings across market regimes (quantile calibration)
- Proves its discipline through transparent exit distribution metrics
- **Operates at zero API cost while respecting documented rate limits**

---

## V3.2.1 Implementation Checklist

### Phase 1: Core Infrastructure (Week 1)
- [ ] Implement DEXScreener with per-endpoint throttles
- [ ] Set up Binance WebSocket with weight monitoring
- [ ] Build `X-MBX-USED-WEIGHT` header parser
- [ ] Create 429/418 handler with exponential backoff
- [ ] Implement Redis caching with proper TTLs

### Phase 2: Data Expansion (Week 2)
- [ ] Register CoinGecko Demo account
- [ ] Implement monthly budget guard (switch at 1k remaining)
- [ ] Add Moralis with CU tracking
- [ ] Build provider-aware circuit breakers
- [ ] Test fallback chains

### Phase 3: Microstructure & Safety (Week 3)
- [ ] Enforce exchange-native-only for depth/spread
- [ ] Implement venue-specific sizing
- [ ] Add correlation matrix monitoring
- [ ] Build emergency symbol blacklist
- [ ] Create venue health scoring

### Phase 4: Production Hardening (Week 4)
- [ ] Load test at 2x expected volume
- [ ] Verify <300ms P99 latency (hot set)
- [ ] Confirm >85% cache hit rate (warm set)
- [ ] Test circuit breakers and recovery
- [ ] Document latency budget breakdown
- [ ] Launch with full monitoring

### Continuous Monitoring
- [ ] Daily: API usage vs budgets
- [ ] Weekly: Exit distribution analysis
- [ ] Monthly: Provider reliability assessment

---

## Appendix: Quick Reference

### Binance 418 Ban Recovery Playbook
1. Stop all requests immediately
2. Check `Retry-After` header
3. Wait full cooldown (2 min minimum)
4. Reduce concurrency by 50%
5. Resume with exponential backoff
6. Consider IP rotation if repeated

### VADR Calculation Notes
- Minimum 20 bars for stability
- Freeze if data incomplete
- Use venue-specific volumes when possible
- Cache for 2 minutes maximum

### Human Override Capabilities
- Emergency symbol blacklist (24h)
- Correlation limit override
- Regime force-set
- Global pause button
- API source manual selection

---

*CProtocol V3.2.1 - Production-ready momentum scanning with fact-checked free APIs and industrial-grade reliability at zero cost.*