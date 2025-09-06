# tools/postflight.ps1
# Run postflight checks: verify file ownership and scope enforcement

$ErrorActionPreference = "Stop"

Write-Host "🔍 Running postflight checks..." -ForegroundColor Green

# Get staged files
Write-Host "  Checking staged files..." -ForegroundColor Yellow
$stagedFiles = git diff --cached --name-only
if (-not $stagedFiles) {
    Write-Host "    No staged files found" -ForegroundColor Yellow
    Write-Host "✅ Postflight checks passed!" -ForegroundColor Green
    exit 0
}

Write-Host "    Staged files:" -ForegroundColor Yellow
foreach ($file in $stagedFiles) {
    Write-Host "      $file" -ForegroundColor Gray
}

# Check if commit message contains WRITE-SCOPE block
$commitMsgFile = ".git/COMMIT_EDITMSG"
if (Test-Path $commitMsgFile) {
    $commitMsg = Get-Content $commitMsgFile -Raw
    if ($commitMsg -match "WRITE-SCOPE.*?ALLOW ONLY:(.*?)(?=\n\n|\n[A-Z]|\Z)") {
        $scopeBlock = $matches[1]
        Write-Host "  Found WRITE-SCOPE block, enforcing scope..." -ForegroundColor Yellow
        
        # Extract allowed paths from scope block
        $allowedPaths = @()
        $scopeLines = $scopeBlock -split "`n"
        foreach ($line in $scopeLines) {
            $line = $line.Trim()
            if ($line -match "^\s*-\s*(.+)$") {
                $allowedPaths += $matches[1].Trim()
            }
        }
        
        if ($allowedPaths.Count -eq 0) {
            Write-Host "❌ WRITE-SCOPE block found but no allowed paths parsed" -ForegroundColor Red
            exit 1
        }
        
        Write-Host "    Allowed paths:" -ForegroundColor Yellow
        foreach ($path in $allowedPaths) {
            Write-Host "      $path" -ForegroundColor Gray
        }
        
        # Check if all staged files are within allowed scope
        $violations = @()
        foreach ($stagedFile in $stagedFiles) {
            $allowed = $false
            foreach ($allowedPath in $allowedPaths) {
                if ($stagedFile -eq $allowedPath -or $stagedFile.StartsWith($allowedPath + "/")) {
                    $allowed = $true
                    break
                }
            }
            if (-not $allowed) {
                $violations += $stagedFile
            }
        }
        
        if ($violations.Count -gt 0) {
            Write-Host "❌ Files outside declared WRITE-SCOPE:" -ForegroundColor Red
            foreach ($violation in $violations) {
                Write-Host "      $violation" -ForegroundColor Red
            }
            exit 1
        }
        
        Write-Host "    All staged files within declared scope ✓" -ForegroundColor Green
    } else {
        Write-Host "    No WRITE-SCOPE block found, skipping scope enforcement" -ForegroundColor Yellow
    }
} else {
    Write-Host "    No commit message file found, skipping scope enforcement" -ForegroundColor Yellow
}

Write-Host "✅ All postflight checks passed!" -ForegroundColor Green