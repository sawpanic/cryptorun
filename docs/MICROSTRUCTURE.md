# CryptoRun Microstructure Enforcement

## UX MUST — Live Progress & Explainability

CryptoRun's microstructure enforcement provides real-time validation of exchange-native L1/L2 data with comprehensive proof generation and transparent decision making. Each validation shows:

- **Live venue checking**: Progress across Binance, OKX, Coinbase with real-time results
- **Detailed requirement validation**: Spread, depth, VADR checks with specific values
- **Proof bundle generation**: Point-in-time evidence for all validation decisions  
- **Interactive threshold adjustment**: Menu-driven configuration with immediate effect

## Exchange-Native Validation Overview

CryptoRun implements strict microstructure validation to ensure only liquid, tradeable assets pass through to position sizing. The system validates three critical requirements using **exchange-native APIs only** - no aggregators permitted for order book data.

### Core Requirements

| Requirement | Threshold | Purpose | Exchange Sources |
|-------------|-----------|---------|------------------|
| **Spread** | < 50 bps | Minimize execution costs | Binance, OKX, Coinbase |
| **Depth** | ≥ $100k within ±2% | Ensure position exit liquidity | Native L2 orderbooks |
| **VADR** | ≥ 1.75× | Volume-adjusted daily range validation | Exchange historical data |

### Supported Venues

- **Binance**: Primary venue, tight spreads, deep liquidity
- **OKX**: Secondary venue, good for alt-coins
- **Coinbase**: Backup venue, USD-focused pairs
- **Kraken**: Preferred venue (planned), regulatory compliant

## Architecture Overview

```
┌─────────────────┐    ┌───────────────────┐    ┌─────────────────┐
│ Menu Interface  │───▶│ Microstructure    │───▶│ Venue Clients   │
│ • Single Asset  │    │ Gate              │    │ • Binance API   │
│ • Batch Check   │    │ • Checker         │    │ • OKX API       │
│ • View Proofs   │    │ • ProofGenerator  │    │ • Coinbase API  │
│ • Thresholds    │    │ • Gate Logic      │    └─────────────────┘
└─────────────────┘    └───────────────────┘           │
                                │                       │
                                ▼                       ▼
                    ┌─────────────────┐    ┌─────────────────┐
                    │ Proof Bundles   │    │ L1/L2 Data      │
                    │ • Master Proof  │    │ • Best Bid/Ask  │
                    │ • Venue Proofs  │    │ • Full Orderbook│
                    │ • Audit Reports │    │ • Sequence IDs  │
                    └─────────────────┘    └─────────────────┘
```

## Venue Client Implementation

### OrderBook Data Structure

```go
type OrderBook struct {
    // Metadata
    Symbol        string    `json:"symbol"`
    Venue         string    `json:"venue"`
    TimestampMono time.Time `json:"timestamp_mono"`
    SequenceNum   int64     `json:"sequence_num"`

    // L1 Data (Best Bid/Ask)
    BestBidPrice  float64   `json:"best_bid_price"`
    BestBidQty    float64   `json:"best_bid_qty"`
    BestAskPrice  float64   `json:"best_ask_price"`
    BestAskQty    float64   `json:"best_ask_qty"`

    // Derived Metrics
    MidPrice               float64 `json:"mid_price"`
    SpreadBPS              float64 `json:"spread_bps"`
    DepthUSDPlusMinus2Pct  float64 `json:"depth_usd_plus_minus_2pct"`

    // L2 Data (Full Book)
    Bids []Level `json:"bids"`
    Asks []Level `json:"asks"`
}
```

### Venue-Specific Implementations

#### Binance Client
- **Endpoint**: `https://api.binance.com/api/v3/depth`
- **Rate Limit**: 1200 requests per minute
- **Cache TTL**: 300 seconds (5 minutes)
- **Depth Levels**: Up to 1000 levels for ±2% calculation

#### OKX Client  
- **Endpoint**: `https://www.okx.com/api/v5/market/books`
- **Rate Limit**: 20 requests per 2 seconds
- **Cache TTL**: 300 seconds
- **Depth Levels**: Up to 400 levels

#### Coinbase Client
- **Endpoint**: `https://api.exchange.coinbase.com/products/{symbol}/book`
- **Rate Limit**: 10 requests per second
- **Cache TTL**: 300 seconds  
- **Depth Levels**: Full L2 book (level=2)

## Validation Logic

### Spread Validation
```go
// Calculate bid-ask spread in basis points
midPrice := (bestBidPrice + bestAskPrice) / 2.0
spread := bestAskPrice - bestBidPrice
spreadBPS := (spread / midPrice) * 10000

// Validate against threshold
spreadValid := spreadBPS < 50.0 // < 50 bps requirement
```

