# CLI Documentation

## 🎯 GOVERNANCE: Menu is Canon

**THE INTERACTIVE MENU IS THE PRIMARY INTERFACE**  
CLI flags and subcommands are automation shims for non-interactive environments.

### Delivery Policy
- Any new feature MUST ship with a menu screen/panel 
- CLI subcommands MUST route to the same underlying functions as menu actions
- Menu owns parameter precedence: Profile defaults → Menu selections → CLI flags
- PRs without menu integration are rejected by CI

### TTY Detection & Routing
```bash
# Interactive terminal → launches menu automatically
cryptorun

# Non-TTY environment → shows guidance and exits
cryptorun  # (in scripts/CI)
# Output: ❌ Interactive menu requires a TTY terminal.
#         Use subcommands and flags for automation.
```

## Start Here → Menu

The interactive Menu is CryptoRun's canonical interface, providing guided navigation through all features with real-time progress indicators and explanatory output.

```bash
# Start the interactive menu (canonical UX)
cryptorun
# OR explicitly
cryptorun menu
```

The Menu provides:
- **Guided Workflows**: Step-by-step scanning, benchmarking, and analysis
- **Live Progress**: Visual indicators with ETA calculations and step attribution  
- **Feature Discovery**: Browse all capabilities without memorizing flags
- **Context-Aware Help**: Relevant documentation and examples for each section

## Non-TTY Fallback: CI/Automation

When running in non-interactive environments (CI, scripts, cron jobs), use direct flag commands:

```bash
# Automated scanning with structured output
cryptorun scan momentum --progress json --venues kraken --max-sample 50

# Automated scanning with regime awareness
cryptorun scan momentum --regime auto --show-weights --venues kraken

# Manual regime override for testing
cryptorun scan momentum --regime bull --explain-regime --venues kraken

# Benchmarking for validation pipelines  
cryptorun bench topgainers --dry-run --progress plain --windows 1h,24h

# QA suite for pre-commit checks
cryptorun qa --progress none --venues kraken --max-sample 10

# Post-merge verification (conformance + alignment + diagnostics)
cryptorun verify postmerge --windows 1h,24h --n 20 --progress
```

## UX MUST — Live Progress & Explainability

CryptoRun provides comprehensive visual progress indicators, step-by-step timing, and ETA calculations for all pipeline operations. Every command includes observable progress with complete step attribution and performance metrics.

## Progress Indicators System

### Visual Progress Elements

CryptoRun uses multiple visual elements to provide real-time feedback:

1. **Step Spinners**: Animated indicators showing active processing
2. **Progress Bars**: Compact bars showing completion percentage with N of M counters  
3. **ETA Calculations**: Dynamic time estimates based on current processing rate
4. **Step Timers**: Individual step durations and total pipeline timing

### Progress Modes

All commands support configurable progress output modes:

```bash
# Auto mode (default) - detects terminal capabilities and adjusts
cryptorun scan momentum --progress auto

# Plain mode - simple text output for scripts/logs
cryptorun scan momentum --progress plain

# JSON mode - structured progress events for automation
cryptorun scan momentum --progress json

# Quiet mode - minimal output, errors only
cryptorun scan momentum --progress none
```

## Pipeline Step Progression

### Standard Pipeline Steps

All CryptoRun scanning operations follow the same 8-step pipeline with individual timing:

1. **Universe** - Build symbol universe from config/universe.json
2. **Data Fetch** - Retrieve market data from exchanges with cache management
3. **Guards** - Apply safety guards (fatigue, freshness, late-fill)
4. **Factors** - Calculate momentum factors across timeframes  
5. **Orthogonalize** - Apply Gram-Schmidt orthogonalization with MomentumCore protection
6. **Score** - Generate composite scores with regime-adaptive weighting (see [Regime Tuner System](./REGIME_TUNER.md))
7. **Gates** - Apply entry gates (volume, spread, depth, ADX)
8. **Output** - Generate results files and explanations

### Visual Output Examples

#### Auto Mode (Terminal with Color Support)
```
⚡ CryptoRun Pipeline [████████████░░░░░░░░] 8/8 (100.0%) ETA: 0s
  ✅ Universe completed (20 symbols, 45ms)  
  🔄 Data Fetch [██████████████████░░] 18/20 (90.0%) ETA: 2s
  📊 Fetching BTCUSD market data - cache hit
```

#### Plain Mode (Scripts/Logs)
```
[INFO] CryptoRun Pipeline: Starting 8 steps
[INFO] Step 1/8: Universe (45ms) - 20 symbols loaded  
[INFO] Step 2/8: Data Fetch (2.3s) - 20/20 symbols processed
[INFO] Step 3/8: Guards (156ms) - 14/20 symbols passed
[INFO] Pipeline completed: 8 steps in 4.2s, 12 candidates
```

#### JSON Mode (Automation/Monitoring)
```json
{"phase":"step","step":"Data Fetch","progress":90,"total":20,"current":18,"eta_seconds":2,"message":"Fetching BTCUSD"}
{"phase":"complete","step":"Data Fetch","duration_ms":2300,"symbols_processed":20,"cache_hits":15,"cache_misses":5}
```

## Regime-Aware Scanning Commands

### Regime Control Flags

All scanning commands support regime-aware factor weighting with the following flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--regime` | `auto` | Market regime detection (auto\|bull\|chop\|highvol) |
| `--show-weights` | `false` | Display 5-way factor weight allocation table |
| `--explain-regime` | `false` | Show regime detection explanation and strategy |

#### Examples

**Automatic regime detection (recommended):**
```bash
cryptorun scan momentum --regime auto --show-weights
```

**Manual regime override for testing:**
```bash
# Force bull market weights
cryptorun scan momentum --regime bull --explain-regime

# Force choppy market weights  
cryptorun scan dip --regime chop --show-weights --venues kraken
```

**Regime analysis workflow:**
```bash
# Full regime visibility
cryptorun scan momentum \
  --regime auto \
  --show-weights \
  --explain-regime \
  --venues kraken,okx \
  --progress auto
