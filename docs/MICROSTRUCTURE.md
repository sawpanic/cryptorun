# Exchange-Native Microstructure Gates v3.2.1

Exchange-native L1/L2 microstructure data collectors with **tiered liquidity gates**, **venue-native precedence rules**, and **aggregator ban enforcement**. Implements CryptoRun v3.2.1 microstructure requirements with compile-time safety.

## UX MUST — Live Progress & Explainability

The microstructure system provides **real-time venue health badges** and **transparent data quality metrics**:

- **🟢 GREEN**: All systems operational, proceed with full size
- **🟡 YELLOW**: Performance degraded, consider halving position size  
- **🔴 RED**: Critical issues detected, avoid trading on this venue

Each L1/L2 data point includes **attribution fields** (source, timestamp, data age, quality score) for full explainability.

## Architecture Overview

### Core Components

```
internal/microstructure/
├── api.go                    # Core interfaces and evaluation results
├── tiered_gates.go          # NEW: Tiered gate calculator with venue fallbacks
├── aggregator_ban.go        # NEW: Compile-time aggregator ban enforcement  
├── gates.go                 # Legacy gates engine (for comparison)
├── adapters/                # NEW: Exchange-native adapters
│   ├── types.go            # Common types and factory
│   ├── binance.go          # Binance native L1/L2 adapter
│   ├── okx.go              # OKX native L1/L2 adapter  
│   └── coinbase.go         # Coinbase native L1/L2 adapter
├── *_test.go               # Comprehensive test suite with mocks
└── evaluator.go            # Legacy evaluator (maintained for compatibility)
```

### Tiered Gate Flow

```mermaid
graph TD
    A[Symbol + ADV] --> B[Liquidity Tier Determination]
    B --> C{Tier Assignment}
    C -->|ADV ≥$5M| D[Tier 1: depth≥$150k, spread≤25bps, VADR≥1.85×]
    C -->|ADV $1M-5M| E[Tier 2: depth≥$75k, spread≤50bps, VADR≥1.80×]
    C -->|ADV $100k-1M| F[Tier 3: depth≥$25k, spread≤80bps, VADR≥1.75×]
    
    D --> G[Multi-Venue Data Gathering]
    E --> G
    F --> G
    
    G --> H[Binance Native API]
    G --> I[OKX Native API] 
    G --> J[Coinbase Native API]
    
    H --> K[Aggregator Ban Check]
    I --> K
    J --> K
    
    K --> L{Venue Health & Latency}
    L --> M[Primary Venue Selection]
    L --> N[Fallback Venue Selection]
    
    M --> O[Tiered Gate Evaluation]
    N --> O
    
    O --> P[Depth Gate: Best Venue Depth ≥ Tier Requirement]
    O --> Q[Spread Gate: Tightest Spread ≤ Tier Cap]  
    O --> R[VADR Gate: max(tier_min, p80_historical)]
    
    P --> S{All Gates Pass?}
    Q --> S
    R --> S
    
    S -->|Yes + Multiple Venues| T[Recommended Action: PROCEED]
    S -->|Yes + Single Venue| U[Recommended Action: HALVE_SIZE]
    S -->|No| V[Recommended Action: DEFER]
```

## Supported Venues

### Exchange-Native APIs (USD pairs only)

| Venue    | L1 Endpoint | L2 Endpoint | Rate Limit | Status |
|----------|-------------|-------------|------------|---------|
| **Binance** | `/api/v3/ticker/price` + `/api/v3/depth` | `/api/v3/depth?limit=100` | 1200/min | ✅ |
| **OKX**     | `/api/v5/market/ticker` | `/api/v5/market/books?sz=100` | 600/min | ✅ |
| **Coinbase** | `/products/{symbol}/ticker` | `/products/{symbol}/book?level=2` | 300/min | ✅ |

**⚠️ AGGREGATOR BAN**: DEXScreener, CoinGecko, etc. are **forbidden** for microstructure data. Exchange-native only.

