# Provider Interface & Live Connector System

## UX MUST — Live Progress & Explainability

Complete provider interface system with live connectors, capability-based routing, provenance tracking, and real-time health monitoring via the `cryptorun providers probe` command. Full transparency into provider performance, fallback chains, and data attribution.

**Updated for EPIC G1.0 — Provider Interfaces & Live Connector Skeletons**  
**Last Updated:** 2025-09-07  
**Version:** v3.2.1 Provider Interface System  
**Status:** Implemented - EPIC G1.0 Complete

## Provider Interface Architecture (EPIC G1.0)

### Capability-Based Provider System

The new provider system implements a comprehensive interface with capability-based routing:

```go
type Provider interface {
    Name() string
    HasCapability(cap Capability) bool
    Probe(ctx context.Context) (*ProbeResult, error)
    
    // Core capabilities
    GetFundingHistory(ctx context.Context, req *FundingRequest) (*FundingResponse, error)
    GetSpotTrades(ctx context.Context, req *SpotTradesRequest) (*SpotTradesResponse, error)
    GetOrderBookL2(ctx context.Context, req *OrderBookRequest) (*OrderBookResponse, error)
    GetKlineData(ctx context.Context, req *KlineRequest) (*KlineResponse, error)
    GetSupplyReserves(ctx context.Context, req *SupplyRequest) (*SupplyResponse, error)
    GetWhaleDetection(ctx context.Context, req *WhaleRequest) (*WhaleResponse, error)
    GetCVD(ctx context.Context, req *CVDRequest) (*CVDResponse, error)
}
```

### Live Provider Capabilities

| Provider  | Funding | Spot Trades | OrderBook L2 | Kline Data | Supply/Reserves | Status |
|-----------|---------|-------------|--------------|------------|-----------------|--------|
| **Binance** | ✅      | ✅          | ✅           | ✅         | ❌              | Live |
| **OKX**     | ✅      | ✅          | ✅           | ✅         | ❌              | Live |
| **Coinbase** | ❌     | ✅          | ✅           | ✅         | ❌              | Live |
| **Kraken**  | ❌      | ✅          | ✅           | ✅         | ❌              | Live |
| **CoinGecko** | ❌    | ❌          | ❌           | ❌         | ✅              | Live |

### Provider Probe Command

Real-time provider health and capability testing:

```bash
# Basic probe - test all providers
cryptorun providers probe

# Verbose output with detailed capability matrix  
cryptorun providers probe --verbose

# JSON output for automation
cryptorun providers probe --format=json --timeout=10s

# Custom configuration file
cryptorun providers probe --config=config/providers.yaml
```

**Example Probe Output**:
```
Provider Capability Report (Generated: 2025-09-07T21:14:29Z)

Provider   Status  Latency  Capabilities
--------   ------  -------  ------------
binance    ✅ UP    342ms    4/4 available
coinbase   ✅ UP    299ms    3/3 available
coingecko  ✅ UP    1314ms   1/1 available
kraken     ❌ DOWN  0ms      0/3 available
okx        ✅ UP    353ms    4/4 available
```

### Provenance Tracking

Every data response includes comprehensive provenance information:

```json
{
  "data": [{"symbol": "BTCUSDT", "price": 43500.50}],
  "provenance": {
    "venue": "binance",
    "endpoint": "/api/v3/trades", 
    "window": 100,
    "latency_ms": 245,
    "timestamp": "2025-09-07T21:14:29Z"
  }
}
```

## Provider Configuration Matrix

### Rate Limits Per Provider (v3.2.1)