```

### Regime Weight Display

When `--show-weights` is specified, the CLI shows the active 5-way factor allocation:

```
🎯 Active Weight Map (bull regime):
┌─────────────┬────────┬──────────────────────────────────┐
│ Factor      │ Weight │ Description                      │
├─────────────┼────────┼──────────────────────────────────┤
│ Momentum    │  50.0% │ Multi-timeframe momentum signals │
│ Technical   │  20.0% │ Chart patterns, RSI, indicators  │
│ Volume      │  15.0% │ Volume surge, OI, liquidity      │
│ Quality     │  10.0% │ Venue health, reserves, ETF      │
│ Catalyst    │   5.0% │ News events, funding divergence  │
└─────────────┴────────┴──────────────────────────────────┘
Social factor: +10 max (applied separately)
Total base allocation: 100.0% (excluding Social)
```

### Regime Detection Explanation

When `--explain-regime` is specified, the CLI shows detection reasoning:

```
💡 Regime Detection Explanation:
🔍 Detected: TRENDING BULL market
• 7d volatility: Low (≤30%)
• Above 20MA: High (≥65% of universe)
• Breadth thrust: Positive (≥15%)
• Strategy: Emphasize momentum (50%), relax guards
```

## Command-Specific Progress Features

### Scanning Commands

#### `cryptorun scan momentum`

Shows pipeline progression with momentum-specific details:

```bash
cryptorun scan momentum --venues kraken --max-sample 50 --progress auto
```

**Progress Output:**
```
⚡ Momentum Pipeline [████████░░░░░░░░░░░░] 4/8 (50.0%) ETA: 8s
  ✅ Universe: 50 symbols (125ms)
  ✅ Data Fetch: 50/50 symbols, 85% cache hit (2.1s)  
  ✅ Guards: 37/50 passed fatigue+freshness+late-fill (234ms)
  🔧 Factors: Computing 4h momentum [██████████░░░░░░░░░░] 37/50 ETA: 6s
```

#### `cryptorun scan dip`

Displays dip-specific progress with quality thresholds:

```bash
cryptorun scan dip --venues kraken --progress auto
```

**Progress Output:**
```
⚡ Quality-Dip Pipeline [██████████████░░░░░░] 6/8 (75.0%) ETA: 3s
  ✅ Guards: 28/35 passed trend+RSI+fibonacci (187ms)
  🔄 Score: Dip quality assessment [████████████████░░░░] 28/35 ETA: 2s
```

### Benchmark Commands  

#### `cryptorun bench topgainers`

Shows benchmark-specific progress with API rate limiting awareness:

```bash
cryptorun bench topgainers --windows 1h,24h --n 25 --progress auto
```

**Progress Output:**
```
🔄 Top Gainers Benchmark [████████████████░░░░] 4/5 (80.0%) ETA: 30s
  ✅ Init: Configuration validated (12ms)
  ✅ Fetch: 1h window (25 gainers, cache hit, 234ms)  
  ✅ Fetch: 24h window (25 gainers, API call, 1.2s)
  📊 Analyze: Computing alignment [████████████░░░░░░░░] 3/5 windows ETA: 25s
  Cache: TTL 300s, Rate limit: 8/10 rpm remaining
```

#### `cryptorun bench factorweights`

Side-by-side comparison between Legacy FactorWeights and Unified Composite scoring systems:

```bash
cryptorun bench factorweights --universe topN:30 --windows 1h,4h,12h,24h --n 20 --progress
```

**Progress Output:**
```
🧮 FactorWeights vs Unified Benchmark [██████████████████░░] 5/6 (83.3%) ETA: 45s
  ✅ Universe: Built topN:30 (28 eligible after guards, 234ms)
  ✅ Guards: Applied shared validation to both systems (456ms)
  ✅ Legacy: Computed FactorWeights scores (uncapped social, 1.2s)
  ✅ Unified: Computed Composite scores (capped social, orthogonal, 1.8s)
  📊 Metrics: Computing correlations [██████████████░░░░░░] 3/4 windows ETA: 10s
  📈 Returns: Fetching forward returns for hit rate analysis ETA: 35s
```

**Key Differences Tested:**
- **Legacy**: No orthogonalization, uncapped social factor, equal-weight timeframes
- **Unified**: MomentumCore protection, social capped at +10, regime-adaptive weights

**Outputs Generated:**
- `side_by_side.csv` - Per-asset scores, deltas, forward returns
- `results.jsonl` - Complete factor breakdowns with metadata
- `report.md` - Executive summary with disagreement analysis

### Explain Commands

#### `cryptorun explain delta`

Forensic analysis of factor contribution shifts with tolerance-based validation:

```bash
# Analyze top 30 universe against latest baseline
cryptorun explain delta --universe topN=30 --baseline latest

# Compare specific symbols against date baseline
cryptorun explain delta --universe BTCUSD,ETHUSD,SOLUSD --baseline 2025-01-01

# Full analysis with custom output directory
cryptorun explain delta --universe topN=50 --baseline latest --out ./forensics --progress
```

**Progress Output:**
```
🔍 Explain Delta — universe=topN=30 baseline=latest
📁 Output: C:\CryptoRun\artifacts\explain_delta

⏳ [10%] Loading baseline...
⏳ [20%] Parsed universe: 30 pairs
⏳ [40%] Loaded current factors for regime: bull
⏳ [60%] Loaded baseline from 2025-01-14T18:30Z
⏳ [80%] Completed delta analysis
⏳ [100%] Analysis complete

● Explain Delta — universe=topN=30 baseline=2025-01-14T18:30Z
  FAIL(2) WARN(5) OK(23) | regime=bull
  worst:
    1) ETH  momentum_core   +17.2 (>±15.0)  hint: momentum strength increased significantly
    2) SOL  composite_score +22.1 (>±20.0)  hint: overall score increased beyond expectations
  ✅ All factor contributions within tolerance

📁 Artifacts Generated:
   • Results JSONL: artifacts/explain_delta/results.jsonl
   • Summary MD: artifacts/explain_delta/summary.md
