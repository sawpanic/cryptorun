# UI Components & Data Contracts

This document describes the UI components and their data contracts for the CryptoRun system.

## UX MUST — Live Progress & Explainability

All UI components must provide:
- Real-time progress indicators
- Clear explainability of actions and data
- Responsive updates ≤1 Hz for SSE components

## Premove Board Data Contract

The Premove Board UI component provides real-time monitoring of the pre-movement detection system with Server-Sent Events (SSE) throttling at ≤1 Hz.

### Data Structures

#### ExecutionMetrics
```go
type ExecutionMetrics struct {
    TotalExecutions        int       `json:"total_executions"`
    SuccessfulExecutions   int       `json:"successful_executions"`
    AvgSlippageBps         float64   `json:"avg_slippage_bps"`
    AvgFillTimeMs          float64   `json:"avg_fill_time_ms"`
    AvgQualityScore        float64   `json:"avg_quality_score"`
    AcceptableSlippageRate float64   `json:"acceptable_slippage_rate"`
    InRecoveryMode         bool      `json:"in_recovery_mode"`
    ConsecutiveFails       int       `json:"consecutive_fails"`
    LastUpdated            time.Time `json:"last_updated"`
}
```

#### AlertRecord
```go
type AlertRecord struct {
    ID             string                 `json:"id"`
    Symbol         string                 `json:"symbol"`
    AlertType      string                 `json:"alert_type"`      // "pre_movement", "risk", etc.
    Severity       string                 `json:"severity"`        // "low", "medium", "high", "critical"
    Score          float64                `json:"score"`
    Message        string                 `json:"message"`
    Reasons        []string               `json:"reasons"`
    Metadata       map[string]interface{} `json:"metadata"`
    Timestamp      time.Time              `json:"timestamp"`
    Source         string                 `json:"source"`          // "detector", "manual", etc.
    Status         string                 `json:"status"`          // "pending", "sent", "rate_limited", "failed"
    ProcessingTime time.Duration          `json:"processing_time"`
}
```

#### Candidate
```go
type Candidate struct {
    Symbol      string                 `json:"symbol"`
    Score       float64                `json:"score"`
    Sector      string                 `json:"sector"`
    Beta        float64                `json:"beta"`
    Size        float64                `json:"size"`
    PassedGates int                    `json:"passed_gates"`
    GateResults map[string]bool        `json:"gate_results"`
    Reasons     []string               `json:"reasons"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    Timestamp   time.Time              `json:"timestamp"`
}
```

#### PreMoveBoardState
```go
type PreMoveBoardState struct {
    LastUpdate       time.Time                 `json:"last_update"`
    ActiveCandidates []premove.Candidate       `json:"active_candidates"`
    RecentAlerts     []premove.AlertRecord     `json:"recent_alerts"`
    PortfolioStatus  map[string]interface{}    `json:"portfolio_status"`
    ExecutionMetrics *premove.ExecutionMetrics `json:"execution_metrics"`
    SystemHealth     map[string]interface{}    `json:"system_health"`
    IsLive           bool                      `json:"is_live"`
    UpdateSequence   int64                     `json:"update_sequence"`
}
```

### Translation Functions

#### toExecMetrics()
Converts `map[string]interface{}` to `*ExecutionMetrics` for UI compatibility:

```go
func toExecMetrics(data map[string]interface{}) *ExecutionMetrics
```

This function handles type conversions between numeric types (int/float64) and provides defaults for missing fields.

### SSE Updates

The Premove Board uses Server-Sent Events with strict throttling:

- **Update Frequency**: ≤1 Hz (maximum 1 update per second)
- **Format**: JSON-encoded state changes
- **Channels**: Buffered channels with 10-message capacity
- **Cleanup**: Automatic removal of stale/disconnected clients

### Example Usage

```go
// Create premove board UI
ui := NewPreMoveBoardUI(runner)

// Serve SSE endpoint
http.HandleFunc("/premove/sse", ui.ServeSSE)

// Get current state
state := ui.GetCurrentState()

// Force refresh
ui.ForceRefresh()
```

### Performance Requirements

- **SSE Throttling**: ≤1 Hz guaranteed via ticker
- **Client Buffer**: 10 messages per client
- **State Updates**: 5-second internal refresh cycle
- **Memory Management**: Automatic stale client cleanup

### Error Handling

- Invalid clients are automatically removed
- Blocked channels trigger client disconnection
- JSON marshaling errors are logged but don't stop the system
- Recovery mode detection affects execution metrics

## Integration Points

### Runner Integration
The UI integrates with the premove Runner via:
- `GetPipelineStatus()` - System health status
- `GetExecutionSummary()` - Execution metrics
- `GetPortfolioConstraints()` - Portfolio status

### Menu Integration
Console display is available via `DisplayConsoleBoard()` method for CLI usage.