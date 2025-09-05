# change-budget.ps1 - Change Size Enforcement
# Prevents overly large changes that increase risk

param(
    [string]$Prompt = ""
)

# Exit codes
$SUCCESS = 0
$BLOCK = 1

# Count change indicators in prompt
$changeIndicators = @(
    "create.*file",
    "add.*function|new.*method",
    "modify.*struct|update.*interface",
    "refactor.*package",
    "implement.*feature",
    "fix.*bug",
    "delete.*file|remove.*code"
)

$changeCount = 0
foreach ($indicator in $changeIndicators) {
    $matches = [regex]::Matches($Prompt, $indicator, [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)
    $changeCount += $matches.Count
}

# Check for high-impact change signals
$HIGH_IMPACT = @(
    "major.*refactor|large.*rewrite",
    "breaking.*change|API.*change",
    "database.*migration|schema.*change",
    "config.*overhaul|settings.*redesign",
    "architecture.*change|pattern.*change"
)

$highImpactCount = 0
foreach ($pattern in $HIGH_IMPACT) {
    if ($Prompt -match $pattern) {
        $highImpactCount++
    }
}

# Enforce change budget limits
if ($changeCount -gt 8 -or $highImpactCount -gt 2) {
    Write-Host "❌ CHANGE-BUDGET: Blocked - change scope too large" -ForegroundColor Red
    Write-Host "Changes detected: $changeCount, High-impact: $highImpactCount" -ForegroundColor Yellow
    Write-Host "Please break into smaller, focused changes (max 8 changes, 2 high-impact)" -ForegroundColor Cyan
    exit $BLOCK
}

# Warn about moderate scope
if ($changeCount -gt 5 -or $highImpactCount -gt 1) {
    Write-Host "⚠️  CHANGE-BUDGET: Moderate scope detected" -ForegroundColor Yellow
    Write-Host "Changes: $changeCount, High-impact: $highImpactCount" -ForegroundColor Cyan
    Write-Host "Consider testing thoroughly and reviewing carefully" -ForegroundColor Cyan
}

exit $SUCCESS