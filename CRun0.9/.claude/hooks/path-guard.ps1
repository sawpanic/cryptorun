# path-guard.ps1 - Directory Boundary Enforcement
# Prevents modifications outside declared prompt ownership

param(
    [string]$Prompt = ""
)

# Exit codes
$SUCCESS = 0
$BLOCK = 1

# Protected paths that require explicit mention
$PROTECTED_PATHS = @(
    "src/cmd/cprotocol/",
    "src/domain/",
    "src/application/", 
    "src/infrastructure/",
    "src/interfaces/",
    "tests/",
    "config/",
    ".github/",
    ".claude/",
    "go.mod",
    "go.sum",
    "Makefile",
    "CLAUDE.md"
)

# Extract file paths from prompt
$mentions = @()
foreach ($path in $PROTECTED_PATHS) {
    if ($Prompt -match [regex]::Escape($path)) {
        $mentions += $path
    }
}

# Check for implicit file modifications without explicit mention
$RISKY_PATTERNS = @(
    "update.*import|change.*dependency",
    "refactor.*across|modify.*all",
    "fix.*everywhere|global.*change",
    "add.*to.*all.*files"
)

foreach ($pattern in $RISKY_PATTERNS) {
    if ($Prompt -match $pattern -and $mentions.Count -eq 0) {
        Write-Host "❌ PATH-GUARD: Blocked - risky cross-file change without explicit paths" -ForegroundColor Red
        Write-Host "Please specify which files/directories will be modified" -ForegroundColor Yellow
        exit $BLOCK
    }
}

# Warn about scope creep
if ($mentions.Count -gt 5) {
    Write-Host "⚠️  PATH-GUARD: Large scope - $($mentions.Count) paths mentioned" -ForegroundColor Yellow
    Write-Host "Consider breaking into smaller focused changes" -ForegroundColor Cyan
}

exit $SUCCESS