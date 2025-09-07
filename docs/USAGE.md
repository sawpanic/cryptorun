# Usage Guide

## UX MUST — Live Progress & Explainability

Comprehensive usage guide with real-time command examples, live progress indicators, and detailed explainability for all CryptoRun operations.

## Quick Start

### Build CryptoRun
```bash
# From project root
go build ./cmd/cryptorun

# Or from src directory  
cd src && go build ./cmd/cryptorun
```

### Basic Commands

#### Scan for Momentum Opportunities
```bash
# Basic scan with dry-run mode
./cryptorun scan --exchange kraken --pairs USD-only --dry-run

# Full scan with top 20 results
./cryptorun scan --exchange kraken --top 20

# Scan specific symbols
./cryptorun scan --symbols BTC-USD,ETH-USD,SOL-USD --dry-run
```

#### System Monitoring
```bash
# Start monitoring server (serves /health, /metrics, /decile)
./cryptorun monitor

# Check system health
./cryptorun health

# Run comprehensive self-diagnostics
./cryptorun selftest
```

#### Generate Reports
```bash
# Generate performance digest for specific date
./cryptorun digest --date 2025-09-01

# Generate backtest report
./cryptorun backtest --start 2025-08-01 --end 2025-09-01
```

## Command Reference

### `scan` - Momentum Scanning

**Purpose**: Identify cryptocurrency momentum opportunities using CryptoRun's 100-point composite scoring system.

```bash
./cryptorun scan [flags]
```

**Flags:**
- `--exchange` (string): Exchange to scan (default: "kraken")
- `--pairs` (string): Pair filter ("USD-only", "ALL") (default: "USD-only") 
- `--top` (int): Number of top results to show (default: 10)
- `--dry-run`: Enable dry-run mode without live data
- `--symbols` (string): Comma-separated symbols to scan
- `--blacklist` (string): Comma-separated symbols to exclude
- `--regime` (string): Force specific regime ("bull", "choppy", "volatile")
- `--output` (string): Output format ("json", "csv", "table") (default: "table")
- `--file` (string): Output file path
- `--timeout` (duration): Scan timeout (default: "30s")

**Examples:**
```bash
# Scan USD pairs with regime detection
./cryptorun scan --exchange kraken --pairs USD-only

# Export top 20 to JSON
./cryptorun scan --top 20 --output json --file results.json

# Force bull market regime
./cryptorun scan --regime bull --top 15

# Exclude specific symbols
./cryptorun scan --blacklist BTC-USD,ETH-USD --top 10
```

### `monitor` - System Monitoring

**Purpose**: Start HTTP monitoring server with health checks and metrics.

```bash
./cryptorun monitor [flags]
```

**Flags:**
- `--port` (int): HTTP port (default: 8080)
- `--metrics-port` (int): Metrics port (default: 8081)
- `--health-port` (int): Health check port (default: 8082)
- `--log-level` (string): Log level ("debug", "info", "warn", "error") (default: "info")

**Endpoints:**
- `GET /health` - System health status
- `GET /metrics` - Prometheus metrics
- `GET /decile` - Decile performance analysis
- `GET /providers` - Provider health status

### `health` - Health Diagnostics

**Purpose**: Check system health and component status.

```bash
./cryptorun health [flags]
```

**Flags:**
- `--json`: Output in JSON format
- `--timeout` (duration): Health check timeout (default: "10s")
- `--providers`: Include provider connectivity checks
- `--verbose`: Show detailed component information

**Example Output:**
```json
{
  "overall": "healthy",
  "timestamp": "2025-09-07T14:30:00Z",
  "components": {
    "database": {"status": "healthy", "latency_ms": 12},
    "cache": {"status": "healthy", "hit_ratio": 0.87},
    "providers": {
      "kraken": {"status": "healthy", "latency_ms": 145, "rate_limit_remaining": 85}
    }
  }
}
```

### `selftest` - Comprehensive Diagnostics  

**Purpose**: Run comprehensive system self-diagnostics and validation.

```bash
./cryptorun selftest [flags]
```

**Flags:**
- `--skip-providers`: Skip provider connectivity tests
- `--skip-database`: Skip database tests
- `--timeout` (duration): Test timeout (default: "60s")

### `backtest` - Historical Analysis

**Purpose**: Run backtests on historical data to validate strategies.

```bash  
./cryptorun backtest [flags]
```

**Flags:**
- `--start` (string): Start date (YYYY-MM-DD)
- `--end` (string): End date (YYYY-MM-DD)  
- `--symbols` (string): Symbols to backtest
- `--regime` (string): Regime override
- `--output` (string): Output format ("json", "csv")

### `digest` - Performance Reports

**Purpose**: Generate daily/weekly performance digests and analysis.

