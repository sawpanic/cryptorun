# spec-guard.ps1 - CProtocol v3.2.1 Conformance Guard
# Enforces architectural constraints and prevents spec drift

param(
    [string]$Prompt = ""
)

# Exit codes
$SUCCESS = 0
$BLOCK = 1

# Spec violation patterns
$VIOLATIONS = @(
    "aggregator.*api.*coingecko|dexscreener|coinmarketcap",
    "hardcode.*weight|threshold|limit",
    "skip.*test|TODO.*test|\\bxskip\\b",
    "new.*file.*domain/.*application/.*infrastructure/",
    "remove.*guard|disable.*gate|bypass.*check",
    "social.*factor.*>.*10|social.*unlimited",
    "momentum.*core.*reorder|momentum.*hierarchy.*change"
)

# Check for violations
foreach ($violation in $VIOLATIONS) {
    if ($Prompt -match $violation) {
        Write-Host "❌ SPEC-GUARD: Blocked - potential violation: $violation" -ForegroundColor Red
        Write-Host "Refer to CLAUDE.md section 'Do's & Don'ts'" -ForegroundColor Yellow
        exit $BLOCK
    }
}

# Check for required conformance keywords when touching critical files
if ($Prompt -match "(factor|guard|regime|microstructure)" -and 
    $Prompt -notmatch "(test|conformance|spec)") {
    Write-Host "⚠️  SPEC-GUARD: Critical change without test mention" -ForegroundColor Yellow
    Write-Host "Consider adding unit tests or conformance checks" -ForegroundColor Cyan
}

exit $SUCCESS