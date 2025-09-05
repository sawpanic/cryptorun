# LocalCI: fast quality gate for dev box and nightly schedule
`$ErrorActionPreference = 'Stop'
function Have(`$cmd){ try{ Get-Command `$cmd -ErrorAction Stop | Out-Null; `$true } catch { `$false } }

`$fail = @()

if(Have 'go'){
  & go build -tags no_net ./... 2>&1; if(`$LASTEXITCODE -ne 0){ `$fail += 'go build' }
  & go test ./... -count=1 2>&1;     if(`$LASTEXITCODE -ne 0){ `$fail += 'go test' }
}

if(Test-Path .\tests\python -or (Get-ChildItem -Recurse -Include "pytest.ini","pyproject.toml" -ErrorAction SilentlyContinue)){
  if(Have 'pytest'){ & pytest -q 2>&1; if(`$LASTEXITCODE -ne 0){ `$fail += 'pytest' } }
}

if(Have 'gitleaks'){
  & gitleaks detect --no-banner --redact --source . 2>$null
  if(`$LASTEXITCODE -ne 0){ `$fail += 'gitleaks' }
}

# Optional: kick Analyst (non-fatal)
try {
  if (Test-Path .\tools\AnalystHourly.ps1) {
    & pwsh -NoProfile -File .\tools\AnalystHourly.ps1 -TopN 20 | Out-Null
  }
} catch {}

if(`$fail.Count -gt 0){
  "`nLocalCI FAIL: $(`$fail -join ', ')" | Write-Host
  exit 2
}else{
  Write-Host "`nLocalCI OK"
  exit 0
}