```bash
./cryptorun digest [flags]
```

**Flags:**
- `--date` (string): Date for digest (YYYY-MM-DD) (default: today)
- `--period` (string): Period ("daily", "weekly") (default: "daily")
- `--format` (string): Output format ("markdown", "json") (default: "markdown")

## Configuration

### Environment Variables

```bash
# Core application settings
ENV=production
LOG_LEVEL=info

# Database configuration  
PG_DSN=postgres://user:pass@localhost:5432/cryptorun
REDIS_ADDR=localhost:6379

# API endpoints
KRAKEN_API_BASE=https://api.kraken.com
KRAKEN_WS_URL=wss://ws.kraken.com
OKX_API_BASE=https://www.okx.com/api/v5

# Performance tuning
CACHE_HIT_RATE_TARGET=0.85
SCAN_TIMEOUT=30s
SCAN_LATENCY_TARGET=300ms
```

### Configuration Files

CryptoRun uses YAML configuration files in the `config/` directory:

- `config/apis.yaml` - API provider settings
- `config/cache.yaml` - Cache TTLs and sizes  
- `config/circuits.yaml` - Circuit breaker thresholds
- `config/regimes.yaml` - Market regime parameters
- `config/premove.yaml` - Pre-movement detector settings

## Common Usage Patterns

### Daily Momentum Scan
```bash
# Morning scan routine
./cryptorun scan --exchange kraken --top 15 --output json --file morning_scan.json

# Generate digest for previous day
./cryptorun digest --date $(date -d '1 day ago' '+%Y-%m-%d')
```

### Live Monitoring Setup
```bash
# Terminal 1: Start monitoring server
./cryptorun monitor --log-level info

# Terminal 2: Health check loop
while true; do
  ./cryptorun health --json | jq '.overall'
  sleep 30
done
```

### Regime-Specific Analysis
```bash  
# Bull market scan
./cryptorun scan --regime bull --top 20

# High volatility scan  
./cryptorun scan --regime volatile --symbols $(curl -s api.kraken.com/0/public/AssetPairs | jq -r '.result | keys[]' | head -10 | tr '\n' ',')
```

## Output Formats

### Table Format (Default)
```
Symbol      Score  Momentum  Volume   Regime   Entry    Exit
BTC-USD     87.5   High      Surge    Bull     $45,200  $47,100  
ETH-USD     79.2   Medium    Normal   Bull     $3,180   $3,350
SOL-USD     82.1   High      Surge    Bull     $142     $156
```

### JSON Format
```json
{
  "timestamp": "2025-09-07T14:30:00Z",
  "regime": "bull", 
  "candidates": [
    {
      "symbol": "BTC-USD",
      "score": 87.5,
      "momentum": "high",
      "volume": "surge",
      "entry_price": 45200,
      "exit_targets": [47100, 48500]
    }
  ]
}
```

### CSV Format
```csv
symbol,score,momentum,volume,regime,entry_price,exit_price
BTC-USD,87.5,high,surge,bull,45200,47100
ETH-USD,79.2,medium,normal,bull,3180,3350
```

## Performance Tips

### Optimize Scan Performance
- Use `--dry-run` for testing without live API calls
- Set appropriate `--timeout` based on market conditions
- Use `--symbols` to limit scope for faster results
- Monitor cache hit rates with `/metrics` endpoint

### Monitor System Health
- Check provider health with `./cryptorun health --providers`
- Monitor P99 latency targets (<300ms)
- Verify cache hit rates (>85% target)
- Watch circuit breaker states

### Troubleshooting
- Use `--verbose` flags for detailed output
- Check logs in monitoring mode
- Verify environment variables are set
- Test individual providers with health checks

## Integration Examples

### Shell Scripting
```bash
#!/bin/bash
# Daily scan automation script

SCAN_RESULTS=$(./cryptorun scan --output json --dry-run)
REGIME=$(echo "$SCAN_RESULTS" | jq -r '.regime')

if [ "$REGIME" = "bull" ]; then
    echo "Bull market detected - running extended scan"
    ./cryptorun scan --top 20 --output csv --file "bull_scan_$(date +%Y%m%d).csv"
fi
```

### Python Integration
```python
import subprocess
import json

# Run scan and parse results
result = subprocess.run(['./cryptorun', 'scan', '--output', 'json'], 
                       capture_output=True, text=True)
data = json.loads(result.stdout)

# Process top candidates
for candidate in data['candidates'][:5]:
    print(f"{candidate['symbol']}: {candidate['score']}")
```

For more advanced integration patterns, see [API_INTEGRATION.md](API_INTEGRATION.md) and [DEPLOYMENT.md](DEPLOYMENT.md).
