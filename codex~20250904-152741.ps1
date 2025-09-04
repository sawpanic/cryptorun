# run-codex.ps1
# Safe PowerShell driver for Codex exec with Markdown context ingestion

# Step 1: Read all Markdown files in repo and subfolders
$mdFiles = Get-ChildItem -Path . -Recurse -Filter *.md
$mdContent = ""
foreach ($file in $mdFiles) {
    $relPath = Resolve-Path -Relative $file.FullName
    $mdContent += "`n----- $relPath -----`n"
    $mdContent += Get-Content $file.FullName -Raw
    $mdContent += "`n"
}

# Step 2: Build Codex prompt as a here-string
$PROMPT = @"
# SHELL CONSTRAINTS (Windows PowerShell only)
- Do NOT use bash heredocs, apply_patch, sed -i, or ed.
- File writes must use PowerShell-safe commands: New-Item, Set-Content -Encoding UTF8, or Out-File.

# CONTEXT REQUIREMENT
The following Markdown files have been read for context:
$($mdFiles | ForEach-Object { $_.FullName } | Out-String)

$mdContent

# IDEMPOTENCY RULES
- If a feature already exists and matches mission.md and other .md requirements: DO NOTHING.
- Only add missing features or fix incorrect implementations.
- Preserve all working code.

# TASKS
- Implement requirements described in mission.md and related docs under ./src with clean architecture.
- Create CLI stubs: scan, backtest, monitor, health.
- Add scoring skeletons including Hurst estimator and EntryGatesDetailed() with late fill guard and cooldown.
- Add network-disabled stubs (Kraken WS, CoinGecko client, historical loader) guarded by a build tag (//go:build no_net).
- Ensure build passes with: go build -tags no_net ./...
- Connect backtest stub to scoring stub for compile-time integration.
- Print API health/circuit breaker monitoring stubs.
- Respect config files described in mission.md (apis.yaml, cache.yaml, circuits.yaml, regimes.yaml).
- Do not use aggregators for microstructure — exchange-native only.

# VERIFICATION
- Print list of all .md files read.
- Print structured table of actions: {Created|Modified|Skipped, Path, Reason}.
- Print build result for go build -tags no_net ./...
- Print any skipped features with justification.
- Print "DONE" only after idempotency checks and successful build.
"@

# Step 3: Execute Codex with the constructed prompt
codex exec -C C:\wallet\CProtocol --sandbox danger-full-access $PROMPT
