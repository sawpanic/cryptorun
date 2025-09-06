# Documentation Policy

## Overview

CryptoRun enforces a documentation-first development policy to ensure code changes are properly documented and transparent.

## Policy

**When you make changes to source code (`src/**`), you must also update documentation.**

This policy is enforced by the `docs_guard.ps1` script in the pre-push hook.

## Requirements

If you modify any files under `src/**`, you must also update at least one of:

- **Documentation files**: Any file under `docs/**`
- **Changelog**: `CHANGELOG.md` in the project root

## Enforcement

The documentation guard runs automatically during:
- Pre-push git hooks  
- CI/CD pipeline checks

### Branch-Based Policy

**Main/Release Branches**: Strict enforcement
- All code changes require accompanying documentation updates
- No exceptions - maintains production quality standards

**Feature Branches**: Auto-stub with reminder
- Code-only changes trigger automatic CHANGELOG.md stub generation
- Stub format: `- chore(wip): auto-stub for commit <short-sha> (to be edited before PR)`
- Modified CHANGELOG.md is automatically staged
- Developer reminded to edit stub before creating PR

## Emergency Override

For critical hotfixes where documentation can be updated separately, set the environment variable:

```bash
export DOCS_GUARD_DISABLE=1
```

Or in PowerShell:
```powershell
$env:DOCS_GUARD_DISABLE = "1"
```

**Note**: This override should only be used in genuine emergencies. Documentation debt should be addressed promptly.

## Rationale

This policy ensures:
1. **Transparency**: Changes are explained and documented
2. **Maintainability**: Future developers understand the reasoning behind changes
3. **Compliance**: Regulatory and audit requirements are met
4. **Knowledge retention**: Tribal knowledge is captured in writing

## Examples

### ✅ Acceptable Changes

**On any branch:**
```
Modified files:
- src/domain/score/composite.go
- docs/ARCHITECTURE.md
```

```
Modified files:
- src/application/scanner.go
- CHANGELOG.md
```

**On feature branches (auto-stubbed):**
```
Modified files:
- src/domain/score/composite.go
(CHANGELOG.md automatically updated with stub entry)
```

### ❌ Blocked Changes

**On main/release branches only:**
```
Modified files:
- src/domain/score/composite.go
(No documentation changes - BLOCKED)
```

## UX MUST — Live Progress & Explainability

This policy supports the CryptoRun UX requirement for live progress tracking and explainable systems by ensuring all changes are documented with clear rationale and impact descriptions.