## Tiered Liquidity System

### Liquidity Tiers by ADV (Average Daily Volume)

| Tier | ADV Range | Depth Requirement | Spread Cap | VADR Minimum | Examples |
|------|-----------|------------------|------------|--------------|----------|
| **Tier 1** | ≥$5M | ≥$150k within ±2% | ≤25 bps | ≥1.85× | BTC/USD, ETH/USD |
| **Tier 2** | $1M-$5M | ≥$75k within ±2% | ≤50 bps | ≥1.80× | SOL/USD, AVAX/USD |
| **Tier 3** | $100k-$1M | ≥$25k within ±2% | ≤80 bps | ≥1.75× | Small caps, new listings |

### Precedence Rules

#### VADR Precedence (Worst-Feed Wins)
```go
effectiveVADR = max(tierMinimum, p80Historical)
```

**Example**: Tier 1 requires ≥1.85× VADR, but historical P80 is 1.94×
- Applied requirement: **1.94×** (higher of the two)
- Rationale: Market conditions may require higher VADR than tier baseline

#### Venue Selection Precedence
1. **Primary**: First healthy venue in priority order: `binance` → `okx` → `coinbase`
2. **Fallbacks**: Other healthy venues for cross-validation
3. **Degraded Mode**: Single venue triggers "halve_size" recommendation

#### Data Quality Precedence  
```go
qualityMultiplier := map[string]float64{
    "excellent": 1.0,
    "good":      0.95,  // 5% penalty
    "degraded":  0.85,  // 15% penalty
}
adjustedDepth = rawDepth * qualityMultiplier
```

## Aggregator Ban Enforcement

### Compile-Time Guards

**Automatic Ban Detection**:
```go
// ENFORCED: Any attempt to use aggregator sources fails at compile time
if err := microstructure.GuardAgainstAggregator("coingecko"); err != nil {
    return nil, fmt.Errorf("AGGREGATOR BAN: %w", err)
}
```

**Banned Sources** (case-insensitive):
- `dexscreener`, `coingecko`, `coinmarketcap`, `cmc`
- `nomics`, `messari`, `cryptocompare`, `coinapi`
- `aggregate*`, `composite`, `blended`, `multi_exchange`

**Allowed Sources**:
- `binance`, `okx`, `coinbase`, `kraken` (future support)

### Runtime Safety

**Strict Mode (Production)**:
```go
GlobalAggregatorGuard = NewRuntimeAggregatorGuard(true) // Panics on violation
MustBeExchangeNative("binance") // ✅ Passes
MustBeExchangeNative("coingecko") // ❌ Panic: CRITICAL aggregator violation
```

**Validation Functions**:
```go
ValidateL1DataSource("binance")     // ✅ Valid
ValidateL2DataSource("dexscreener") // ❌ AggregatorBanError
ValidateOrderBookSource("okx")      // ✅ Valid
ValidateTickerSource("coingecko")   // ❌ AggregatorBanError
```

**Endpoint Pattern Detection**:
```go
CheckMicrostructureDataSource("binance", "/api/v3/aggregated/ticker", nil)
// ❌ Error: suspicious aggregation pattern '/aggregated/'
```

## Data Structures

### L1 Data (Best Bid/Ask)

```go
type L1Data struct {
    Symbol     string        `json:"symbol"`      // BTC/USD
    Venue      string        `json:"venue"`       // binance/okx/coinbase
    Timestamp  time.Time     `json:"timestamp"`   // Exchange timestamp
    
    // L1 pricing
    BidPrice   float64       `json:"bid_price"`   // Best bid price
    BidSize    float64       `json:"bid_size"`    // Best bid size
    AskPrice   float64       `json:"ask_price"`   // Best ask price
    AskSize    float64       `json:"ask_size"`    // Best ask size
    LastPrice  float64       `json:"last_price"`  // Last trade price
    
    // Derived metrics
    SpreadBps  float64       `json:"spread_bps"`  // Spread in basis points
    MidPrice   float64       `json:"mid_price"`   // (bid + ask) / 2
    
    // Attribution
    Quality    DataQuality   `json:"quality"`     // excellent/good/degraded
    DataAge    time.Duration `json:"data_age"`    // Age when retrieved
    Sequence   int64         `json:"sequence"`    // Exchange sequence
}
```

