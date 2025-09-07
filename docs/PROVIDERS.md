# Provider Runtime System

## UX MUST — Live Progress & Explainability

Real-time provider health monitoring, rate limit management, and circuit breaker protection: transparent fallback chains, adaptive cache TTLs, and comprehensive degradation handling for reliable multi-provider cryptocurrency data access.

**Updated for PROMPT_ID=PROVIDERS.GUARDS.RATELIMITS**  
**Last Updated:** 2025-09-07  
**Version:** v3.2.1 Provider Runtime  
**Status:** Implemented

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