#!/usr/bin/env pwsh

<#
.SYNOPSIS
Count total number of Go tests in the repository.

.DESCRIPTION  
Uses `go test -list .` to parse and count all test functions across packages.
Provides accurate test counting for CI guard delta tracking.

.EXAMPLE
.\tools\test_count.ps1
# Output: 42

.NOTES
Integrates with CI guard workflow to track test count deltas.
#>

param()

$ErrorActionPreference = "SilentlyContinue"

try {
    # Get all test functions using go test -list
    $testOutput = go test -list . ./... 2>$null
    
    if ($LASTEXITCODE -ne 0) {
        # Fallback: search for Test functions in Go files
        $goFiles = Get-ChildItem -Path . -Recurse -Include "*.go" -Exclude "vendor/*"
        $testCount = 0
        
        foreach ($file in $goFiles) {
            $content = Get-Content $file.FullName -Raw -ErrorAction SilentlyContinue
            if ($content) {
                # Match test functions: func TestXxx(t *testing.T)
                $matches = [regex]::Matches($content, '^\s*func\s+(Test\w+)\s*\([^)]*\*testing\.T[^)]*\)', [System.Text.RegularExpressions.RegexOptions]::Multiline)
                $testCount += $matches.Count
            }
        }
        
        Write-Output $testCount
        return
    }
    
    # Count lines that look like test functions
    $testLines = $testOutput | Where-Object { $_ -match '^Test\w+$' }
    $count = ($testLines | Measure-Object).Count
    
    Write-Output $count
    
} catch {
    # Final fallback: return 0
    Write-Output 0
}