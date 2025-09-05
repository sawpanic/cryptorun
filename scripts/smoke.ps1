# CryptoRun Smoke Test Script
# Runs build, tests, and prints system health metrics

Write-Host "🔥 CryptoRun Smoke Test" -ForegroundColor Cyan
Write-Host "═══════════════════════" -ForegroundColor Cyan

# 1. Build check
Write-Host "`n🔨 Building..."
$buildResult = & go build -tags no_net ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Build failed" -ForegroundColor Red
    exit 1
}
Write-Host "✅ Build successful" -ForegroundColor Green

# 2. Test execution
Write-Host "`n🧪 Running tests..."
$testOutput = & go test ./... -count=1
$testExitCode = $LASTEXITCODE
if ($testExitCode -eq 0) {
    Write-Host "✅ All tests passed" -ForegroundColor Green
} else {
    Write-Host "❌ Tests failed" -ForegroundColor Red
    Write-Host $testOutput
}

# 3. Check candidates file
Write-Host "`n📊 Checking outputs..."
$candidatesPath = "out/scanner/latest_candidates.jsonl"
if (Test-Path $candidatesPath) {
    $candidateLines = (Get-Content $candidatesPath | Measure-Object -Line).Lines
    Write-Host "✅ Candidates file: $candidateLines entries" -ForegroundColor Green
} else {
    Write-Host "⚠️ No candidates file found (run scan first)" -ForegroundColor Yellow
}

# 4. Check coverage file
$coveragePath = "out/analyst/latest/coverage.json"
if (Test-Path $coveragePath) {
    Write-Host "✅ Coverage metrics present" -ForegroundColor Green
} else {
    Write-Host "⚠️ No coverage file found (run analyst first)" -ForegroundColor Yellow
}

# 5. Check universe config
if (Test-Path "config/universe.json") {
    $universe = Get-Content "config/universe.json" | ConvertFrom-Json
    $pairCount = $universe.usd_pairs.Length
    $minAdv = $universe._criteria.min_adv_usd
    $hashLen = $universe._hash.Length
    
    Write-Host "✅ Universe: $pairCount pairs, ADV≥$minAdv, hash($hashLen)" -ForegroundColor Green
} else {
    Write-Host "⚠️ No universe config found (run pairs sync first)" -ForegroundColor Yellow
}

# 6. Check latest DRYRUN line
if (Test-Path "CHANGELOG.md") {
    $lastDryrun = Get-Content "CHANGELOG.md" | Select-String "^DRYRUN:" | Select-Object -Last 1
    if ($lastDryrun) {
        Write-Host "✅ Latest DRYRUN: $($lastDryrun.Line)" -ForegroundColor Green
    } else {
        Write-Host "⚠️ No DRYRUN entries found (run dry-run first)" -ForegroundColor Yellow
    }
}

# Summary
Write-Host "`n🏁 Smoke Test Summary:" -ForegroundColor Cyan
if ($testExitCode -eq 0) {
    Write-Host "✅ System healthy - build & tests passing" -ForegroundColor Green
} else {
    Write-Host "❌ System issues detected" -ForegroundColor Red
    exit 1
}

Write-Host "📊 Run full verification via MENU → Verification Sweep" -ForegroundColor Cyan