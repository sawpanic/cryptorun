# CryptoRun Benchmarks

CryptoRun provides comprehensive benchmarking capabilities to compare scanning results against external market references, enabling validation and performance analysis of momentum and dip signals.

## UX MUST — Live Progress & Explainability

All benchmarks provide full transparency and explainability:
- **Real-time Progress**: Phase indicators with detailed status updates
- **Complete Attribution**: Data sources, methodology, processing times, confidence metrics
- **Explainability Artifacts**: Comprehensive reports with interpretation guidelines

## Top Gainers Benchmark

### Overview

The Top Gainers Benchmark compares CryptoRun's momentum/dip scanner outputs against CoinGecko's top gainers lists at multiple timeframes (1h, 24h, 7d). This provides insight into how well our signals align with actual market performance.

### Usage

#### Basic Usage
```bash
# Run benchmark with default settings
cryptorun bench topgainers

# Custom configuration
cryptorun bench topgainers --limit 20 --windows "1h,24h,7d" --progress plain

# Automated/CI usage  
cryptorun bench topgainers --progress json --ttl 600 > benchmark_results.log
```

#### Available Flags
- `--progress` (auto|plain|json): Progress output mode (default: auto)
- `--ttl` (int): Cache TTL in seconds, minimum 300 (default: 300)
- `--limit` (int): Maximum top gainers per window (default: 20)
- `--windows` (string): Time windows to analyze (default: "1h,24h,7d")

### Output Files

#### Primary Artifacts
- **`out/bench/topgainers_alignment.json`**: Complete results in JSON format
- **`out/bench/topgainers_alignment.md`**: Human-readable analysis report
- **`out/bench/topgainers_{1h,24h,7d}.json`**: Per-window raw data

#### Progress Traces
- **`out/audit/progress_trace.jsonl`**: Detailed progress events for debugging

### Scoring Methodology

#### Composite Scoring
The benchmark uses a weighted composite score combining three components:

1. **Symbol Overlap (60%)**: Jaccard similarity coefficient
   - Measures intersection over union of symbol sets
   - Range: 0.0 (no overlap) to 1.0 (perfect overlap)

2. **Rank Correlation (30%)**: Spearman-like correlation
   - Compares ranking positions of common symbols
   - Range: 0.0 (no correlation) to 1.0 (perfect correlation)

3. **Percentage Alignment (10%)**: Future enhancement
   - Will compare price change percentages with momentum scores
   - Currently returns neutral score (0.5)

#### Interpretation Guidelines

**High Alignment (>70%)**
- Scanner is well-aligned with market gainers
- Signals correspond closely to price movements
- Good market timing capability

**Medium Alignment (30-70%)**
- Partial alignment with market movements  
- May indicate different time horizons or strategies
- Scanner captures some but not all opportunities

**Low Alignment (<30%)**
- Scanner focuses on different opportunities than pure gainers
- May prioritize quality over momentum
- Potential for contrarian or value-based strategies

### Configuration

#### config/bench.yaml
```yaml
topgainers:
  ttl: 300s              # Minimum cache TTL (respects rate limits)
  limit: 20              # Default number of top gainers per window
  windows: ["1h", "24h", "7d"]  # Default time windows
  
scoring:
  weights:
    symbol_overlap: 0.6   # Primary component (Jaccard similarity)
    rank_correlation: 0.3 # Secondary component (ranking)
    percentage_align: 0.1 # Future component (percentage correlation)
    
  thresholds:
    high_alignment: 0.7   # High alignment threshold
    medium_alignment: 0.3 # Medium alignment threshold
```

#### Rate Limiting Compliance
- **TTL Minimum**: 300 seconds to respect CoinGecko rate limits
- **API Usage**: Lists/indices only, no microstructure data
- **Caching Strategy**: Cache-first with TTL validation
- **Request Budget**: Respects rpm and monthly limits

### Sample Output

#### JSON Structure
```json
{
  "timestamp": "2025-09-06T13:14:14Z",
  "overall_alignment": 0.62,
  "methodology": "CryptoRun TopGainers Benchmark v3.2.1 with CoinGecko trending indices",
  "window_alignments": {
    "1h": {
      "window": "1h",
      "score": 0.45,
      "matches": 4,
      "total": 10,
      "details": "Found 4 matches out of 10 top gainers"
    },
    "24h": {
      "window": "24h", 
      "score": 0.67,
      "matches": 7,
      "total": 10,
      "details": "Found 7 matches out of 10 top gainers"
    }
  },
  "top_gainers": {
    "1h": [
      {"symbol": "BTC", "price_change_percentage": "12.45"},
      {"symbol": "ETH", "price_change_percentage": "8.92"}
    ]
  },
  "scan_results": {
    "1h": ["BTC", "ADA", "SOL", "MATIC"],
    "24h": ["BTC", "ETH", "DOT", "LINK", "UNI"]
  }
}
```

