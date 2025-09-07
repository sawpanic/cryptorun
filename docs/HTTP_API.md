# CryptoRun HTTP API Documentation

## Overview

CryptoRun exposes a read-only HTTP service that provides operational and analytical views of the momentum scanning system. All endpoints are local-only by default (127.0.0.1), require no authentication, and return JSON responses.

**Base URL:** `http://127.0.0.1:8080` (configurable via `--host` and `--port` flags)

## Quick Start

```bash
# Start the monitoring server
./cryptorun monitor

# Check system health
curl http://localhost:8080/health

# Get top 10 candidates
curl http://localhost:8080/candidates?n=10

# Explain a specific symbol
curl http://localhost:8080/explain/BTC-USD

# Get current regime
curl http://localhost:8080/regime
```

## Endpoints

### GET /health

Returns system health status including provider limits, circuit breakers, and latencies.

**Response:** Standard health check format with service status breakdown.

### GET /candidates

Returns top composite scoring candidates with gate status and microstructure data.

**Query Parameters:**
- `n` (optional): Number of candidates to return (1-200, default: 50)

**Response Schema:**
```json
{
  "timestamp": "2025-09-06T20:30:00Z",
  "regime": "trending_bull",
  "total_count": 50,
  "requested": 50,
  "candidates": [
    {
      "symbol": "BTC-USD",
      "exchange": "kraken",
      "score": 87.4,
      "rank": 1,
      "gate_status": {
        "score_gate": true,
        "vadr_gate": true,
        "funding_gate": true,
        "spread_gate": true,
        "depth_gate": true,
        "fatigue_gate": true,
        "freshness_gate": true,
        "overall_passed": true,
        "failure_reasons": []
      },
      "microstructure": {
        "spread_bps": 2.5,
        "depth_usd": 250000,
        "vadr": 2.8,
        "volume_24h": 1500000000,
        "last_price": 50000.0,
        "bid_price": 49993.75,
        "ask_price": 50006.25
      },
      "attribution": {
        "momentum_score": 43.7,
        "technical_score": 17.48,
        "volume_score": 13.11,
        "quality_score": 8.74,
        "social_bonus": 4.37,
        "weight_profile": "trending_bull"
      },
      "last_updated": "2025-09-06T20:29:30Z"
    }
  ],
  "summary": {
    "passed_all_gates": 15,
    "avg_score": 68.2,
    "median_score": 65.8,
    "top_decile_threshold": 82.1,
    "gate_pass_rates": {
      "score": 0.85,
      "vadr": 0.92,
      "funding": 0.78,
      "spread": 0.88,
      "depth": 0.94,
      "fatigue": 0.96,
      "freshness": 0.91
    }
  }
}
```

**Gate Status Fields:**
- `score_gate`: Score ≥ 75 ✓
- `vadr_gate`: VADR ≥ 1.8 ✓
- `funding_gate`: Funding divergence ≥ 2σ ✓
- `spread_gate`: Spread < 50bps ✓
- `depth_gate`: Depth ≥ $100k within ±2% ✓
- `fatigue_gate`: Not fatigued (RSI4h < 70 or acceleration ↑) ✓
- `freshness_gate`: ≤2 bars old & within 1.2×ATR(1h) ✓

**cURL Examples:**
```bash
# Get default 50 candidates
curl http://localhost:8080/candidates

# Get top 10 candidates
curl http://localhost:8080/candidates?n=10

# Get maximum 200 candidates
curl http://localhost:8080/candidates?n=200

# Pretty-print JSON
curl -s http://localhost:8080/candidates?n=5 | jq
```

### GET /explain/{symbol}

Returns comprehensive explainability information for a specific symbol from artifacts or live data.

**Path Parameters:**
- `symbol`: Trading pair in format `XXX-USD` (e.g., `BTC-USD`, `ETH-USD`)