```

**Flags and Options:**

| Flag | Default | Description |
|------|---------|-------------|
| `--universe` | *required* | Universe spec (topN=X or symbol list) |
| `--baseline` | "latest" | Baseline to compare (latest/date/path) |
| `--out` | "artifacts/explain_delta" | Output directory for artifacts |
| `--progress` | true | Show progress indicators |

**Exit Codes for CI Integration:**
- **0**: All factors within tolerance (OK/WARN only)
- **1**: Critical factor shifts detected (FAIL status)
- **2**: Command/configuration error

**Forensic Methodology:**
1. **Universe Parsing**: Supports topN=X format or explicit symbol lists
2. **Baseline Loading**: Latest artifact, specific date, or file path
3. **Factor Generation**: Current explain() data for all universe symbols  
4. **Delta Calculation**: `current_factor - baseline_factor` per asset
5. **Tolerance Validation**: Regime-aware thresholds from `config/explain_tolerances.yaml`
6. **Artifact Generation**: JSONL results and markdown summary with worst offenders

**Tolerance Configuration:** Per-regime thresholds in `config/explain_tolerances.yaml`:
- **Bull regime**: Tight tolerances (momentum ±15.0, composite ±20.0)
- **Choppy regime**: Relaxed tolerances (momentum ±20.0, composite ±25.0)  
- **High-vol regime**: Maximum tolerances (momentum ±25.0, composite ±35.0)

For complete regime detection methodology, see [Regime Tuner System](./REGIME_TUNER.md).

### Utility Commands

#### Long-Running Operations

Commands with multiple iterations show compact progress bars:

```bash
cryptorun pairs sync --venue kraken --min-adv 100000
```

**Progress Output:**  
```
🔍 Pair Discovery [████████████████████] 142/142 (100.0%) - Filtering by ADV
  ✅ Discovered 142 pairs, 89 meet ADV threshold (4.5s)
  📝 Updated config/universe.json with 89 pairs
```

## Metrics Integration

### Prometheus Metrics Exposure

All progress steps emit Prometheus metrics accessible via `/metrics` endpoint, including regime-specific metrics:

```
# Step duration histograms
cryptorun_step_duration_seconds{step="universe",result="success"} 0.045
cryptorun_step_duration_seconds{step="data_fetch",result="success"} 2.300  
cryptorun_step_duration_seconds{step="guards",result="success"} 0.156

# Cache performance  
cryptorun_cache_hit_ratio 0.85
cryptorun_cache_hits_total{cache_type="market_data"} 45
cryptorun_cache_misses_total{cache_type="market_data"} 8

# WebSocket latency (P99)
cryptorun_ws_latency_p99_ms{exchange="kraken",endpoint="ticker"} 125.0

# Pipeline metrics
cryptorun_pipeline_steps_total{step="factors",status="success"} 23
cryptorun_pipeline_errors_total{step="gates",error_type="timeout"} 2
cryptorun_active_scans 3
cryptorun_scans_total 156

# Regime metrics
cryptorun_regime_switches_total{from_regime="chop",to_regime="bull"} 3
cryptorun_regime_duration_hours{regime="bull"} 18.5
cryptorun_active_regime 1.0  # 0=choppy, 1=bull, 2=highvol
cryptorun_regime_health{regime="bull",indicator="volatility_7d"} 0.25
```

### Regime Status Endpoint

The monitoring HTTP server provides a dedicated `/regime` endpoint with current regime information:

```bash
# Start monitoring server
cryptorun monitor --port 8080

# Query regime status
curl http://localhost:8080/regime
```

**Response format:**
```json
{
  "timestamp": "2025-01-15T14:30:00Z",
  "current_regime": "trending_bull",
  "regime_numeric": 1.0,
  "health": {
    "volatility_7d": 0.45,
    "above_ma_pct": 0.68,
    "breadth_thrust": 0.23,
    "stability_score": 0.85
  },
  "weights": {
    "momentum": 50.0,
    "technical": 20.0,
    "volume": 15.0,
    "quality": 10.0,
    "catalyst": 5.0
  },
  "switches_today": 2,
  "avg_duration_hours": 18.5
}
```

### Step Timer Integration

Every pipeline step automatically emits duration metrics:

```go
// Internal step timing (automatic)
stepTimer := metrics.StartStepTimer("factors")
// ... step execution ...
stepTimer.Stop("success") // or "error", "timeout", "skipped"
```

### Performance Monitoring

Commands expose performance data in multiple formats:

#### Compact Summary (Default)
```
✅ Momentum scan completed
Processed: 50 symbols | Candidates: 12 | Duration: 4.2s  
Steps: Universe(45ms) → Fetch(2.3s) → Guards(156ms) → Factors(847ms) → Score(234ms)
Results: out/scan/momentum/candidates.json
```

#### Detailed Timing (Verbose)
```bash
cryptorun scan momentum --verbose
```

```
Pipeline Timing Breakdown:
1. Universe    :   45ms (  1.1%) - Symbol universe built  
2. Data Fetch  : 2300ms ( 54.8%) - Market data retrieved, 85% cache hit
3. Guards      :  156ms (  3.7%) - Safety guards applied, 74% pass rate
4. Factors     :  847ms ( 20.2%) - 4-timeframe momentum calculated
5. Orthogonal  :   89ms (  2.1%) - Gram-Schmidt applied, MomentumCore protected  
6. Score       :  234ms (  5.6%) - Composite scores with bull regime weights
7. Gates       :  167ms (  4.0%) - Entry gates applied, 67% pass rate
8. Output      :  362ms (  8.6%) - Results and explanations generated

Total Pipeline: 4200ms (100.0%)
Throughput: 11.9 symbols/second
```

## Error Handling and Progress Recovery

### Failed Step Handling

When a pipeline step fails, progress indicators clearly show the failure point:

```
❌ CryptoRun Pipeline failed at step 4/8: Factors
  ✅ Universe: 50 symbols (125ms)
  ✅ Data Fetch: 50/50 symbols (2.1s)  
  ✅ Guards: 37/50 passed (234ms)
  ❌ Factors: API timeout after 30s - kraken websocket unresponsive

FAIL API_TIMEOUT + Check venue health at /health
```

### Partial Success Progress

Some commands support partial success with progress continuation:

```
⚠️  Pipeline completed with warnings (6/8 steps successful)
  ✅ 28/50 symbols processed successfully  
  ⚠️  12 symbols skipped due to stale data
  ⚠️  10 symbols failed factor calculation  
  📊 Results: 15 candidates from successful symbols
```

### Recovery Recommendations

Failed operations include actionable recovery steps:

```
❌ Data Fetch failed: rate limit exceeded
💡 Recommendations:
  - Increase --ttl to reduce API calls (current: 300s, suggest: 600s)  
  - Enable --cache-only mode for development
  - Check /metrics for current rate limit status
  - Retry in 2m 15s when rate limit resets
```

## Configuration and Customization

### Progress Style Configuration

Global progress preferences can be set via environment variables:

```bash
# Default progress mode for all commands
export CRYPTORUN_PROGRESS=auto