#### Markdown Report Sample
```markdown
# Top Gainers Benchmark Report

**Generated**: 2025-09-06T13:14:14Z
**Overall Alignment**: 62.34%

## Window Analysis

### 1H Window
- **Alignment Score**: 45.00%
- **Matches**: 4 out of 10
- **Details**: Found 4 matches out of 10 top gainers

**Top 5 Gainers:**
- BTC: 12.45%
- ETH: 8.92%
- ADA: 7.23%

**Our Scan Results:**
- Symbols: BTC, ADA, SOL, MATIC
```

### Progress Streaming

#### Plain Mode Output
```
🔍 Starting topgainers-benchmark (3 windows)
Windows: [1h 24h 7d]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 Initializing benchmark...
📊 Fetching top gainers data...
  [20%] fetch: 1h
  ✅ 1h: 5 gainers fetched
🧮 Analyzing scan alignment...
📐 Calculating alignment scores...
  [85%] score: 7d
  ✅ 7d: alignment=0.34
📁 Writing output artifacts...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Benchmark completed: 3 windows analyzed (4s)
📁 Results: out/bench/topgainers_alignment.json, out/bench/topgainers_alignment.md
```

#### JSON Mode Output
```json
{"event":"scan_start","pipeline":"topgainers-benchmark","windows":["1h","24h","7d"]}
{"phase":"fetch","symbol":"1h","status":"success","metrics":{"gainers_count":5}}
{"phase":"score","symbol":"1h","status":"success","metrics":{"alignment_score":0.45,"matches":4}}
{"event":"scan_complete","candidates":3}
```

### Integration with Scanning

#### Live Scan Integration (Future)
When `integration.use_live_scans: true` in config:
```bash
# Trigger momentum scans then benchmark
cryptorun bench topgainers --use-live-scans

# This will:
# 1. Run momentum scans for each window
# 2. Fetch CoinGecko top gainers  
# 3. Calculate alignment scores
# 4. Generate comprehensive comparison
```

#### Mock Data Mode (Current)
For testing and development:
- Generates realistic mock top gainers data
- Uses configurable overlap ratios
- Consistent results for reproducible testing
- No external API dependencies

### Performance Considerations

#### Caching Strategy
- **Cache Location**: `out/bench/.cache/`
- **TTL Enforcement**: Minimum 300s, configurable
- **Cache Keys**: Per window, includes timestamp
- **Cleanup**: Automatic cleanup of expired cache files

#### Rate Limiting
- **CoinGecko Free Tier**: 10 requests/minute, 10K/month
- **Batch Processing**: Single request per window
- **Error Handling**: Graceful degradation with cached data
- **Monitoring**: Request count tracking and budget alerts

### Troubleshooting

#### Common Issues

**Cache Errors**
```bash
# Clear cache if corrupted
rm -rf out/bench/.cache/

# Force fresh data
cryptorun bench topgainers --ttl 0  # Not recommended for production
```

**Rate Limit Exceeded**
```bash
# Use longer TTL
cryptorun bench topgainers --ttl 3600  # 1 hour cache

# Check cache status
ls -la out/bench/.cache/
```

**Low Alignment Scores**
- Review time windows - some may be more aligned than others
- Check if scan results are current and relevant
- Consider scanner parameters (regime, venues, sample size)
- Validate that comparison is fair (same symbols/venues)

#### Debug Information
```bash
# Enable detailed progress
cryptorun bench topgainers --progress json | jq '.'

# Check individual window files
cat out/bench/topgainers_1h.json | jq '.top_gainers[0:5]'

# Review progress trace
tail -f out/audit/progress_trace.jsonl | jq 'select(.phase=="score")'
```

### Future Enhancements

#### Planned Features
1. **Percentage Alignment**: Correlation between gain percentages and momentum scores
2. **Volume Correlation**: Compare volume surges with our volume gates
3. **Multiple References**: Support for additional data sources beyond CoinGecko
4. **Historical Analysis**: Track alignment over time, identify patterns
5. **Real-time Alerts**: Notify when alignment drops below thresholds

#### Integration Opportunities
1. **Live Scanning**: Automatic scans triggered by benchmark runs
2. **Portfolio Optimization**: Use alignment scores for strategy weighting
3. **Regime Adaptation**: Adjust scanner parameters based on alignment patterns
4. **ML Features**: Use alignment data for machine learning model training

### Best Practices

#### Production Usage
```bash
# Scheduled benchmarking (cron job)
0 */6 * * * /path/to/cryptorun bench topgainers --progress json >> /var/log/cryptorun-bench.log

# Monitoring integration
cryptorun bench topgainers --progress json | jq -r '.overall_alignment' > /tmp/alignment_score.txt
```

#### Development Testing
```bash
# Quick validation
cryptorun bench topgainers --limit 5 --windows "1h" --progress plain

# Full integration test
cryptorun bench topgainers --limit 20 --ttl 300 --progress json > test_results.json
```

