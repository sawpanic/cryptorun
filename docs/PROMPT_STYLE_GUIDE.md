# Prompt Style Guide

## UX MUST — Live Progress & Explainability

CryptoRun prompts follow standardized headers and patterns to ensure:
- **Scope enforcement** preventing scope creep and maintaining focus
- **Quality gates** with test-first development and documentation requirements
- **Progress tracking** with automated milestone validation
- **Consistent structure** enabling reliable batch processing

## Required Headers

### Core Headers (Required for All Prompts)

#### OUTPUT DISCIPLINE
```
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
```
Keeps responses concise and terminal-friendly. Prevents overwhelming output that exceeds console buffer limits.

#### DOCS MANDATE  
```
DOCS MANDATE — UPDATE MD ON EVERY PROMPT
```
Ensures documentation is updated alongside code changes. Every prompt that modifies functionality must update relevant `.md` files.

#### SPEED/BATCH
```
SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=[UNIQUE_ID]
```
Enables reliable batch processing with unique identifiers. No shortcuts or simplified implementations allowed.

#### WRITE-SCOPE
```
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/[MODULE]/**
  - tests/unit/[MODULE]/**
  - docs/[MODULE].md
  - CHANGELOG.md
```
**Most critical header.** Explicitly constrains which files can be modified. Prevents scope creep and maintains focused changes.

#### PATCH-ONLY
```
PATCH-ONLY — Emit unified diffs or complete file blocks. No prose.
```
Enforces concise technical output. Responses should contain code/diffs, not explanatory prose.

### Quality Gates (Situational)

#### TEST-FIRST
```
TEST-FIRST — Write failing test before implementation.
```
Required for new features or algorithms. Ensures comprehensive test coverage and specification clarity.

#### POSTFLIGHT
```
POSTFLIGHT — Run tests and update PROGRESS.yaml if milestone achieved.
```
Required when work may complete a milestone. Ensures progress tracking stays current.

## Header Usage Patterns

### Domain Logic Development
```
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT  
SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=CORRELATION.MATRIX.IMPL
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/portfolio/correlation.go
  - tests/unit/portfolio/correlation_test.go
  - docs/PORTFOLIO_CORRELATION.md
  - CHANGELOG.md
PATCH-ONLY — Emit unified diffs or complete file blocks. No prose.
TEST-FIRST — Write failing test before implementation.
POSTFLIGHT — Run tests and update PROGRESS.yaml if milestone achieved.
```

### UI Component Development
```
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT
SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=SCORING.DASHBOARD.IMPL  
WRITE-SCOPE — ALLOW ONLY:
  - internal/ui/dashboard/scoring_board.go
  - internal/interfaces/http/scoring_handler.go
  - tests/integration/ui/scoring_test.go
  - docs/UI_SCORING.md
  - CHANGELOG.md
PATCH-ONLY — Emit unified diffs or complete file blocks. No prose.
POSTFLIGHT — Test SSE throttling ≤1 Hz and update progress.
```

### Configuration Updates
```
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
DOCS MANDATE — UPDATE MD ON EVERY PROMPT
SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=CONFIG.PREMOVE.UPDATE
WRITE-SCOPE — ALLOW ONLY:
  - config/premove.yaml
  - docs/PREMOVE_CONFIG.md
  - CHANGELOG.md
PATCH-ONLY — Emit unified diffs or complete file blocks. No prose.
```

### Documentation-Only Changes
```
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
SPEED/BATCH — NO SIMPLIFICATIONS — PROMPT_ID=DOCS.API.UPDATE
WRITE-SCOPE — ALLOW ONLY:
  - docs/API_REFERENCE.md
  - docs/USAGE_EXAMPLES.md
  - CHANGELOG.md
PATCH-ONLY — Emit unified diffs or complete file blocks. No prose.
```

## WRITE-SCOPE Best Practices

### Scope Granularity

#### ✅ Good - Specific and Focused
```
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/scoring/momentum.go
  - tests/unit/scoring/momentum_test.go
  - docs/MOMENTUM_CALCULATION.md
```

#### ✅ Good - Related File Group
```
WRITE-SCOPE — ALLOW ONLY:
  - src/application/premove/**
  - tests/integration/premove_pipeline_test.go
  - docs/PREMOVE_PIPELINE.md
```

