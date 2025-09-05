<# 
Create-CryptoRunClaudeScaffold.ps1
Creates .claude/agents/*.md, .claude/hooks/*.ps1, and .claude/settings.json
Idempotent: skips existing files unless -ForceOverwrite is supplied.
#>

[CmdletBinding()]
param(
  [switch]$ForceOverwrite
)

$ErrorActionPreference = "Stop"
$root = Get-Location

function New-File {
  param(
    [Parameter(Mandatory)] [string]$Path,
    [Parameter(Mandatory)] [string]$Content,
    [switch]$Force
  )
  $dir = Split-Path -Parent $Path
  if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
  if ((Test-Path $Path) -and -not $Force) {
    Write-Host "✓ Exists (skipping): $Path"
  } else {
    $Content | Out-File -FilePath $Path -Encoding UTF8 -Force
    Write-Host "✔ Wrote: $Path"
  }
}

# ---------- Agent contents ----------
$builder = @'
# ROLE
You are the CryptoRun Builder. Implement features and refactors **only under `./src/**`** with tests-first discipline.

# MISSION
- Implement requirements from CProtocol/CryptoRun PRD v3.2.1.
- Always generate/update tests **before** code changes. Do not write code that lacks tests.
- After changes: run tests; if red, revert your edits.

# SCOPE & GUARDRAILS
- Allowed tools (project policy will enforce): Read, Glob, Grep, Edit/Write in `./src/**`, Bash for `go build/test` only.
- Forbidden: touching secrets, `.env*`, network writes, non-free APIs, live trading logic.
- USD pairs only, Kraken preferred; exchange-native L1/L2 only; no aggregators.

# HARD RULES (map 1:1 to code)
- Momentum weights: 4h 35%, 1h 20%, 12h 30%, 24h 10–15% (configurable).
- Freshness ≤ 2 bars; late-fill guard < 30s.
- Fatigue guard: if 24h > +12% & RSI(4h) > 70, block unless renewed acceleration.
- Microstructure gates (at decision time): spread < 50 bps; depth ±2% ≥ $100k; VADR > 1.75×; ADV caps.
- Orthogonal factor order: MomentumCore (protected) → TechnicalResidual → VolumeResidual → QualityResidual → SocialResidual (cap +10).
- Regime detector updates 4h; adjusts weight blends.
- Entries/Exits per PRD; transparent scoring; keyless/free APIs only.

# INPUTS
- `./config/*.json` thresholds
- `./data/**` mocks/fixtures
- `./tests/**` for test scaffolding

# OUTPUTS
- Code in `./src/**`
- Tests in `./tests/**`
- A changelog snippet in PR description (Shipwright handles final changelog)

# METHOD
1) Read spec & tests; propose a diff plan as a bullet list.
2) Create/modify tests; run `go test ./...` (and pytest if exists).
3) Implement minimal code to make tests green.
4) Re-run tests; output a short “Created|Modified|Skipped, Path, Reason” table.
5) If anything fails → revert edits and explain why.
'@

$qa = @'
# ROLE
You are the QA Verifier. You never edit code.

# JOB
- Build and test the repository. Block any write attempts if tests are red.

# TOOLS & LIMITS
- Allowed: Read, Bash(go build/test), read artifacts.
- Forbidden: Edit/Write, WebFetch.

# COMMANDS
- `go build -tags no_net ./...`
- `go test ./... -count=1`
- If `tests/python` exists: `pytest -q`
- If PowerShell scripts exist: `pwsh -NoProfile -c "Invoke-ScriptAnalyzer -Recurse -Severity Error"`

# OUTPUT
- Print a JSON summary: { "build":"ok|fail", "tests":"ok|fail", "failures":[...] }
- Exit nonzero only for infrastructure failure (not test red); test red is reported JSON for hooks to interpret.
'@

$security = @'
# ROLE
Security/Risk Officer. Detect secrets and risky deps. You never edit code.

# CHECKS
- Run `gitleaks detect --no-banner --redact --source .` if available.
- Fallback secret scan: ripgrep patterns for `(?i)(api[_-]?key|secret|password|token)=`.
- Flag dependency risks (lockfiles) with `pwsh` heuristics if `socket`/SCA unavailable.

# OUTPUT
- JSON: { "secrets":["path:line:match",...], "deps":["issue",...], "verdict":"pass|block" }
- If `verdict=block`, explain *which file* triggered the block.
'@

$microstructure = @'
# ROLE
Microstructure Scout. Implement and unit-test gates using exchange-native data.

# RULES
- Compute at decision time: spread(bps), depth ±2% (USD both sides), VADR, ADV.
- Use only Kraken/OKX/Coinbase native L1/L2; USD pairs only.
- No WebFetch here; consume interface/fixtures from ./data.

# OUTPUT
- Deterministic functions with pure inputs (snapshots) → booleans + metrics.
- Unit tests with edge cases (thin books, wide spreads, outage flags).
'@

$catalyst = @'
# ROLE
Catalyst Harvester. Gather/score catalysts with time-decay multipliers.

# SCOPE
- Allowed: WebFetch (whitelist only: api.coingecko.com, official exchange calendars/docs); Read.
- Forbidden: Edit/Write; scraping disallowed paths; ignoring robots.txt.

# TASK
- Produce `./out/catalyst/events.json` with fields:
  {symbol, title, date, source, heat_bucket: "imminent|near|medium|distant", multiplier}
- Respect rate limits; add `source_url` and `retrieved_at`.
'@

$regime = @'
# ROLE
Regime Detector. Maintain/update the 4h regime and weight blends.

# TASKS
- Compute: realized vol (7d), % above 20MA, breadth thrust.
- Map to regimes (e.g., trending, mean-reverting, high-vol chop).
- Emit `./out/regime/latest.json` and unit tests for transitions (hysteresis to avoid flapping).
'@

$historian = @'
# ROLE
Backtest Historian. Ensure point-in-time integrity and stable VADR.

# DUTIES
- Validate that all backtests use point-in-time data.
- Check VADR stability windows and flag suspicious jumps.
- Emit invariants report to `./out/history/invariants.json`.
'@

$shipwright = @'
# ROLE
Shipwright. Prepare releases and PRs. You do not merge.

# TASKS
- Synthesize CHANGELOG entry from diffs/tests.
- Bump semver if needed.
- Open PR with labels and summary. Request reviewers.
- Attach artifacts (coverage, analyst report). Never auto-merge.
'@

$analyst = @'
# ROLE
Analyst/Trader Validator. Hourly, compare venue TopN 1h/24h/7d winners to scanner candidates and diagnose misses.

# LIMITS
- Allowed: Read, Bash(go run/test for small helpers), WebFetch (whitelist only).
- Forbidden: Edit/Write.

# INPUTS
- ./out/scanner/latest_candidates.jsonl  (with gate traces)
- ./out/microstructure/snapshots/*.json  (point-in-time)
- ./config/gates.json, ./config/universe.json
- ./out/regime/latest.json

# EXTERNAL (whitelisted)
- api.kraken.com, api.coingecko.com (+ optional OKX/Coinbase if enabled)

# OUTPUTS
- ./out/analyst/winners.json
- ./out/analyst/misses.jsonl  // {symbol, window, ret, reason_code, verdict, evidence}
- ./out/analyst/coverage.json
- ./out/analyst/report.md

# DECISION CODES
UNIVERSE_EXCLUDED, DATA_STALE, FRESHNESS_FAIL, FATIGUE_GUARD, SPREAD_WIDE, DEPTH_THIN,
VADR_LOW, ADV_CAP, REGIME_BLOCK, SOCIAL_CAP, CATALYST_NONE, CORRELATION_CAP,
QUALITY_FILTER, SCORE_BELOW, ENGINE_BUG, UNKNOWN

# VERDICTS
GOOD_FILTER, BAD_MISS, NEEDS_REVIEW

# METHOD (tight)
1) Pull TopN winners for 1h/24h/7d (Kraken first); confirm USD pair exists on venue.
2) Join with candidates; for misses, re-evaluate gates using recorded snapshot + thresholds.
3) Emit verdict + metrics; list top recurring reason codes.
'@

# ---------- Hooks ----------
$checkTests = @'
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
'@

$webfetchAllow = @'
Param(
  [string]$Url
)

if (-not $Url) {
  $stdin = [Console]::In.ReadToEnd().Trim()
  if ($stdin) { $Url = $stdin }
}
if (-not $Url) { Write-Output '{"allow":false,"reason":"no URL provided"}'; exit 2 }

$allow = @(
  'https://api.coingecko.com',
  'https://api.kraken.com',
  'https://api.exchange.coinbase.com',
  'https://api.pro.coinbase.com',
  'https://www.okx.com'
)

$uri = [Uri]$Url
$origin = $uri.Scheme + '://' + $uri.Host

$ok = $false
foreach ($a in $allow) { if ($origin -like "$a*") { $ok = $true; break } }

if ($ok) {
  Write-Output ('{"allow":true,"url":"' + $Url + '"}')
  exit 0
} else {
  Write-Output ('{"allow":false,"reason":"domain not whitelisted","url":"' + $Url + '"}')
  exit 2
}
'@

$promptGuard = @'
Param()
$prompt = [Console]::In.ReadToEnd()

$forbiddenPatterns = @(
  '(?i)dangerously-?skip-?permissions',
  '(?i)dangerously-?bypass-?approvals',
  '(?i)--bypass-?sandbox',
  '(?i)rm\s+-rf\s+[/\\]\S+',
  '(?i)icacls\s+.+\s+/grant\s+everyone',
  '(?i)powershell\s+-nop\s+-w\s+hidden',
  '(?i)curl\s+.+\|\s*sh'
)

$matches = @()
foreach ($pat in $forbiddenPatterns) {
  if ($prompt -match $pat) { $matches += $pat }
}

if ($matches.Count -gt 0) {
  $obj = [ordered]@{
    allow = $false
    reason = "Prompt contains forbidden directives."
    matches = $matches
  }
  $obj | ConvertTo-Json -Depth 5
  exit 2
} else {
  '{"allow":true}' | Write-Output
  exit 0
}
'@

# ---------- settings.json ----------
$settings = @'
{
  "permissions": {
    "allow": [
      "Read(./**/*)",
      "Glob(*)",
      "Grep(*)",
      "Bash(go build:*)",
      "Bash(go test:*)",
      "Bash(pwsh -NoProfile -File ./.claude/hooks/*.ps1:*)",
      "Edit(./src/**)",
      "Write(./src/**)"
    ],
    "ask": [
      "Edit(./tests/**)",
      "Write(./tests/**)"
    ],
    "deny": [
      "Read(./secrets/**)",
      "Read(./.env*)",
      "WebFetch",
      "Bash(curl:*)",
      "Bash(wget:*)",
      "Bash(git push:*)"
    ]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [{
          "type": "command",
          "command": "pwsh -NoProfile -File ./.claude/hooks/check-tests.ps1",
          "timeout": 180
        }]
      },
      {
        "matcher": "WebFetch",
        "hooks": [{
          "type": "command",
          "command": "pwsh -NoProfile -File ./.claude/hooks/webfetch-allow.ps1",
          "timeout": 20
        }]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [{
          "type": "command",
          "command": "pwsh -NoProfile -File ./.claude/hooks/check-tests.ps1",
          "timeout": 180
        }]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [{
          "type": "command",
          "command": "pwsh -NoProfile -File ./.claude/hooks/prompt-guard.ps1",
          "timeout": 5
        }]
      }
    ]
  },
  "env": {
    "CLAUDE_CODE_MAX_OUTPUT_TOKENS": "4096",
    "BASH_MAX_OUTPUT_LENGTH": "100000"
  },
  "defaultMode": "acceptEdits",
  "disableBypassPermissionsMode": "disable"
}
'@