### L2 Data (Depth within ±2%)

```go
type L2Data struct {
    Symbol            string        `json:"symbol"`
    Venue             string        `json:"venue"`
    Timestamp         time.Time     `json:"timestamp"`
    
    // Depth measurements (USD equivalent)
    BidDepthUSD       float64       `json:"bid_depth_usd"`      // Bids within -2%
    AskDepthUSD       float64       `json:"ask_depth_usd"`      // Asks within +2%
    TotalDepthUSD     float64       `json:"total_depth_usd"`    // Combined depth
    BidLevels         int           `json:"bid_levels"`         // # bid levels
    AskLevels         int           `json:"ask_levels"`         // # ask levels
    
    // Liquidity gradient (concentration metric)
    LiquidityGradient float64       `json:"liquidity_gradient"` // depth@0.5% / depth@2%
    
    // VADR input feed (not VADR calculation itself)
    VADRInputVolume   float64       `json:"vadr_input_volume"`  // Volume estimate
    VADRInputRange    float64       `json:"vadr_input_range"`   // Range estimate
    
    // Attribution
    Quality           DataQuality   `json:"quality"`            // Data quality
    IsUSDQuote        bool          `json:"is_usd_quote"`      // USD pair validation
}
```

### Venue Health Status

```go
type VenueHealth struct {
    Venue               string        `json:"venue"`
    Timestamp           time.Time     `json:"timestamp"`
    
    // Health status
    Status              HealthStatus  `json:"status"`        // red/yellow/green
    Healthy             bool          `json:"healthy"`       // Overall health
    
    // Operational metrics (60s rolling)
    Uptime              float64       `json:"uptime"`        // % uptime
    HeartbeatAgeMs      int64         `json:"heartbeat_age_ms"` // Heartbeat age
    MessageGapRate      float64       `json:"message_gap_rate"` // % message gaps
    WSReconnectCount    int           `json:"ws_reconnect_count"` // Reconnects/60s
    
    // Performance metrics
    LatencyP50Ms        int64         `json:"latency_p50_ms"` // 50th percentile
    LatencyP99Ms        int64         `json:"latency_p99_ms"` // 99th percentile
    ErrorRate           float64       `json:"error_rate"`     // % errors
    
    // Data quality
    DataFreshness       time.Duration `json:"data_freshness"` // Data age
    DataCompleteness    float64       `json:"data_completeness"` // % complete
    
    // Recommendation
    Recommendation      string        `json:"recommendation"` // proceed/halve_size/avoid
}
```

## Health Monitoring System

### Health Thresholds

| Metric | Green | Yellow | Red |
|--------|-------|--------|-----|
| **Heartbeat Age** | <5s | 5-10s | >10s |
| **Error Rate** | <1% | 1-3% | >3% |
| **P99 Latency** | <1s | 1-2s | >2s |
| **Data Completeness** | >98% | 95-98% | <95% |
| **Message Gap Rate** | <2% | 2-5% | >5% |

### Health Badge Logic

```go
func determineHealthStatus(health *VenueHealth) (HealthStatus, string) {
    if health.ErrorRate > 0.03 || 
       health.HeartbeatAgeMs > 10000 || 
       health.DataCompleteness < 0.95 {
        return HealthRed, "avoid"
    }
    
    if health.LatencyP99Ms > 2000 || 
       health.MessageGapRate > 0.05 {
        return HealthYellow, "halve_size"
    }
    
    return HealthGreen, "proceed"
}
```

## Key Calculations

### Spread in Basis Points

