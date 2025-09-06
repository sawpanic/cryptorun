# Domain Layer Prompt Pack Template

```
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT
SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=[YOUR_PROMPT_ID]
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/[MODULE]/**
  - tests/unit/[MODULE]/**
  - docs/[MODULE_DOCS].md
  - CHANGELOG.md
PATCH-ONLY — Emit unified diffs or complete file blocks. No prose.
TEST-FIRST — Write failing test before implementation.
POSTFLIGHT — Run tests and update PROGRESS.yaml if milestone achieved.

GOAL
[Specific domain logic goal, e.g., "Add correlation matrix calculation with Pearson coefficient validation"]

SCOPE (Atomic)
[List specific files and functions to modify/create]

Example:
- Create src/domain/portfolio/correlation.go:
  - CalculateCorrelationMatrix(priceData map[string][]float64) (*Matrix, error)
  - ValidateMatrix(matrix *Matrix) error
  - PearsonCorrelation(x, y []float64) float64
- Add tests/unit/portfolio/correlation_test.go:
  - TestCalculateCorrelationMatrix_ValidInput
  - TestValidateMatrix_SymmetryCheck
  - TestPearsonCorrelation_KnownValues
- Update docs/PORTFOLIO.md with correlation section

GUARDS
- All correlations must be in [-1, 1] range
- Matrix must be symmetric
- Minimum 10 observations required
- NaN/Inf values handled gracefully

ACCEPTANCE
- TestCorrelationMatrix passes with known test vectors
- Matrix symmetry validation works
- Error handling for edge cases covered
- Documentation updated with examples

GIT COMMIT CHECKLIST
1) git add src/domain/[MODULE]/** tests/unit/[MODULE]/** docs/[MODULE_DOCS].md CHANGELOG.md
2) go test ./tests/unit/[MODULE]/... -count=1 -v
3) go build ./src/domain/[MODULE]/...
4) Update PROGRESS.yaml if milestone achieved
5) git commit -m "[type](domain): [description] with tests and validation"
6) git push -u origin HEAD
```

## Usage Notes

### Scope Patterns
- **Domain Logic**: Pure business rules, no external dependencies
- **Mathematical Functions**: Correlation, orthogonalization, scoring algorithms  
- **Data Structures**: Types, validation, transformations
- **Constants**: Thresholds, configuration defaults

### File Structure Template
```
src/domain/[MODULE]/
├── types.go          # Core data structures
├── calculations.go   # Business logic functions  
├── validation.go     # Input/output validation
└── constants.go      # Module constants

tests/unit/[MODULE]/
├── types_test.go
├── calculations_test.go
├── validation_test.go
└── fixtures/         # Test data files
    ├── valid_inputs.json
    └── expected_outputs.json
```

### Common WRITE-SCOPE Patterns
```
# Single module focus
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/correlation/**
  - tests/unit/correlation/**

# Cross-module integration  
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/portfolio/**
  - src/domain/risk/**
  - tests/unit/portfolio_risk_integration_test.go

# Documentation updates
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/scoring/**
  - docs/SCORING_ALGORITHM.md
  - docs/FACTOR_BREAKDOWN.md
  - CHANGELOG.md
```

### Test-First Examples
```go
// 1. Write failing test first
func TestCalculateCorrelation_KnownValues(t *testing.T) {
    x := []float64{1, 2, 3, 4, 5}
    y := []float64{2, 4, 6, 8, 10}
    expected := 1.0 // Perfect positive correlation
    
    result := CalculateCorrelation(x, y)
    assert.InDelta(t, expected, result, 0.001)
}

// 2. Implement to make test pass
func CalculateCorrelation(x, y []float64) float64 {
    // Implementation here
}

// 3. Add edge case tests
func TestCalculateCorrelation_EmptyInputs(t *testing.T) {
    result := CalculateCorrelation([]float64{}, []float64{})
    assert.Equal(t, 0.0, result)
}
```