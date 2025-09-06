#!/usr/bin/env pwsh
# CryptoRun Pre-Commit Hook
# Runs preflight and postflight checks plus existing guards

$ErrorActionPreference = "Stop"

Write-Host "🔍 Running CryptoRun pre-commit checks..." -ForegroundColor Cyan

# Change to repository root first
$RepoRoot = git rev-parse --show-toplevel
if (-not $RepoRoot) {
    Write-Error "Not in a git repository"
    exit 1
}

Set-Location $RepoRoot

# Run preflight checks first
Write-Host "`n🚀 Running preflight checks..." -ForegroundColor Cyan
& "$RepoRoot\tools\preflight.ps1"
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Preflight checks failed" -ForegroundColor Red
    exit 1
}

# Run postflight checks
Write-Host "`n🔍 Running postflight checks..." -ForegroundColor Cyan
& "$RepoRoot\tools\postflight.ps1"
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Postflight checks failed" -ForegroundColor Red
    exit 1
}

# Continue with existing documentation and branding checks
$ExitCode = 0
$ChecksPassed = 0
$ChecksFailed = 0

function Invoke-Check {
    param(
        [string]$CheckName,
        [scriptblock]$CheckCommand
    )
    
    Write-Host "`n📋 $CheckName" -ForegroundColor Yellow
    Write-Host ("─" * 50) -ForegroundColor Gray
    
    try {
        $Result = & $CheckCommand
        $CommandExitCode = $LASTEXITCODE
        
        if ($CommandExitCode -eq 0) {
            Write-Host "✅ $CheckName: PASSED" -ForegroundColor Green
            $script:ChecksPassed++
        } else {
            Write-Host "❌ $CheckName: FAILED" -ForegroundColor Red
            $script:ChecksFailed++
            $script:ExitCode = 1
        }
    }
    catch {
        Write-Host "❌ $CheckName: ERROR - $($_.Exception.Message)" -ForegroundColor Red
        $script:ChecksFailed++
        $script:ExitCode = 1
    }
}

# Check 1: Documentation UX Guard
Invoke-Check "Documentation UX Guard" {
    if (Test-Path "scripts/check_docs_ux.ps1") {
        pwsh -File "scripts/check_docs_ux.ps1"
    } elseif (Test-Path "scripts/check_docs_ux.go") {
        go run scripts/check_docs_ux.go
    } else {
        throw "Documentation UX checker not found"
    }
}

# Check 2: Branding Guard Test
Invoke-Check "Branding Guard Test" {
    if (Test-Path "tests/branding/branding_guard_test.go") {
        go test -v ./tests/branding -run TestBrandConsistency
    } else {
        Write-Warning "Branding guard test not found - skipping"
        return 0
    }
}

# Check 3: Basic Go Build (if Go files exist)
if (Get-ChildItem -Recurse -Filter "*.go" -ErrorAction SilentlyContinue | Select-Object -First 1) {
    Invoke-Check "Go Build Verification" {
        go build ./...
    }
}

# Check 4: Go Tests (if test files exist)
if (Get-ChildItem -Recurse -Filter "*_test.go" -ErrorAction SilentlyContinue | Select-Object -First 1) {
    Invoke-Check "Go Test Suite" {
        go test -short ./...
    }
}

# Summary
Write-Host "`n" + ("=" * 60) -ForegroundColor Cyan
Write-Host "📊 PRE-COMMIT SUMMARY" -ForegroundColor Cyan
Write-Host ("=" * 60) -ForegroundColor Cyan

if ($ExitCode -eq 0) {
    Write-Host "✅ ALL CHECKS PASSED ($ChecksPassed passed, 0 failed)" -ForegroundColor Green
    Write-Host "🚀 Ready to commit!" -ForegroundColor Green
} else {
    Write-Host "❌ SOME CHECKS FAILED ($ChecksPassed passed, $ChecksFailed failed)" -ForegroundColor Red
    Write-Host "🛑 Fix issues before committing" -ForegroundColor Red
    
    Write-Host "`nCommon fixes:" -ForegroundColor Yellow
    Write-Host "  • Add '## UX MUST — Live Progress & Explainability' to markdown files" -ForegroundColor White
    Write-Host "  • Replace 'CryptoEdge' or 'Crypto Edge' with 'CryptoRun'" -ForegroundColor White
    Write-Host "  • Fix any Go build or test failures" -ForegroundColor White
}

Write-Host ""
exit $ExitCode