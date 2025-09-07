# Repository Write Locks

## UX MUST — Live Progress & Explainability

CryptoRun implements a repository write lock system to enforce controlled development phases and prevent accidental changes to critical code during documentation-only or code-only work periods.

## Overview

The write lock system consists of three main components:

1. **`.crun_write_lock`** - Configuration file that defines the current lock state
2. **`tools/lock_guard.ps1`** - PowerShell script that enforces lock restrictions
3. **Pre-commit integration** - Automatic checking via `.githooks/pre-commit.ps1`

## Lock States

### UNLOCKED (Default)
- **Behavior**: All files can be modified freely
- **Usage**: Normal development mode
- **When to use**: Regular feature development, bug fixes, mixed code/docs work

### LOCKED: code
- **Behavior**: Only documentation and metadata files can be modified
- **Blocked**: Source code, configuration, tests, build files
- **Allowed**: `docs/`, `*.md`, `CHANGELOG.md`, lock system files
- **When to use**: Documentation-only sprints, writing guides, updating specs

### LOCKED: docs
- **Behavior**: Only code files can be modified
- **Blocked**: Documentation, markdown files, README updates
- **Allowed**: Source code, tests, configuration, build files, lock system files
- **When to use**: Code-only sprints, feature freeze with bug fixes only

## File Classification

### Always Allowed
These files bypass all lock restrictions:
- `.crun_write_lock` (the lock configuration itself)
- `CODEOWNERS`
- `.githooks/` (git hooks)
- `tools/lock_guard.ps1` (the guard script)

### Code Files (blocked by `LOCKED: code`)
- `src/**` - All source code
- `internal/**` - Internal packages
- `interfaces/**` - API interfaces
- `cmd/**` - Command-line interfaces
- `domain/**` - Domain logic
- `application/**` - Application layer
- `infrastructure/**` - Infrastructure layer
- `*.go` - Go source files
- `go.mod`, `go.sum` - Go module files
- `config/*.yaml` - Configuration files
- `tests/**` - Test files

### Documentation Files (blocked by `LOCKED: docs`)
- `docs/**` - Documentation directory
- `*.md` - Markdown files
- `README.md` - Project readme
- `CHANGELOG.md` - Change log

## Usage

### Checking Current State
```bash
cat .crun_write_lock
```

### Changing Lock State
Edit `.crun_write_lock` and change the state line:

```bash
# For documentation-only mode
echo "LOCKED: code" > .crun_write_lock

# For code-only mode  
echo "LOCKED: docs" > .crun_write_lock

# For normal development
echo "UNLOCKED" > .crun_write_lock
```

### Manual Lock Check
```powershell
# Check if current staged changes are allowed
pwsh -File tools/lock_guard.ps1

# Test with specific commit type
pwsh -File tools/lock_guard.ps1 "pre-commit"
```

## Integration

### Pre-commit Hooks
The lock guard runs automatically on every commit attempt via `.githooks/pre-commit.ps1`. If any staged files violate the current lock state, the commit will be blocked.

### Error Resolution
When a commit is blocked:

1. **Check lock state**: `cat .crun_write_lock`
2. **Option 1 - Change lock state**: Edit `.crun_write_lock` to `UNLOCKED`
3. **Option 2 - Unstage blocked files**: `git restore --staged <blocked-file>`
4. **Option 3 - Modify only allowed files**: Work within current restrictions

### Example Workflow

```bash
# Start documentation sprint
echo "LOCKED: code" > .crun_write_lock
git add .crun_write_lock
git commit -m "chore: lock code changes for docs sprint"

# Try to modify code (will fail)
git add src/some-file.go
git commit -m "fix: some bug"  # ❌ BLOCKED

# Modify documentation (will succeed)  
git add docs/new-guide.md
git commit -m "docs: add new user guide"  # ✅ ALLOWED

# End documentation sprint
echo "UNLOCKED" > .crun_write_lock
git add .crun_write_lock  
git commit -m "chore: unlock repository for normal development"
```

## CODEOWNERS Integration

The `CODEOWNERS` file defines ownership for different repository sections:

### Key Areas
- **Domain layers**: Specialized teams for scoring, gates, regime detection
- **Infrastructure**: Data, cache, and reliability teams
- **High-impact files**: Require multiple approvals (e.g., composite scorer)

### Approval Requirements
- Most changes require approval from relevant team
- Critical files require multiple team approvals
- Lock system files require security team approval

## Benefits

### Development Phase Control
- **Documentation sprints**: Focus exclusively on docs without code drift
- **Code freeze periods**: Allow only essential code fixes
- **Release preparation**: Prevent code changes during doc finalization

### Risk Reduction
- Prevents accidental code changes during doc-only work
- Reduces merge conflicts between concurrent code/docs work
- Enforces deliberate switching between development modes

### Team Coordination
- Clear signal to team about current development focus
- Automated enforcement reduces human error
- Audit trail of lock state changes

## Technical Details

### Implementation
- **Language**: PowerShell for cross-platform compatibility
- **Integration**: Git pre-commit hooks with staged file checking
- **Pattern matching**: Regex-based file classification
- **Exit codes**: Standard success (0) and failure (1) codes

### Performance
- Minimal overhead: Only runs on commit attempts
- Fast execution: Simple pattern matching and file checks
- No network dependencies: Pure local operation

### Reliability  
- Graceful fallbacks: Unknown states default to unlocked
- Clear error messages: Detailed feedback on blocked files
- Self-documenting: Comprehensive help text in lock file