# Disable all progress indicators (CI/automation)
export CRYPTORUN_PROGRESS=none

# Force plain text mode (Docker logs)  
export CRYPTORUN_PROGRESS=plain
```

### Custom Progress Themes

Advanced users can customize spinner styles and progress bar characters:

```bash
# Pipeline-themed spinners (default)
cryptorun scan momentum --progress auto --spinner pipeline

# Classic dots spinner  
cryptorun scan momentum --progress auto --spinner dots

# Bouncing bar animation
cryptorun scan momentum --progress auto --spinner bounce
```

### Integration with External Tools

#### Shell Scripts
```bash
#!/bin/bash
# Monitor progress via exit codes and structured output
cryptorun scan momentum --progress json | jq -r '.progress'
echo "Scan result: $?" 
```

#### Monitoring Tools
```bash
# Prometheus metrics scraping
curl -s http://localhost:8080/metrics | grep cryptorun_step_duration

# Grafana dashboard queries  
rate(cryptorun_step_duration_seconds_sum[5m]) / rate(cryptorun_step_duration_seconds_count[5m])
```

This comprehensive progress system ensures complete visibility into CryptoRun operations while supporting both interactive use and automated integration.

## Verification Commands

### `cryptorun verify postmerge`

Comprehensive post-merge verification combining conformance testing, topgainers alignment, and diagnostics policy validation.

```bash
# Complete verification with default settings
cryptorun verify postmerge

# Custom time windows and sample size
cryptorun verify postmerge --windows 1h,24h --n 20 --progress

# Silent mode for CI/automation
cryptorun verify postmerge --windows 24h --n 50 --progress none
```

**Progress Output:**
```
🔍 CryptoRun Post-Merge Verification
====================================
⏳ Starting verification process...
📋 Step 1/3: Running conformance suite...
📊 Step 2/3: Running topgainers alignment...  
🩺 Step 3/3: Checking diagnostics policy...

📊 CONFORMANCE CONTRACTS
┌─────────────────────────────┬────────┐
│ Contract                    │ Status │
├─────────────────────────────┼────────┤
│ Single Scoring Path         │ ✅ PASS │
│ Weight Normalization        │ ✅ PASS │
│ Social Hard Cap             │ ✅ PASS │
│ Menu-CLI Alignment          │ ✅ PASS │
└─────────────────────────────┴────────┘

📈 TOPGAINERS ALIGNMENT
┌────────┬─────────┬──────┬──────┬──────┬─────────┐
│ Window │ Jaccard │   τ  │   ρ  │ MAE  │ Overlap │
├────────┼─────────┼──────┼──────┼──────┼─────────┤
│   1H   │  0.342  │ 0.287│ 0.453│ 3.21 │  12/25  │
│  24H   │  0.456  │ 0.398│ 0.567│ 2.89 │  18/32  │
└────────┴─────────┴──────┴──────┴──────┴─────────┘

🩺 DIAGNOSTICS POLICY: ✅ spec_pnl_pct basis confirmed

📁 Artifacts:
   Report: out/verify/postmerge_20250906_143052.md
   Data:   out/verify/postmerge_20250906_143052.json
   Bench:  out/bench/topgainers_1h.json, out/bench/topgainers_24h.json

✅ Verification PASSED - ready for deployment
```

**Exit Codes:**
- `0`: All verification steps passed
- `1`: One or more verification steps failed

**Flags:**
- `--windows`: Time windows for alignment check (default: `1h,24h`)
- `--n`: Minimum sample size for recommendations (default: `20`)
- `--progress`: Show progress indicators (default: `false`)

**Artifacts Generated:**
- **Report**: `out/verify/postmerge_{timestamp}.md` - Human-readable summary
- **Data**: `out/verify/postmerge_{timestamp}.json` - Machine-readable results  
- **Benchmarks**: `out/bench/topgainers_{window}.json` - Per-window alignment data

## Menu System - Guards Interface

### Guard Status & Results Screen

The CryptoRun menu system provides comprehensive guard status viewing with real-time evaluation and detailed explanations. Access via:

```bash
cryptorun menu
# → Select "2. 📊 Scan & Generate Candidates"
# → Select "3. 🛡️ View Guard Status & Results"
```

### Guard Status Display

The guard status screen shows a comprehensive overview of all candidates and their guard evaluation results:

```
═══════════════════════════════════════════════════════════════════════════════
                           🛡️ GUARD STATUS & RESULTS
═══════════════════════════════════════════════════════════════════════════════

Current Regime: normal | Active Guards: 8 types | Last Update: 2025-01-15 12:00:00

🛡️ Guard Evaluation Results
┌──────────┬────────┬─────────────┬──────────────────────────────────────┐
│ Symbol   │ Status │ Failed Guard│ Reason                               │
├──────────┼────────┼─────────────┼──────────────────────────────────────┤
│ BTCUSD   │ ✅ PASS │ -           │ -                                    │
│ ETHUSD   │ ❌ FAIL │ fatigue     │ 24h momentum 17.0% > 15.0% + RSI... │
│ SOLUSD   │ ✅ PASS │ -           │ -                                    │
│ ADAUSD   │ ❌ FAIL │ freshness   │ Bar age 3 > 2 bars maximum          │
│ DOTUSD   │ ✅ PASS │ -           │ -                                    │
│ LINKUSD  │ ❌ FAIL │ spread      │ Spread 75.0 bps > 50.0 bps limit    │
│ AVAXUSD  │ ✅ PASS │ -           │ -                                    │
│ MATICUSD │ ❌ FAIL │ social_cap  │ Social score 12.0 exceeds 10.0 cap  │
└──────────┴────────┴─────────────┴──────────────────────────────────────┘

Summary: 4 passed, 4 failed (exit code 1)

Progress Log:
⏳ Starting guard evaluation (regime: normal)
📊 Processing 8 candidates
🛡️ [20%] Evaluating freshness guards...
🛡️ [40%] Evaluating fatigue guards...
🛡️ [60%] Evaluating liquidity guards...
🛡️ [80%] Evaluating caps guards...
🛡️ [100%] Evaluating final guards...
✅ Guard evaluation completed

Options:
1. 🔍 View Detailed Guard Reasons
2. ⚙️ Quick Threshold Adjustments  
3. 📋 Export Guard Results
4. 🔄 Re-run Guard Evaluation
5. 🏠 Back to Scan Menu

