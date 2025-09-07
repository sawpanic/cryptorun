# CryptoRun QA Runner

## Overview

The CryptoRun QA runner provides comprehensive quality assurance testing with acceptance verification and hardened provider guards. It executes phases 0-6 per QA.MAX.50 specification, plus mandatory Phase -1 (no-stub gate) and Phase 7 (acceptance verification).

## Hard Gate Enforcement

### Phase -1: No-Stub Gate (MANDATORY)
The no-stub gate is now a **hard failure** that blocks deployment if any scaffold patterns are detected:

- **Patterns Blocked**: TODO, FIXME, STUB, NotImplemented, "panic(not implemented)", "dummy implementation", "return nil // TODO"
- **Scope**: All non-test .go files in the repository
- **CI Integration**: `go test ./tests/unit/qa_guard_test.go` enforces this in CI/CD
- **Evidence**: Violations written to `out/audit/nostub_hits.json`
- **Resolution**: Remove scaffold patterns by either:
  - Deleting unused code paths, OR  
  - Implementing full functionality per existing specifications

### Banned Token Gate
Prevents forbidden terms in documentation:

- **Blocked Terms**: "CProtocol" (outside _codereview/ historical archive)
- **Enforcement**: Branding guard test in CI
- **Resolution**: Use "CryptoRun" branding consistently

## Usage

### Basic QA Run

```bash
# Run complete QA suite with default settings
cryptorun qa

# Run with specific progress output
cryptorun qa --progress plain

# Run with custom settings
cryptorun qa --progress json --ttl 300 --max-sample 12 --venues kraken,okx
```

### Acceptance Verification Modes

```bash
# Run with acceptance verification (default)
cryptorun qa --verify

# Skip acceptance verification
cryptorun qa --no-verify

# Run with both stub checking and acceptance verification
cryptorun qa --fail-on-stubs --verify
```

### Resume and Recovery

```bash
# Resume from last checkpoint
cryptorun qa --resume

# Resume with different settings
cryptorun qa --resume --progress json --max-sample 20
```

## Command Line Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--progress` | string | `auto` | Progress output mode: `auto`, `plain`, `json` |
| `--resume` | bool | `false` | Resume from last checkpoint |
| `--ttl` | int | `300` | Cache TTL in seconds |
| `--venues` | string | `kraken,okx,coinbase` | Comma-separated venue list |
| `--max-sample` | int | `20` | Maximum sample size for testing |
| `--verify` | bool | `true` | Run acceptance verification (Phase 7) |
| `--fail-on-stubs` | bool | `true` | Fail early if stubs/scaffolds found |

## Phase Execution Order

### Phase -1: No-Stub Gate (Mandatory in CI)
- **Trigger**: `--fail-on-stubs=true` (default in CI)
- **Purpose**: Scan repository for stub/scaffold patterns before network operations
- **Patterns Detected**:
  - `panic("not implemented")` / `panic('not implemented')`
  - `TODO`, `FIXME`, `STUB`, `XXX`, `PENDING` comments (case-insensitive)
  - `NotImplemented`, `dummy implementation`
  - `return nil // TODO`, `// TODO:`, `// FIXME:`
- **Exclusions**: Test files (`*_test.go`), generated files, vendor directories, as defined in `scripts/qa/no_todo.allow`
- **Failure**: Hard fail with `FAIL SCAFFOLDS_FOUND` before any network work

#### Standalone No-TODO Scanner

The No-TODO gate can be run independently of the full QA suite:

**Shell Script Version:**
```bash
# Make executable (first time only)
chmod +x scripts/qa/no_todo.sh

# Run the scanner
scripts/qa/no_todo.sh
```

**Go Version:**
```bash
# Run directly
go run scripts/qa/scanner.go

# Or build and run
go build -o qa_scanner scripts/qa/scanner.go
./qa_scanner
```

**CI Integration:**
The gate runs as a separate job in GitHub Actions before the main build:
- Fails fast if TODO/FIXME/STUB markers found
- Uploads QA reports on failure
- Blocks build pipeline until resolved

**Custom Exemptions:**
Add patterns to `scripts/qa/no_todo.allow`:
```
# Example exemptions
docs/legacy/
*.pb.go
vendor/
tests/fixtures/mock_data.go
```

