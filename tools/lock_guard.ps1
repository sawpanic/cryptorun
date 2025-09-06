# CryptoRun Write Lock Guard
# Enforces repository write restrictions based on .crun_write_lock configuration

param(
    [string]$CommitType = "pre-commit"
)

$ErrorActionPreference = "Stop"

# Read the lock configuration
$lockFile = ".crun_write_lock"
if (-not (Test-Path $lockFile)) {
    Write-Host "✓ No write lock file found, allowing all changes"
    exit 0
}

$lockContent = Get-Content $lockFile -Raw
$lockState = ($lockContent -split "`n" | Where-Object { $_ -match "^(UNLOCKED|LOCKED:.+)$" } | Select-Object -First 1).Trim()

if (-not $lockState) {
    Write-Host "✓ No valid lock state found, allowing all changes"
    exit 0
}

Write-Host "Lock state: $lockState"

# If unlocked, allow everything
if ($lockState -eq "UNLOCKED") {
    Write-Host "✓ Repository unlocked, allowing all changes"
    exit 0
}

# Get staged files
$stagedFiles = @()
try {
    $gitOutput = git diff --cached --name-only 2>$null
    if ($LASTEXITCODE -eq 0) {
        $stagedFiles = $gitOutput -split "`n" | Where-Object { $_.Trim() -ne "" }
    }
} catch {
    Write-Host "Warning: Could not get staged files, proceeding with caution"
    exit 0
}

if ($stagedFiles.Count -eq 0) {
    Write-Host "✓ No staged files to check"
    exit 0
}

Write-Host "Checking $($stagedFiles.Count) staged files against lock state: $lockState"

# Define file patterns
$codePatterns = @(
    "^src/",
    "^interfaces/",
    "^internal/", 
    "^cmd/",
    "^domain/",
    "^application/",
    "^infrastructure/",
    "\.go$",
    "go\.mod$",
    "go\.sum$",
    "^config/.*\.yaml$",
    "^tests/"
)

$docsPatterns = @(
    "^docs/",
    "\.md$",
    "^CHANGELOG\.md$",
    "^README\.md$"
)

$alwaysAllowedPatterns = @(
    "^\.crun_write_lock$",
    "^CODEOWNERS$",
    "^\.githooks/",
    "^tools/lock_guard\.ps1$"
)

function Test-FileAgainstPatterns($file, $patterns) {
    foreach ($pattern in $patterns) {
        if ($file -match $pattern) {
            return $true
        }
    }
    return $false
}

$blockedFiles = @()
$allowedFiles = @()

foreach ($file in $stagedFiles) {
    # Always allow certain files
    if (Test-FileAgainstPatterns $file $alwaysAllowedPatterns) {
        $allowedFiles += $file
        continue
    }
    
    $isCode = Test-FileAgainstPatterns $file $codePatterns
    $isDocs = Test-FileAgainstPatterns $file $docsPatterns
    
    switch ($lockState) {
        "LOCKED: code" {
            if ($isCode) {
                $blockedFiles += $file
                Write-Host "✗ BLOCKED: $file (code changes not allowed)"
            } else {
                $allowedFiles += $file
                Write-Host "✓ ALLOWED: $file"
            }
        }
        "LOCKED: docs" {
            if ($isDocs) {
                $blockedFiles += $file
                Write-Host "✗ BLOCKED: $file (docs changes not allowed)"
            } else {
                $allowedFiles += $file
                Write-Host "✓ ALLOWED: $file"
            }
        }
        default {
            Write-Host "Warning: Unknown lock state '$lockState', allowing file: $file"
            $allowedFiles += $file
        }
    }
}

if ($blockedFiles.Count -gt 0) {
    Write-Host ""
    Write-Host "❌ COMMIT BLOCKED by write lock restrictions!" -ForegroundColor Red
    Write-Host "Current lock state: $lockState" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Blocked files ($($blockedFiles.Count)):" -ForegroundColor Red
    foreach ($file in $blockedFiles) {
        Write-Host "  - $file" -ForegroundColor Red
    }
    Write-Host ""
    Write-Host "To resolve:" -ForegroundColor Yellow
    Write-Host "  1. Change .crun_write_lock to 'UNLOCKED' to allow all changes"
    Write-Host "  2. Or unstage the blocked files: git restore --staged <file>"
    Write-Host "  3. Or modify only allowed files for current lock state"
    Write-Host ""
    exit 1
}

Write-Host ""
Write-Host "✅ All staged files are allowed under current lock state" -ForegroundColor Green
Write-Host "Allowed files ($($allowedFiles.Count)):" -ForegroundColor Green
foreach ($file in $allowedFiles) {
    Write-Host "  ✓ $file" -ForegroundColor Green
}
Write-Host ""
exit 0