Select option (1-5): _
```

### Detailed Guard Reasons View

Selecting "1. 🔍 View Detailed Guard Reasons" shows comprehensive failure analysis:

```
═══════════════════════════════════════════════════════════════════════════════
                         🔍 DETAILED GUARD FAILURE ANALYSIS
═══════════════════════════════════════════════════════════════════════════════

❌ ETHUSD - Fatigue Guard Failure
┌─────────────────────┬──────────────────────────────────────────────────────┐
│ Failed Guard        │ fatigue                                              │
│ Reason              │ 24h momentum 17.0% > 15.0% + RSI4h 75.0 > 70.0     │
│ Fix Hint            │ Wait for momentum cooldown or RSI retreat            │
│ Current Values      │ Momentum: 17.0%, RSI: 75.0                         │
│ Regime Thresholds   │ Momentum: 15.0% (volatile), RSI: 70.0              │
│ Next Check          │ In 2 hours (next regime detection)                  │
└─────────────────────┴──────────────────────────────────────────────────────┘

❌ ADAUSD - Freshness Guard Failure
┌─────────────────────┬──────────────────────────────────────────────────────┐
│ Failed Guard        │ freshness                                            │
│ Reason              │ Bar age 3 > 2 bars maximum                          │
│ Fix Hint            │ Wait for fresh data or increase bar age tolerance   │
│ Current Values      │ Bar Age: 3, ATR Move: 0.8×                         │
│ Regime Thresholds   │ Max Age: 2 bars (volatile), ATR: 1.0×              │
│ Data Source         │ Kraken WebSocket (last update: 11:54:00)           │
└─────────────────────┴──────────────────────────────────────────────────────┘

❌ LINKUSD - Liquidity Guard Failure
┌─────────────────────┬──────────────────────────────────────────────────────┐
│ Failed Guard        │ spread                                               │
│ Reason              │ Spread 75.0 bps > 50.0 bps limit                   │
│ Fix Hint            │ Wait for tighter spread or increase spread tolerance │
│ Current Values      │ Spread: 75.0 bps, Depth: $120k, VADR: 1.8×        │
│ Static Thresholds   │ Max Spread: 50.0 bps, Min Depth: $100k, VADR: 1.75×│
│ Order Book Status   │ Bid: $14.245, Ask: $14.256, Mid: $14.251           │
└─────────────────────┴──────────────────────────────────────────────────────┘

Press any key to continue...
```

### Quick Threshold Adjustments

The menu provides quick threshold adjustment options for testing scenarios:

```
═══════════════════════════════════════════════════════════════════════════════
                          ⚙️ QUICK THRESHOLD ADJUSTMENTS
═══════════════════════════════════════════════════════════════════════════════

Current Regime: normal

🔧 Adjustment Options:
1. 📉 Tighten Guards (reduce all thresholds by 20%)
2. 📈 Relax Guards (increase all thresholds by 20%)  
3. 🎯 Reset to Config Defaults
4. 🔄 Switch Regime (for testing)
5. 🏠 Back to Guard Status

WARNING: These are temporary adjustments for analysis only.
Production settings should be configured via config/quality_policies.json

Current Thresholds (Normal Regime):
┌────────────────┬─────────────────┬─────────────────┬─────────────────┐
│ Guard Type     │ Current         │ After Tighten   │ After Relax     │
├────────────────┼─────────────────┼─────────────────┼─────────────────┤
│ Fatigue        │ 12.0% momentum  │ 9.6% momentum   │ 14.4% momentum  │
│ Freshness      │ 2 bars max age  │ 1.6 bars (→2)   │ 2.4 bars (→3)   │
│ Spread         │ 50.0 bps        │ 40.0 bps        │ 60.0 bps        │
│ Social Cap     │ 10.0 points     │ 8.0 points      │ 12.0 points     │
└────────────────┴─────────────────┴─────────────────┴─────────────────┘

Select adjustment (1-5): _
```

### Guard Results Export

Export functionality provides structured data for further analysis:

```
═══════════════════════════════════════════════════════════════════════════════
                            📋 EXPORT GUARD RESULTS
═══════════════════════════════════════════════════════════════════════════════

Export Options:
1. 📄 JSON Format (structured data)
2. 📊 CSV Format (spreadsheet compatible)  
3. 📝 Markdown Report (documentation)
4. 🔍 Debug Format (full evaluation context)
5. 🏠 Back to Guard Status

Select export format (1-5): 1

📁 Exporting to: out/guards/guard_results_20250115_120000.json

✅ Export completed successfully!

File Contents Preview:
{
  "timestamp": "2025-01-15T12:00:00Z",
  "regime": "normal",
  "summary": {
    "total_candidates": 8,
    "passed": 4,
    "failed": 4,
    "exit_code": 1
  },
  "results": [
    {
      "symbol": "ETHUSD",
      "status": "FAIL",
      "failed_guard": "fatigue",
      "reason": "24h momentum 17.0% > 15.0% + RSI4h 75.0 > 70.0",
      "fix_hint": "Wait for momentum cooldown or RSI retreat"
    }
  ]
}

Press any key to continue...
```

This comprehensive guard interface provides complete visibility into the guard evaluation process with actionable insights and clear explanations for every decision.

### Guard Testing & Validation

CryptoRun includes comprehensive end-to-end testing for the guard system with seeded fixtures and golden file validation:

**Test Coverage:**
```bash
# Run all guard tests (target: <5s execution)
go test ./... -run Guards -count=1

