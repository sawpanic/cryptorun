# Contributing to CryptoRun

## UX MUST — Live Progress & Explainability

Contributing to CryptoRun follows structured patterns that ensure measurable progress and maintainable code:
- **Prompt Pack templates** for consistent domain-scoped development
- **Progress tracking** with automated milestone validation
- **Test-first development** with comprehensive coverage requirements
- **Documentation mandates** ensuring every change is explained

## Getting Started

### Prerequisites
- Go 1.21+
- PowerShell 7+ (for progress tracking and hooks)
- Git with hooks enabled: `git config core.hooksPath .githooks`

### Development Workflow
1. **Choose appropriate Prompt Pack template** from `.claude/prompt_packs/`
2. **Follow WRITE-SCOPE constraints** to maintain focused changes
3. **Write tests first** before implementing features
4. **Update documentation** for all user-facing changes
5. **Run pre-push checks** to ensure quality and progress

## Using Prompt Packs

Prompt Packs provide standardized templates for common development patterns with built-in scope constraints and quality gates.

### Available Templates

#### Domain Template (`.claude/prompt_packs/DOMAIN_TEMPLATE.md`)
For pure business logic and mathematical functions:
```
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/[MODULE]/**
  - tests/unit/[MODULE]/**
  - docs/[MODULE_DOCS].md
```

**Use for:**
- Correlation calculations, scoring algorithms
- Data structure definitions and validation
- Mathematical transformations
- Business rule implementations

#### UI Template (`.claude/prompt_packs/UI_TEMPLATE.md`) 
For user interfaces with SSE throttling requirements:
```
WRITE-SCOPE — ALLOW ONLY:
  - internal/ui/**
  - internal/interfaces/http/**  
  - tests/integration/ui/**
```

**Use for:**
- Real-time dashboards with Server-Sent Events
- HTTP handlers and REST endpoints
- Interactive console interfaces
- Web UI components

### Template Usage Pattern

1. **Copy template content** to your prompt
2. **Replace placeholders** with specific module/component names
3. **Customize WRITE-SCOPE** to match your exact file changes
4. **Follow TEST-FIRST** by writing failing tests before implementation
5. **Execute POSTFLIGHT** checks including progress updates

### Example: Adding Correlation Module

```markdown
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/portfolio/correlation.go
  - tests/unit/portfolio/correlation_test.go
  - docs/PORTFOLIO_CORRELATION.md
  - CHANGELOG.md

GOAL
Add Pearson correlation calculation for portfolio risk management

SCOPE (Atomic)
- Create src/domain/portfolio/correlation.go with CalculateMatrix function
- Add comprehensive unit tests with known test vectors
- Document correlation algorithm and usage in portfolio docs
```

## Code Standards

### Test Requirements
- **Unit tests** for all domain logic with >90% coverage
- **Integration tests** for HTTP handlers and UI components  
- **Test-first development** - write failing tests before implementation
- **Known test vectors** for mathematical functions

### Documentation Requirements  
- **Function comments** for all exported functions
- **Module documentation** in `docs/` for new features
- **CHANGELOG.md entries** for all user-visible changes
- **API documentation** for HTTP endpoints

### File Organization
```
src/domain/[MODULE]/          # Pure business logic
├── types.go                  # Data structures
├── calculations.go           # Core algorithms  
├── validation.go             # Input validation
└── constants.go              # Module constants

src/application/[MODULE]/     # Use cases and workflows
├── pipeline.go               # Main processing pipeline
├── handlers.go               # External integrations
└── config.go                 # Configuration management

internal/ui/[COMPONENT]/      # User interface
├── board.go                  # Main UI component
├── sse.go                   # Real-time updates (≤1 Hz)
└── state.go                 # State management
```

## Quality Gates

### Pre-Push Enforcement
The `.githooks/pre-push.ps1` hook ensures:
- **Progress increases** by ≥0.1% or tests/docs improve
- **All tests pass** with `go test ./... -count=1`
- **Documentation guard** validates required doc updates

### CI Enforcement  
Pull requests must provide at least one of:
- **Progress milestone advancement** (≥0.1% increase)
- **Test count improvements** (new unit/integration tests)
- **Documentation updates** (`docs/` or `CHANGELOG.md` changes)

### Progress Tracking
- **PROGRESS.yaml** contains weighted milestones summing to 100%
- **Automatic calculation** via `tools/progress.ps1`
- **Current completion**: Check `.progress` file for latest percentage

## Architectural Patterns

### Single Pipeline Principle
All menu and CLI commands must route to the **same underlying functions**:
```go
// ✅ Correct - unified pipeline
func (cli *CLI) ScanCommand() error {
    return pipeline.RunScan(ctx, opts)
}

func (menu *Menu) HandleScan() error {
    return pipeline.RunScan(ctx, opts) // Same function
}

// ❌ Wrong - duplicate implementations
func (cli *CLI) ScanCommand() error {
    // CLI-specific implementation
}
func (menu *Menu) HandleScan() error {  
    // Different implementation
}
```

### Error Handling
- **Wrap errors** with context using `fmt.Errorf("context: %w", err)`
- **Validate inputs** at function boundaries
- **Return early** on errors to minimize nesting
- **Log errors** with structured logging (zerolog)

### Performance Requirements
- **Scanner latency**: <300ms P99 target
- **SSE throttling**: ≤1 Hz for all real-time updates
- **Cache efficiency**: >85% hit rate target  
- **Memory management**: Clean up goroutines and channels

## Getting Help

### Documentation
- **Architecture**: `docs/V3_TECH_BUSINESS_BLUEPRINT.md`
- **API Integration**: `docs/API_INTEGRATION.md`  
- **Build Instructions**: `docs/BUILD.md`
- **Progress Tracking**: `docs/PROGRESS.md`

### Code Examples
- **Domain logic**: `src/domain/momentum/core.go`
- **UI components**: `internal/ui/menu/page_premove_board.go`
- **Test patterns**: `tests/unit/momentum/core_test.go`
- **Integration tests**: `tests/integration/menu_actions_test.go`

### Common Issues
- **Import errors**: Run `go mod tidy` to fix module dependencies
- **Test failures**: Check test data in `tests/*/fixtures/` directories  
- **Progress not increasing**: Complete milestone work or add tests/docs
- **Hook failures**: Ensure PowerShell 7+ and passing tests

---

*Structured development with Prompt Packs ensures consistent quality and measurable progress toward production readiness.*