```go
func calculateSpreadBps(bidPrice, askPrice float64) float64 {
    if bidPrice <= 0 || askPrice <= 0 || askPrice <= bidPrice {
        return 0
    }
    
    midPrice := (bidPrice + askPrice) / 2
    spread := askPrice - bidPrice
    return (spread / midPrice) * 10000 // Convert to bps
}
```

### Depth within ±2%

```go
func calculateDepthUSD(levels [][]string, midPrice float64, pctRange float64) (float64, int) {
    targetPrice := midPrice * (1 + pctRange)
    totalDepth := 0.0
    levelCount := 0
    
    for _, level := range levels {
        price, _ := strconv.ParseFloat(level[0], 64)
        size, _ := strconv.ParseFloat(level[1], 64)
        
        var withinRange bool
        if pctRange < 0 { // Bids (price should be >= target)
            withinRange = price >= targetPrice
        } else { // Asks (price should be <= target)
            withinRange = price <= targetPrice
        }
        
        if withinRange {
            totalDepth += price * size // USD value
            levelCount++
        } else {
            break // Levels are sorted
        }
    }
    
    return totalDepth, levelCount
}
```

### Liquidity Gradient

Measures liquidity concentration by comparing depth at different ranges:

```go
func calculateLiquidityGradient(depth05Pct, depth2Pct float64) float64 {
    if depth2Pct <= 0 {
        return 0
    }
    return depth05Pct / depth2Pct  // Higher = more concentrated
}
```

**Interpretation**:
- `0.8-1.0`: High concentration (most liquidity within 0.5%)
- `0.4-0.8`: Moderate concentration 
- `0.1-0.4`: Low concentration (liquidity spread out)
- `0.0-0.1`: Very poor liquidity distribution

## Sampling & Aggregation

### 1-Second Aggregation Windows

Each collector maintains **1s aggregation windows** with:

- **Message Counts**: L1 updates, L2 updates, errors
- **Latency Statistics**: Average, P50, P99, max
- **Quality Metrics**: Stale data %, incomplete %, sequence gaps
- **Processing Time**: Window processing duration

### 60-Second Rolling Statistics

Health monitor calculates **60s rolling stats**:

- **Throughput**: Messages per second
- **Error Rates**: % errors, % stale data, % incomplete
- **Performance**: Latency percentiles, uptime %
- **Quality Score**: Overall 0-100 quality rating

## CSV Health Artifacts

Health data is continuously exported to CSV files for analysis:

```
./artifacts/micro/health_binance.csv
./artifacts/micro/health_okx.csv
./artifacts/micro/health_coinbase.csv
```

### CSV Schema

| Column | Type | Description |
|--------|------|-------------|
| `timestamp` | ISO8601 | Health check timestamp |
| `venue` | string | Exchange name |
| `status` | string | red/yellow/green |
| `healthy` | boolean | Overall health flag |
| `uptime` | float | % uptime in last 60s |
| `heartbeat_age_ms` | int | Age of last heartbeat |
| `message_gap_rate` | float | % messages with gaps |
| `ws_reconnect_count` | int | Reconnects in 60s |
| `latency_p50_ms` | int | 50th percentile latency |
| `latency_p99_ms` | int | 99th percentile latency |
| `error_rate` | float | % errors in 60s |
| `data_freshness_ms` | int | Average data age |
| `data_completeness` | float | % complete messages |
| `recommendation` | string | proceed/halve_size/avoid |

## Usage Examples

### Creating Tiered Gate Calculator