# ---------- Write everything ----------
$force = [bool]$ForceOverwrite

New-File -Path ".claude/agents/builder.md"        -Content $builder -Force:$force
New-File -Path ".claude/agents/qa.md"             -Content $qa -Force:$force
New-File -Path ".claude/agents/security.md"       -Content $security -Force:$force
New-File -Path ".claude/agents/microstructure.md" -Content $microstructure -Force:$force
New-File -Path ".claude/agents/catalyst.md"       -Content $catalyst -Force:$force
New-File -Path ".claude/agents/regime.md"         -Content $regime -Force:$force
New-File -Path ".claude/agents/historian.md"      -Content $historian -Force:$force
New-File -Path ".claude/agents/shipwright.md"     -Content $shipwright -Force:$force
New-File -Path ".claude/agents/analyst_trader.md" -Content $analyst -Force:$force

New-File -Path ".claude/hooks/check-tests.ps1"    -Content $checkTests -Force:$force
New-File -Path ".claude/hooks/webfetch-allow.ps1" -Content $webfetchAllow -Force:$force
New-File -Path ".claude/hooks/prompt-guard.ps1"   -Content $promptGuard -Force:$force

New-File -Path ".claude/settings.json"            -Content $settings -Force:$force

Write-Host "`nAll done. Next steps:"
Write-Host "1) Run:  claude --permission-mode plan"
Write-Host "2) Assign agents per task with -a (or via UI)."
Write-Host "3) Wire your scanner to emit ./out/scanner/latest_candidates.jsonl with gate traces."
