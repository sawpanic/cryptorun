#!/usr/bin/env pwsh
#
# claude_doctor.ps1 - Fix Claude Code agent front-matter and settings
#
# This script ensures:
# 1. All .claude/agents/*.md files have proper front-matter with name: field
# 2. .claude/settings.json has normalized permissions formatting
# 3. All fixes are idempotent and safe
#

param(
    [switch]$DryRun,
    [switch]$Verbose
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-Info($msg) { Write-Host "ℹ️  $msg" -ForegroundColor Cyan }
function Write-Success($msg) { Write-Host "✅ $msg" -ForegroundColor Green }
function Write-Warning($msg) { Write-Host "⚠️  $msg" -ForegroundColor Yellow }
function Write-Error($msg) { Write-Host "❌ $msg" -ForegroundColor Red }

function Get-SlugFromPath {
    param([string]$FilePath)
    $basename = [System.IO.Path]::GetFileNameWithoutExtension($FilePath)
    return $basename -replace '_', '-'
}

function Test-FrontMatter {
    param([string]$Content)
    return $Content.StartsWith('---') -and $Content.Contains('name:')
}

function Add-FrontMatter {
    param([string]$FilePath, [string]$Content)
    
    $slug = Get-SlugFromPath $FilePath
    $frontMatter = @"
---
name: $slug
description: $slug agent (add details here).
---

"@
    
    return $frontMatter + $Content
}

function Fix-AgentFiles {
    $agentsDir = Join-Path $PSScriptRoot '..' '.claude' 'agents'
    
    if (-not (Test-Path $agentsDir)) {
        Write-Warning "No .claude/agents directory found"
        return 0
    }
    
    $agentFiles = Get-ChildItem -Path $agentsDir -Filter '*.md'
    $fixedCount = 0
    
    Write-Info "Scanning $($agentFiles.Count) agent files..."
    
    foreach ($file in $agentFiles) {
        if ($Verbose) { Write-Info "Checking $($file.Name)" }
        
        $content = Get-Content -Path $file.FullName -Raw
        
        if (-not (Test-FrontMatter $content)) {
            Write-Warning "Missing front-matter in $($file.Name)"
            
            if (-not $DryRun) {
                $newContent = Add-FrontMatter $file.FullName $content
                Set-Content -Path $file.FullName -Value $newContent -NoNewline
                Write-Success "Added front-matter to $($file.Name)"
            } else {
                Write-Info "Would add front-matter to $($file.Name)"
            }
            $fixedCount++
        } else {
            if ($Verbose) { Write-Success "$($file.Name) has valid front-matter" }
        }
    }
    
    return $fixedCount
}

function Fix-SettingsPermissions {
    $settingsPath = Join-Path $PSScriptRoot '..' '.claude' 'settings.json'
    
    if (-not (Test-Path $settingsPath)) {
        Write-Warning "No .claude/settings.json found"
        return $false
    }
    
    Write-Info "Checking settings.json permissions..."
    
    try {
        $settings = Get-Content -Path $settingsPath -Raw | ConvertFrom-Json -Depth 10
        $changed = $false
        
        # Normalize permissions formatting
        if ($settings.permissions) {
            $expectedAllow = @(
                'Read(./**/*)',
                'Glob(*)',
                'Grep(*)',
                'WebFetch',
                'Bash(*)',
                'Edit(./**/*)',
                'Write(./**/*)'
            )
            
            $expectedDeny = @(
                'Read(./secrets/**)',
                'Read(./.env*)'
            )
            
            # Check if permissions need normalization
            $currentAllow = $settings.permissions.allow | Sort-Object
            $currentDeny = $settings.permissions.deny | Sort-Object
            
            if (($currentAllow -join ',') -ne ($expectedAllow -join ',')) {
                Write-Warning "Permissions 'allow' array needs normalization"
                if (-not $DryRun) {
                    $settings.permissions.allow = $expectedAllow
                    $changed = $true
                }
            }
            
            if (($currentDeny -join ',') -ne ($expectedDeny -join ',')) {
                Write-Warning "Permissions 'deny' array needs normalization"
                if (-not $DryRun) {
                    $settings.permissions.deny = $expectedDeny
                    $changed = $true
                }
            }
        }
        
        if ($changed -and -not $DryRun) {
            $settings | ConvertTo-Json -Depth 10 | Set-Content -Path $settingsPath
            Write-Success "Normalized settings.json permissions"
        } elseif ($changed) {
            Write-Info "Would normalize settings.json permissions"
        } else {
            Write-Success "settings.json permissions are already normalized"
        }
        
        return $changed
        
    } catch {
        Write-Error "Failed to process settings.json: $($_.Exception.Message)"
        return $false
    }
}

function Main {
    Write-Info "🏥 Claude Doctor - Fixing agent front-matter and settings"
    Write-Info "Working directory: $((Get-Location).Path)"
    
    if ($DryRun) {
        Write-Info "🔍 DRY RUN MODE - No files will be modified"
    }
    
    # Fix agent files
    $agentFixCount = Fix-AgentFiles
    
    # Fix settings permissions
    $settingsFixed = Fix-SettingsPermissions
    
    # Summary
    Write-Info ""
    Write-Info "📋 SUMMARY"
    Write-Info "Agent files fixed: $agentFixCount"
    Write-Info "Settings normalized: $(if ($settingsFixed) { 'Yes' } else { 'No' })"
    
    if ($agentFixCount -gt 0 -or $settingsFixed) {
        if (-not $DryRun) {
            Write-Success "🎉 Claude Doctor completed successfully!"
            Write-Info "You can now run '/doctor' in Claude Code to validate the fixes"
        } else {
            Write-Info "Run without -DryRun to apply fixes"
        }
        return 0
    } else {
        Write-Success "✨ All agent files and settings are already healthy!"
        return 0
    }
}

# Run main function
exit (Main)