**Response Schema:**
```json
{
  "symbol": "BTC-USD",
  "exchange": "kraken",
  "timestamp": "2025-09-06T20:30:00Z",
  "data_source": "live",
  "score": {
    "final_score": 87.4,
    "pre_orthogonal": {
      "momentum": 34.96,
      "technical": 30.59,
      "volume": 21.85,
      "quality": 26.22,
      "social": 8.5
    },
    "post_orthogonal": {
      "momentum": 34.96,
      "technical": 26.00,
      "volume": 15.73,
      "quality": 17.83,
      "social": 8.5
    },
    "weighted_scores": {
      "momentum": 43.70,
      "technical": 17.48,
      "volume": 13.11,
      "quality": 8.74
    },
    "social_bonus": 8.5,
    "calculation_steps": [
      {
        "step": "raw_factors",
        "description": "Initial factor calculations",
        "input": 0.0,
        "output": 87.4,
        "applied": "market_data"
      }
    ]
  },
  "gates": {
    "overall": true,
    "score_gate": {
      "passed": true,
      "threshold": 75.0,
      "actual_value": 87.4,
      "margin": 12.4,
      "description": "Composite score must be >= 75",
      "last_checked": "2025-09-06T20:30:00Z"
    }
  },
  "factors": {
    "momentum_core": {
      "raw_score": 34.96,
      "weight": 50.0,
      "weighted_score": 43.70,
      "timeframes": {
        "1h": 15.2,
        "4h": 22.8,
        "12h": 18.5,
        "24h": 25.1
      },
      "protected": true,
      "confidence": 0.87
    }
  },
  "regime": {
    "current_regime": "trending_bull",
    "regime_weights": {
      "momentum": 50.0,
      "technical": 20.0,
      "volume": 15.0,
      "quality": 10.0,
      "catalyst": 5.0
    },
    "confidence": 0.84,
    "last_switch": "2025-09-06T16:30:00Z"
  },
  "attribution": {
    "total_contributions": {
      "momentum": 43.7,
      "technical": 14.48,
      "volume": 9.44,
      "quality": 5.96,
      "social": 10.0
    },
    "performance_metrics": {
      "total_duration": 210000000,
      "cache_hits": 3,
      "cache_misses": 1,
      "api_calls_made": 2,
      "data_freshness": 30000000000
    }
  }
}
```

**Data Sources:**
- `artifacts`: Loaded from `C:\wallet\artifacts\` JSON files
- `live`: Generated from current scan pipeline

**Key Features:**
- **Protected MomentumCore**: Never orthogonalized
- **Gram-Schmidt Sequence**: Technical → Volume → Quality → Social
- **Social Capping**: Strictly limited to +10 points
- **Regime Attribution**: Shows weight profile influence
- **Performance Tracking**: Cache hits, API calls, latencies

**cURL Examples:**
```bash
# Explain BTC
curl http://localhost:8080/explain/BTC-USD

# Explain ETH with pretty printing
curl -s http://localhost:8080/explain/ETH-USD | jq

# Check if data comes from artifacts or live
curl -s http://localhost:8080/explain/SOL-USD | jq '.data_source'

# Get just the score breakdown
curl -s http://localhost:8080/explain/BTC-USD | jq '.score'