# Expected output:
# PASS: TestFatigueGuardCalmRegime (0.02s)
# PASS: TestFreshnessGuardNormalRegime (0.03s)  
# PASS: TestLiquidityGuards (0.04s)
# PASS: TestMenuGuardStatusDisplay (0.05s)
# PASS: TestMenuGuardProgressBreadcrumbs (0.02s)
# ok    internal/application/guards/e2e    0.16s
# ok    internal/application/menu/e2e      0.18s
```

**Seeded Test Data:**
- `testdata/guards/fatigue_calm.json` - Tests 10% momentum limit in calm regime
- `testdata/guards/freshness_normal.json` - Tests 2-bar age limit, 1.2×ATR movement  
- `testdata/guards/liquidity_gates.json` - Tests 50bps spread, $100k depth, 1.75× VADR
- `testdata/guards/social_caps.json` - Tests volatile regime caps (social 8.0, brand 5.0)

**Golden File Validation:**
Menu progress indicators and table outputs are validated against golden files:
```
🛡️ Guard Results (calm regime) - 2025-01-15T12:00:00
┌──────────┬────────┬─────────────┬──────────────────────────────────────┐
│ Symbol   │ Status │ Failed Guard│ Reason                               │
├──────────┼────────┼─────────────┼──────────────────────────────────────┤
│ BTCUSD   │ ✅ PASS │ -           │ -                                    │
│ ETHUSD   │ ❌ FAIL │ fatigue     │ 24h momentum 12.0% > 10.0% + RSI... │
│ SOLUSD   │ ✅ PASS │ -           │ -                                    │
└──────────┴────────┴─────────────┴──────────────────────────────────────┘
Summary: 2 passed, 1 failed
```

**Menu UX Testing:**
The `internal/application/menu/e2e/` suite validates:
- Progress breadcrumb display during guard evaluation
- Detailed failure reason formatting with fix hints
- Quick threshold adjustment interface
- Exit code communication (exit code 1 for hard failures)

## Microstructure Validation Commands

### `cryptorun validate microstructure`

Exchange-native L1/L2 orderbook validation to ensure asset eligibility for position sizing. Validates spread, depth, and VADR requirements using venue-native APIs only.

```bash
# Validate single asset across all venues
cryptorun validate microstructure BTCUSDT

# Validate multiple assets
cryptorun validate microstructure BTCUSDT,ETHUSDT,SOLUSDT

# Validate with custom thresholds
cryptorun validate microstructure BTCUSDT --max-spread 75 --min-depth 75000 --min-vadr 1.5

# Generate audit report for trading universe
cryptorun validate microstructure --universe --audit-report
```

**Progress Output:**
```
🔍 Microstructure Validation: BTCUSDT
═════════════════════════════════════

[33%] Checking binance orderbook...
   ✅ binance: Spread 35.2bps, Depth $150k, VADR 2.10x

[67%] Checking okx orderbook...  
   ✅ okx: Spread 42.1bps, Depth $120k, VADR 1.85x

[100%] Checking coinbase orderbook...
   ❌ coinbase: Spread 65.0bps, Depth $85k, VADR 1.90x
      ❌ Spread 65.0bps > 50.0bps limit
      ❌ Depth $85k < $100k limit

📊 BTCUSDT Validation Results:
✅ ELIGIBLE - Passed on 2/3 venues: [binance, okx]
📁 Proof bundle: ./artifacts/proofs/2025-01-15/microstructure/BTCUSDT_master_proof.json

⏳ Generating proof bundle...
✅ Validation completed with proof artifacts
```

**Flags:**
- `--max-spread`: Maximum spread in basis points (default: `50.0`)
- `--min-depth`: Minimum depth in USD within ±2% (default: `100000`)
- `--min-vadr`: Minimum VADR threshold (default: `1.75`)
- `--venues`: Comma-separated venue list (default: `binance,okx,coinbase`)
- `--require-all-venues`: Require all venues to pass (default: `false`)
- `--universe`: Validate entire trading universe from config
- `--audit-report`: Generate comprehensive audit report
- `--artifacts-dir`: Proof bundle directory (default: `./artifacts`)

**Exit Codes:**
- `0`: All validated assets meet microstructure requirements
- `1`: One or more assets failed validation
- `2`: API/network errors preventing validation

### Menu System - Microstructure Interface

Access comprehensive microstructure validation through the menu system:

```bash
cryptorun menu
# → Select "11. ⚙️ Settings"
# → Select "5. 🏛️ Microstructure Validation"
```

#### Single Asset Validation Screen

```
╔════════ MICROSTRUCTURE VALIDATION ════════╗

Exchange-native L1/L2 validation for trading pairs:
• Spread < 50 bps requirement
• Depth ≥ $100k within ±2% requirement  
• VADR ≥ 1.75× requirement
• Point-in-time proof generation

 1. 🔍 Check Asset Eligibility (Single)
 2. 📊 Check Multiple Assets
 3. 📁 View Generated Proofs  
 4. 🏭 View Venue Statistics
 5. 📈 Run Audit Report
 6. ⚙️ Configure Thresholds
 0. ← Back to Settings

Enter choice: 1

Enter trading pair (e.g., BTCUSDT): BTCUSDT

🔍 Checking microstructure eligibility for BTCUSDT...

[33%] Checking binance...
   ✅ binance: Spread 35.1fbps, Depth $150k, VADR 2.10x

[67%] Checking okx...
   ✅ okx: Spread 45.2bps, Depth $120k, VADR 1.90x

[100%] Checking coinbase...
   ❌ coinbase: Spread 55.0bps, Depth $90k, VADR 1.70x
      ❌ Spread 55.0bps > 50.0bps limit
      ❌ Depth $90k < $100k limit
      ❌ VADR 1.70x < 1.75x limit

📊 Summary for BTCUSDT:
✅ ELIGIBLE - Passed on 2 venue(s): [binance, okx]
📁 Proof bundle generated: ./artifacts/proofs/2025-01-15/microstructure/BTCUSDT_master_proof.json

Press Enter to continue...
```

#### Batch Validation Screen

```
Enter symbols (comma-separated, e.g., BTCUSDT,ETHUSDT,SOLUSDT): BTCUSDT,ETHUSDT,SOLUSDT

🔍 Checking 3 assets across venues...

[33%] Processing BTCUSDT...
   ✅ BTCUSDT: ELIGIBLE on 2/3 venues

[67%] Processing ETHUSDT...
   ✅ ETHUSDT: ELIGIBLE on 3/3 venues

[100%] Processing SOLUSDT...
   ❌ SOLUSDT: NOT ELIGIBLE (spread violations)

📊 Batch Results:
   Total Assets: 3
   Eligible: 2 (66.7%)
   Not Eligible: 1
📁 Audit report: ./artifacts/proofs/2025-01-15/reports/microstructure_audit_143052.json

Press Enter to continue...
```

#### Proof Browsing Interface

```
📁 Generated Proof Bundles:
=====================================

1. ✅ BTCUSDT (2025-01-15) - 3 venues
   📄 ./artifacts/proofs/2025-01-15/microstructure/BTCUSDT_master_proof.json

2. ✅ ETHUSDT (2025-01-15) - 2 venues  
   📄 ./artifacts/proofs/2025-01-15/microstructure/ETHUSDT_master_proof.json

