# CryptoRun Reports System

Automated daily and weekly operational reports with decile lift analysis, system KPIs, and comprehensive performance metrics.

## UX MUST — Live Progress & Explainability

CryptoRun's automated reporting system provides comprehensive analysis of regime performance, exit distributions, and score→return lift with complete explainability. All reports include point-in-time data integrity, KPI violation alerts, and actionable recommendations for system optimization.

## Overview

The reporting system generates weekly analysis reports that track regime detector performance, position exit patterns, and factor effectiveness. Reports combine markdown summaries with CSV datasets for both human readability and programmatic analysis.

**Core Reports Available**:
- **Regime Analysis**: Weekly flip history, exit distribution tracking, and score→return lift analysis
- **Performance Tracking**: Coming soon - overall system performance metrics
- **Portfolio Monitoring**: Coming soon - position-level analysis and risk metrics

## Regime Weekly Report

### Command Usage

```bash
# Generate 28-day regime analysis (recommended)
cryptorun report regime --since 28d --out ./artifacts/reports

# Generate with different time periods
cryptorun report regime --since 4w --out ./reports
cryptorun report regime --since 90d --out ./quarterly

# Include visual charts (if supported)
cryptorun report regime --since 28d --charts --out ./reports

# Use point-in-time data integrity (default)
cryptorun report regime --since 28d --pit --out ./artifacts/reports
```

### Report Components

#### 1. KPI Alert System

The report automatically flags violations of key performance indicators:

**Exit Distribution KPIs:**
- **Time Limit Exits**: ≤40% target (warns if >40%, critical if >50%)
- **Hard Stop Exits**: ≤20% target (always critical if exceeded)  
- **Profit Target Exits**: ≥25% target (warns if below)

**Score Lift KPIs:**
- **Decile Lift**: ≥2.0× minimum (top vs bottom decile)
- **Correlation**: ≥0.15 minimum (score vs 48h return)

**Automatic Actions:**
- Flags suggest tightening entry gates by +0.5pp when time-limit or hard-stop thresholds are breached
- Recommends factor rebalancing when correlation degrades
- Highlights regime-specific performance issues

#### 2. Regime Flip Timeline

**4-Week Flip History Analysis:**

| Metric | Description | Interpretation |
|--------|-------------|----------------|
| **Flip Frequency** | Transitions per week | Healthy: 2-4 flips/week, Unstable: >6 flips/week |
| **Duration Stability** | Average regime duration | Target: 12-48 hours per regime |
| **Detector Inputs** | Vol/Breadth/Thrust snapshot | Validates detection logic consistency |

**Sample Output:**
```
Regime Flip History (28 days)
Total Flips: 11 | Average Duration: 25.6h

| Date | From | To | Duration | Vol 7d | Above 20MA | Breadth | Momentum Δ |
|------|------|----|---------:|-------:|-----------:|--------:|-----------:|
| 01-15 14:30 | 📈 bull | ↔️ choppy | 18.5h | 0.42 | 0.58 | -0.08 | -15.0% |
| 01-16 09:00 | ↔️ choppy | ⚡ high_vol | 31.2h | 0.63 | 0.45 | -0.15 | -5.0% |
```

#### 3. Exit Distribution Analysis

**Performance by Regime:**

| Regime | Exits | Time Limit | Hard Stop | Profit Target | Avg Return | Status |
|--------|------:|-----------:|----------:|--------------:|-----------:|--------|
| 📈 Bull | 200 | 25.0% ✅ | 12.0% ✅ | 35.0% ✅ | +18.5% | Healthy |
| ↔️ Choppy | 150 | 45.0% ⚠️ | 25.0% 🔴 | 20.0% ⚠️ | +8.2% | **Action Required** |
| ⚡ High Vol | 180 | 35.0% ✅ | 15.0% ✅ | 28.0% ✅ | +22.1% | Healthy |

**KPI Interpretation:**
- ✅ **Green**: Within target thresholds
- ⚠️ **Yellow**: Warning threshold exceeded, monitor closely
- 🔴 **Red**: Critical threshold exceeded, immediate action required

#### 4. Score→Return Lift Analysis

**Decile Performance by Regime:**

**Trending Bull Regime** (Correlation: 0.75, Lift: 4.2×)
| Decile | Score Range | Count | Avg Return | Hit Rate | Performance |
|-------:|-------------|------:|-----------:|---------:|------------|
| 10 | 90-110 | 65 | +27.5% | 81% | Excellent |
| 9 | 80-90 | 60 | +22.8% | 76% | Strong |
| 8 | 70-80 | 58 | +19.2% | 72% | Good |
| ... | ... | ... | ... | ... | ... |
| 1 | 0-10 | 45 | +5.0% | 45% | Weak |

**Choppy Market Regime** (Correlation: 0.45, Lift: 2.1×)  
| Decile | Score Range | Count | Avg Return | Hit Rate | Performance |
|-------:|-------------|------:|-----------:|---------:|------------|
| 10 | 90-110 | 52 | +14.2% | 68% | Moderate |
| 9 | 80-90 | 48 | +11.8% | 64% | Fair |
| ... | ... | ... | ... | ... | ... |
| 1 | 0-10 | 38 | -2.0% | 42% | Poor |

### Generated Artifacts

Each report generates timestamped files with point-in-time data integrity:

**Markdown Report:**
- `regime_weekly_20250115_143000.md` - Human-readable summary with KPI alerts

**CSV Datasets:**
- `regime_flips_20250115_143000.csv` - Complete flip history with detector inputs
- `regime_exits_20250115_143000.csv` - Exit distribution by regime with percentages  
- `regime_deciles_20250115_143000.csv` - Score→return analysis by decile and regime
- `regime_alerts_20250115_143000.csv` - KPI violations with recommended actions

### Sample Report Screenshots

#### KPI Alert Dashboard
```
🚨 KPI Alerts

🔴 hard_stop_breach - Choppy Regime
• Current: 25.0%
• Target: 20.0%
• Action: Tighten entry gates by +0.5pp and review risk management

Choppy regime: 25.0% hard-stop exits exceed target of 20.0%

🟡 time_limit_breach - Choppy Regime  
• Current: 45.0%
• Target: 40.0%
• Action: Tighten entry gates by +0.5pp to reduce position sizing

Choppy regime: 45.0% time-limit exits exceed target of 40.0%
```

#### Flip Timeline Visualization
```
📈 Regime Flip History

Jan 15 ████████████████████ (18.5h) 📈→↔️ Vol:0.42 MA:0.58 Thrust:-0.08
Jan 16 ████████████████████████████████████ (31.2h) ↔️→⚡ Vol:0.63 MA:0.45 Thrust:-0.15  
Jan 17 ████████████████████████ (22.8h) ⚡→📈 Vol:0.38 MA:0.72 Thrust:+0.12
```

### KPI Threshold Configuration

Default KPI thresholds align with CryptoRun's operational targets:

```yaml
kpi_thresholds:
  time_limit_max: 40.0     # ≤40% time-limit exits
  hard_stop_max: 20.0      # ≤20% hard-stop exits  
  profit_target_min: 25.0  # ≥25% profit-target exits
  lift_min: 2.0           # ≥2.0× decile lift ratio
  correlation_min: 0.15    # ≥0.15 score→return correlation
```

**Customization**: Thresholds can be adjusted via config files or command-line flags for different risk tolerances.

## Data Architecture & Integrity

### Point-in-Time (PIT) Data

All regime reports maintain **strict point-in-time integrity**:
- Regime states recorded at decision time (no retroactive adjustments)
- Exit classifications use position entry regime, not exit-time regime
- Factor weights locked to regime active during position entry
- Detector inputs snapshot at exact flip timestamp

**Benefits**:
- Eliminates look-ahead bias in performance analysis
- Ensures reproducible results across report generations  
- Maintains audit trail of actual system behavior
- Supports regulatory compliance and backtesting validation

### Data Sources

Reports pull from CryptoRun's three-tier data architecture:

**Hot Tier**: Real-time regime states, position entries/exits  
**Warm Tier**: Cached detector inputs, factor snapshots  
**Cold Tier**: Historical regime transitions, performance summaries

**Data Validation**:
- Cross-checks against ledger files for position accuracy
- Validates detector input ranges and threshold compliance
- Ensures weight allocation sums to 100% (excluding social cap)
- Confirms timestamp chronology and gap detection

## Automation & Scheduling

### Recommended Schedule

**Weekly Reports**: Generate every Sunday at midnight UTC
```bash
# Weekly cron job
0 0 * * 0 /usr/local/bin/cryptorun report regime --since 28d --out /opt/cryptorun/reports/
```

**Monthly Deep Dive**: Generate 90-day analysis first of each month
```bash  
# Monthly comprehensive analysis
0 6 1 * * /usr/local/bin/cryptorun report regime --since 90d --charts --out /opt/cryptorun/monthly/
```

### Alert Integration

Reports can be integrated with monitoring systems:

**Slack/Discord Notifications**:
```bash
# Generate report and send alerts
cryptorun report regime --since 28d --out /tmp/reports/
if [ $? -eq 0 ]; then
  ./scripts/send_regime_alerts.sh /tmp/reports/regime_alerts_*.csv
fi
```

**Email Summaries**:
```bash
# Email weekly summary to stakeholders
cryptorun report regime --since 28d --out /tmp/reports/
python scripts/email_regime_summary.py /tmp/reports/regime_weekly_*.md
```

## Troubleshooting

### Common Issues

**1. Missing Data**
```
Error: insufficient flip history for period
```
- **Solution**: Reduce analysis period or check regime detector is running
- **Command**: `cryptorun report regime --since 7d` (shorter period)

**2. KPI Threshold Breaches**
```
Critical: Choppy regime hard-stop exits at 25.0%
```
- **Action**: Tighten entry gates: `config/quality_policies.json` +0.5pp
- **Monitor**: Re-run report after 3-5 days to validate improvement

**3. Low Correlation Warnings**
```
Warning: High-vol regime correlation 0.08 below target 0.15
```
- **Investigation**: Check factor orthogonalization, review regime weights
- **Command**: `cryptorun explain delta --universe topN=30` for factor analysis

### Performance Optimization

**Large Data Sets**: Use smaller time windows for faster generation
**Network Storage**: Generate locally, then copy to network share
**Concurrent Reports**: Stagger automated report generation to avoid resource conflicts

---

**Next Steps**: See [Regime Tuner System](./REGIME_TUNER.md) for regime detection methodology and [CLI Documentation](./CLI.md) for complete command reference.