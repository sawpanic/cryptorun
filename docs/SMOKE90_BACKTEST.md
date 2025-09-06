# Smoke90 Backtest System

## Overview

The Smoke90 backtest is CryptoRun's comprehensive 90-day cache-only validation system that tests the unified scanner end-to-end using historical data. It validates the entire trading pipeline from candidate selection through guard evaluation, microstructure checks, and provider operations.

## UX MUST — Live Progress & Explainability

The Smoke90 system provides real-time progress indicators and comprehensive explanations:

- **Live Progress**: Window-by-window processing with percentage completion
- **Guard Attribution**: Detailed pass/fail reasons for each guard type
- **Provider Health**: Throttling and circuit breaker event tracking
- **P99 Relaxation**: Late-fill guard relaxation events with timestamps
- **TopGainers Alignment**: Hit/miss rate against market references
- **Comprehensive Reports**: Markdown and JSON artifacts with full methodology

## Architecture

### Core Components

```
Smoke90 Backtest
├── Runner (main orchestrator)
├── Metrics (statistical collection)
├── Writer (artifact generation)
└── Types (data structures)
```

### Data Flow

1. **Window Generation**: Creates 90-day timeline with configurable stride (default 4h)
2. **Candidate Loading**: Loads cached candidates or generates mock data for each window
3. **Pipeline Processing**: Each candidate goes through the complete unified pipeline:
   - Unified scoring (Score ≥ 75 threshold)
   - Hard gates (VADR ≥ 1.8×, funding divergence)
   - Guards evaluation (freshness, fatigue, late-fill)
   - Microstructure validation (spread, depth, VADR)
   - Provider simulation (throttling, circuit breakers)
4. **Metrics Collection**: Aggregates statistics across all windows
5. **Artifact Generation**: Creates JSONL, markdown, and JSON reports

## Key Features

### Cache-Only Operation
- **No Live Fetches**: Operates entirely on cached/historical data
- **Explicit SKIP Reasons**: Clear explanations when data is unavailable
- **Deterministic Results**: Reproducible outcomes for testing

### Comprehensive Testing
- **Unified Scanner**: Tests the complete scan → guard → microstructure pipeline
- **Guard Attribution**: Tracks which guards pass/fail and why
- **P99 Relaxation**: Simulates late-fill guard relaxation under high latency
- **Provider Operations**: Models throttling, circuit breakers, and budget limits

### Performance Analysis
- **TopGainers Alignment**: Compares results against CoinGecko top gainers
- **Hit Rate Metrics**: 1h, 24h, and 7d alignment percentages
- **Guard Pass Rates**: Statistical analysis of guard effectiveness
- **Provider Health**: Throttling frequency and circuit breaker events

## Usage

### CLI Interface

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

### Menu Interface

Access via the interactive menu:
```
Main Menu → 3. Backtest → 1. Run Smoke90
```

### Flag Options

| Flag | Default | Description |
|------|---------|-------------|
| `--top-n` | 20 | Top N candidates per window |
| `--stride` | 4h | Time stride between windows |
| `--hold` | 24h | Hold period for P&L calculation |
| `--output` | "out/backtest" | Output directory for results |
| `--use-cache` | true | Use cached data only (no live fetches) |
| `--progress` | "auto" | Progress output mode (auto/plain/json) |

## Configuration

### Default Configuration

```go
Config{
    TopN:      30,
    Stride:    4 * time.Hour,
    Hold:      48 * time.Hour,
    Horizon:   90 * 24 * time.Hour,
    UseCache:  true,
    Progress:  false,
    OutputDir: "./artifacts/smoke90",
}
```

### Customization

The system supports:
- **Variable Window Sizes**: Adjust stride for different time resolutions
- **Sample Size Control**: TopN parameter controls candidates per window
- **Hold Period Flexibility**: Configure P&L calculation timeframe
- **Output Directory**: Specify custom artifact location

## Methodology

### Unified Scanner Validation

The backtest validates the complete CryptoRun trading pipeline:

1. **Unified Scoring**: Candidates must achieve Score ≥ 75
2. **Hard Gates**: 
   - VADR ≥ 1.8× requirement
   - Funding divergence presence check
3. **Guards Pipeline**: 
   - Freshness guard (≤2 bars old, within 1.2×ATR)
   - Fatigue guard (24h momentum vs RSI4h thresholds)
   - Late-fill guard (≤30s delay) with P99 relaxation
4. **Microstructure Validation**: 
   - Spread <50bps requirement
   - Depth ≥$100k within ±2% requirement
   - VADR validation across venues
5. **Provider Operations**: 
   - Rate limiting simulation
   - Circuit breaker modeling
   - Daily budget tracking

### Guard Attribution

Each candidate's guard results are tracked with detailed reasons:
- **Hard Guards**: Fail immediately on violation
- **Soft Guards**: Allow with warning/notification
- **P99 Relaxation**: Late-fill guard relaxation under high latency conditions
- **Relax Events**: Timestamp and reason tracking for all relaxations

### TopGainers Alignment

Results are compared against CoinGecko top gainers:
- **1h Window**: Short-term momentum alignment
- **24h Window**: Daily performance correlation
- **7d Window**: Weekly trend alignment
- **Hit Rate**: Percentage of scan results matching top gainers
- **Correlation Metrics**: Statistical alignment measures

## Output Artifacts

### Directory Structure

