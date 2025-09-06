# CryptoRun CLI Guide

CryptoRun provides a comprehensive command-line interface for cryptocurrency momentum scanning, quality assurance, pair management, and system monitoring.

## UX MUST — Live Progress & Explainability

All scan operations provide real-time progress streaming with explainability artifacts:
- **Live Progress**: Phase indicators (init→fetch→analyze→orthogonalize→filter→complete) 
- **Explainability**: Detailed attribution and methodology tracking in JSON outputs
- **Transparency**: Source data, processing time, confidence scores, and guard results

## Quick Start

```bash
# Build the CLI
go build -o cryptorun.exe ./cmd/cryptorun

# Run momentum scan with progress streaming
cryptorun scan momentum --progress plain --max-sample 5

# Run QA suite with verification
cryptorun qa --verify --progress auto

# Start monitoring server
cryptorun monitor --port 8080
```

## Core Commands

### Scan Operations

#### `cryptorun scan momentum`
Multi-timeframe momentum scanning with Gram-Schmidt orthogonalization.

```bash
cryptorun scan momentum [flags]

Flags:
  --venues string     Comma-separated venue list (default "kraken,okx,coinbase")
  --max-sample int    Maximum sample size for scanning (default 20)
  --progress string   Progress output mode (auto|plain|json) (default "auto")
  --regime string     Market regime (bull, choppy, high_vol) (default "bull")
  --top-n int         Number of top candidates to select (default 20)
  --ttl int          Cache TTL in seconds (default 300)
```

**Progress Modes:**
- `auto`: Adapts based on terminal detection (plain for interactive, JSON for CI)
- `plain`: Human-readable with emoji phase indicators and real-time feedback
- `json`: Structured JSON events for programmatic consumption

**Output Files:**
- `out/scan/momentum_explain.json`: Explainability artifacts with methodology
- `out/audit/progress_trace.jsonl`: Progress event audit trail

**Example:**
```bash
# Interactive scan with live progress
cryptorun scan momentum --progress plain --venues kraken --max-sample 10

# Automated scan for CI/scripting
cryptorun scan momentum --progress json --regime choppy > scan_results.log
```

#### `cryptorun scan dip`
Quality-dip scanning pipeline (implementation pending - uses momentum baseline).

```bash
cryptorun scan dip [flags]
```

Same flags as momentum scanner. Currently outputs placeholder results with note to use momentum scanner.

### Quality Assurance

#### `cryptorun qa`
Comprehensive QA runner with phases 0-6, provider health metrics, and hardened guards.

```bash
cryptorun qa [flags]

Flags:
  --verify bool         Run acceptance verification (Phase 7) after QA phases (default true)
  --fail-on-stubs bool  Fail early if stubs/scaffolds found in non-test code (default true)
  --progress string     Progress output mode (auto|plain|json) (default "auto")
  --resume bool         Resume from last checkpoint
  --venues string       Comma-separated venue list (default "kraken,okx,coinbase")
  --max-sample int      Maximum sample size for testing (default 20)
  --ttl int            Cache TTL in seconds (default 300)
```

**QA Phases:**
- **Phase 0**: Bootstrap validation and dependency checks
- **Phase 1-6**: Progressive testing with provider guards
- **Phase 7**: Acceptance verification (if --verify enabled)

**Example:**
```bash
# Full QA with verification
cryptorun qa --verify --progress plain

# Quick QA without acceptance phase
cryptorun qa --verify=false --max-sample 5
```

### Pair Management

#### `cryptorun pairs sync`
Discovers USD spot pairs from exchanges with ADV filtering.

```bash
cryptorun pairs sync [flags]

Flags:
  --venue string    Exchange venue (kraken) (default "kraken")
  --quote string    Quote currency filter (default "USD")
  --min-adv int64   Minimum average daily volume in USD (default 100000)
```

**Example:**
```bash
# Sync pairs from Kraken with $100K minimum ADV
cryptorun pairs sync --venue kraken --min-adv 100000
```

### System Monitoring

#### `cryptorun monitor`
HTTP server with health, metrics, and decile endpoints.

```bash
cryptorun monitor [flags]

Flags:
  --host string   HTTP server host (default "0.0.0.0")
  --port string   HTTP server port (default "8080")
```

**Endpoints:**
- `GET /health`: System health status
- `GET /metrics`: Prometheus metrics including provider health
- `GET /decile`: Performance decile analysis

**Example:**
```bash
# Start monitoring server
cryptorun monitor --port 9090

# Check health
curl http://localhost:9090/health

# Get Prometheus metrics
curl http://localhost:9090/metrics
```

### Benchmarking

#### `cryptorun bench topgainers`
Benchmark scanning results against CoinGecko top gainers lists.