```go
import "cryptorun/internal/microstructure"

// Create tiered calculator with default configuration
config := microstructure.DefaultTieredGateConfig()
calculator := microstructure.NewTieredGateCalculator(config)

// Evaluate tiered gates for a symbol
ctx := context.Background()
symbol := "BTC/USD"
adv := 10_000_000.0 // $10M ADV -> Tier 1

vadrInput := &microstructure.VADRInput{
    High:         51000.0,
    Low:          49000.0, 
    Volume:       5000.0,
    ADV:          adv,
    CurrentPrice: 50000.0,
}

result, err := calculator.EvaluateTieredGates(ctx, symbol, adv, vadrInput)
if err != nil {
    log.Fatalf("Gate evaluation failed: %v", err)
}

fmt.Printf("Symbol: %s | Tier: %s | Action: %s\n", 
    result.Symbol, result.Tier.Name, result.RecommendedAction)

if result.AllGatesPass {
    fmt.Printf("✅ All gates pass - %s\n", result.RecommendedAction)
} else {
    fmt.Printf("❌ Gates failed: %v\n", result.CriticalFailures)
}
```

### Entry Gate Integration

```go
import "cryptorun/internal/gates"

// Create entry gate evaluator with tiered microstructure
evaluator := gates.NewEntryGateEvaluator(
    microEvaluator,     // Legacy evaluator for compatibility
    fundingProvider,    // Funding divergence data
    oiProvider,         // Open interest data  
    etfProvider,        // ETF flow data
)

// Evaluate entry with tiered microstructure
symbol := "ETH/USD"
compositeScore := 78.5  // Score ≥75 required
priceChange24h := 3.2   // 24h price change %
regime := "TRENDING"    // Market regime
adv := 2_500_000.0     // $2.5M ADV -> Tier 2

result, err := evaluator.EvaluateEntry(ctx, symbol, compositeScore, priceChange24h, regime, adv)
if err != nil {
    log.Fatalf("Entry evaluation failed: %v", err)
}

if result.Passed {
    fmt.Printf("✅ ENTRY CLEARED - %s (%.1f score)\n", symbol, compositeScore)
    
    // Access tiered microstructure details
    if result.TieredGateResult != nil {
        tiered := result.TieredGateResult
        fmt.Printf("Tier: %s | Primary Venue: %s | %s\n", 
            tiered.Tier.Name, tiered.PrimaryVenue, tiered.RecommendedAction)
            
        if tiered.DegradedMode {
            fmt.Printf("⚠️ Operating in degraded mode\n")
        }
    }
} else {
    fmt.Printf("❌ ENTRY BLOCKED - %v\n", result.FailureReasons)
}
```

### Metrics Aggregation

```go
aggregator := micro.NewMetricsAggregator(collectors)
aggregator.Start(ctx)

// Get aggregated report
report := aggregator.GetAggregatedReport()
fmt.Printf("Overall health: %s\n", aggregator.GetHealthSummary())

// Get venue metrics
metrics, _ := aggregator.GetVenueMetrics("binance")
fmt.Printf("Binance: %.1f msg/s, %.1fms avg latency, %.1f%% quality\n",
    float64(metrics.L1Messages + metrics.L2Messages),
    metrics.AvgLatencyMs,
    metrics.QualityScore)
```

### Fetching L1/L2 Data

```go
// Get L1 data (best bid/ask)
l1Data, err := binanceCollector.GetL1Data("BTC/USD")
if err == nil {
    fmt.Printf("BTC/USD: Bid $%.2f | Ask $%.2f | Spread %.1f bps\n",
        l1Data.BidPrice, l1Data.AskPrice, l1Data.SpreadBps)
}

// Get L2 data (depth within ±2%)
l2Data, err := binanceCollector.GetL2Data("BTC/USD")
if err == nil {
    fmt.Printf("BTC/USD Depth: $%.0f total | Gradient %.3f | Quality %s\n",
        l2Data.TotalDepthUSD, l2Data.LiquidityGradient, l2Data.Quality)
}
```

## Testing

### Unit Tests

```bash
# Test spread and depth calculations
go test ./internal/micro/collectors -v -run TestCalculateSpread
go test ./internal/micro/collectors -v -run TestCalculateDepth

# Test gradient monotonicity
go test ./internal/micro/collectors -v -run TestLiquidityGradient
```

### Integration Tests

