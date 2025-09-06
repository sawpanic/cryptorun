# Artifacts Documentation

## UX MUST — Live Progress & Explainability

This document defines the artifact writing system for CryptoRun's data ledger.

## Overview

The artifact writer provides point-in-time (PIT) file generation with stable schemas for JSON and CSV formats. All artifacts are written to `artifacts/ledger/` with UTC timestamps for chronological ordering.

## API Reference

### WriteJSON(name string, v interface{}) error
Writes any JSON-serializable value to a timestamped file.

**Filename Format:** `YYYYMMDD-HHMMSS-{name}.json`

**Example:**
```go
data := map[string]interface{}{
    "pair": "BTC-USD",
    "score": 82.5,
    "timestamp": time.Now(),
}
err := artifacts.WriteJSON("scan-results", data)
// Creates: 20240906-143022-scan-results.json
```

### WriteCSV(name string, rows [][]string) error
Writes CSV data with header and data rows to a timestamped file.

**Filename Format:** `YYYYMMDD-HHMMSS-{name}.csv`

**Example:**
```go
rows := [][]string{
    {"pair", "score", "volume"},
    {"BTC-USD", "82.5", "1234567"},
    {"ETH-USD", "76.2", "987654"},
}
err := artifacts.WriteCSV("top-pairs", rows)
// Creates: 20240906-143022-top-pairs.csv
```

## Directory Structure

```
artifacts/
└── ledger/
    ├── .gitkeep
    ├── 20240906-143022-scan-results.json
    ├── 20240906-143022-top-pairs.csv
    └── ...
```

## Naming Convention

- **Timestamp:** UTC format `YYYYMMDD-HHMMSS` for lexicographic sorting
- **Component:** Descriptive name (e.g., `scan-results`, `regime-weights`, `gate-violations`)
- **Extension:** `.json` or `.csv` based on format

## Schema Discipline

### JSON Artifacts
- Use consistent field names across components
- Include metadata: `timestamp`, `version`, `component`
- Prefer flat structures when possible
- Use ISO 8601 for datetime fields

**Standard Fields:**
```json
{
  "timestamp": "2024-09-06T14:30:22Z",
  "component": "scanner",
  "version": "v3.2.1",
  "data": { ... }
}
```

### CSV Artifacts
- First row must be header
- Use consistent column names across files
- Prefer numeric formats for calculations
- Include timestamp column for time series

**Standard Columns:**
```csv
timestamp,pair,component,value,unit
2024-09-06T14:30:22Z,BTC-USD,momentum,82.5,score
```

## Retention Policy

- **Hot:** Last 7 days in `artifacts/ledger/`
- **Archive:** Older files can be moved to cold storage
- **Cleanup:** Manual cleanup recommended (no auto-deletion)

## Integration Points

- **Scanner:** Write scan results and candidate lists
- **Regime:** Write regime detection and weight changes  
- **Gates:** Write entry/exit decisions and violations
- **Backtest:** Write performance metrics and trade logs

## Error Handling

All functions return errors for:
- Directory creation failures
- File write permissions
- JSON marshaling errors
- CSV formatting issues

Handle errors appropriately in calling code:
```go
if err := artifacts.WriteJSON("results", data); err != nil {
    log.Printf("Failed to write artifact: %v", err)
}
```

