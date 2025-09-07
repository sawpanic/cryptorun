#!/usr/bin/env pwsh
# tools/patch_only.ps1 - Enforce patch-only output limits

param(
    [int]$MaxLinesPerFile = 600,
    [switch]$Check,
    [switch]$Help
)

if ($Help) {
    Write-Host "PATCH-ONLY ENFORCEMENT"
    Write-Host "====================="
    Write-Host ""
    Write-Host "Usage: pwsh -File tools/patch_only.ps1 [-MaxLinesPerFile 600] [-Check]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -MaxLinesPerFile N   Maximum lines changed per file (default: 600)"
    Write-Host "  -Check              Check staged files without failing"
    Write-Host "  -Help               Show this help"
    Write-Host ""
    Write-Host "Environment Variables:"
    Write-Host "  PATCH_ONLY_DISABLE=1  Disable enforcement (for human hotfixes)"
    Write-Host ""
    Write-Host "Purpose: Prevent giant rewrites by failing when too many lines are touched"
    Write-Host "         per file or when files are staged outside of prompt WRITE-SCOPE."
    exit 0
}

# Allow human override
if ($env:PATCH_ONLY_DISABLE -eq "1") {
    Write-Host "PATCH-ONLY: Disabled via PATCH_ONLY_DISABLE=1"
    exit 0
}

# Check if we're in a git repository
if (-not (Test-Path ".git")) {
    Write-Host "PATCH-ONLY: Not in a git repository, skipping check"
    exit 0
}

# Get staged files with line counts
try {
    $stagedFiles = git diff --cached --numstat 2>$null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "PATCH-ONLY: No staged changes detected"
        exit 0
    }
} catch {
    Write-Host "PATCH-ONLY: Error checking staged files: $_"
    exit 0
}

if (-not $stagedFiles) {
    Write-Host "PATCH-ONLY: No staged changes detected"
    exit 0
}

$violations = @()
$totalFiles = 0
$totalLines = 0

foreach ($line in $stagedFiles) {
    if ($line -match '^(\d+|-)\s+(\d+|-)\s+(.+)$') {
        $added = if ($matches[1] -eq '-') { 0 } else { [int]$matches[1] }
        $deleted = if ($matches[2] -eq '-') { 0 } else { [int]$matches[2] }
        $filepath = $matches[3]
        
        $linesChanged = $added + $deleted
        $totalFiles++
        $totalLines += $linesChanged
        
        if ($linesChanged -gt $MaxLinesPerFile) {
            $violations += "  $filepath`: $linesChanged lines (limit: $MaxLinesPerFile)"
        }
    }
}

# Check for WRITE-SCOPE restrictions in commit message if available
$scopeViolations = @()
$commitMsgFile = ".git/COMMIT_EDITMSG"
if (Test-Path $commitMsgFile) {
    $commitMsg = Get-Content $commitMsgFile -Raw
    if ($commitMsg -match 'WRITE-SCOPE — ALLOW ONLY:(.*?)(?=\n\w+|\n\n|\Z)') {
        $allowedPatterns = $matches[1] -split '\n' | ForEach-Object { $_.Trim().TrimStart('-').Trim() } | Where-Object { $_ -ne '' }
        
        foreach ($line in $stagedFiles) {
            if ($line -match '^(\d+|-)\s+(\d+|-)\s+(.+)$') {
                $filepath = $matches[3]
                $allowed = $false
                
                foreach ($pattern in $allowedPatterns) {
                    if ($filepath -like $pattern -or $filepath -eq $pattern) {
                        $allowed = $true
                        break
                    }
                }
                
                if (-not $allowed) {
                    $scopeViolations += "  $filepath (not in WRITE-SCOPE)"
                }
            }
        }
    }
}

# Report results
if ($Check) {
    Write-Host "PATCH-ONLY CHECK RESULTS:"
    Write-Host "========================="
    Write-Host "Files staged: $totalFiles"
    Write-Host "Total lines changed: $totalLines"
    Write-Host "Max lines per file: $MaxLinesPerFile"
    
    if ($violations.Count -gt 0) {
        Write-Host ""
        Write-Host "Line limit violations:"
        $violations | ForEach-Object { Write-Host $_ }
    }
    
    if ($scopeViolations.Count -gt 0) {
        Write-Host ""
        Write-Host "Scope violations:"
        $scopeViolations | ForEach-Object { Write-Host $_ }
    }
    
    if ($violations.Count -eq 0 -and $scopeViolations.Count -eq 0) {
        Write-Host ""
        Write-Host "✅ All checks passed"
    }
    
    exit 0
}

# Enforcement mode
$failed = $false

if ($violations.Count -gt 0) {
    Write-Host "❌ PATCH-ONLY VIOLATION: Files exceed line limit ($MaxLinesPerFile lines/file)" -ForegroundColor Red
    Write-Host ""
    $violations | ForEach-Object { Write-Host $_ -ForegroundColor Yellow }
    $failed = $true
}

if ($scopeViolations.Count -gt 0) {
    Write-Host "❌ PATCH-ONLY VIOLATION: Files outside WRITE-SCOPE" -ForegroundColor Red
    Write-Host ""
    $scopeViolations | ForEach-Object { Write-Host $_ -ForegroundColor Yellow }
    $failed = $true
}

if ($failed) {
    Write-Host ""
    Write-Host "To override (human hotfixes only): PATCH_ONLY_DISABLE=1 git commit ..." -ForegroundColor Cyan
    Write-Host "To check without failing: pwsh -File tools/patch_only.ps1 -Check" -ForegroundColor Cyan
    exit 1
}

Write-Host "✅ PATCH-ONLY: All checks passed ($totalFiles files, $totalLines lines)" -ForegroundColor Green