3. ❌ SOLUSDT (2025-01-15) - 0 venues
   📄 ./artifacts/proofs/2025-01-15/microstructure/SOLUSDT_master_proof.json

4. ✅ ADAUSDT (2025-01-14) - 1 venue
   📄 ./artifacts/proofs/2025-01-14/microstructure/ADAUSDT_master_proof.json

🔍 Actions:
 1. Open Proof Directory
 2. View Specific Proof
 0. Back

Enter choice: 2
Enter symbol to view: BTCUSDT

📂 Opening: ./artifacts/proofs/2025-01-15/microstructure/BTCUSDT_master_proof.json
✅ File opened in default application
```

#### Venue Statistics Screen

```
🏭 Venue Statistics:
=====================================

Binance:
  Checked: 25 assets
  Passed: 20 (80.0%)
  Avg Spread: 42.3 bps
  Avg Depth: $185,000

OKX:
  Checked: 25 assets
  Passed: 18 (72.0%)
  Avg Spread: 48.7 bps
  Avg Depth: $142,000

Coinbase:
  Checked: 25 assets
  Passed: 15 (60.0%)
  Avg Spread: 52.1 bps
  Avg Depth: $105,000

Press Enter to continue...
```

#### Comprehensive Audit Report

```
📈 Running comprehensive microstructure audit...

[14%] Loading trading universe...
[29%] Fetching orderbook data from venues...
[43%] Validating spread requirements...
[57%] Checking depth requirements...
[71%] Calculating VADR metrics...
[86%] Generating proof bundles...
[100%] Creating audit report...

📊 Audit Completed:
   Total Assets: 50
   Eligible: 35 (70%)
   Not Eligible: 15 (30%)
   Top Blocker: Spread violations (60%)
📁 Report: ./artifacts/proofs/2025-01-15/reports/microstructure_audit_143215.json

Press Enter to continue...
```

#### Threshold Configuration Screen

```
⚙️ Microstructure Threshold Configuration:

Current Requirements:
• Max Spread: 50.0 bps
• Min Depth: $100,000 (±2%)
• Min VADR: 1.75×

Adjustments:
 1. Relax Spread Limit (50 → 75 bps)
 2. Lower Depth Requirement ($100k → $75k)
 3. Reduce VADR Requirement (1.75× → 1.50×)
 4. View Venue-Specific Overrides
 0. Back

Enter choice: 1
✅ Spread limit relaxed to 75.0 bps
💾 Thresholds updated - next validation will use new settings

Press Enter to continue...
```

### Proof Bundle Structure

Generated proof bundles provide comprehensive audit trails:

```json
{
  "asset_symbol": "BTCUSDT",
  "timestamp_mono": "2025-01-15T14:32:15Z",
  "proven_valid": true,
  "eligible_venues": ["binance", "okx"],
  "venue_proofs": {
    "binance": {
      "spread_proof": {
        "metric": "spread_bps",
        "actual_value": 35.2,
        "required_value": 50.0,
        "operator": "<",
        "passed": true,
        "evidence": "Spread 35.2 bps meets required max 50.0 bps"
      },
      "depth_proof": {
        "metric": "depth_usd_plus_minus_2pct",
        "actual_value": 150000,
        "required_value": 100000,
        "operator": ">=",
        "passed": true,
        "evidence": "Depth $150,000 meets required min $100,000 within ±2%"
      },
      "vadr_proof": {
        "metric": "vadr",
        "actual_value": 2.1,
        "required_value": 1.75,
        "operator": ">=", 
        "passed": true,
        "evidence": "VADR 2.10x meets required min 1.75x"
      }
    }
  },
  "order_book_snapshots": { /* Full L1/L2 data */ },
  "generated_at": "2025-01-15T14:32:16Z",
  "proof_version": "1.0"
}
```

### Integration with Scanning Pipeline

Microstructure validation integrates seamlessly with the main scanning pipeline:

```bash
# Enable microstructure gate for scans
cryptorun scan momentum --gates microstructure --venues binance,okx

# Progress shows microstructure validation step
⚡ Momentum Pipeline [██████████████████░░] 7/8 (87.5%) ETA: 2s
  ✅ Universe: 50 symbols (125ms)
  ✅ Data Fetch: 50/50 symbols (2.1s)  
  ✅ Guards: 37/50 passed (234ms)
  ✅ Factors: 4-timeframe momentum (847ms)
  ✅ Score: Bull regime weights (189ms)
  ✅ Microstructure: 28/37 eligible (1.2s)  # New validation step
  🔄 Gates: Entry gates [████████████░░░░░░░░] 28/37 ETA: 1s
```

This microstructure validation system ensures CryptoRun only processes assets with sufficient exchange-native liquidity while providing comprehensive audit trails and transparent validation results.

## Backtest Commands

### `cryptorun backtest smoke90`

Comprehensive 90-day cache-only validation backtest that tests the unified scanner end-to-end using historical data. Validates the complete trading pipeline from candidate selection through guard evaluation, microstructure checks, and provider operations.

```bash
# Basic smoke90 backtest
cryptorun backtest smoke90

# Custom configuration
cryptorun backtest smoke90 \
  --top-n 30 \
  --stride 6h \
  --hold 48h \
  --output ./my-backtest

# Quick test run
cryptorun backtest smoke90 \
  --top-n 5 \
  --stride 1h \
  --hold 2h
```

**Progress Output:**
```
🔍 Running smoke90 backtest (90-day cache-only validation)...
   Configuration: TopN=20, Stride=4h, Hold=24h
   Output: out/backtest
   Use Cache Only: true

🔥 Starting Smoke90 backtest (cache-only)
📅 Period: 2024-10-17 to 2025-01-15 (90 days)
⚙️  Config: TopN=20, Stride=4h, Hold=24h

⏳ [25.0%] Processing window 135/540 (2024-12-15 08:00)
⏳ [50.0%] Processing window 270/540 (2024-11-25 12:00)
⏳ [75.0%] Processing window 405/540 (2024-11-05 16:00)

✅ Smoke90 backtest completed successfully!

📊 Summary:
   • Coverage: 512/540 windows (94.8%)
   • Candidates: 8,420 total
   • Pass Rate: 76.3% (6,428 passed, 1,992 failed)
   • Errors: 24

📈 TopGainers Alignment:
   • 1h Hit Rate: 68.5% (137/200)
   • 24h Hit Rate: 72.1% (144/200)
   • 7d Hit Rate: 81.2% (162/200)