#### ❌ Avoid - Too Broad
```
WRITE-SCOPE — ALLOW ONLY:
  - src/**  
  - tests/**
  - docs/**
```

#### ❌ Avoid - Unrelated Files
```
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/scoring/momentum.go
  - internal/ui/dashboard/board.go    # Unrelated to momentum scoring
  - config/database.yaml              # Unrelated to the change
```

### Common WRITE-SCOPE Patterns

#### Single Module Implementation
```
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/[MODULE]/[FEATURE].go
  - tests/unit/[MODULE]/[FEATURE]_test.go
  - docs/[MODULE]_[FEATURE].md
  - CHANGELOG.md
```

#### Cross-Module Integration
```
WRITE-SCOPE — ALLOW ONLY:
  - src/domain/portfolio/**
  - src/domain/risk/**  
  - tests/integration/portfolio_risk_test.go
  - docs/PORTFOLIO_RISK_INTEGRATION.md
  - CHANGELOG.md
```

#### UI + Backend Integration
```
WRITE-SCOPE — ALLOW ONLY:
  - internal/ui/[COMPONENT]/**
  - src/application/[MODULE]/pipeline.go
  - tests/integration/[COMPONENT]_integration_test.go
  - docs/UI_[COMPONENT].md
  - CHANGELOG.md
```

#### Configuration + Implementation
```
WRITE-SCOPE — ALLOW ONLY:
  - config/[MODULE].yaml
  - src/[LAYER]/[MODULE]/config.go
  - tests/unit/[MODULE]/config_test.go
  - docs/[MODULE]_CONFIG.md  
  - CHANGELOG.md
```

## PROMPT_ID Conventions

### Naming Pattern
```
PROMPT_ID=[COMPONENT].[FEATURE].[ACTION]

Examples:
MOMENTUM.CORE.IMPL          # Implementing momentum core
PORTFOLIO.CORRELATION.FIX   # Fixing correlation calculation  
UI.DASHBOARD.SSE.THROTTLE   # Adding SSE throttling to dashboard
CONFIG.PREMOVE.UPDATE       # Updating premove configuration
DOCS.API.REFERENCE.UPDATE   # Updating API documentation
```

### ID Categories
- **IMPL** - New implementation
- **FIX** - Bug fixes  
- **UPDATE** - Modifications to existing functionality
- **REFACTOR** - Code restructuring without behavior change
- **TEST** - Adding/updating tests
- **DOCS** - Documentation updates
- **CONFIG** - Configuration changes

## Commit Message Integration

Prompt IDs should align with conventional commit types:

```
PROMPT_ID=MOMENTUM.CORE.IMPL → feat(momentum): implement core calculation engine
PROMPT_ID=PORTFOLIO.CORRELATION.FIX → fix(portfolio): correct correlation matrix symmetry  
PROMPT_ID=UI.DASHBOARD.SSE.THROTTLE → feat(ui): add SSE throttling to dashboard
PROMPT_ID=CONFIG.PREMOVE.UPDATE → chore(config): update premove configuration defaults
PROMPT_ID=DOCS.API.REFERENCE.UPDATE → docs(api): update endpoint documentation
```

## Quality Enforcement

### Required Elements Checklist
- [ ] OUTPUT DISCIPLINE header present
- [ ] DOCS MANDATE header present (if code changes)
- [ ] SPEED/BATCH header with unique PROMPT_ID  
- [ ] WRITE-SCOPE explicitly lists allowed files
- [ ] PATCH-ONLY header for technical prompts
- [ ] TEST-FIRST for new features/algorithms
- [ ] POSTFLIGHT for milestone-completing work

### Scope Validation
- [ ] All modified files listed in WRITE-SCOPE
- [ ] No unrelated file modifications
- [ ] Documentation files included for user-facing changes
- [ ] CHANGELOG.md included for notable changes
- [ ] Test files included for new functionality

### Style Consistency  
- [ ] Headers use — (em dash) separators
- [ ] File paths use forward slashes
- [ ] Module names match directory structure
- [ ] PROMPT_ID follows naming conventions

---

*Structured prompts with standardized headers ensure consistent quality and maintainable development velocity.*