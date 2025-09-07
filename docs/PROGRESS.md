# Progress Tracking System

## UX MUST — Live Progress & Explainability

CryptoRun enforces measurable progress through automated tracking:
- **Weighted milestone completion** tracked in `PROGRESS.yaml`
- **CI enforcement** blocks PRs without progress, test, or doc improvements
- **Pre-push hooks** prevent unproductive commits locally
- **Real-time calculation** via `tools/progress.ps1`

## Overview

The progress tracking system ensures every commit provides measurable value through:

1. **Milestone-based tracking** with weighted completion percentages
2. **Multi-dimensional progress** (features, tests, documentation)
3. **Automated enforcement** in CI and pre-push hooks
4. **Transparent reporting** showing exactly what contributes to completion

## Progress Calculation

### Milestone Weights

Progress is calculated using weighted milestones in `PROGRESS.yaml`:

```yaml
milestones:
  unified_composite_scoring:
    weight: 15          # 15% of total project
    completed: true     # Full weight applied
    
  data_facade_hot_warm:
    weight: 10          # 10% of total project  
    completed: false    # Partial completion
    progress: 40        # 40% of 10% = 4% applied
```

### Calculation Formula

```
Total Progress % = Σ(milestone_weight × completion_factor)

Where completion_factor = 1.0 if completed, else (progress / 100)
```

### Current Milestones

| Category | Milestone | Weight | Status |
|----------|-----------|---------|---------|
| **Core Architecture (25%)** |
| | Unified Composite Scoring | 15% | ✅ Complete |
| | Gram-Schmidt Orthogonalization | 10% | ✅ Complete |
| **Data Pipeline (20%)** |
| | Data Facade Hot/Warm | 10% | 🔄 40% Progress |
| | Exchange-Native Microstructure | 10% | ✅ Complete |
| **Risk & Portfolio (20%)** |
| | Regime Adaptive Weights | 8% | ✅ Complete |
| | Portfolio Risk Controls | 12% | ✅ Complete |
| **Detection Systems (15%)** |
| | PreMove Detector | 15% | ✅ Complete |
| **Quality & Testing (10%)** |
| | Unit Test Coverage | 5% | 🔄 70% Progress |
| | Integration Tests | 5% | 🔄 60% Progress |
| **User Interface (10%)** |
| | Menu System Unified | 6% | ✅ Complete |
| | Real-time Dashboard | 4% | ✅ Complete |

## Enforcement Rules

### CI Enforcement (.github/workflows/progress.yml)

Pull requests must provide **at least one** of:

1. **Progress increase ≥ 0.1%** - Measurable milestone advancement
2. **Test count increase** - New unit/integration tests added  
3. **Documentation changes** - Updates to `docs/` or `CHANGELOG.md`

### Pre-Push Enforcement (.githooks/pre-push.ps1)

Local commits are blocked unless:

1. **Progress increased** since last successful push
2. **All tests pass** (`go test ./... -count=1`)

## Usage

### Check Current Progress

```powershell
# Basic progress calculation
pwsh tools/progress.ps1

# Enforce progress increase (CI mode)  
pwsh tools/progress.ps1 -FailIfNoGain -BaselineRef origin/main
```

### Update Milestones

Edit `PROGRESS.yaml` to:

1. **Mark milestones complete**: Set `completed: true`
2. **Update partial progress**: Adjust `progress: N` (0-100)
3. **Add new milestones**: Follow existing weight/structure patterns

### Enable Pre-Push Hooks

```bash
# One-time setup
git config core.hooksPath .githooks

# Windows PowerShell
# Hooks are automatically enabled for pwsh users
```

## Progress File Format

### `.progress` File

Contains single decimal number (e.g., `87.3`) representing current completion percentage.

- **Updated by**: `tools/progress.ps1`
- **Read by**: CI, pre-push hooks, monitoring systems
- **Format**: Plain text, single line, 1 decimal place

### Example Output

```
Progress: 87.3% (87.3/100 weighted points)
Baseline (origin/main): 85.2%
Progress delta: 2.1%
✅ Progress increased by 2.1%
```

## Troubleshooting

### "Progress must increase by at least 0.1%"

- **Solution**: Complete milestone work or update `progress:` values in `PROGRESS.yaml`
- **Alternative**: Add tests or update documentation

### "Tests failed"

- **Solution**: Fix failing tests before pushing
- **Check**: `go test ./... -count=1` locally

### "PowerShell 7+ recommended"

- **Solution**: Install PowerShell 7+ for consistent behavior
- **Fallback**: System PowerShell works but may have minor differences

## Integration

### Monitoring Systems

Progress data can be integrated with monitoring via:

```powershell
# Get progress percentage
$progress = Get-Content .progress
Write-Host "Current completion: $progress%"

# Parse detailed breakdown
$data = & pwsh tools/progress.ps1
# Outputs structured progress information
```

### Build Systems

Incorporate into build pipelines:

```yaml
- name: Update Progress
  run: pwsh tools/progress.ps1
  
- name: Validate Progress
  run: pwsh tools/progress.ps1 -FailIfNoGain
```

---

*Measurable progress ensures every commit moves CryptoRun toward production readiness.*