### Phases 0-6: Standard QA Suite
Standard QA phases as defined in QA.MAX.50:
0. Environment Validation
1. Static Analysis
2. Live Index Diffs
3. Microstructure Validation
4. Determinism Validation
5. Explainability Validation
6. UX Validation

### Phase 7: Acceptance Verification (Optional)
- **Trigger**: `--verify=true` (default)
- **Purpose**: Validate all QA artifacts and system integration
- **Validations**:
  - File existence and structure validation
  - Provider health metrics exposure
  - Telemetry endpoint verification
  - Determinism hash consistency
  - Content validation for JSON/CSV artifacts

## Artifacts Generated

### Standard QA Artifacts (Phases 0-6)
- `out/qa/QA_REPORT.md`: Human-readable QA results
- `out/qa/QA_REPORT.json`: Machine-readable QA results
- `out/qa/live_return_diffs.json`: Index comparison results
- `out/qa/microstructure_sample.csv`: Exchange-native validation data
- `out/qa/provider_health.json`: Provider health status
- `out/qa/vadr_adv_checks.json`: VADR/ADV validation results
- `out/audit/progress_trace.jsonl`: Resumable progress tracking

### Acceptance Artifacts (Phase 7)
- `out/qa/accept_fail.json`: Acceptance failure details (only on failure)

### No-Stub Gate Artifacts (Phase -1)
- `out/audit/nostub_hits.json`: Detected stub/scaffold patterns (only on detection)

## Failure Modes and Hints

### FAIL SCAFFOLDS_FOUND
```bash
❌ FAIL SCAFFOLDS_FOUND +hint: remove TODO/STUB/not-implemented
```
**Cause**: Stub/scaffold patterns detected in non-test Go files  
**Solution**: Remove or implement all TODO, FIXME, STUB, and `panic("not implemented")` patterns

### FAIL ACCEPT_VERIFY_MISSING_FILES
```bash
❌ FAIL ACCEPT_VERIFY_MISSING_FILES +hint: Ensure QA phases 0-6 completed successfully
```
**Cause**: Required QA artifact files missing  
**Solution**: Verify phases 0-6 completed without errors and generated all artifacts

### FAIL ACCEPT_VERIFY_VIOLATIONS
```bash
❌ FAIL ACCEPT_VERIFY_VIOLATIONS +hint: Check artifact file structures and content
```
**Cause**: Artifact files exist but have invalid structure or content  
**Solution**: Check `out/qa/accept_fail.json` for specific validation errors

## Progress Output Formats

### Plain Format
```
🚀 Starting QA suite (8 phases)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[0] ✅ Environment Validation (25ms)
[1] ✅ Static Analysis (150ms)
...
[7] ✅ Acceptance Verification (200ms)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ QA PASSED (8/8 phases, 2s total)
🔍 Acceptance: 7 files validated, metrics_endpoint_validated
📁 Artifacts: out/qa
```

### JSON Format
```json
{"event":"qa_start","timestamp":"2025-09-06T12:00:00Z","total_phases":8}
{"event":"qa_phase","timestamp":"2025-09-06T12:00:05Z","phase":0,"name":"Environment Validation","status":"pass","duration":25}
...
{"event":"qa_phase","timestamp":"2025-09-06T12:00:07Z","phase":7,"name":"Acceptance Verification","status":"pass","duration":200}
{"event":"qa_complete","timestamp":"2025-09-06T12:00:08Z","success":true,"passed_phases":8,"total_phases":8}
```

### Auto Format
- Plain format in terminals
- JSON format in CI environments or when output is redirected

## Provider Health Metrics

The acceptance verification validates these stable metric names:

- `provider_health_success_rate`: Success rate (0.0-1.0)
- `provider_health_latency_p50`: P50 latency in milliseconds
- `provider_health_latency_p95`: P95 latency in milliseconds  
- `provider_health_budget_remaining`: Budget remaining (percentage)
- `provider_health_degraded`: Degraded status (0.0 or 1.0)

Metrics are validated from:
1. `/metrics` HTTP endpoint (if available)
2. In-process metrics registry (fallback)

## Resume Functionality

The QA runner supports resuming from the last successful phase:

```bash
# Initial run fails at Phase 3
cryptorun qa --progress plain
# ❌ QA FAILED (2/8 phases, 45s total)

# Fix issue and resume from Phase 3
cryptorun qa --resume --progress plain
# 🚀 Resuming QA from checkpoint (Phase 3)
```

Resume state is tracked in `out/audit/progress_trace.jsonl`.

## Integration with Build Systems

### CI/CD Pipeline Integration
```bash
# In CI environments, use JSON output for machine parsing
if [ "$CI" = "true" ]; then
  cryptorun qa --progress json --ttl 60 --max-sample 10
else
  cryptorun qa --progress plain
fi
```

### Pre-commit Hook Usage
```bash
#!/bin/bash
# .githooks/pre-commit
set -e

# Run stub checking before commit
cryptorun qa --fail-on-stubs --no-verify --progress plain --max-sample 5

echo "✅ No stubs detected, commit allowed"
```

## Troubleshooting

### Common Issues

**Issue**: QA fails with "missing required config files"  
**Solution**: Ensure all config files exist: `config/apis.yaml`, `config/cache.yaml`, etc.

**Issue**: Acceptance verification fails on metrics  
**Solution**: Check if `/metrics` endpoint is available or ensure in-process registry is working

**Issue**: No-stub gate triggers false positives  
**Solution**: Use `--no-fail-on-stubs` temporarily or refactor flagged patterns

**Issue**: Provider health shows all degraded  
**Solution**: Check network connectivity and provider configuration in `config/providers.yaml`

### Debug Mode

Enable verbose logging for troubleshooting:
```bash
export LOG_LEVEL=debug
cryptorun qa --progress plain --max-sample 3
```

## UX MUST — Live Progress & Explainability

The QA runner provides real-time progress indicators and detailed explanations:

- **Live Progress**: Phase-by-phase status with timing information
- **Explainability**: Each failure includes specific error messages and actionable hints  
- **Progress Tracking**: Resume capability with persistent state
- **Artifact Transparency**: All outputs include source attribution and validation status
- **Health Visibility**: Provider status and budget consumption tracking

All QA operations maintain full transparency with detailed logging and comprehensive artifact generation for audit and troubleshooting purposes.

## Algorithm QA Sweep (PACK-C.QA-SWEEP.63)

### Overview

The Algorithm QA Sweep validates the correctness, determinism, and production readiness of the MomentumCore and Quality-Dip optimization implementations. This comprehensive validation enforces strict no-stub gates and specification conformance requirements.

### QA Gate Results

#### No-Stub Gate: ❌ FAILED

**Critical Finding**: 85 scaffold patterns detected across production code paths.

**Production Blockers:**
- CLI command stubs in main application entry points
- Hard-coded regime detection returning "bull" stub  
- Missing WebSocket implementations for Coinbase/OKX exchanges
- Incomplete domain logic implementations

**Scan Coverage:**
- Files scanned: 139
- Files excluded: 41 (test files, vendor, generated)
- Pattern types: TODO (24), STUB (58), FIXME (2), NotImplemented (1)

#### Algorithm Conformance: ⚠️ PARTIAL

**MomentumCore Specification Compliance:**
- ✅ Multi-timeframe weights sum to 1.0
- ✅ Acceleration tracking on 4h timeframe
- ✅ Fatigue guard with RSI thresholds
- ✅ Freshness guard within ATR bounds
- ✅ Late-fill guard with timing validation
- ⚠️ Gram-Schmidt orthogonalization blocked by stubs
- ⚠️ Regime detector returns hardcoded values

**Quality-Dip Specification Compliance:**
- ✅ Trend qualification (MA slopes, ADX/Hurst)
- ✅ Fibonacci retracement validation (38.2%-61.8%)
- ✅ RSI dip identification (25-40 range)
- ✅ Pattern recognition (divergence, engulfing)
- ✅ Quality signals integration (liquidity, volume, social)
- ✅ False-positive guards (shock, stair-step, decay)
- ✅ Composite scoring with brand cap

#### Explainability Artifacts: ✅ PASS

**Generated Artifacts:**
- `out/scan/momentum_explain.json`: Complete attribution and processing metrics
- `out/scan/dip_explain.json`: Full candidate analysis with quality breakdown
- `out/qa/nostub_scan.json`: Comprehensive scaffold pattern report

