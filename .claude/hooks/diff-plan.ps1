# diff-plan.ps1 - Implementation Plan Validation
# Ensures prompts include clear implementation plan and diff expectations

param(
    [string]$Prompt = ""
)

# Exit codes
$SUCCESS = 0
$BLOCK = 1

# Check for implementation requests without plans
$IMPL_REQUESTS = @(
    "implement|add|create|build",
    "fix|refactor|update|modify",
    "integrate|connect|setup"
)

$hasImplRequest = $false
foreach ($request in $IMPL_REQUESTS) {
    if ($Prompt -match $request) {
        $hasImplRequest = $true
        break
    }
}

# If implementation request, check for planning elements
if ($hasImplRequest) {
    $PLAN_INDICATORS = @(
        "plan:|steps:|approach:|strategy:",
        "scope:|acceptance:|success:",
        "diff.*table|changes.*include|files.*modified",
        "\\b1\\)|\\b2\\)|\\b3\\)|step.*1|first.*step",
        "then|next|after.*that|finally"
    )
    
    $hasPlan = $false
    foreach ($indicator in $PLAN_INDICATORS) {
        if ($Prompt -match $indicator) {
            $hasPlan = $true
            break
        }
    }
    
    if (-not $hasPlan) {
        Write-Host "❌ DIFF-PLAN: Blocked - implementation request without clear plan" -ForegroundColor Red
        Write-Host "Please include:" -ForegroundColor Yellow
        Write-Host "  • Clear implementation steps or approach" -ForegroundColor Cyan
        Write-Host "  • Expected file changes (DIFF TABLE)" -ForegroundColor Cyan
        Write-Host "  • Success/acceptance criteria" -ForegroundColor Cyan
        exit $BLOCK
    }
}

# Check for ambiguous requests
$AMBIGUOUS = @(
    "^(fix|update|improve|optimize)\\s*\$",
    "make.*better|enhance.*it|clean.*up",
    "handle.*case|deal.*with|take.*care"
)

foreach ($pattern in $AMBIGUOUS) {
    if ($Prompt -match $pattern) {
        Write-Host "⚠️  DIFF-PLAN: Ambiguous request detected" -ForegroundColor Yellow
        Write-Host "Consider providing more specific requirements" -ForegroundColor Cyan
        break
    }
}

exit $SUCCESS