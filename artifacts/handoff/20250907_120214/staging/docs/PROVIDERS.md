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
| **DEXScreener** | ✅    | 300          | 1,800         | 25,000       | 500,000        | - |
| **CoinGecko** | ✅      | 50           | 3,000         | 10,000       | 300,000        | - |
| **Moralis** | ✅        | 25           | 1,500         | 40,000       | 1,000,000      | - |
| **CMC**     | Basic     | 30           | 1,800         | 10,000       | 333 credits    | - |
| **Etherscan** | ✅      | 5            | 300           | 100,000      | 3,000,000      | - |
| **Paprika** | ✅        | 100          | 6,000         | 25,000       | 750,000        | - |

### Circuit Breaker Settings

| Provider | Failure Threshold | Success Threshold | Timeout | Max Concurrent | Health Check Interval |
|----------|-------------------|-------------------|---------|----------------|-----------------------|
| Binance | 5 failures | 3 successes | 2 min | 10 | 30s |
| DEXScreener | 3 failures | 2 successes | 5 min | 5 | 1 min |
| CoinGecko | 4 failures | 2 successes | 3 min | 3 | 1 min |
| Moralis | 3 failures | 2 successes | 10 min | 2 | 2 min |
| CMC | 2 failures | 1 success | 15 min | 2 | 3 min |
| Etherscan | 2 failures | 1 success | 30 min | 1 | 5 min |
| Paprika | 4 failures | 2 successes | 4 min | 5 | 1 min |

### Cache Tier Configuration

| Provider | Warm TTL | Hot TTL | Cold TTL | Degraded TTL | Max Size | Compression |
|----------|----------|---------|----------|--------------|----------|-------------|
| Binance | 5 min | 30s | 24h | 10 min | 10,000 | ✅ |
| DEXScreener | 5 min | 2 min | 12h | 10 min | 5,000 | ✅ |
| CoinGecko | 5 min | 3 min | 6h | 15 min | 3,000 | ❌ |
| Moralis | 5 min | 5 min | 24h | 20 min | 2,000 | ✅ |
| CMC | 5 min | 5 min | 12h | 25 min | 3,000 | ✅ |
| Etherscan | 5 min | 10 min | 48h | 30 min | 1,000 | ❌ |
| Paprika | 5 min | 2 min | 8h | 12 min | 4,000 | ✅ |

## Fallback Chain Specifications

### Data Type Priority Chains

**Price Data:**
- Primary: Binance → Fallbacks: CoinGecko → CMC → Paprika
- Max retries: 3, Retry delay: 2s

**Market Data:**
- Primary: CoinGecko → Fallbacks: CMC → Paprika → DEXScreener  
- Max retries: 2, Retry delay: 3s

**Social Data:**
- Primary: DEXScreener → Fallbacks: CoinGecko → CMC
- Max retries: 2, Retry delay: 5s (less critical)

**DeFi Data:**
- Primary: Moralis → Fallbacks: DEXScreener → Etherscan
- Max retries: 2, Retry delay: 10s

**Ethereum Data:**
- Primary: Etherscan → Fallbacks: Moralis
- Max retries: 1, Retry delay: 15s

**Exchange Data:**
- Primary: Binance → Fallbacks: Paprika → CMC
- Max retries: 2, Retry delay: 3s

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

**📋 Engineering Notes:**
- All providers respect rate limits with exponential backoff
- Circuit breakers prevent cascade failures
- 5-minute warm cache standard across all providers
- Degraded mode doubles cache TTLs automatically
- Health monitoring integrated into CLI banner for transparency