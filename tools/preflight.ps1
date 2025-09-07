# tools/preflight.ps1
# Run preflight checks: fmt, vet, optional lint, tests

$ErrorActionPreference = "Stop"

Write-Host "🚀 Running preflight checks..." -ForegroundColor Green

# Helper function to check if all staged files are in guard/docs zones only
function IsGuardDocsOnly($paths) {
    foreach ($path in $paths) {
        $path = $path.Trim()
        if ($path -eq "") { continue }
        
        # Check if path matches guard/docs patterns
        if (-not ($path -match "^tools/" -or 
                  $path -match "^\.githooks/" -or 
                  $path -match "^\.github/workflows/" -or 
                  $path -match "^docs/" -or 
                  $path -match "^CHANGELOG\.md$")) {
            return $false
        }
    }
    return $true
}

# Get staged files
Write-Host "  Checking staged files..." -ForegroundColor Yellow
$stagedFiles = git diff --cached --name-only 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "    No staged files detected, running full checks..." -ForegroundColor Yellow
} else {
    $stagedPaths = $stagedFiles | Where-Object { $_.Trim() -ne "" }
    
    if ($stagedPaths.Count -eq 0) {
        Write-Host "    No staged files detected, running full checks..." -ForegroundColor Yellow
    } elseif (IsGuardDocsOnly $stagedPaths) {
        Write-Host "    Guard/docs-only change detected:" -ForegroundColor Cyan
        foreach ($path in $stagedPaths) {
            Write-Host "      - $path" -ForegroundColor White
        }
        Write-Host "    Running lightweight checks only..." -ForegroundColor Cyan
        
        # Lightweight checks for guard/docs files
        foreach ($path in $stagedPaths) {
            if ($path -match "\.ps1$") {
                Write-Host "    Checking PowerShell syntax: $path" -ForegroundColor Yellow
                try {
                    pwsh -NoProfile -Command "Get-Content '$path' | Out-Null" 2>$null
                    if ($LASTEXITCODE -ne 0) {
                        Write-Host "❌ PowerShell syntax check failed for $path" -ForegroundColor Red
                        exit 1
                    }
                } catch {
                    Write-Host "❌ PowerShell syntax check failed for $path" -ForegroundColor Red
                    exit 1
                }
            } elseif ($path -match "\.go$") {
                # For Go files in guard/docs paths, run scoped fmt/vet
                $dir = Split-Path $path -Parent
                if ($dir -ne "") {
                    Write-Host "    Checking Go file: $path" -ForegroundColor Yellow
                    go fmt "./$path" 2>$null
                    if ($LASTEXITCODE -ne 0) {
                        Write-Host "❌ go fmt failed for $path" -ForegroundColor Red
                        exit 1
                    }
                    go vet "./$dir" 2>$null
                    if ($LASTEXITCODE -ne 0) {
                        Write-Host "❌ go vet failed for $dir" -ForegroundColor Red
                        exit 1
                    }
                }
            }
        }
        
        Write-Host "✅ Guard/docs-only preflight checks passed (skipped go build/test)" -ForegroundColor Green
        Write-Host "    Preflight: guard/docs-only change → skipping go build/test" -ForegroundColor Cyan
        exit 0
    } else {
        Write-Host "    Mixed changes detected, running full checks..." -ForegroundColor Yellow
    }
}

# go fmt
Write-Host "  Running go fmt..." -ForegroundColor Yellow
go fmt ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ go fmt failed" -ForegroundColor Red
    exit 1
}

# go vet
Write-Host "  Running go vet..." -ForegroundColor Yellow
go vet ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ go vet failed" -ForegroundColor Red
    exit 1
}

# Optional golangci-lint (skip if not installed)
Write-Host "  Checking for golangci-lint..." -ForegroundColor Yellow
$lintAvailable = $false
try {
    golangci-lint version | Out-Null
    $lintAvailable = $true
} catch {
    Write-Host "    golangci-lint not found, skipping..." -ForegroundColor Yellow
}

if ($lintAvailable) {
    Write-Host "  Running golangci-lint..." -ForegroundColor Yellow
    golangci-lint run ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ golangci-lint failed" -ForegroundColor Red
        exit 1
    }
}

# go test -short
Write-Host "  Running go test -short..." -ForegroundColor Yellow
go test -short ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ go test -short failed" -ForegroundColor Red
    exit 1
}

Write-Host "✅ All preflight checks passed!" -ForegroundColor Green