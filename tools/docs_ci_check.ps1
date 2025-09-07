#!/usr/bin/env pwsh
# tools/docs_ci_check.ps1 - Docs-first CI check for CryptoRun

param(
    [string]$DocsDir = "docs",
    [string]$SrcDir = "internal",
    [string]$ConfigFile = ".docs-ci-config.yaml",
    [switch]$Fix,
    [switch]$Verbose
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Write-Host "📚 CryptoRun Docs-First CI Check v1.0" -ForegroundColor Green
Write-Host "=====================================" -ForegroundColor Green

$issues = @()
$warnings = @()

# Load configuration if it exists
$config = @{
    required_docs = @("DEPLOYMENT.md", "MONITORING.md", "SECURITY.md", "STREAMING.md")
    required_sections = @{
        "*.md" = @("## UX MUST — Live Progress & Explainability")
    }
    code_patterns = @{
        "*.go" = @{
            pattern = 'type\s+(\w+)\s+(?:struct|interface)'
            doc_requirement = "public types must be documented"
        }
        "*.yaml" = @{
            pattern = '^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*:'
            doc_requirement = "config keys should be documented"
        }
    }
    ignore_patterns = @("*_test.go", "*/testdata/*", "**/.*")
}

function Test-PathPattern {
    param($Path, $Pattern)
    return $Path -like $Pattern
}

function Write-Issue {
    param($Type, $File, $Line, $Message)
    $script:issues += [PSCustomObject]@{
        Type = $Type
        File = $File
        Line = $Line
        Message = $Message
    }
    
    if ($Verbose -or $Type -eq "ERROR") {
        $color = if ($Type -eq "ERROR") { "Red" } elseif ($Type -eq "WARNING") { "Yellow" } else { "Cyan" }
        Write-Host "[$Type] $File$(if($Line){":$Line"}) - $Message" -ForegroundColor $color
    }
}

# Check 1: Required documentation files exist
Write-Host "🔍 Checking required documentation files..." -ForegroundColor Blue

foreach ($requiredDoc in $config.required_docs) {
    $docPath = Join-Path $DocsDir $requiredDoc
    if (-not (Test-Path $docPath)) {
        Write-Issue "ERROR" $docPath 0 "Required documentation file missing"
        
        if ($Fix) {
            Write-Host "🔧 Creating template for $docPath" -ForegroundColor Yellow
            $template = @"
# $(($requiredDoc -replace '.md','') -replace '_',' ')

## UX MUST — Live Progress & Explainability

[Description of this document's purpose and scope]

## Overview

[Content goes here]

## References

- [Related documentation]
"@
            New-Item -Path $docPath -Value $template -Force | Out-Null
            Write-Host "✅ Created template: $docPath" -ForegroundColor Green
        }
    } else {
        Write-Host "✅ Found: $docPath" -ForegroundColor Green
    }
}

# Check 2: UX MUST sections in markdown files
Write-Host "`n🔍 Checking UX MUST sections..." -ForegroundColor Blue

$markdownFiles = Get-ChildItem -Path $DocsDir -Filter "*.md" -Recurse
foreach ($mdFile in $markdownFiles) {
    $content = Get-Content $mdFile.FullName -Raw -ErrorAction SilentlyContinue
    if ($content) {
        if ($content -notmatch '## UX MUST — Live Progress & Explainability') {
            Write-Issue "ERROR" $mdFile.Name 0 "Missing required UX MUST section"
            
            if ($Fix) {
                Write-Host "🔧 Adding UX MUST section to $($mdFile.Name)" -ForegroundColor Yellow
                $lines = Get-Content $mdFile.FullName
                
                # Find insertion point (after title)
                $insertIndex = 0
                for ($i = 0; $i -lt $lines.Count; $i++) {
                    if ($lines[$i] -match '^# ') {
                        $insertIndex = $i + 2
                        break
                    }
                }
                
                $newLines = @()
                $newLines += $lines[0..($insertIndex-1)]
                $newLines += "## UX MUST — Live Progress & Explainability"
                $newLines += ""
                $newLines += "[Description of progress tracking and explainability features]"
                $newLines += ""
                if ($insertIndex -lt $lines.Count) {
                    $newLines += $lines[$insertIndex..($lines.Count-1)]
                }
                
                Set-Content $mdFile.FullName $newLines
                Write-Host "✅ Added UX MUST section to $($mdFile.Name)" -ForegroundColor Green
            }
        } else {
            Write-Host "✅ UX MUST section found in $($mdFile.Name)" -ForegroundColor Green
        }
        
        # Check for broken internal links
        $internalLinks = [regex]::Matches($content, '\[([^\]]+)\]\(([^)]+\.md[^)]*)\)')
        foreach ($match in $internalLinks) {
            $linkPath = $match.Groups[2].Value
            $absoluteLinkPath = Join-Path (Split-Path $mdFile.FullName) $linkPath
            if (-not (Test-Path $absoluteLinkPath)) {
                Write-Issue "WARNING" $mdFile.Name 0 "Broken internal link: $linkPath"
            }
        }
    }
}

# Check 3: Code references and documentation alignment
Write-Host "`n🔍 Checking code-to-docs alignment..." -ForegroundColor Blue

$goFiles = Get-ChildItem -Path $SrcDir -Filter "*.go" -Recurse | Where-Object {
    $ignored = $false
    foreach ($ignorePattern in $config.ignore_patterns) {
        if (Test-PathPattern $_.FullName $ignorePattern) {
            $ignored = $true
            break
        }
    }
    -not $ignored
}

$publicTypes = @()
$publicFunctions = @()

foreach ($goFile in $goFiles) {
    $content = Get-Content $goFile.FullName -Raw -ErrorAction SilentlyContinue
    if ($content) {
        # Find public types
        $typeMatches = [regex]::Matches($content, 'type\s+([A-Z]\w+)\s+(?:struct|interface)')
        foreach ($match in $typeMatches) {
            $typeName = $match.Groups[1].Value
            $publicTypes += [PSCustomObject]@{
                Name = $typeName
                File = $goFile.Name
                RelativePath = $goFile.FullName.Replace($PWD, "").TrimStart('\').TrimStart('/')
            }
        }
        
        # Find public functions
        $funcMatches = [regex]::Matches($content, 'func\s+([A-Z]\w+)\s*\(')
        foreach ($match in $funcMatches) {
            $funcName = $match.Groups[1].Value
            $publicFunctions += [PSCustomObject]@{
                Name = $funcName
                File = $goFile.Name
                RelativePath = $goFile.FullName.Replace($PWD, "").TrimStart('\').TrimStart('/')
            }
        }
    }
}

Write-Host "📊 Found $($publicTypes.Count) public types and $($publicFunctions.Count) public functions" -ForegroundColor Cyan

# Check if critical types are documented
$criticalTypes = @("Scanner", "CompositeScore", "MomentumCore", "RegimeDetector", "EntryGates")
$allDocsContent = ""
$markdownFiles | ForEach-Object {
    $allDocsContent += Get-Content $_.FullName -Raw -ErrorAction SilentlyContinue
}

foreach ($criticalType in $criticalTypes) {
    $typeFound = $publicTypes | Where-Object { $_.Name -eq $criticalType }
    if ($typeFound) {
        if ($allDocsContent -notmatch $criticalType) {
            Write-Issue "WARNING" $typeFound.File 0 "Critical type '$criticalType' not documented in any markdown file"
        } else {
            Write-Host "✅ Critical type '$criticalType' found in documentation" -ForegroundColor Green
        }
    }
}

# Check 4: Configuration documentation
Write-Host "`n🔍 Checking configuration documentation..." -ForegroundColor Blue

$configFiles = Get-ChildItem -Path "config" -Filter "*.yaml" -Recurse -ErrorAction SilentlyContinue
$configKeys = @()

foreach ($configFile in $configFiles) {
    $content = Get-Content $configFile.FullName -Raw -ErrorAction SilentlyContinue
    if ($content) {
        $keyMatches = [regex]::Matches($content, '^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*:', [System.Text.RegularExpressions.RegexOptions]::Multiline)
        foreach ($match in $keyMatches) {
            $configKeys += [PSCustomObject]@{
                Key = $match.Groups[1].Value
                File = $configFile.Name
                ConfigPath = $configFile.FullName.Replace($PWD, "").TrimStart('\').TrimStart('/')
            }
        }
    }
}

Write-Host "📊 Found $($configKeys.Count) configuration keys" -ForegroundColor Cyan

# Check if major config sections are documented
$majorConfigKeys = @("cache", "providers", "gates", "regime", "metrics")
foreach ($majorKey in $majorConfigKeys) {
    $keyFound = $configKeys | Where-Object { $_.Key -eq $majorKey }
    if ($keyFound) {
        if ($allDocsContent -notmatch $majorKey) {
            Write-Issue "INFO" $keyFound.File 0 "Major config key '$majorKey' could benefit from documentation"
        }
    }
}

# Check 5: API endpoint documentation
Write-Host "`n🔍 Checking API endpoint documentation..." -ForegroundColor Blue

$httpFiles = Get-ChildItem -Path "internal/interfaces/http" -Filter "*.go" -Recurse -ErrorAction SilentlyContinue
$endpoints = @()

foreach ($httpFile in $httpFiles) {
    $content = Get-Content $httpFile.FullName -Raw -ErrorAction SilentlyContinue
    if ($content) {
        # Find HTTP route registrations
        $routeMatches = [regex]::Matches($content, '\.HandleFunc\("([^"]+)"')
        foreach ($match in $routeMatches) {
            $endpoint = $match.Groups[1].Value
            $endpoints += $endpoint
        }
    }
}

Write-Host "📊 Found $($endpoints.Count) API endpoints" -ForegroundColor Cyan

# Check if critical endpoints are documented
$criticalEndpoints = @("/health", "/metrics", "/scan", "/stream")
foreach ($criticalEndpoint in $criticalEndpoints) {
    if ($endpoints -contains $criticalEndpoint) {
        if ($allDocsContent -notmatch [regex]::Escape($criticalEndpoint)) {
            Write-Issue "WARNING" "API docs" 0 "Critical endpoint '$criticalEndpoint' not documented"
        } else {
            Write-Host "✅ Critical endpoint '$criticalEndpoint' found in documentation" -ForegroundColor Green
        }
    }
}

# Generate summary
Write-Host "`n📋 Summary Report" -ForegroundColor Green
Write-Host "=================" -ForegroundColor Green

$errorCount = ($issues | Where-Object { $_.Type -eq "ERROR" }).Count
$warningCount = ($issues | Where-Object { $_.Type -eq "WARNING" }).Count
$infoCount = ($issues | Where-Object { $_.Type -eq "INFO" }).Count

Write-Host "Errors: $errorCount" -ForegroundColor $(if($errorCount -gt 0){"Red"}else{"Green"})
Write-Host "Warnings: $warningCount" -ForegroundColor $(if($warningCount -gt 0){"Yellow"}else{"Green"})
Write-Host "Info: $infoCount" -ForegroundColor Cyan

if ($issues.Count -eq 0) {
    Write-Host "✅ All docs-first CI checks passed!" -ForegroundColor Green
    exit 0
} else {
    if ($Fix) {
        Write-Host "🔧 Fix mode enabled - attempted to resolve issues" -ForegroundColor Yellow
    }
    
    # Group issues by type
    $issuesByType = $issues | Group-Object Type
    foreach ($group in $issuesByType) {
        Write-Host "`n$($group.Name) Issues:" -ForegroundColor $(if($group.Name -eq "ERROR"){"Red"}elseif($group.Name -eq "WARNING"){"Yellow"}else{"Cyan"})
        foreach ($issue in $group.Group) {
            Write-Host "  - $($issue.File)$(if($issue.Line){":$($issue.Line)"}) $($issue.Message)" -ForegroundColor White
        }
    }
    
    # Exit with error code if there are errors
    if ($errorCount -gt 0) {
        Write-Host "`n❌ Documentation CI checks failed with $errorCount errors" -ForegroundColor Red
        exit 1
    } else {
        Write-Host "`n⚠️  Documentation CI checks completed with warnings" -ForegroundColor Yellow
        exit 0
    }
}