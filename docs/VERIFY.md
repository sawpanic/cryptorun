# CryptoRun Verification System

## UX MUST — Live Progress & Explainability

The verification system provides comprehensive safety validation with live progress indicators, detailed failure analysis, and compact status reporting. Every verification step shows real-time progress, specific failure reasons, and actionable fix guidance.

## Overview

CryptoRun's verification system ensures system integrity through the **GREEN-WALL** verification suite - a single command that runs the complete safety stack and provides a compact ✅/❌ status display with artifact links.

The system integrates with automated **artifact management** to maintain verification history while controlling disk usage through intelligent retention policies and safe compaction.

## GREEN-WALL Verification Suite

### Command Usage

```bash
# Run complete verification suite
cryptorun verify all --n 30 --progress

# Run with custom timeout
cryptorun verify all --n 50 --timeout 10m --progress

# Run only post-merge verification
cryptorun verify postmerge --n 20 --progress
```

### Verification Steps

The GREEN-WALL suite runs five critical verification steps in sequence:

#### 1. Unit/E2E Tests (`go test ./...`)
- **Purpose**: Validate all unit and integration tests
- **Coverage**: Captures test coverage percentage
- **Includes**: MG-Tests, ME-Proofs, OPS-CB, LAT-P99, Smoke90 tests
- **Timeout**: Fail-fast on first test failure
- **Output**: Pass/fail status with coverage percentage

#### 2. Microstructure Proofs (`cryptorun menu --microstructure --sample 6`)
- **Purpose**: Validate exchange-native L1/L2 orderbook data
- **Sample Size**: 6 assets across multiple venues
- **Validation**: Spread <50bps, depth ≥$100k, VADR >1.75×
- **Artifacts**: Generates proof bundles in `./artifacts/proofs/{date}/`
- **Output**: Pass/fail/unproven counts with artifact links

#### 3. TopGainers Benchmark (`cryptorun bench topgainers`)
- **Purpose**: Sanity check momentum ranking system
- **Windows**: 1h, 4h, 12h, 24h timeframes
- **Sample Size**: Configurable via `--n` parameter
- **Metrics**: Spearman correlation (ρ) and hit rate
- **Output**: Window count with alignment metrics

#### 4. Smoke90 Cached Backtest (`cryptorun backtest smoke90`)
- **Purpose**: Validate end-to-end pipeline performance
- **Configuration**: 30 entries, 4h stride, 48h hold period
- **Cache**: Uses cached data for speed (`--use-cache`)
- **Metrics**: Hit rate, relaxation usage, throttle rate
- **Output**: Entry count with performance metrics

#### 5. Post-merge Verification (`cryptorun verify postmerge`)
- **Purpose**: Ensure system consistency after code changes
- **Checks**: Directory structure, go.mod validity, build success
- **Dependencies**: Runs `go mod tidy` and test compilation
- **Output**: Simple pass/fail status

### GREEN-WALL Output Format

```
● GREEN-WALL — ✅ PASS
  - tests: ✅ pass (coverage 85.7%)
  - microstructure: ✅ 5/0/1 | artifacts: ./artifacts/proofs/2025-09-06/
  - bench topgainers: ✅ 4 windows | alignment ρ=0.753, hit=65.2%
  - smoke90: ✅ 30 entries | hit 58.3% | relax/100 3 | throttle 12.5%
  - postmerge: ✅ pass
  - elapsed: 45.0s
```

### Status Indicators

| Component | ✅ Pass Criteria | ❌ Fail Criteria |
|-----------|------------------|-------------------|
| **tests** | All tests pass | Any test fails |
| **microstructure** | No failed proofs | Any proof failures |
| **bench topgainers** | Windows > 0 | No windows completed |
| **smoke90** | Entries > 0 | No entries processed |
| **postmerge** | All checks pass | Any check fails |

### Error Reporting

When failures occur, the GREEN-WALL displays specific error details:

```
● GREEN-WALL — ❌ FAIL
  - tests: ❌ pass (coverage 45.2%)
  - microstructure: ❌ 2/3/1 | artifacts: none
  - bench topgainers: ❌ 0 windows | alignment ρ=0.000, hit=0.0%
  - smoke90: ❌ 0 entries | hit 0.0% | relax/100 0 | throttle 0.0%
  - postmerge: ❌ pass
  - elapsed: 12.0s
  - errors:
    * tests: coverage too low
    * microstructure: API timeout
    * bench: insufficient data
    * smoke90: cache miss
    * postmerge: build failed
```

## Individual Verification Commands

### Post-merge Verification

```bash
cryptorun verify postmerge --n 20 --progress
```

Validates system state after code changes:

**Directory Structure Checks:**
- `internal/verify/greenwall/` exists
- `cmd/cryptorun/` exists  
- `docs/` exists
- `config/` exists

**Dependency Validation:**
- `go.mod` exists and is readable
- `go mod tidy` succeeds
- Basic compilation succeeds (`go build ./cmd/cryptorun`)

**Sample Output:**
```
Post-merge verification: ✅ pass
```

