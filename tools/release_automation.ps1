#!/usr/bin/env pwsh
# tools/release_automation.ps1 - Automated release and changelog management

param(
    [Parameter(Mandatory=$true)]
    [string]$Version,
    
    [string]$ChangelogPath = "CHANGELOG.md",
    [switch]$DryRun,
    [switch]$Force
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Write-Host "🚀 CryptoRun Release Automation v1.0" -ForegroundColor Green
Write-Host "======================================" -ForegroundColor Green

# Validate version format (semantic versioning)
if ($Version -notmatch '^v?\d+\.\d+\.\d+(-[a-zA-Z0-9]+)?$') {
    Write-Error "❌ Invalid version format. Use semantic versioning (e.g., v3.2.1, 1.0.0-beta)"
    exit 1
}

# Normalize version (ensure 'v' prefix)
$NormalizedVersion = if ($Version.StartsWith('v')) { $Version } else { "v$Version" }

Write-Host "📋 Release Configuration:" -ForegroundColor Yellow
Write-Host "  Version: $NormalizedVersion"
Write-Host "  Changelog: $ChangelogPath"
Write-Host "  Dry Run: $DryRun"
Write-Host ""

# Check for uncommitted changes
try {
    $gitStatus = git status --porcelain 2>$null
    if ($gitStatus -and -not $Force) {
        Write-Error "❌ Uncommitted changes detected. Please commit or stash changes first. Use -Force to override."
        exit 1
    }
} catch {
    Write-Error "❌ Not in a git repository or git not available"
    exit 1
}

# Check if changelog exists
if (-not (Test-Path $ChangelogPath)) {
    Write-Error "❌ Changelog not found at $ChangelogPath"
    exit 1
}

Write-Host "📝 Processing changelog..." -ForegroundColor Blue

$changelog = Get-Content $ChangelogPath -Raw
$releaseDate = Get-Date -Format "yyyy-MM-dd"

# Check for unreleased section
if ($changelog -notmatch '## \[Unreleased\]') {
    Write-Warning "⚠️  No [Unreleased] section found in changelog"
    if (-not $Force) {
        $continue = Read-Host "Continue anyway? (y/N)"
        if ($continue -ne 'y') {
            Write-Host "❌ Release cancelled"
            exit 1
        }
    }
} else {
    # Extract unreleased changes
    $unreleasedPattern = '## \[Unreleased\]\s*\n(.*?)(?=\n## |$)'
    if ($changelog -match $unreleasedPattern) {
        $unreleasedContent = $matches[1].Trim()
        
        if ($unreleasedContent -eq '' -or $unreleasedContent -match '^\s*$') {
            Write-Warning "⚠️  No changes in [Unreleased] section"
            if (-not $Force) {
                $continue = Read-Host "Continue with empty release? (y/N)"
                if ($continue -ne 'y') {
                    Write-Host "❌ Release cancelled"
                    exit 1
                }
            }
            $unreleasedContent = "No changes documented"
        }
        
        Write-Host "📋 Unreleased changes found:" -ForegroundColor Green
        Write-Host $unreleasedContent -ForegroundColor Cyan
        Write-Host ""
        
        # Create new release section
        $releaseSection = @"
## [$NormalizedVersion] - $releaseDate
$unreleasedContent

## [Unreleased]

"@
        
        # Replace unreleased section with release section
        $updatedChangelog = $changelog -replace '## \[Unreleased\]\s*\n.*?(?=\n## |\Z)', $releaseSection, 1
        
        if ($DryRun) {
            Write-Host "🔍 DRY RUN: Would update changelog:" -ForegroundColor Yellow
            Write-Host $releaseSection -ForegroundColor Cyan
        } else {
            Set-Content $ChangelogPath $updatedChangelog -NoNewline
            Write-Host "✅ Changelog updated with release $NormalizedVersion" -ForegroundColor Green
        }
    }
}

# Create git tag
Write-Host "🏷️  Creating git tag..." -ForegroundColor Blue

if ($DryRun) {
    Write-Host "🔍 DRY RUN: Would create git tag: $NormalizedVersion" -ForegroundColor Yellow
    
    # Show what would be tagged
    $commitHash = git rev-parse HEAD 2>$null
    $commitMessage = git log -1 --pretty=format:"%s" 2>$null
    Write-Host "  Commit: $commitHash" -ForegroundColor Cyan
    Write-Host "  Message: $commitMessage" -ForegroundColor Cyan
} else {
    try {
        # Check if tag already exists
        $existingTag = git tag -l $NormalizedVersion 2>$null
        if ($existingTag) {
            Write-Error "❌ Tag $NormalizedVersion already exists"
            exit 1
        }
        
        # Add changelog to git if modified
        if (git diff --name-only | Select-String $ChangelogPath) {
            git add $ChangelogPath
            git commit -m "docs: update changelog for release $NormalizedVersion"
        }
        
        # Create annotated tag
        git tag -a $NormalizedVersion -m "Release $NormalizedVersion"
        Write-Host "✅ Created git tag: $NormalizedVersion" -ForegroundColor Green
        
    } catch {
        Write-Error "❌ Failed to create git tag: $($_.Exception.Message)"
        exit 1
    }
}

# Generate release notes
Write-Host "📄 Generating release notes..." -ForegroundColor Blue

$releaseNotes = @"
# CryptoRun $NormalizedVersion Release Notes

**Release Date:** $releaseDate

## Changes
$unreleasedContent

## Installation

\`\`\`bash
# Download binary
curl -L https://github.com/yourorg/cryptorun/releases/download/$NormalizedVersion/cryptorun-$NormalizedVersion-linux-amd64.tar.gz

# Or build from source
git checkout $NormalizedVersion
go build -o cryptorun ./cmd/cryptorun
\`\`\`

## Verification

\`\`\`bash
# Verify version
./cryptorun --version

# Run health check
./cryptorun health
\`\`\`

## Documentation

- [Deployment Guide](docs/DEPLOYMENT.md)
- [Monitoring Setup](docs/MONITORING.md)
- [Security Policy](docs/SECURITY.md)
- [Streaming Architecture](docs/STREAMING.md)

---
*Generated by release automation on $releaseDate*
"@

$releaseNotesPath = "RELEASE_NOTES_$NormalizedVersion.md"

if ($DryRun) {
    Write-Host "🔍 DRY RUN: Would create release notes:" -ForegroundColor Yellow
    Write-Host $releaseNotesPath -ForegroundColor Cyan
} else {
    Set-Content $releaseNotesPath $releaseNotes -NoNewline
    Write-Host "✅ Release notes created: $releaseNotesPath" -ForegroundColor Green
}

# Build summary
Write-Host ""
Write-Host "🎉 Release Summary" -ForegroundColor Green
Write-Host "==================" -ForegroundColor Green
Write-Host "Version: $NormalizedVersion" -ForegroundColor White
Write-Host "Date: $releaseDate" -ForegroundColor White
Write-Host "Files modified:" -ForegroundColor White
Write-Host "  - $ChangelogPath (updated)" -ForegroundColor Cyan
Write-Host "  - $releaseNotesPath (created)" -ForegroundColor Cyan
Write-Host "  - Git tag: $NormalizedVersion (created)" -ForegroundColor Cyan

if ($DryRun) {
    Write-Host ""
    Write-Host "🔍 This was a DRY RUN - no changes were made" -ForegroundColor Yellow
    Write-Host "To execute: Remove -DryRun flag" -ForegroundColor Yellow
} else {
    Write-Host ""
    Write-Host "✅ Release $NormalizedVersion completed successfully!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Yellow
    Write-Host "1. Push changes: git push origin main --tags" -ForegroundColor White
    Write-Host "2. Create GitHub release with $releaseNotesPath" -ForegroundColor White
    Write-Host "3. Build and publish artifacts" -ForegroundColor White
    Write-Host "4. Update deployment environments" -ForegroundColor White
}