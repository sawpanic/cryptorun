# CryptoRun Top Gainers Benchmark Diagnostic

**Analysis Timestamp:** 2025-09-06T13:35:00+03:00  
**Benchmark Timestamp:** 2025-09-06T13:32:48+03:00  
**Overall Alignment:** 60%

## Executive Summary

The CryptoRun momentum scanner achieved **60% alignment** with CoinGecko top gainers across 1h and 24h timeframes. This analysis reveals that **guards are the primary filter** (75% of misses), not scoring quality. The system correctly identified high-momentum signals but applied conservative timing and fatigue protections that filtered profitable opportunities.

**Key Finding:** Most missed signals had strong momentum scores but were blocked by operational constraints rather than technical quality issues.

## Performance Breakdown

### 1h Window Analysis
| Metric | Value |
|--------|-------|
| **Alignment Score** | 60% |
| **Total Gainers** | 5 |
| **Matches Found** | 3 |
| **Kendall Tau** | 0.67 |
| **Spearman Rho** | 0.80 |

#### Hits (3/5)
| Rank | Symbol | Gain% | Scanner Rank | Status | Reason |
|------|--------|-------|--------------|--------|---------|
| 1 | BTC | 15.00% | 1 | ✅ HIT | Perfect rank match |
| 2 | ETH | 14.20% | 2 | ✅ HIT | Perfect rank match |
| 4 | SOL | 12.60% | 3 | ✅ HIT | Close rank match (-1) |

#### Misses (2/5)
| Rank | Symbol | Gain% | Primary Block | Secondary Block | Impact |
|------|--------|-------|---------------|-----------------|---------|
| 3 | ADA | 13.40% | freshness_guard | volume_gate | Signal >2 bars old |
| 5 | DOT | 11.80% | score_gate | adx_gate | Score 2.3 < 2.5 min |

### 24h Window Analysis
| Metric | Value |
|--------|-------|
| **Alignment Score** | 60% |
| **Total Gainers** | 5 |
| **Matches Found** | 3 |
| **Kendall Tau** | 0.33 |
| **Spearman Rho** | 0.40 |

#### Hits (3/5)
| Rank | Symbol | Gain% | Scanner Rank | Status | Reason |
|------|--------|-------|--------------|--------|---------|
| 1 | BTC | 45.00% | 1 | ✅ HIT | Dominant performer |
| 3 | ADA | 40.60% | 2 | ✅ HIT | Scanner ranked higher |
| 5 | DOT | 36.20% | 3 | ✅ HIT | Early momentum detection |

#### Misses (2/5)
| Rank | Symbol | Gain% | Primary Block | Secondary Block | Impact |
|------|--------|-------|---------------|-----------------|---------|
| 2 | ETH | 42.80% | fatigue_guard | rsi_threshold | 24h return >12% |
| 4 | SOL | 38.40% | late_fill_guard | execution_delay | Signal delay >30s |

## Gate & Guard Analysis

### Primary Failure Reasons
| Component | Count | % | Description | Missed Gains |
|-----------|-------|---|-------------|--------------|
| **freshness_guard** | 1 | 25% | Signal >2 bars old | 13.4% |
| **score_gate** | 1 | 25% | Momentum <2.5 threshold | 11.8% |
| **fatigue_guard** | 1 | 25% | 24h return >12%, RSI >70 | 42.8% |
| **late_fill_guard** | 1 | 25% | Signal delay >30s | 38.4% |

### Guards vs Gates Impact
- **Guards (Timing/Fatigue):** 3/4 misses = 75%
- **Gates (Quality):** 1/4 misses = 25%

**Interpretation:** The system's technical analysis is sound, but operational constraints are filtering profitable signals.

## Correlation Statistics

### Rank Correlation Quality
| Window | Kendall τ | Spearman ρ | Interpretation |
|--------|-----------|------------|----------------|
| **1h** | 0.67 | 0.80 | Strong short-term correlation |
| **24h** | 0.33 | 0.40 | Weak longer-term correlation |
| **Overall** | 0.50 | 0.60 | Moderate combined performance |

### Statistical Significance
- **1h window:** Marginally significant (p ≈ 0.08)
- **24h window:** Not significant (p ≈ 0.34)
- **Sample size:** Too small (n=5) for robust statistical conclusions

## Actionable Insights

### High-Impact Configuration Changes

#### 1. Fatigue Guard Relaxation (Highest Priority)
```yaml
# Current: momentum_config.Fatigue.Return24hThreshold: 12
# Suggested: momentum_config.Fatigue.Return24hThreshold: 18
```
**Impact:** Would recover ETH (42.8% gain)  
**Risk:** May increase false positives in overextended markets  
**Regime Context:** Trending regimes should tolerate higher momentum continuation

#### 2. Late Fill Tolerance (High Priority)
```yaml
# Current: momentum_config.LateFill.MaxDelaySeconds: 30
# Suggested: momentum_config.LateFill.MaxDelaySeconds: 45
```
**Impact:** Would recover SOL (38.4% gain)  
**Risk:** Slightly increased execution slippage  
**Note:** Within PRD bounds, low operational risk

#### 3. Freshness Constraint (Medium Priority)
```yaml
# Current: momentum_config.Freshness.MaxBarsAge: 2
# Suggested: momentum_config.Freshness.MaxBarsAge: 3
```
**Impact:** Would recover ADA (13.4% gain)  
**Risk:** May include stale signals  
**Context:** 3 bars still within reasonable freshness for 1h timeframe

### Low-Priority Changes
#### Score Gate Adjustment (Proceed with Caution)
```yaml
# Current: entry_exit_config.Entry.MinScore: 2.5
# Suggested: entry_exit_config.Entry.MinScore: 2.0
```
**Impact:** Would recover DOT (11.8% gain)  
**Risk:** HIGH - May degrade overall signal quality  
**Recommendation:** Only adjust after testing other changes

## Regime Context

**Current Regime:** Trending  
**Expected Behavior:** Higher momentum tolerance, reduced guard sensitivity

**Observation:** Guards appear calibrated for choppy/volatile regimes rather than trending conditions. Consider regime-dependent parameter adjustment.

## Performance Summary

| Metric | Value |
|--------|-------|
| **Total Potential Gains Missed** | 134.90% |
| **Average Missed Gain** | 33.73% |
| **Strongest Missed Opportunity** | ETH (42.8%) |
| **Primary Optimization Target** | fatigue_guard |

## Success Patterns

**BTC Perfect Performance:** Demonstrates the system works excellently when conditions align:
- Strong momentum score (>4.0 both windows)
- Clean technical picture (no guard conflicts)
- Dominant liquidity profile
- Perfect rank alignment (#1 gainer → #1 scan result)

## Recommendations

### Immediate Actions
1. **Implement regime-dependent guard thresholds**
2. **Increase fatigue threshold to 18% for trending regimes**
3. **Extend late fill tolerance to 45 seconds**
4. **Consider 3-bar freshness limit for 1h scans**

### Medium-Term Improvements
1. **Expand sample size to n≥20 for statistical validity**
2. **Add acceleration renewal mechanism tuning**
3. **Implement dynamic parameter adjustment based on regime**
4. **Add infrastructure latency monitoring**

### Quality Assurance
- **Score gate (2.5 threshold):** Maintain current level - lowest miss priority
- **Volume/ADX gates:** Secondary factors, monitor after guard adjustments
- **Preserve BTC-level performance** as quality baseline

---

*This diagnostic was generated by CryptoRun MomentumCore v3.2.1 using granular hit/miss analysis with gate attribution methodology.*