#### CI/CD Integration
```bash
# Automated quality gate
ALIGNMENT=$(cryptorun bench topgainers --progress json | jq -r '.overall_alignment')
if (( $(echo "$ALIGNMENT < 0.3" | bc -l) )); then
  echo "WARNING: Low alignment score: $ALIGNMENT"
fi
```

## Diagnostics (Top Gainers)

### Overview

The diagnostic analysis provides granular hit/miss attribution for benchmark results, explaining why alignment scores are at specific levels and identifying optimization opportunities.

### Generated Artifacts

When benchmark alignment requires analysis, run:
```bash
# Diagnostic analysis already generated for existing benchmark results
# Artifacts located at: out/bench/diagnostics/
```

#### Core Diagnostic Files
- **`bench_diag.json`**: Comprehensive JSON analysis with hit/miss attribution
- **`bench_diag.md`**: Human-readable diagnostic report with tables
- **`rank_corr.json`**: Detailed correlation statistics and significance testing
- **`gate_breakdown.json`**: Gate/guard failure analysis with optimization priorities
- **`miss_examples.jsonl`**: Line-delimited detailed case studies of missed signals

### Diagnostic Methodology

#### Hit/Miss Attribution
- **Primary Reasons**: Gate or guard that caused the miss
- **Secondary Reasons**: Contributing factors that compound the issue
- **Config Tweaks**: Specific configuration adjustments to recover missed signals
- **Impact Assessment**: Quantified gain/loss from each missed opportunity

#### Gate vs Guard Analysis
- **Guards**: Timing/fatigue/freshness protections (operational constraints)
- **Gates**: Quality thresholds for momentum/volume/ADX (technical requirements)
- **Pattern Recognition**: Identifies whether misses are due to conservative operations or poor signal quality

#### Correlation Deep-dive
- **Kendall Tau**: Rank correlation robustness measure
- **Spearman Rho**: Monotonic relationship strength
- **Statistical Significance**: P-value estimation with confidence intervals
- **Sample Size Validation**: Minimum sample requirements for statistical validity

### Sample Diagnostic Results

#### Gate Breakdown Summary
```json
{
  "primary_reasons": {
    "fatigue_guard": {"count": 1, "percentage": 25.0, "missed_gains": 42.8},
    "late_fill_guard": {"count": 1, "percentage": 25.0, "missed_gains": 38.4},
    "freshness_guard": {"count": 1, "percentage": 25.0, "missed_gains": 13.4},
    "score_gate": {"count": 1, "percentage": 25.0, "missed_gains": 11.8}
  },
  "guard_vs_gate_impact": {
    "guards_primary": 3,
    "gates_primary": 1,
    "interpretation": "Guards (75%) are main blockers, not scoring gates (25%)"
  }
}
```

#### Optimization Priorities
1. **Fatigue Guard** (Highest Impact): Increase 24h return threshold from 12% to 18%
2. **Late Fill Guard** (Low Risk): Extend max delay from 30s to 45s  
3. **Freshness Guard** (Medium Risk): Increase max age from 2 to 3 bars
4. **Score Gate** (High Risk): Consider reducing minimum score from 2.5 to 2.0

### Integration with Configuration

#### Regime-Dependent Adjustments
```yaml
# Suggested config adjustments based on diagnostic findings
momentum_config:
  Fatigue:
    Return24hThreshold: 18  # Increased from 12 for trending regimes
  LateFill:
    MaxDelaySeconds: 45     # Increased from 30 for operational tolerance
  Freshness:
    MaxBarsAge: 3          # Increased from 2 for signal persistence
```

#### Risk Assessment Matrix
| Component | Impact | Risk | Priority | Regime Dependency |
|-----------|--------|------|----------|-------------------|
| fatigue_guard | High (42.8%) | Medium | 1 | Trending-sensitive |
| late_fill_guard | High (38.4%) | Low | 2 | Infrastructure |
| freshness_guard | Medium (13.4%) | Medium | 3 | Timeframe-dependent |
| score_gate | Low (11.8%) | High | 4 | Quality-critical |

### Quality Assurance

#### Success Pattern Recognition
The diagnostics identify when the system works perfectly (e.g., BTC perfect performance):
- Strong momentum score (>4.0)
- Clean technical picture (no guard conflicts)  
- Perfect rank alignment (#1 gainer → #1 scan result)
- Demonstrates system capability when conditions align

#### Statistical Validation
- **Sample Size Requirements**: Minimum n≥20 for statistical significance
- **Confidence Intervals**: Correlation estimates with uncertainty bounds
- **P-Value Interpretation**: Significance testing for correlation coefficients
- **Power Analysis**: Required sample sizes for different effect sizes

This benchmark provides essential validation of CryptoRun's scanning effectiveness against real market performance, ensuring our signals remain relevant and profitable.