```
out/backtest/
├── 2025-01-15/           # Date-stamped results
│   ├── results.jsonl     # Complete window results
│   ├── report.md         # Comprehensive markdown report
│   └── summary.json      # Compact summary statistics
└── latest -> 2025-01-15/ # Symlink to most recent
```

### Results JSONL Format

Each line contains a complete window result:
```json
{
  "timestamp": "2025-01-15T12:00:00Z",
  "candidates": [
    {
      "symbol": "BTCUSD",
      "score": 78.5,
      "passed": true,
      "guard_result": {
        "freshness": {"type": "hard", "passed": true, "reason": "within base threshold"},
        "fatigue": {"type": "hard", "passed": true, "reason": "24h momentum 8.5% ≤ 15.0%"},
        "late_fill": {"type": "hard", "passed": true, "reason": "within base threshold: 25.0s ≤ 30.0s"}
      },
      "micro_result": {
        "passed": true,
        "reason": "Passed on 2/3 venues",
        "venues": ["binance", "okx"]
      },
      "pnl": 3.24
    }
  ],
  "guard_pass_rate": 85.0,
  "throttle_events": [],
  "relax_events": []
}
```

### Markdown Report Structure

The comprehensive report includes:
- **Executive Summary**: Coverage, pass rates, error counts
- **TopGainers Alignment**: Hit rates by timeframe
- **Guard Attribution**: Pass/fail statistics by guard type
- **P99 Relaxation Events**: Frequency and impact analysis
- **Provider Throttling**: Rate limiting event breakdown
- **Skip Analysis**: Reasons for window skips
- **Performance Analysis**: Throughput and efficiency metrics
- **Methodology**: Complete validation approach
- **Limitations**: Cache-only disclaimers

### Summary JSON Structure

Compact overview for programmatic access:
```json
{
  "timestamp": "2025-01-15T12:00:00Z",
  "period": "2024-10-17 to 2025-01-15",
  "coverage": 94.8,
  "total_candidates": 8420,
  "pass_rate": 76.3,
  "error_count": 24,
  "artifacts": {
    "results": "out/backtest/2025-01-15/results.jsonl",
    "report": "out/backtest/2025-01-15/report.md",
    "summary": "out/backtest/2025-01-15/summary.json"
  }
}
```

## Testing and Validation

### Unit Tests

Comprehensive test coverage includes:
- **Runner Creation**: Configuration validation
- **Clock Injection**: Deterministic time handling for tests
- **Short Duration Tests**: Fast validation with minimal data
- **Artifact Generation**: Output file creation and format validation

### Integration Tests

- **CLI Integration**: End-to-end command execution
- **Menu Integration**: Interactive interface testing
- **Configuration Tests**: Flag parsing and validation
- **Error Handling**: Graceful failure scenarios

### Performance Requirements

- **Cache-Only Speed**: Sub-second processing for small test runs
- **Memory Efficiency**: Handles 90-day datasets without excessive RAM usage
- **Artifact Size**: Reasonable output file sizes with compression options
- **Progress Reporting**: Real-time feedback during long runs

## Error Handling

### Cache Miss Scenarios

When cached data is unavailable:
- **Explicit SKIP**: Clear reason in window results
- **Graceful Degradation**: Continue processing other windows
- **Statistical Tracking**: Count and categorize skip reasons

### Provider Simulation Errors

- **Circuit Breaker Events**: Modeled failures with recovery
- **Throttling Simulation**: Rate limit violations with backoff
- **Budget Exhaustion**: Daily limit reached scenarios

### Validation Failures

- **Schema Validation**: Ensure proper result structure
- **Data Completeness**: Verify required fields present
- **Metrics Consistency**: Cross-check aggregated statistics

## Best Practices

### Configuration Selection

- **Development**: Use small TopN (5-10) and short stride (1h) for fast iteration
- **Validation**: Standard settings (TopN=20, Stride=4h) for thorough testing
- **Stress Testing**: Large TopN (30+) and dense stride (2h) for comprehensive coverage

### Interpreting Results

- **Pass Rate**: Target ≥70% for healthy scanner performance
- **TopGainers Alignment**: Target ≥60% hit rate across timeframes
- **Guard Balance**: No single guard should fail >30% of candidates
- **Error Rate**: Target <5% cache miss/error rate

### Troubleshooting

Common issues and solutions:
- **Low Pass Rate**: Check guard thresholds and market conditions
- **High Skip Rate**: Verify cache data availability for test period
- **Poor Alignment**: Review scoring methodology and regime settings
- **Performance Issues**: Reduce TopN or increase stride for faster runs

## Future Enhancements

### Planned Features

- **Multi-Timeframe Analysis**: Parallel backtests across different horizons
- **Regime-Aware Testing**: Separate validation for different market conditions
- **Monte Carlo Simulation**: Statistical confidence intervals for results
- **Comparative Analysis**: A/B testing different configuration sets

### Integration Opportunities

- **CI/CD Pipeline**: Automated backtesting on code changes
- **Performance Monitoring**: Continuous validation tracking
- **Alert System**: Notification on significant performance degradation
- **Dashboard Integration**: Real-time backtest status and trends

## Conclusion

The Smoke90 backtest system provides comprehensive end-to-end validation of CryptoRun's trading pipeline. By using cache-only data and explicit SKIP reasons, it enables reproducible testing while maintaining realistic market conditions. The detailed attribution and comprehensive reporting make it an essential tool for validating scanner performance and ensuring system reliability.