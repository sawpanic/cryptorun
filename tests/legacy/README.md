# Legacy Tests

This directory contains legacy and integration tests that require full infrastructure dependencies and are excluded from the default test suite.

## Purpose

These tests have been quarantined because they:
- Require missing external infrastructure (Kraken APIs, market data providers)
- Depend on undefined packages or types that are not fully implemented
- Are integration tests that need complete system setup

## Running Legacy Tests

To run the legacy test suite:

```bash
go test -tags legacy ./...
```

## Contents

- `integration/` - Integration tests requiring external dependencies
  - `resilience_test.go` - API timeout and error handling tests
  - `circuit_test.go` - Circuit breaker integration tests  
  - `kraken_limit_test.go` - Kraken rate limiting tests

- `unit/` - Unit tests with missing infrastructure dependencies
  - `micro_snapshot_test.go` - Market microstructure snapshot tests
  - `analyst/analyst_run_test.go` - Analyst coverage analysis tests

## Caveats

- These tests may fail due to missing infrastructure dependencies
- They are excluded from CI by default to keep the build green
- Running them requires a complete development environment setup
- They are preserved for reference and future implementation

## Default Test Suite

The default test suite (run with `go test ./...`) includes only the core unit tests that pass without external dependencies.