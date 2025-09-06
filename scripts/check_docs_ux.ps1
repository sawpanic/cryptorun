# CryptoRun Documentation UX Guard
# Validates UX MUST blocks and brand consistency in markdown files

param(
    [switch]$Verbose = $false
)

$ErrorActionPreference = "Stop"

# Configuration
$RequiredUXHeading = "## UX MUST — Live Progress & Explainability"
$ExcludedPaths = @(".git", "vendor", "_codereview", "out")

# Results tracking
$MissingUXFiles = @()
$BrandViolations = @()
$TotalFiles = 0

function Write-Verbose-Custom {
    param([string]$Message)
    if ($Verbose) {
        Write-Host $Message -ForegroundColor Gray
    }
}

function Test-ShouldExcludePath {
    param([string]$Path)
    
    $CleanPath = $Path -replace '\\', '/'
    foreach ($Excluded in $ExcludedPaths) {
        if ($CleanPath -match "^\.?/?$Excluded" -or $CleanPath -match "/$Excluded/") {
            return $true
        }
    }
    return $false
}

function Test-UXMustBlock {
    param([string]$FilePath)
    
    Write-Verbose-Custom "Checking UX MUST block: $FilePath"
    
    try {
        $Content = Get-Content -Path $FilePath -ErrorAction Stop
        $HasUXMustBlock = $Content -contains $RequiredUXHeading
        
        if (-not $HasUXMustBlock) {
            Write-Verbose-Custom "  Missing UX MUST block"
            $script:MissingUXFiles += $FilePath
        } else {
            Write-Verbose-Custom "  UX MUST block found"
        }
    }
    catch {
        Write-Warning "Failed to read file: $FilePath - $($_.Exception.Message)"
    }
}

function Test-BrandConsistency {
    param([string]$FilePath)
    
    Write-Verbose-Custom "Checking brand consistency: $FilePath"
    
    # Allow historic mentions only inside _codereview/**
    $IsCodereviewPath = $FilePath -match "_codereview"
    
    if ($IsCodereviewPath) {
        Write-Verbose-Custom "  Skipping _codereview path"
        return
    }
    
    try {
        $Lines = Get-Content -Path $FilePath -ErrorAction Stop
        $LineNum = 0
        
        foreach ($Line in $Lines) {
            $LineNum++
            
            # Check for brand violations, but skip documentation about the violations themselves
            $IsDocumentationAboutBrandRules = $Line.ToLower() -match 'forbidden|brand|consistency|except in|allowed only'
            
            # Check for brand violations (case insensitive)
            if (-not $IsDocumentationAboutBrandRules -and ($Line -match '(?i)\bcrypto\s*edge\b' -or $Line -match '\bCryptoEdge\b')) {
                Write-Verbose-Custom "  Brand violation found at line $LineNum"
                $script:BrandViolations += @{
                    FilePath = $FilePath
                    LineNum = $LineNum
                    Content = $Line.Trim()
                    Violation = "Found 'CryptoEdge' or 'Crypto Edge' outside _codereview/**"
                }
            }
        }
    }
    catch {
        Write-Warning "Failed to read file for brand check: $FilePath - $($_.Exception.Message)"
    }
}

function Write-Results {
    Write-Host "📋 CryptoRun Documentation UX Guard" -ForegroundColor Cyan
    Write-Host "Scanned $script:TotalFiles markdown files`n" -ForegroundColor White
    
    # Report UX MUST block violations
    if ($script:MissingUXFiles.Count -gt 0) {
        Write-Host "❌ UX MUST Block Violations ($($script:MissingUXFiles.Count) files):" -ForegroundColor Red
        Write-Host "Missing required heading: $RequiredUXHeading`n" -ForegroundColor Yellow
        
        foreach ($FilePath in $script:MissingUXFiles) {
            Write-Host "  - $FilePath" -ForegroundColor Red
        }
        Write-Host ""
    } else {
        Write-Host "✅ UX MUST Block: All files compliant`n" -ForegroundColor Green
    }
    
    # Report brand violations
    if ($script:BrandViolations.Count -gt 0) {
        Write-Host "❌ Brand Consistency Violations ($($script:BrandViolations.Count) issues):" -ForegroundColor Red
        Write-Host "Only 'CryptoRun' is permitted. 'CryptoEdge'/'Crypto Edge' allowed only in _codereview/**`n" -ForegroundColor Yellow
        
        foreach ($Violation in $script:BrandViolations) {
            Write-Host "  - $($Violation.FilePath):$($Violation.LineNum)" -ForegroundColor Red
            Write-Host "    $($Violation.Violation)" -ForegroundColor Yellow
            Write-Host "    Content: $($Violation.Content)`n" -ForegroundColor Gray
        }
    } else {
        Write-Host "✅ Brand Consistency: All mentions compliant`n" -ForegroundColor Green
    }
}

function Write-Summary {
    $HasViolations = ($script:MissingUXFiles.Count -gt 0) -or ($script:BrandViolations.Count -gt 0)
    
    if (-not $HasViolations) {
        Write-Host "✅ DOCS_UX_GUARD: PASS - $script:TotalFiles files validated" -ForegroundColor Green
        return
    }
    
    Write-Host "❌ DOCS_UX_GUARD: FAIL" -ForegroundColor Red
    if ($script:MissingUXFiles.Count -gt 0) {
        Write-Host "   UX_MUST_MISSING: $($script:MissingUXFiles.Count) files" -ForegroundColor Red
    }
    if ($script:BrandViolations.Count -gt 0) {
        Write-Host "   BRAND_VIOLATIONS: $($script:BrandViolations.Count) issues" -ForegroundColor Red
    }
}

# Main execution
try {
    Write-Verbose-Custom "Starting documentation UX guard check..."
    
    # Find all markdown files
    $MarkdownFiles = Get-ChildItem -Recurse -Filter "*.md" | Where-Object {
        -not (Test-ShouldExcludePath $_.FullName.Substring((Get-Location).Path.Length))
    }
    
    Write-Verbose-Custom "Found $($MarkdownFiles.Count) markdown files to check"
    
    foreach ($File in $MarkdownFiles) {
        $script:TotalFiles++
        $RelativePath = $File.FullName.Substring((Get-Location).Path.Length + 1)
        
        # Check UX MUST block requirement
        Test-UXMustBlock -FilePath $File.FullName
        
        # Check branding consistency
        Test-BrandConsistency -FilePath $File.FullName
    }
    
    # Output results
    Write-Results
    Write-Summary
    
    # Exit with appropriate code
    $HasViolations = ($script:MissingUXFiles.Count -gt 0) -or ($script:BrandViolations.Count -gt 0)
    if ($HasViolations) {
        exit 1
    } else {
        exit 0
    }
}
catch {
    Write-Error "Documentation UX guard failed: $($_.Exception.Message)"
    exit 1
}