## Configuration Options

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--n` | 30 | Sample size for tests requiring sample count |
| `--progress` | false | Show progress indicators during execution |
| `--timeout` | 0 | Overall timeout (0 for no timeout) |

### Environment Considerations

The verification system respects existing environment configuration:

- **Cache Settings**: Uses configured Redis/file cache settings
- **API Limits**: Respects rate limiting for external API calls
- **Artifact Paths**: Uses standard `./artifacts/` directory structure
- **Log Levels**: Inherits global logging configuration

## Common Failure Scenarios

### Test Failures
**Symptoms:**
- `tests: ❌ pass (coverage XX.X%)`
- Low coverage percentage

**Solutions:**
- Run `go test ./... -v` for detailed test output
- Check for new code without test coverage
- Verify test fixtures and mock data are up-to-date

### Microstructure Failures
**Symptoms:**
- `microstructure: ❌ X/Y/Z | artifacts: none`
- High failure count in proof validation

**Solutions:**
- Check network connectivity to exchanges
- Verify API rate limits not exceeded  
- Ensure exchange APIs are responding
- Review orderbook data quality

### Benchmark Failures
**Symptoms:**
- `bench topgainers: ❌ 0 windows`
- Low correlation or hit rates

**Solutions:**
- Check market data availability
- Verify momentum calculation logic
- Review time window configurations
- Ensure sufficient historical data

### Backtest Failures
**Symptoms:**
- `smoke90: ❌ 0 entries`
- Cache miss or data unavailability

**Solutions:**
- Run without `--use-cache` to force fresh data
- Check data provider connectivity
- Verify backtest configuration parameters
- Review entry signal generation logic

### Post-merge Failures
**Symptoms:**
- `postmerge: ❌ pass`
- Build or dependency issues

**Solutions:**
- Run `go mod tidy` manually
- Check for missing dependencies
- Verify directory structure is intact
- Ensure all required files are committed

## Performance Characteristics

### Execution Times (Typical)
- **Tests**: 15-30 seconds (depending on coverage)
- **Microstructure**: 10-20 seconds (API-dependent)
- **Benchmark**: 20-45 seconds (sample size dependent)
- **Smoke90**: 30-60 seconds (cache hit ratio dependent)
- **Post-merge**: 5-15 seconds (build complexity dependent)

**Total**: 80-170 seconds for complete GREEN-WALL suite

### Resource Usage
- **CPU**: Moderate during test execution and compilation
- **Memory**: ~100MB peak during test runs
- **Network**: Moderate for API calls (respects rate limits)
- **Disk**: Minimal temporary files, proof artifacts persist

## Integration with CI/CD

### Exit Codes
- **0**: All verification steps passed
- **1**: Any verification step failed

### CI Pipeline Integration

```yaml
- name: Run GREEN-WALL Verification  
  run: cryptorun verify all --n 30 --progress
  timeout-minutes: 5
```

The GREEN-WALL verification can serve as a comprehensive pre-merge gate, ensuring all critical system components are functioning correctly before code integration.

### Artifact Persistence

Verification artifacts are preserved with intelligent retention:

- **Test Coverage**: Available in go test output
- **Microstructure Proofs**: Stored in `./artifacts/proofs/` (last 10 runs retained)
- **Benchmark Results**: Stored in `./artifacts/bench/` (last 10 runs retained)  
- **Backtest Results**: Stored in `./artifacts/smoke90/` (last 8 runs retained)
- **Complete GREEN-WALL**: Stored in `./artifacts/greenwall/` (last 12 runs retained)

## Artifact Management Integration

The verification system seamlessly integrates with CryptoRun's artifact management:

### Automated Cleanup Workflow

```bash
# Run verification and clean up in one workflow
cryptorun verify all --n 30 --progress        # Generate new artifacts
cryptorun artifacts gc --apply                # Remove old artifacts per policy  
cryptorun artifacts compact --apply           # Compress remaining files
```

### Retention Policies

- **Always preserved**: Most recent PASS, most recent run, and pinned artifacts
- **Configurable retention**: Keep last N runs per artifact family
- **Safe deletion**: Files moved to trash with 30-day recovery window
- **Audit trail**: Complete GC reports with operation details

### Disk Usage Control

```bash
# Monitor artifact disk usage
cryptorun artifacts list --verbose

# Preview cleanup before applying
cryptorun artifacts gc --dry-run

# Compact large verification logs
cryptorun artifacts compact --family greenwall --apply
```

See [Artifact Management Guide](ARTIFACTS.md) for detailed configuration and usage.

## Troubleshooting

### Common Issues

**Timeout During Execution:**
- Increase `--timeout` parameter
- Check network connectivity for API-dependent steps
- Verify system resources are sufficient

**Intermittent Failures:**
- Run individual verification steps to isolate issues
- Check for external dependency availability
- Review system logs for detailed error information

**Artifact Access Issues:**
- Verify write permissions to `./artifacts/` directory
- Check disk space availability
- Ensure directory structure exists

**Performance Degradation:**
- Monitor system resources during execution
- Check for competing processes
- Consider reducing sample sizes with `--n` parameter

This verification system ensures CryptoRun maintains high reliability and consistency across all critical components while providing clear visibility into system health.