### Depth Validation  
```go
// Calculate depth within ±2% of mid price
lowerBound := midPrice * 0.98 // -2%
upperBound := midPrice * 1.02 // +2%

totalDepthUSD := 0.0

// Sum bid depth within range
for _, bid := range bids {
    if bid.Price >= lowerBound {
        totalDepthUSD += bid.Price * bid.Quantity
    }
}

// Sum ask depth within range  
for _, ask := range asks {
    if ask.Price <= upperBound {
        totalDepthUSD += ask.Price * ask.Quantity
    }
}

// Validate against threshold
depthValid := totalDepthUSD >= 100000 // >= $100k requirement
```

### VADR Validation
```go
// VADR = Volume-Adjusted Daily Range
// Calculated from exchange historical data
vadrValid := vadr >= 1.75 // >= 1.75× requirement
```

## Proof Bundle System

### Proof Bundle Structure
```json
{
  "asset_symbol": "BTCUSDT",
  "timestamp_mono": "2025-01-15T14:30:00Z",
  "proven_valid": true,
  "order_book_snapshot": { /* Full L1/L2 data */ },
  "microstructure_metrics": { /* Validation results */ },
  "spread_proof": {
    "metric": "spread_bps",
    "actual_value": 35.2,
    "required_value": 50.0,
    "operator": "<",
    "passed": true,
    "evidence": "Spread 35.2 bps meets required max 50.0 bps"
  },
  "depth_proof": { /* Similar structure */ },
  "vadr_proof": { /* Similar structure */ },
  "proof_generated_at": "2025-01-15T14:30:01Z",
  "venue_used": "binance",
  "proof_id": "BTCUSDT_binance_1705329001"
}
```

### Proof Persistence
- **Directory Structure**: `./artifacts/proofs/{date}/microstructure/`
- **Master Proof**: `{symbol}_master_proof.json` - Complete validation record
- **Venue Proofs**: `{symbol}_{venue}_proof.json` - Per-venue evidence
- **Metrics Summary**: `{symbol}_metrics_summary.json` - Validation metrics only

## Menu Interface Integration

### Microstructure Validation Screen

Accessible via: **Menu → Settings (11) → Microstructure Validation (5)**

#### Single Asset Check
```
🔍 Checking microstructure eligibility for BTCUSDT...

[33%] Checking binance...
   ✅ binance: Spread 35.2bps, Depth $150k, VADR 2.10x

[67%] Checking okx...
   ✅ okx: Spread 42.1bps, Depth $120k, VADR 1.85x

[100%] Checking coinbase...
   ❌ coinbase: Spread 65.0bps, Depth $85k, VADR 1.90x
      ❌ Spread 65.0bps > 50.0bps limit
      ❌ Depth $85k < $100k limit

📊 Summary for BTCUSDT:
✅ ELIGIBLE - Passed on 2 venue(s): [binance, okx]
📁 Proof bundle generated: ./artifacts/proofs/2025-01-15/microstructure/BTCUSDT_master_proof.json
```

#### Batch Asset Check
```
🔍 Checking 5 assets across venues...

[20%] Processing BTCUSDT...
   ✅ BTCUSDT: ELIGIBLE on 2/3 venues

[40%] Processing ETHUSDT...
   ✅ ETHUSDT: ELIGIBLE on 3/3 venues

[60%] Processing SOLUSDT...
   ❌ SOLUSDT: NOT ELIGIBLE (spread violations)

📊 Batch Results:
   Total Assets: 5
   Eligible: 3 (60.0%)
   Not Eligible: 2
📁 Audit report: ./artifacts/proofs/2025-01-15/reports/microstructure_audit_143022.json
```

#### Proof Browsing
```
📁 Generated Proof Bundles:
=====================================

1. ✅ BTCUSDT (2025-01-15) - 3 venues
   📄 ./artifacts/proofs/2025-01-15/microstructure/BTCUSDT_master_proof.json

2. ✅ ETHUSDT (2025-01-15) - 2 venues  
   📄 ./artifacts/proofs/2025-01-15/microstructure/ETHUSDT_master_proof.json

3. ❌ SOLUSDT (2025-01-15) - 0 venues
   📄 ./artifacts/proofs/2025-01-15/microstructure/SOLUSDT_master_proof.json

🔍 Actions:
 1. Open Proof Directory
 2. View Specific Proof
 0. Back
```

## Gate Integration

### MicrostructureGate Implementation

The microstructure validation integrates with CryptoRun's gate system to block ineligible assets:

```go
type MicrostructureGate struct {
    checker         *microstructure.Checker
    proofGenerator  *microstructure.ProofGenerator
    venueClients    map[string]microstructure.VenueClient
    enabled         bool
}

func (mg *MicrostructureGate) Evaluate(ctx context.Context, symbol string) (*GateResult, error) {
    // Check asset eligibility across venues
    eligibilityResult, err := mg.checker.CheckAssetEligibility(ctx, symbol, mg.venueClients)
    
    // Generate proof bundle (for audit trail)
    mg.proofGenerator.GenerateProofBundle(ctx, eligibilityResult)
    
    // Return gate result
    return &GateResult{
        GateName: "microstructure",
        Symbol:   symbol,
        Passed:   eligibilityResult.OverallEligible,
        Reason:   "microstructure_validation",
        Metadata: map[string]interface{}{
            "eligible_venues": eligibilityResult.EligibleVenues,
            "venue_count": len(eligibilityResult.EligibleVenues),
        },
    }, nil
}
```

### Gate Configuration

```yaml
# config/gates.yaml
gates:
  microstructure:
    enabled: true
    max_spread_bps: 50.0
    min_depth_usd: 100000
    min_vadr: 1.75
    require_all_venues: false  # Any venue passing is sufficient
    enabled_venues: ["binance", "okx", "coinbase"]
    artifacts_dir: "./artifacts"
```

## Audit & Reporting

### Audit Report Structure
```json
{
  "generated_at": "2025-01-15T15:00:00Z",
  "total_assets": 50,
  "eligible_assets": ["BTCUSDT", "ETHUSDT", "ADAUSDT", ...],
  "ineligible_assets": ["BADCOIN", "ILLIQUID", ...],
  "venue_stats": {
    "binance": {
      "venue_name": "binance",
      "total_checked": 50,
      "passed_checks": 42,
      "failed_checks": 8,
      "average_spread_bps": 38.5,
      "average_depth_usd": 165000
    }
  },
  "summary": {
    "eligible_count": 35,
    "ineligible_count": 15,
    "eligibility_rate_percent": 70.0
  }
}
```

### Common Failure Patterns
1. **Spread Violations (60%)**: Wide bid-ask spreads on low-volume pairs
2. **Depth Violations (25%)**: Insufficient liquidity within ±2%
3. **VADR Violations (10%)**: Low trading activity/volatility
4. **API Errors (5%)**: Venue connectivity issues

## Configuration & Customization

### Threshold Adjustment

Via menu interface:
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
```

### Venue-Specific Overrides

```yaml
# Advanced configuration for different requirements per venue
venue_overrides:
  binance:
    max_spread_bps: 40.0  # Tighter for high-volume venue
  okx:
    max_spread_bps: 60.0  # Relaxed for alt-coins
  coinbase:
    min_depth_usd: 75000  # Reduced for USD-focused pairs
```

## Performance & Caching

### Cache Strategy
- **TTL**: 300 seconds (5 minutes) for all venue data
- **Invalidation**: Automatic on TTL expiry
- **Hit Rate Target**: >85% to minimize API calls
- **Jitter**: ±30 seconds to prevent thundering herd

### Rate Limit Management
- **Binance**: 1200 req/min → 20 req/sec burst
- **OKX**: 20 req/2sec → 10 req/sec sustained  
- **Coinbase**: 10 req/sec → Direct throttling
- **Circuit Breakers**: Automatic failover on rate limit hits

### Latency Targets
- **P50**: <200ms for single asset check
- **P95**: <500ms for single asset check
- **P99**: <1000ms for single asset check
- **Batch (10 assets)**: <5 seconds total

## Testing Strategy

### Unit Tests
- **Validation Logic**: Spread, depth, VADR calculations
- **Proof Generation**: Bundle creation and persistence
- **Gate Integration**: Pass/fail logic with metadata

### Integration Tests  
- **Venue Clients**: API integration with fixture data
- **End-to-End**: Menu → Validation → Proof generation
- **Error Handling**: Network failures, API errors, malformed data

### Test Fixtures
- **Binance**: `tests/fixtures/binance_orderbook_btcusdt.json`
- **OKX**: `tests/fixtures/okx_orderbook_ethusdt.json`
- **Coinbase**: `tests/fixtures/coinbase_orderbook_solusdt.json`

## Future Enhancements

### Planned Features
- **Kraken Integration**: Add preferred regulatory-compliant venue
- **WebSocket Streams**: Real-time orderbook updates
- **Cross-Venue Arbitrage**: Detect and avoid spread arbitrage opportunities
- **Historical Analysis**: Trend analysis of microstructure quality

### Research Areas
- **Dynamic Thresholds**: ML-based threshold adjustment based on market conditions
- **Venue Quality Scoring**: Weight venues based on historical reliability
- **Predictive Liquidity**: Forecast depth changes during volatile periods

This microstructure enforcement system ensures CryptoRun only processes assets with sufficient liquidity for safe position entry and exit, with full audit trails and transparent decision making.