# Code Review Bundle Documentation

## UX MUST — Live Progress & Explainability

Real-time code review bundle generation with comprehensive analysis: automated build testing, diff capture, static analysis, and review summary generation with full progress tracking and explainability.

## Overview

This document describes the automated code review bundle generation system that creates comprehensive review packages from the current repository state.

## Bundle Generation Process

### Automated Analysis
1. **Branch Detection**: Identifies current branch and merge-base with main
2. **Change Capture**: Generates unified diffs, statistics, and file lists
3. **Build Testing**: Runs Go build, test, and vet with output capture
4. **Code Analysis**: Harvests annotations (TODO/FIXME/HACK/BUG)
5. **Hotspot Analysis**: Identifies frequently changed files (90d window)
6. **Review Synthesis**: Creates human-readable REVIEW.md summary

### Bundle Contents
- `branch_info.txt`: Branch names and merge-base commit
- `log_last30.txt`: Recent commit history
- `diff_stat.txt`: Change statistics
- `diff_full.patch`: Complete unified diff
- `changed_files.txt`: List of modified files
- `build.txt`: Go build output with errors
- `test.txt`: Test execution results with coverage
- `vet.txt`: Static analysis findings
- `annotations.txt`: Code annotations harvest
- `hotspots_90d.txt`: File change frequency analysis
- `REVIEW.md`: Executive summary and recommendations

## Usage

```bash
# Generate review bundle (PROMPT_ID=REVIEW.PACKAGE.ZIP.NOW)
# Creates timestamped folder and ZIP archive automatically
```

### Output Location
```
artifacts/review/
├── YYYYMMDD-HHMMSS/          # Timestamped review folder
│   ├── REVIEW.md             # Executive summary
│   ├── branch_info.txt       # Branch and merge-base info
│   ├── diff_*.txt|.patch     # Change analysis
│   ├── build.txt             # Build results
│   ├── test.txt              # Test execution
│   ├── vet.txt               # Static analysis
│   └── *.txt                 # Additional analysis files
└── CryptoRun_review_YYYYMMDD_HHMMSS.zip  # Packaged bundle
```

## Review Assessment Framework

### Risk Categories
- **🔴 High Risk**: Build failures, type safety issues, interface violations
- **🟡 Medium Risk**: Code duplication, unused code, documentation gaps
- **🟢 Low Risk**: Minor style issues, optimization opportunities

### Quality Gates
- **Build Status**: Must pass compilation
- **Test Status**: Core tests must execute successfully  
- **Static Analysis**: Critical issues must be resolved
- **Documentation**: UX MUST sections required

## Integration

The review bundle system integrates with:
- **Git Workflow**: Branch-aware analysis from merge-base
- **Go Toolchain**: Build, test, and vet integration
- **Documentation Standards**: UX MUST compliance checking
- **Quality Gates**: Automated pass/fail recommendations

## Failure Modes

### Common Issues
- **Build Failures**: Compilation errors prevent test execution
- **PowerShell Restrictions**: Archive creation may require Python fallback
- **Network Dependencies**: Some tests may fail without Redis/external services

### Recovery Actions
- **Continue on Tool Failure**: Best-effort analysis with warnings
- **Fallback Methods**: Alternative ZIP creation approaches
- **Error Documentation**: All failures captured in bundle

---
*Generated via PROMPT_ID=REVIEW.PACKAGE.ZIP.NOW on 2025-09-06*