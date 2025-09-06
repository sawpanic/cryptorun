#!/usr/bin/env pwsh

param(
    [string]$RemoteName = "origin",
    [string]$RemoteBranch = "main"
)

$ErrorActionPreference = "Stop"

if ($env:DOCS_GUARD_DISABLE -eq "1") {
    Write-Host "DOCS_GUARD: Disabled via DOCS_GUARD_DISABLE=1" -ForegroundColor Yellow
    exit 0
}

Write-Host "DOCS_GUARD: Checking for code changes without documentation updates..." -ForegroundColor Blue

try {
    $remote_ref = git rev-parse "$RemoteName/$RemoteBranch" 2>$null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "DOCS_GUARD: Cannot find remote branch $RemoteName/$RemoteBranch, checking against HEAD~1" -ForegroundColor Yellow
        $remote_ref = "HEAD~1"
    }
} catch {
    Write-Host "DOCS_GUARD: Cannot find remote branch, checking against HEAD~1" -ForegroundColor Yellow
    $remote_ref = "HEAD~1"
}

$changed_files = git diff-tree --name-only -r $remote_ref HEAD 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "DOCS_GUARD: No commits to check" -ForegroundColor Green
    exit 0
}

$src_changes = @()
$docs_changes = @()

foreach ($file in $changed_files) {
    if ($file.StartsWith("src/")) {
        $src_changes += $file
    } elseif ($file.StartsWith("docs/") -or $file -eq "CHANGELOG.md") {
        $docs_changes += $file
    }
}

if ($src_changes.Count -eq 0) {
    Write-Host "DOCS_GUARD: No source code changes detected" -ForegroundColor Green
    exit 0
}

Write-Host "DOCS_GUARD: Source code changes detected:" -ForegroundColor Yellow
foreach ($file in $src_changes) {
    Write-Host "  - $file" -ForegroundColor White
}

if ($docs_changes.Count -eq 0) {
    Write-Host ""
    Write-Host "DOCS_GUARD: BLOCKED - Source code changes require documentation updates" -ForegroundColor Red
    Write-Host "Please update one of the following:" -ForegroundColor Red
    Write-Host "  - docs/** (any file under docs/)" -ForegroundColor White
    Write-Host "  - CHANGELOG.md" -ForegroundColor White
    Write-Host ""
    Write-Host "To bypass this check (emergency use only):" -ForegroundColor Yellow
    Write-Host "  set DOCS_GUARD_DISABLE=1" -ForegroundColor White
    Write-Host ""
    exit 1
}

Write-Host "DOCS_GUARD: Documentation changes detected:" -ForegroundColor Green
foreach ($file in $docs_changes) {
    Write-Host "  - $file" -ForegroundColor White
}

Write-Host "DOCS_GUARD: PASSED - Code changes accompanied by documentation" -ForegroundColor Green
exit 0