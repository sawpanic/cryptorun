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
    # Check if we're on main/release branch - if so, enforce strict policy
    $current_branch = git branch --show-current 2>$null
    if ($LASTEXITCODE -eq 0 -and ($current_branch -eq "main" -or $current_branch -like "*release*")) {
        Write-Host ""
        Write-Host "DOCS_GUARD: BLOCKED - Source code changes on main/release require documentation updates" -ForegroundColor Red
        Write-Host "Please update one of the following:" -ForegroundColor Red
        Write-Host "  - docs/** (any file under docs/)" -ForegroundColor White
        Write-Host "  - CHANGELOG.md" -ForegroundColor White
        Write-Host ""
        Write-Host "To bypass this check (emergency use only):" -ForegroundColor Yellow
        Write-Host "  set DOCS_GUARD_DISABLE=1" -ForegroundColor White
        Write-Host ""
        exit 1
    }
    
    # For non-main branches, auto-stub CHANGELOG.md
    Write-Host ""
    Write-Host "DOCS_GUARD: Code-only changes on branch '$current_branch' - auto-stubbing CHANGELOG.md" -ForegroundColor Yellow
    
    $short_sha = git rev-parse --short HEAD 2>$null
    if ($LASTEXITCODE -ne 0) {
        $short_sha = "unknown"
    }
    
    $stub_entry = "- chore(wip): auto-stub for commit $short_sha (to be edited before PR)"
    
    # Check if CHANGELOG.md exists
    if (Test-Path "CHANGELOG.md") {
        # Read existing content
        $changelog_content = Get-Content "CHANGELOG.md" -Raw
        
        # Find insertion point after first header
        $lines = $changelog_content -split "`n"
        $insert_index = -1
        
        for ($i = 0; $i -lt $lines.Count; $i++) {
            if ($lines[$i] -match "^#+\s+" -and $insert_index -eq -1) {
                # Found first header, look for next non-empty line or next header
                for ($j = $i + 1; $j -lt $lines.Count; $j++) {
                    if ($lines[$j].Trim() -eq "") {
                        continue
                    } elseif ($lines[$j] -match "^#+\s+") {
                        # Next header found, insert before it
                        $insert_index = $j
                        break
                    } else {
                        # Content found, insert after this section
                        $insert_index = $j
                        break
                    }
                }
                if ($insert_index -eq -1) {
                    # No content after first header, append after it
                    $insert_index = $i + 1
                }
                break
            }
        }
        
        if ($insert_index -ne -1) {
            # Insert the stub entry
            $lines = $lines[0..($insert_index-1)] + "" + $stub_entry + $lines[$insert_index..($lines.Count-1)]
            $new_content = $lines -join "`n"
            Set-Content "CHANGELOG.md" -Value $new_content -NoNewline
        } else {
            # No headers found, append to end
            Add-Content "CHANGELOG.md" -Value "`n$stub_entry"
        }
    } else {
        # Create new CHANGELOG.md
        Set-Content "CHANGELOG.md" -Value "# Changelog`n`n$stub_entry`n"
    }
    
    # Stage the file
    git add CHANGELOG.md 2>$null
    
    Write-Host "DOCS_GUARD: Auto-stubbed CHANGELOG.md with placeholder entry" -ForegroundColor Green
    Write-Host "  Entry: $stub_entry" -ForegroundColor White
    Write-Host "  Please edit this entry before creating PR" -ForegroundColor Yellow
    Write-Host ""
    exit 0
}

Write-Host "DOCS_GUARD: Documentation changes detected:" -ForegroundColor Green
foreach ($file in $docs_changes) {
    Write-Host "  - $file" -ForegroundColor White
}

Write-Host "DOCS_GUARD: PASSED - Code changes accompanied by documentation" -ForegroundColor Green
exit 0