| Provider    | Free Tier | Requests/Min | Requests/Hour | Daily Budget | Monthly Budget | Special Headers |
|-------------|-----------|--------------|---------------|--------------|----------------|-----------------|
| **Binance** | ✅        | 1,200        | 7,200         | 100,000      | 2,000,000      | X-MBX-USED-WEIGHT |
| **Kraken**  | ✅        | 60           | 3,600         | Unlimited    | Unlimited      | - |
| **OKX**     | ✅        | 900          | 54,000        | Unlimited    | Unlimited      | - |
| **DEXScreener** | ✅    | 1,800        | 108,000       | Unlimited    | Unlimited      | - |
| **CoinGecko** | ✅      | 600          | 36,000        | 10,000       | 300,000        | - |
| **CoinPaprika** | ✅    | 6,000        | 360,000       | 25,000       | 750,000        | - |
| **DeFiLlama** | ✅      | 180          | 10,800        | Unlimited    | Unlimited      | - |
| **TheGraph** | ✅       | 300          | 18,000        | 100,000      | 3,000,000      | - |

### Circuit Breaker Settings

| Provider | Failure Threshold | Success Threshold | Timeout | Max Concurrent | Health Check Interval |
|----------|-------------------|-------------------|---------|----------------|-----------------------|
| Binance | 5 failures | 3 successes | 2 min | 10 | 30s |
| Kraken | 2 failures | 1 success | 4 min | 2 | 1 min |
| OKX | 3 failures | 2 successes | 3 min | 5 | 45s |
| DEXScreener | 4 failures | 2 successes | 2 min | 5 | 1 min |
| CoinGecko | 3 failures | 2 successes | 4 min | 3 | 2 min |
| CoinPaprika | 4 failures | 2 successes | 3 min | 5 | 1 min |
| DeFiLlama | 2 failures | 1 success | 6 min | 2 | 3 min |
| TheGraph | 3 failures | 2 successes | 8 min | 3 | 2 min |

### Cache Tier Configuration

| Provider | Warm TTL | Hot TTL | Cold TTL | Degraded TTL | Max Size | Compression |
|----------|----------|---------|----------|--------------|----------|-------------|
| Binance | 5 min | 30s | 24h | 10 min | 10,000 | ✅ |
| Kraken | 4 min | 60s | 30h | 15 min | 8,000 | ✅ |
| OKX | 3 min | 45s | 12h | 8 min | 6,000 | ✅ |
| DEXScreener | 5 min | 2 min | 12h | 10 min | 5,000 | ✅ |
| CoinGecko | 8 min | 5 min | 6h | 20 min | 3,000 | ❌ |
| CoinPaprika | 6 min | 3 min | 8h | 15 min | 4,000 | ✅ |
| DeFiLlama | 10 min | 5 min | 24h | 30 min | 2,000 | ✅ |
| TheGraph | 8 min | 3 min | 18h | 25 min | 3,000 | ✅ |

## Fallback Chain Specifications

### Data Type Priority Chains

**Exchange-Native Microstructure:**
- Primary: Binance → Fallbacks: Kraken → OKX
- Max retries: 3, Retry delay: 2s
- **BANNED**: Aggregators (CoinGecko, CoinPaprika, DEXScreener)

**Derivatives & Futures:**
- Primary: OKX → Fallbacks: Binance (futures)
- Max retries: 2, Retry delay: 3s
- **Exclusive**: Only exchange-native providers

**Market Data (Fallback Only):**
- Primary: CoinGecko → Fallbacks: CoinPaprika → Binance
- Max retries: 2, Retry delay: 4s
- **Constraint**: Never used for microstructure

**DeFi/On-Chain Volume:**
- Primary: DeFiLlama → Fallbacks: TheGraph → DEXScreener
- Max retries: 2, Retry delay: 8s
- **Purpose**: Volume residual factor enhancement only

**DEX Volume/Events:**
- Primary: DEXScreener → Fallbacks: DeFiLlama → TheGraph
- Max retries: 2, Retry delay: 5s
- **BANNED**: Microstructure data (depth/spread)

**Protocol Analytics:**
- Primary: TheGraph → Fallbacks: DeFiLlama
- Max retries: 1, Retry delay: 12s
- **Usage**: TVL, AMM metrics, subgraph data

## Rate Limit Handling

### Binance Weight System