```bash
cryptorun bench topgainers [flags]

Flags:
  --progress string   Progress output mode (auto|plain|json) (default "auto")  
  --ttl int          Cache TTL in seconds (minimum 300) (default 300)
  --limit int        Maximum number of top gainers to fetch per window (default 20)
  --windows string   Comma-separated time windows to analyze (default "1h,24h,7d")
```

**Time Windows:**
- `1h`: 1-hour top gainers comparison
- `24h`: 24-hour top gainers comparison  
- `7d`: 7-day top gainers comparison

**Output Files:**
- `out/bench/topgainers_alignment.json`: Complete results for programmatic use
- `out/bench/topgainers_alignment.md`: Human-readable analysis report
- `out/bench/topgainers_{1h,24h,7d}.json`: Per-window detailed breakdowns

**Example:**
```bash
# Basic benchmark with live progress
cryptorun bench topgainers --progress plain

# Custom configuration for CI
cryptorun bench topgainers --limit 10 --windows "1h,24h" --progress json

# Extended caching for development
cryptorun bench topgainers --ttl 1800 --limit 25
```

**Interpretation:**
- **>70% alignment**: High correlation with market gainers
- **30-70% alignment**: Moderate correlation, different strategies  
- **<30% alignment**: Focus on quality over pure momentum

### Utility Commands

#### `cryptorun ship`
Prepare release with results validation and PR creation.

```bash
cryptorun ship --title "Release Title" [--description "Description"] [--dry-run]
```

#### `cryptorun selftest`
Offline resilience self-test validating atomicity, gates, microstructure.

```bash
cryptorun selftest
```

#### `cryptorun digest`
Generate nightly results digest from ledger and daily summaries.

```bash
cryptorun digest [--date YYYY-MM-DD]
```

#### `cryptorun alerts`
Send actionable alerts to Discord/Telegram with deduplication.

```bash
cryptorun alerts [--dry-run] [--send] [--test] [--symbol SYMBOL]
```

#### `cryptorun universe`
Rebuild USD-only trading universe with ADV filtering.

```bash
cryptorun universe [--force] [--dry-run]
```

#### `cryptorun spec`
Run specification compliance suite.

```bash
cryptorun spec [--compact]
```

## Progress Streaming Integration

### Live Progress Features

All scanning operations support real-time progress streaming:

1. **Phase Indicators**: Visual progress through scan phases
2. **Symbol Tracking**: Per-symbol processing status
3. **Metrics Display**: Key metrics like momentum scores, qualification status
4. **Error Reporting**: Real-time error notifications with context

### JSON Event Structure

When using `--progress json`, events follow this structure:

```json
{
  "timestamp": "2025-09-06T13:14:14+03:00",
  "phase": "analyze",
  "symbol": "BTCUSD", 
  "status": "success",
  "progress": 75,
  "total": 5,
  "current": 4,
  "message": "Symbol analyzed",
  "metrics": {
    "momentum_score": 2.45,
    "qualified": true
  }
}
```

### Progress Persistence

All progress events are automatically written to `out/audit/progress_trace.jsonl` for:
- **Audit Trails**: Complete record of scan operations
- **Debugging**: Detailed error context and timing
- **Analytics**: Performance analysis and optimization
- **Compliance**: Full transparency of processing steps

## Best Practices

### Production Usage

```bash
# Structured logging for production
cryptorun scan momentum --progress json --venues kraken,okx > production.log 2>&1

# Health monitoring
cryptorun monitor --host 0.0.0.0 --port 8080 &

# Automated QA in CI/CD
cryptorun qa --progress json --verify --fail-on-stubs
```

### Development Workflow

```bash
# Interactive development with live feedback
cryptorun scan momentum --progress plain --max-sample 3

# Quick QA validation
cryptorun qa --progress plain --max-sample 5 --verify=false

# Monitor system during development
cryptorun monitor --port 8080
```

### Error Handling

- **Invalid Commands**: CLI shows help text with available options
- **Invalid Flags**: Clear error messages with correction suggestions  
- **Connection Issues**: Automatic retry with exponential backoff
- **Rate Limits**: Built-in throttling and circuit breakers

## Configuration

The CLI respects configuration files in `config/` directory:
- `config/universe.json`: Trading pair universe
- `config/apis.yaml`: API endpoint configurations
- `config/cache.yaml`: Cache TTL settings
- `config/circuits.yaml`: Circuit breaker thresholds

## Integration with Monitoring

The CLI integrates with the monitoring system:
- **Metrics Collection**: Provider health metrics automatically collected
- **Health Endpoints**: Real-time system status via HTTP endpoints
- **Prometheus Export**: Metrics in Prometheus format for monitoring stacks
- **Alert Integration**: Structured events for alert processing