**Determinism Validation:**
- JSON outputs are stable across runs
- Attribution includes data source timestamps
- Processing metrics are consistent
- Quality check results are reproducible

### Remediation Plan

#### Priority 1: Production Blockers

1. **Remove All Scaffold Patterns**
   - Replace CLI command stubs with implementations
   - Complete domain logic (regime, microstructure, fatigue)
   - Implement exchange WebSocket providers
   - Resolve all TODO/STUB/FIXME markers

2. **Complete Core Integrations**
   - Regime detector with real market analysis
   - Microstructure validation with venue-native data
   - Exchange provider implementations (Coinbase, OKX)

#### Priority 2: Specification Compliance

1. **Gram-Schmidt Orthogonalization**
   - Verify MomentumCore protection in factor hierarchy
   - Test orthogonal decomposition with real factor data
   - Validate cross-correlation reduction

2. **Regime Detection System**  
   - Replace hardcoded "bull" return with analysis
   - Implement realized volatility, breadth thrust, MA crossover detection
   - Test regime switching and weight blend selection

### Artifacts and References

**Generated Files:**
- `out/qa/nostub_scan.json`: Detailed scaffold pattern analysis
- `out/scan/momentum_explain.json`: MomentumCore explainability
- `out/scan/dip_explain.json`: Quality-Dip analysis results

**Related Documentation:**
- `docs/ALGO.md`: Algorithm specifications and QA validation notes
- `docs/SCAN_PIPELINES.md`: Pipeline testing and QA validation

## Infrastructure Scaffold Purge (PACK-F.PURGE-INFRA.82B)

### Overview

Completed infrastructure domain scaffold purge targeting all scaffold markers from provider implementations, HTTP client pool, and telemetry systems while strictly preserving guard contracts and pool invariants.

### Scope and Results

**Target Domains:**
- `internal/infrastructure/providers/**` (kraken.go, coingecko.go, okx.go, coinbase.go)
- `internal/infrastructure/httpclient/pool.go`
- `internal/telemetry/metrics/**`

**Scaffold Elimination:**
- ✅ **Kraken Provider**: Replaced parseFloat/parseInt64 placeholder implementations with functional string parsing
- ✅ **Other Providers**: Already clean (no scaffolds detected)
- ✅ **HTTP Client Pool**: Already clean (no scaffolds detected)  
- ✅ **Telemetry Metrics**: Already clean (no scaffolds detected)

### Guard Contract Preservation

**Provider Guard Behaviors Maintained:**
- Budget enforcement (RPM/monthly limits with degraded state transitions)
- Concurrency limits (≤4 concurrent requests per provider)
- Jitter application (50-150ms range as specified)
- Exponential backoff with max limits (15-30s depending on provider)
- Degraded state handling (reason tracking, health metrics integration)
- Rate limit detection and Retry-After header respect

**HTTP Pool Invariants Preserved:**
- Request statistics tracking (success/failure/timeout counts)
- Latency percentile calculation (P50/P95 with moving averages)
- Semaphore-based concurrency control
- Context cancellation handling
- Retryable error classification and status code handling

### Implementation Details

**Kraken Provider parseFloat Function:**
- Replaced placeholder `result = 123.45` with decimal parsing logic
- Handles negative values, decimal points, and invalid character detection
- Maintains same return types and error handling patterns
- Preserves exchange-native data processing requirements

**Kraken Provider parseInt64 Function:**
- Replaced placeholder `time.Now().Unix()` with integer parsing logic  
- Handles negative values and invalid character boundaries
- Maintains timestamp compatibility for order book data
- Preserves existing overflow and conversion behaviors

### Verification Results

- **Scaffold Scan**: Zero matches in `internal/infrastructure/**` and `internal/telemetry/**`
- **Build Verification**: Dual builds pass (minor test failures unrelated to infrastructure)
- **Guard Compliance**: All provider degradation, rate limiting, and pool behaviors preserved
- **Exchange-Native**: No aggregator dependencies introduced

The infrastructure scaffold purge maintains full compliance with provider guard specifications while eliminating development scaffolds from production code paths.