#!/usr/bin/env pwsh
# .githooks/prepare-commit-msg.ps1 - Append PATCH-ONLY reminder and scope summary

param(
    [Parameter(Mandatory=$true)]
    [string]$CommitMsgFile,
    
    [string]$CommitSource = "",
    [string]$CommitSha = ""
)

# Skip if not a regular commit (merge, squash, etc.)
if ($CommitSource -eq "merge" -or $CommitSource -eq "squash" -or $CommitSource -eq "commit") {
    exit 0
}

# Skip if PATCH_ONLY is disabled
if ($env:PATCH_ONLY_DISABLE -eq "1") {
    exit 0
}

# Read existing commit message
if (-not (Test-Path $CommitMsgFile)) {
    exit 0
}

$commitMsg = Get-Content $CommitMsgFile -Raw

# Skip if message already contains PATCH-ONLY info
if ($commitMsg -match "PATCH-ONLY") {
    exit 0
}

# Get staged files summary
try {
    $stagedFiles = git diff --cached --numstat 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $stagedFiles) {
        exit 0
    }
} catch {
    exit 0
}

$totalFiles = 0
$totalLines = 0
$fileList = @()

foreach ($line in $stagedFiles) {
    if ($line -match '^(\d+|-)\s+(\d+|-)\s+(.+)$') {
        $added = if ($matches[1] -eq '-') { 0 } else { [int]$matches[1] }
        $deleted = if ($matches[2] -eq '-') { 0 } else { [int]$matches[2] }
        $filepath = $matches[3]
        
        $linesChanged = $added + $deleted
        $totalFiles++
        $totalLines += $linesChanged
        
        # Truncate long file paths
        $displayPath = if ($filepath.Length -gt 50) { "..." + $filepath.Substring($filepath.Length - 47) } else { $filepath }
        $fileList += "$displayPath ($linesChanged lines)"
    }
}

# Limit file list to prevent spam
if ($fileList.Count -gt 10) {
    $fileList = $fileList[0..9] + "... and $($fileList.Count - 10) more files"
}

# Check for WRITE-SCOPE in current message
$scopeInfo = ""
if ($commitMsg -match 'WRITE-SCOPE — ALLOW ONLY:(.*?)(?=\n\w+|\n\n|\Z)') {
    $allowedPatterns = $matches[1] -split '\n' | ForEach-Object { $_.Trim().TrimStart('-').Trim() } | Where-Object { $_ -ne '' }
    if ($allowedPatterns.Count -gt 0) {
        $scopeInfo = "`nWrite scope: " + ($allowedPatterns -join ", ")
    }
}

# Prepare patch-only footer
$patchFooter = @"


# PATCH-ONLY COMMIT SUMMARY
# =========================
# Files changed: $totalFiles
# Lines changed: $totalLines
# Max lines/file: 600 (configurable)$scopeInfo
#
# Files modified:
$(($fileList | ForEach-Object { "# - $_" }) -join "`n")
#
# To disable enforcement: PATCH_ONLY_DISABLE=1 git commit ...
# To check without committing: pwsh -File tools/patch_only.ps1 -Check
"@

# Append to commit message
$commitMsg + $patchFooter | Set-Content $CommitMsgFile -NoNewline