# Get gate evaluation details
curl -s http://localhost:8080/explain/BTC-USD | jq '.gates'
```

### GET /regime

Returns current regime information, weights, and recent switching history.

**Response Schema:**
```json
{
  "timestamp": "2025-09-06T20:30:00Z",
  "current_regime": "trending_bull",
  "regime_numeric": 1.0,
  "health": {
    "volatility_7d": 0.42,
    "above_ma_pct": 0.68,
    "breadth_thrust": 0.24,
    "stability_score": 0.87
  },
  "weights": {
    "momentum": 50.0,
    "technical": 20.0,
    "volume": 15.0,
    "quality": 10.0,
    "catalyst": 5.0
  },
  "switches_today": 1,
  "avg_duration_hours": 19.5,
  "next_evaluation": "2025-09-07T00:00:00Z",
  "history": [
    {
      "timestamp": "2025-09-06T16:30:00Z",
      "from_regime": "choppy",
      "to_regime": "trending_bull",
      "trigger": "volatility_threshold",
      "confidence": 0.85,
      "duration": 21600000000000
    }
  ]
}
```

**Regime Types:**
- `trending_bull` (numeric: 1.0): High momentum, trending markets
- `choppy` (numeric: 0.0): Ranging, low-momentum markets
- `high_vol` (numeric: 2.0): High volatility, uncertain direction

**Weight Profiles:**
- **Trending Bull**: 50% momentum, 20% technical, 15% volume, 10% quality, 5% catalyst
- **Choppy**: 35% momentum, 30% technical, 15% volume, 15% quality, 5% catalyst  
- **High Vol**: 30% momentum, 25% technical, 20% volume, 20% quality, 5% catalyst

**Evaluation Schedule:** Every 4 hours (00:00, 04:00, 08:00, 12:00, 16:00, 20:00 UTC)

**cURL Examples:**
```bash
# Get current regime
curl http://localhost:8080/regime

# Get just the current regime name
curl -s http://localhost:8080/regime | jq '.current_regime'

# Get regime weights
curl -s http://localhost:8080/regime | jq '.weights'

# Get regime health indicators
curl -s http://localhost:8080/regime | jq '.health'

# Get switching history
curl -s http://localhost:8080/regime | jq '.history'

# Check when next evaluation occurs
curl -s http://localhost:8080/regime | jq '.next_evaluation'
```

### GET /metrics

Returns comprehensive system metrics including API health, circuit breakers, cache hit rates, latencies, and risk envelope data.

**Response:** Detailed metrics in JSON format with operational dashboards.

### GET /decile

Returns score vs forward returns decile analysis for performance validation.

**Response:** Decile performance breakdown with statistical analysis.

### GET /risk

Returns risk envelope monitoring dashboard with position limits, drawdown tracking, and emergency controls.

**Response:** Risk management metrics and alerts.

## Error Handling

All endpoints return consistent error responses:

```json
{
  "error": "invalid_parameter",
  "code": "VALIDATION_ERROR",
  "message": "Parameter 'n' must be an integer between 1 and 200",
  "details": "limit=invalid",
  "timestamp": "2025-09-06T20:30:00Z"
}
```

**Common Error Codes:**
- `400 Bad Request`: Invalid parameters or malformed requests
- `404 Not Found`: Symbol not found or endpoint doesn't exist
- `405 Method Not Allowed`: Only GET methods supported
- `500 Internal Server Error`: Server processing error
- `503 Service Unavailable`: System degraded or overloaded

## Rate Limits & Caching

**Rate Limits:** None for local access (127.0.0.1)

**Caching:**
- `/health`: No cache (`no-cache`)
- `/candidates`: No cache (`no-cache`) - live data
- `/explain`: 1 minute cache (`max-age=60`)
- `/regime`: 30 seconds cache (`max-age=30`)
- `/metrics`: No cache (`no-cache`)

## Performance Targets

**P95 Latency Targets:**
- `/candidates`: < 300ms
- `/explain`: < 300ms  
- `/regime`: < 100ms
- `/health`: < 50ms

**Load Test Baseline:**
```bash
# Install Apache Bench
apt-get install apache2-utils

# Test candidates endpoint
ab -n 1000 -c 10 http://localhost:8080/candidates?n=20

# Test explain endpoint
ab -n 500 -c 5 http://localhost:8080/explain/BTC-USD