⚡ P99 Relaxation Events:
   • Total: 15 (0.18 per 100 signals)
   • Average P99: 425.3 ms, Grace: 25.8 ms

🚦 Provider Throttling:
   • Total: 12 (0.14 per 100 signals)
   • Most Throttled: binance

📁 Artifacts Generated:
   • Results JSONL: out/backtest/2025-01-15/results.jsonl (2.1 MB)
   • Report MD: out/backtest/2025-01-15/report.md (156.3 kB)
   • Output Directory: out/backtest/2025-01-15
```

**Flags:**
- `--top-n`: Top N candidates per window (default: `20`)
- `--stride`: Time stride between windows (default: `4h`)
- `--hold`: Hold period for P&L calculation (default: `24h`)
- `--output`: Output directory for results (default: `out/backtest`)
- `--use-cache`: Use cached data only, no live fetches (default: `true`)
- `--progress`: Progress output mode (default: `auto`)

**Exit Codes:**
- `0`: Backtest completed successfully
- `1`: Backtest failed due to configuration or runtime errors

### Menu System - Backtest Interface

Access comprehensive backtesting through the menu system:

```bash
cryptorun menu
# → Select "3. 🔬 Backtest - Historical Validation"
```

#### Backtest Menu Screen

```
╔══════════ BACKTEST MENU ══════════╗

Historical validation against cached data:
• Cache-only operation (no live fetches)
• Comprehensive guard & gate testing
• Provider throttling simulation
• TopGainers alignment analysis

 1. 🔥 Run Smoke90 (90-day validation)
 2. 📊 View Last Backtest Results
 3. 📁 Open Backtest Directory
 4. ⚙️  Configure Backtest Settings
 0. ← Back to Main Menu

Enter choice: 1

🔥 Running Smoke90 backtest (90-day cache-only validation)...
   Configuration: TopN=20, Stride=4h, Hold=24h
   Output: out/backtest
   Use Cache Only: true

✅ Smoke90 backtest completed via unified function
📄 View results in next menu option
```

#### Results Viewer Screen

```
📊 Last Backtest Results (Smoke90):
=====================================

✅ Smoke90 Backtest Summary:
• Period: 90 days (cache-only validation)
• Coverage: 512/540 windows processed (94.8%)
• Candidates: 8,420 total analyzed
• Pass Rate: 76.3% (6,428 passed, 1,992 failed)
• Errors: 24 (cache misses and timeouts)

📈 TopGainers Alignment:
• 1h Hit Rate: 68.5% (137/200)
• 24h Hit Rate: 72.1% (144/200)
• 7d Hit Rate: 81.2% (162/200)

🛡️  Guard Performance:
• Freshness: 92.1% pass rate
• Fatigue: 83.7% pass rate
• Late-fill: 89.4% pass rate (15 P99 relaxations)

🚦 Provider Throttling:
• Total Events: 12 (0.14 per 100 signals)
• Most Throttled: binance (7 events)

📁 Artifacts:
• Results JSONL: out/backtest/2025-01-15/results.jsonl
• Report MD: out/backtest/2025-01-15/report.md
• Summary JSON: out/backtest/2025-01-15/summary.json

Actions:
 1. 📄 Open Report (Markdown)
 2. 📋 Open Results (JSONL)
 3. 🔍 View Raw Summary JSON
 0. Back
```

#### Configuration Screen

```
⚙️  Backtest Configuration:

Current Default Settings:
• TopN: 20 candidates per window
• Stride: 4h between windows
• Hold: 24h P&L calculation period
• Output: out/backtest
• Cache-Only: true (no live fetches)

Quick Adjustments:
 1. Increase Sample Size (20 → 30 candidates)
 2. Faster Stride (4h → 2h windows)
 3. Longer Hold (24h → 48h period)
 4. Change Output Directory
 5. View Advanced Settings
 0. Back

Enter choice: 1
✅ Sample size increased to 30 candidates per window
💾 Settings saved for next backtest run
```

### Backtest Methodology

The Smoke90 backtest validates the complete CryptoRun trading pipeline using the following methodology:

#### Pipeline Validation
1. **Unified Scoring**: Candidates must achieve Score ≥ 75
2. **Hard Gates**: VADR ≥ 1.8× and funding divergence requirements
3. **Guards Pipeline**: Freshness, fatigue, and late-fill guards with P99 relaxation
4. **Microstructure Validation**: Spread/depth/VADR proofs across venues
5. **Provider Operations**: Rate limiting and circuit breaker simulation
6. **Cache-Only**: Zero live fetches, explicit SKIP reasons for gaps

#### Key Features
- **Cache-Only Operation**: No live API calls, uses historical/cached data only
- **Explicit SKIP Reasons**: Clear explanations when data unavailable
- **Guard Attribution**: Detailed pass/fail tracking with specific reasons
- **P99 Relaxation**: Late-fill guard relaxation simulation under high latency
- **TopGainers Alignment**: Hit/miss rate analysis against market references
- **Provider Simulation**: Throttling and circuit breaker event modeling

#### Output Artifacts

**Directory Structure:**
```
out/backtest/
├── 2025-01-15/           # Date-stamped results
│   ├── results.jsonl     # Complete window results
│   ├── report.md         # Comprehensive markdown report
│   └── summary.json      # Compact summary statistics
└── latest -> 2025-01-15/ # Symlink to most recent
```

**Results JSONL Format:**
Each line contains a complete window result with candidate details, guard results, microstructure validation, throttling events, and P99 relaxation data.

**Markdown Report Contents:**
- Executive Summary with coverage and pass rates
- TopGainers Alignment analysis by timeframe
- Guard Attribution with pass/fail statistics
- P99 Relaxation Events frequency and impact
- Provider Throttling breakdown by venue
- Skip Analysis with reasons for missing data
- Performance Analysis and methodology explanation

### Integration with Testing

The Smoke90 backtest integrates with CryptoRun's testing infrastructure:

```bash
# Run smoke90 tests
go test ./internal/backtest/smoke90 -v

# Integration tests
go test ./tests/integration -run TestSmoke90 -v

# CLI integration validation
cryptorun backtest smoke90 --top-n 5 --stride 1h --hold 2h
```

This comprehensive backtest system provides end-to-end validation of CryptoRun's trading pipeline using reproducible cache-only data while maintaining realistic market conditions and comprehensive reporting.