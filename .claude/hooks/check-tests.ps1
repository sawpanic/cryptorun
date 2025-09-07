Param()
$ErrorActionPreference = "Stop"

function Has-Path { param($p) Test-Path -LiteralPath $p }

$result = [ordered]@{ build="ok"; tests="ok"; failures=@() }

# Go build/tests
if (Test-Path ".") {
  try {
    $null = & go build -tags no_net ./... 2>&1
  } catch {
    $result.build = "fail"
    $result.failures += "go build failed: $($_.Exception.Message)"
  }
  try {
    $tests = & go test ./... -count=1 2>&1
    if ($LASTEXITCODE -ne 0) {
      $result.tests = "fail"
      $result.failures += ($tests | Out-String).Trim()
    }
  } catch {
    $result.tests = "fail"
    $result.failures += "go test crashed"
  }
}

# Python tests (optional)
if (Has-Path ".\tests\python" -or (Get-ChildItem -Recurse -Include "pytest.ini","pyproject.toml" -ErrorAction SilentlyContinue)) {
  try {
    $py = & pytest -q 2>&1
    if ($LASTEXITCODE -ne 0) {
      $result.tests = "fail"
      $result.failures += ($py | Out-String).Trim()
    }
  } catch {
    $result.tests = "fail"
    $result.failures += "pytest crashed"
  }
}

# PowerShell script analysis (optional)
try {
  if (Get-Command Invoke-ScriptAnalyzer -ErrorAction SilentlyContinue) {
    $sa = Invoke-ScriptAnalyzer -Recurse -Severity Error 2>&1
    if ($sa) {
      $result.tests = "fail"
      $result.failures += ($sa | Out-String).Trim()
    }
  }
} catch {}

$resultJson = ($result | ConvertTo-Json -Depth 6)
Write-Output $resultJson

# Exit code contract for Claude hooks:
# 0 => allow; nonzero => deny
if ($result.build -eq "fail" -or $result.tests -eq "fail") { exit 2 } else { exit 0 }