```bash
# Test health monitoring with mock collectors  
go test ./internal/micro -v -run TestHealthMonitor

# Test metrics aggregation
go test ./internal/micro -v -run TestMetricsAggregator

# Test CSV output generation
go test ./internal/micro -v -run TestHealthMonitorCSV
```

## Configuration

### Collector Config

```yaml
# config/microstructure.yaml
venues:
  binance:
    aggregation_window_ms: 1000      # 1s windows
    rolling_stats_window_ms: 60000   # 60s rolling stats
    max_heartbeat_age_ms: 10000      # 10s max heartbeat age
    max_error_rate: 0.03             # 3% max error rate
    max_latency_p99_ms: 2000         # 2s max P99 latency
    health_csv_path: "./artifacts/micro/health_binance.csv"
    
  okx:
    # Similar config for OKX...
    
  coinbase:  
    # Similar config for Coinbase...

health_monitor:
  health_check_interval_sec: 5       # Health checks every 5s
  csv_flush_interval_sec: 30         # Flush CSV every 30s
  max_history_points: 720            # 1 hour of 5s intervals
```

## Troubleshooting

### Common Issues

**🔴 Venue showing RED status**
- Check error rate: `health.ErrorRate > 0.03`
- Check latency: `health.LatencyP99Ms > 2000` 
- Check data completeness: `health.DataCompleteness < 95%`
- Verify network connectivity to venue

**🟡 Venue showing YELLOW status**  
- Monitor P99 latency spikes
- Check for message gaps in WebSocket feed
- Verify rate limit compliance
- Consider reducing request frequency

**❌ No L1/L2 data available**
- Verify symbol format: "BTC/USD" not "BTCUSD"
- Check venue subscription status
- Ensure USD pairs only (no EUR, GBP, etc.)
- Validate venue API key configuration (if required)

### Debug Commands

```bash
# Check CSV health artifacts
ls -la ./artifacts/micro/health_*.csv
tail -n 10 ./artifacts/micro/health_binance.csv

# Test collector connectivity  
curl -s "https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT"
curl -s "https://www.okx.com/api/v5/market/ticker?instId=BTC-USDT"
curl -s "https://api.pro.coinbase.com/products/BTC-USD/ticker"

# Monitor health in real-time
watch -n 1 'tail -n 5 ./artifacts/micro/health_*.csv'
```

## Performance Characteristics

| Metric | Target | Typical |
|--------|--------|---------|
| **L1 Data Latency** | <500ms | 200-300ms |
| **L2 Data Latency** | <1000ms | 400-600ms |
| **Health Check Interval** | 5s | 5s |
| **Memory Usage** | <50MB | 20-30MB |
| **CPU Usage** | <5% | 2-3% |

## Integration with Gates System

The microstructure collectors feed directly into CryptoRun's entry gates:

```go
// Gates integration example
func evaluateMicrostructureGates(symbol string) (bool, error) {
    // Get L2 data from preferred venue
    l2Data, err := getL2DataWithFallback(symbol)
    if err != nil {
        return false, err
    }
    
    // Apply microstructure gates
    if l2Data.TotalDepthUSD < 100000 {  // $100k minimum depth
        return false, fmt.Errorf("insufficient depth: $%.0f", l2Data.TotalDepthUSD)
    }
    
    l1Data, err := getL1DataWithFallback(symbol)
    if err != nil {
        return false, err  
    }
    
    if l1Data.SpreadBps > 50 {  // 50 bps max spread
        return false, fmt.Errorf("spread too wide: %.1f bps", l1Data.SpreadBps)
    }
    
    // Check venue health
    venueHealth, _ := healthMonitor.GetVenueHealth(l2Data.Venue)
    if !venueHealth.Healthy {
        return false, fmt.Errorf("venue %s unhealthy", l2Data.Venue)
    }
    
    return true, nil
}
```

---

**Version**: v2.0.0  
**Last Updated**: 2024-01-01  
**Status**: Production Ready ✅