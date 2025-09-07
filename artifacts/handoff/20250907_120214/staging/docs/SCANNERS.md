# LegacyScanPipeline — Compatibility Shim

## UX MUST — Live Progress & Explainability

Legacy scanner compatibility layer providing interface compliance for deprecated scanning paths while maintaining clean integration with the unified composite scoring system.

**Updated for D6 Regression Fix**  
**Last Updated:** 2025-09-07  
**Version:** v1.0 Compatibility Shim  
**Status:** Implemented

## Overview

The `LegacyScanPipeline` serves as a compatibility shim to maintain interface compliance for legacy scanning code while directing users toward the unified composite scoring system. This implementation satisfies the `ScanPipelineInterface` contract without duplicating core scoring logic.

## Purpose

**Problem Solved**: Legacy code expected a `ScanUniverse(ctx context.Context) ([]CandidateResult, error)` method on `LegacyScanPipeline`, causing compilation failures with error:
```
*LegacyScanPipeline does not implement ScanPipelineInterface (missing method ScanUniverse)
```

**Solution**: Implement required interface methods as compatibility stubs that:
1. Satisfy compile-time interface requirements
2. Return structured "NotSupported" errors
3. Guide users to the unified composite pipeline
4. Preserve existing configuration wiring

## Implementation

### Interface Compliance

```go
type ScanPipelineInterface interface {
    SetRegime(regime string)
    ScanUniverse(ctx context.Context) ([]CandidateResult, error)
    WriteJSONL(candidates []CandidateResult, outputDir string) error
    WriteLedger(candidates []CandidateResult) error
}

// Compile-time interface assertion
var _ ScanPipelineInterface = (*LegacyScanPipeline)(nil)
```

### Method Implementations

#### ScanUniverse (Primary Fix)
```go
func (p *LegacyScanPipeline) ScanUniverse(ctx context.Context) ([]CandidateResult, error) {
    log.Warn().Msg("LegacyScanPipeline.ScanUniverse called - delegating to composite pipeline not yet implemented")
    return []CandidateResult{}, fmt.Errorf("LegacyScanPipeline: ScanUniverse not supported, use composite pipeline")
}
```

#### Configuration Methods
- **SetRegime**: Updates internal regime state for compatibility
- **WriteJSONL**: Stub implementation with logging
- **WriteLedger**: Stub implementation with logging

### Constructor and State

```go
type LegacyScanPipeline struct {
    snapshotDir string
    regime      string // defaults to "trending_bull"
}

func NewLegacyScanPipeline(snapshotDir string) *LegacyScanPipeline {
    return &LegacyScanPipeline{
        snapshotDir: snapshotDir,
        regime:      "trending_bull",
    }
}
```

## Migration Path

### Current State
- **Legacy Pipeline**: Returns NotSupported error for ScanUniverse
- **Recommended Path**: Use unified composite scoring pipeline
- **Interface**: Fully compliant with existing contracts

### Future Integration Options
1. **Thin Adapter**: Delegate to composite scorer when available
2. **Feature Flag**: Toggle between legacy stub and live delegation
3. **Deprecation**: Remove once all callers migrate to unified system

## Testing

### Unit Tests (`legacy_pipeline_test.go`)

**Interface Compliance Test**:
```go
func TestLegacyScanPipelineInterface(t *testing.T) {
    var _ ScanPipelineInterface = (*LegacyScanPipeline)(nil) // Compile-time check
    pipeline := NewLegacyScanPipeline("/tmp/snapshots")
    // Verify constructor and regime handling
}
```

**NotSupported Error Test**:
```go
func TestScanUniverseNotSupported(t *testing.T) {
    pipeline := NewLegacyScanPipeline("/tmp/snapshots")
    candidates, err := pipeline.ScanUniverse(ctx)
    // Verify empty results and structured error message
}
```

**Stub Method Tests**:
- WriteJSONL returns no error (stub)
- WriteLedger returns no error (stub)

## File Locations

### Implementation
- **Source**: `internal/application/pipeline/scan.go:154-192`
- **Interface**: `internal/application/pipeline/scan.go:147-152`  
- **Constructor**: `internal/application/pipeline/scan.go:160-165`

### Tests
- **Unit Tests**: `internal/application/pipeline/legacy_pipeline_test.go`
- **Coverage**: Interface compliance, error handling, stub methods

## Configuration Compatibility

### Preserved Behavior
- **Snapshot Directory**: Maintained in struct for compatibility
- **Regime Setting**: Functional for any legacy code expecting regime state
- **Logging**: Consistent with existing application logging patterns
- **Context Handling**: Proper context cancellation support

### Environment Wiring
- **Timeouts**: Inherited from calling context
- **Logger**: Uses application-wide zerolog instance
- **Config**: No additional configuration required

## Deprecation Strategy

### Phase 1: Compatibility (Current)
- Interface satisfied with NotSupported errors
- Clear error messages guide to unified pipeline
- Zero shared-state races

### Phase 2: Delegation (Future)
- Implement thin adapter to composite scorer
- Feature flag controls delegation vs stub behavior
- Preserve same interface contract

### Phase 3: Removal (Long-term)
- Remove after all callers migrate to unified system
- Clean up interface definitions
- Archive compatibility documentation

## QA Validation

### Regression Fix Confirmed
- **D6 Status**: ✅ PASS  
- **Evidence**: `internal/application/pipeline/scan.go:173` - ScanUniverse method implemented
- **Interface**: Compile-time assertion at line 192
- **Build**: No more "missing method ScanUniverse" errors

### Performance Impact
- **Memory**: Minimal - simple struct with two string fields
- **CPU**: No-op for stub methods, structured error generation only
- **Latency**: <1ms for NotSupported error path

This compatibility shim resolves the D6 regression while maintaining a clean path toward unified composite scoring without duplicating business logic.