# Test regime endpoint
ab -n 2000 -c 20 http://localhost:8080/regime
```

## Configuration

The HTTP server is configured via command-line flags:

```bash
./cryptorun monitor --host 127.0.0.1 --port 8080
```

**Environment Variables:**
- `CRYPTORUN_HTTP_HOST`: Override default host (default: 127.0.0.1)
- `CRYPTORUN_HTTP_PORT`: Override default port (default: 8080)
- `CRYPTORUN_HTTP_TIMEOUT`: Request timeout in seconds (default: 30)

## UX MUST — Live Progress & Explainability

This API provides real-time visibility into CryptoRun's decision-making process:

**Live Progress:**
- `/candidates` shows current top candidates with real-time gate status
- `/regime` displays active regime and weight adjustments every 4 hours
- `/explain` provides step-by-step scoring attribution with performance metrics

**Explainability Features:**
- **Score Attribution**: Breakdown of momentum, technical, volume, quality, and social contributions
- **Gate Evaluation**: Detailed pass/fail status for all entry gates with thresholds and margins
- **Gram-Schmidt Transparency**: Shows before/after orthogonalization effects (momentum protected)
- **Regime Influence**: Weight profile application and regime detection indicators
- **Performance Tracking**: Cache efficiency, API call counts, and calculation duration

**Operational Intelligence:**
- Real-time microstructure validation (spread, depth, VADR)
- Fatigue and freshness guard status
- Social factor capping enforcement (+10 limit)
- Circuit breaker and rate limit monitoring

## Integration Examples

### Python Client
```python
import requests
import json

# Get top candidates
response = requests.get('http://localhost:8080/candidates?n=10')
candidates = response.json()

for candidate in candidates['candidates']:
    if candidate['gate_status']['overall_passed']:
        print(f"{candidate['symbol']}: {candidate['score']:.1f}")

# Explain specific symbol
btc_explanation = requests.get('http://localhost:8080/explain/BTC-USD').json()
print(f"BTC Score: {btc_explanation['score']['final_score']}")
print(f"Attribution: {btc_explanation['attribution']['total_contributions']}")

# Check regime
regime = requests.get('http://localhost:8080/regime').json()
print(f"Current Regime: {regime['current_regime']}")
print(f"Weights: {regime['weights']}")
```

### JavaScript/Node.js Client
```javascript
const axios = require('axios');

async function getCandidates(limit = 10) {
  const response = await axios.get(`http://localhost:8080/candidates?n=${limit}`);
  return response.data;
}

async function explainSymbol(symbol) {
  const response = await axios.get(`http://localhost:8080/explain/${symbol}`);
  return response.data;
}

async function getCurrentRegime() {
  const response = await axios.get('http://localhost:8080/regime');
  return response.data;
}

// Usage
getCandidates(5).then(data => {
  console.log(`Found ${data.candidates.length} candidates in ${data.regime} regime`);
  data.candidates.forEach(c => {
    console.log(`${c.symbol}: ${c.score} (${c.gate_status.overall_passed ? 'PASS' : 'FAIL'})`);
  });
});
```

### Bash/curl Scripts
```bash
#!/bin/bash

# Monitor regime changes
REGIME=$(curl -s http://localhost:8080/regime | jq -r '.current_regime')
echo "Current regime: $REGIME"

# Get passing candidates
PASSING=$(curl -s http://localhost:8080/candidates?n=20 | jq '.candidates[] | select(.gate_status.overall_passed) | .symbol' | wc -l)
echo "Candidates passing all gates: $PASSING"

# Check system health
HEALTH=$(curl -s http://localhost:8080/health | jq -r '.status')
echo "System health: $HEALTH"

# Top 5 candidates with scores
curl -s http://localhost:8080/candidates?n=5 | jq '.candidates[] | "\(.symbol): \(.score)"'
```

## Security & Access Control

**Network Security:**
- Local-only binding (127.0.0.1) by default
- No authentication required for local access
- Read-only operations (no state modification)
- No sensitive data exposure (scores and analysis only)

**Data Protection:**
- No API keys or credentials in responses
- No personal or financial data
- Market data only (public information)
- Structured logging without secrets

**Production Considerations:**
- Consider reverse proxy (nginx) for production deployment
- Enable HTTPS for external access
- Implement rate limiting for external clients
- Add monitoring and alerting for API health

---

*This documentation covers CryptoRun v3.2.1 HTTP API. For updates and additional endpoints, see the latest release notes in CHANGELOG.md.*