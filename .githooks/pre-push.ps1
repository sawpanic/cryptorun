# Pre-push hook for CryptoRun
# Enforces progress or test improvements before pushing

Write-Host "🔍 Running pre-push checks..."

# Check PowerShell version
if ($PSVersionTable.PSVersion.Major -lt 7) {
    Write-Warning "PowerShell 7+ recommended. Current version: $($PSVersionTable.PSVersion)"
}

# Run progress check with failure enforcement
Write-Host "📊 Checking progress..."
try {
    & pwsh -File "tools/progress.ps1" -FailIfNoGain
    if ($LASTEXITCODE -ne 0) {
        Write-Error "❌ Progress check failed"
        exit 1
    }
} catch {
    Write-Error "❌ Error running progress check: $_"
    exit 1
}

# Run tests to ensure quality
Write-Host "🧪 Running tests..."
try {
    go test ./... -count=1
    if ($LASTEXITCODE -ne 0) {
        Write-Error "❌ Tests failed"
        exit 1
    }
} catch {
    Write-Error "❌ Error running tests: $_"
    exit 1
}

Write-Host "✅ All pre-push checks passed"
exit 0