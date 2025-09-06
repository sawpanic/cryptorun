# PATCH_POLICY.md

## UX MUST — Live Progress & Explainability

This document explains CryptoRun's patch-only enforcement system designed to prevent giant rewrites and maintain focused, atomic changes.

## Overview

The patch-only system enforces disciplined development by:
- **Limiting file size**: Maximum 600 lines changed per file (configurable)
- **Scope enforcement**: Respecting WRITE-SCOPE declarations in commit messages
- **Human override**: Emergency bypass for critical hotfixes
- **Commit metadata**: Automatic summary generation for transparency

## Components

### 1. Patch-Only Enforcer (`tools/patch_only.ps1`)

**Purpose**: Pre-commit validation to prevent giant rewrites

**Usage**:
```powershell
# Check staged files (no failure)
pwsh -File tools/patch_only.ps1 -Check

# Enforce limits (default in hooks)
pwsh -File tools/patch_only.ps1

# Custom line limit
pwsh -File tools/patch_only.ps1 -MaxLinesPerFile 300

# Show help
pwsh -File tools/patch_only.ps1 -Help
```

**Validation Rules**:
- ❌ Fail if any file has >600 lines changed (added + deleted)
- ❌ Fail if staged files are outside WRITE-SCOPE (when declared)
- ✅ Pass with summary when all checks clear

**Environment Variables**:
- `PATCH_ONLY_DISABLE=1`: Disable enforcement (human hotfixes only)

### 2. Commit Message Enhancer (`.githooks/prepare-commit-msg.ps1`)

**Purpose**: Append patch summary to commit messages

**Features**:
- Files changed count and line totals
- Individual file breakdown (max 10 files shown)
- Write scope summary (if declared)
- Enforcement instructions

**Auto-appended footer example**:
```
# PATCH-ONLY COMMIT SUMMARY
# =========================
# Files changed: 4
# Lines changed: 89
# Max lines/file: 600 (configurable)
# Write scope: tools/*.ps1, docs/PATCH_POLICY.md
#
# Files modified:
# - tools/patch_only.ps1 (45 lines)
# - .githooks/prepare-commit-msg.ps1 (32 lines)
# - docs/PATCH_POLICY.md (12 lines)
#
# To disable enforcement: PATCH_ONLY_DISABLE=1 git commit ...
# To check without committing: pwsh -File tools/patch_only.ps1 -Check
```

## WRITE-SCOPE Integration

When commit messages include WRITE-SCOPE declarations, the system enforces file restrictions:

```
WRITE-SCOPE — ALLOW ONLY:
  - tools/patch_only.ps1
  - .githooks/prepare-commit-msg.ps1
  - docs/PATCH_POLICY.md
  - CHANGELOG.md
```

Files staged outside these patterns will trigger violations.

## Hook Integration

### Setup
```bash
# Enable git hooks (one-time setup)
git config core.hooksPath .githooks

# Make hooks executable (Linux/Mac)
chmod +x .githooks/prepare-commit-msg.ps1

# Windows: hooks are automatically executable
```

### Pre-commit Hook Integration
Add to `.githooks/pre-commit`:
```bash
# Patch-only enforcement
pwsh -File tools/patch_only.ps1
if [ $? -ne 0 ]; then
    exit 1
fi
```

## Override Scenarios

### Human Hotfixes
For emergency fixes that require large changes:
```bash
PATCH_ONLY_DISABLE=1 git commit -m "fix: emergency database schema migration"
```

### Bulk Refactoring
For approved bulk operations:
```bash
PATCH_ONLY_DISABLE=1 git commit -m "refactor: rename all instances of deprecated API"
```

**⚠️ Important**: Override should be rare and documented in commit messages.

## Configuration

### Default Limits
- **Max lines per file**: 600 lines (added + deleted)
- **File list display**: 10 files max in commit footer
- **Path truncation**: 50 characters max

### Customization
```powershell
# Custom line limit
pwsh -File tools/patch_only.ps1 -MaxLinesPerFile 300

# Set permanent custom limit via alias/function
function Patch-Check { pwsh -File tools/patch_only.ps1 -MaxLinesPerFile 300 @args }
```

## Benefits

1. **Prevents giant rewrites**: Forces focused, reviewable changes
2. **Scope discipline**: Enforces declared file restrictions
3. **Transparency**: Automatic commit metadata for easy review
4. **Human-friendly**: Emergency override for critical situations
5. **Configurable**: Adjustable limits for different scenarios

## Troubleshooting

### Common Issues

**"Files exceed line limit"**
- Solution: Split large changes into multiple commits
- Override: Use `PATCH_ONLY_DISABLE=1` for justified exceptions

**"Files outside WRITE-SCOPE"**
- Solution: Update WRITE-SCOPE declaration or unstage unrelated files
- Check: Verify file patterns match exactly

**Hook not running**
- Verify: `git config core.hooksPath` points to `.githooks`
- Windows: Ensure PowerShell execution policy allows scripts

### Manual Checks
```powershell
# Check current staged files
pwsh -File tools/patch_only.ps1 -Check

# View git diff summary
git diff --cached --stat

# Check hook configuration
git config core.hooksPath
```

## Integration with CI/CD

The patch-only system integrates with existing CI workflows:

```yaml
# .github/workflows/ci.yaml
- name: Patch-only compliance check
  run: |
    pwsh -File tools/patch_only.ps1 -Check
```

This ensures patch discipline is maintained across all contributions.

---

**Remember**: Patch-only is about discipline, not rigidity. The goal is focused, reviewable changes that maintain codebase integrity while allowing human judgment for exceptional cases.