Binance uses a weight-based system with special header tracking:
```go
// Weight tracking headers
X-MBX-USED-WEIGHT     // Current minute weight usage
X-MBX-USED-WEIGHT-1M  // 1-minute weight usage
```

### Response Code Handling

| Status Code | Action | Backoff Strategy |
|-------------|--------|------------------|
| **429** | Rate Limited | Exponential backoff with Retry-After header |
| **418** | IP banned | Extended backoff (30+ minutes) |
| **403** | Forbidden | Circuit breaker trip |
| **5xx** | Server error | Circuit breaker counting |

### Budget Depletion Response

When quotas are exhausted:
1. **Cache Extension**: Double cache TTLs automatically
2. **Fallback Activation**: Immediate provider switching
3. **Degraded Mode**: Extended cache retention
4. **Health Monitoring**: Continuous recovery checks

## CLI Integration

### Provider Health Banner

The interactive menu displays real-time provider status:

```
 ╔═══════════════════════════════════════════════════════════╗
 ║                    🚀 CryptoRun v3.2.1                    ║
 ║              Cryptocurrency Momentum Scanner              ║
 ║                                                           ║
 ║    🎯 This is the CANONICAL INTERFACE                     ║
 ║       All features are accessible through this menu      ║
 ║                                                           ║
 ║    📡 Provider Status: 🟢 All healthy                     ║
 ╚═══════════════════════════════════════════════════════════╝
```

### Status Indicators

| Indicator | Meaning |
|-----------|---------|
| 🟢 All healthy | All providers operational |
| 🟡 6/7 OK | Majority healthy, some degraded |
| 🔴 3/7 failed | Critical provider failures |
| ⚪ Not initialized | System starting up |

## Testing & Validation

### Degradation Scenarios

The test suite validates:

1. **Rate Limit Exhaustion**: Quota depletion triggers fallbacks
2. **Circuit Breaker Trips**: Failure threshold enforcement  
3. **Provider Recovery**: Half-open to closed transitions
4. **Cache Behavior**: TTL extension during degradation
5. **Fallback Chains**: Primary failure → fallback success
6. **Weight Tracking**: Binance header parsing accuracy

---

## Provider Compliance & Classification

### Exchange-Native (✅ Microstructure Allowed)
- **Binance**: Full L1/L2 access, weight-based rate limiting
- **Kraken**: Full L1/L2 access, conservative rate limits, server time sync
- **OKX**: Derivatives focus, perpetuals, funding rates, basis calculations

### DeFi/On-Chain (📊 Volume/Analytics Only)
- **DeFiLlama**: Protocol TVL, DeFi metrics (free tier)
- **TheGraph**: Subgraph data, AMM pools, on-chain analytics
- **DEXScreener**: **Volume/Events Only** - banned from microstructure

### Aggregators (⚠️ Fallback Only - Microstructure BANNED)
- **CoinGecko**: Market data fallback, **NO** depth/spread/orderbook
- **CoinPaprika**: Market data fallback, **NO** depth/spread/orderbook

### Compliance Enforcement
1. **Compile-Time Guards**: Build tags prevent aggregator microstructure usage
2. **Runtime Bans**: Explicit error messages for banned methods
3. **Circuit Integration**: Banned providers bypass microstructure circuits
4. **Volume Enhancement**: On-chain volume integrated into VolumeResidual factor

### Provider Health Indicators
| Status | Meaning | Action |
|--------|---------|--------|
| 🟢 All healthy | All providers operational | Normal operation |
| 🟡 6/8 OK | Majority healthy, some degraded | Monitor fallbacks |
| 🔴 <5 healthy | Critical provider failures | Escalate, check exchanges |
| ⚪ Not initialized | System starting up | Wait for initialization |

**📋 Engineering Notes:**
- All providers respect rate limits with exponential backoff
- Circuit breakers prevent cascade failures  
- Aggregator ban enforced at compile-time and runtime
- DeFi data enhances volume analysis without compromising microstructure integrity
- Health monitoring integrated into CLI banner for transparency
- Volume residual factor now includes on-chain DEX volume from DeFi providers