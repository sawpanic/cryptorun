# tools/preflight.ps1
# Run preflight checks: fmt, vet, optional lint, tests

$ErrorActionPreference = "Stop"

Write-Host "🚀 Running preflight checks..." -ForegroundColor Green

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