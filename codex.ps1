# run-codex.ps1
# Script to run Codex exec with all Markdown docs as context

# Step 1: Gather all Markdown docs
$mdFiles = Get-ChildItem -Path . -Recurse -Filter *.md
$mdContent = ""
foreach ($file in $mdFiles) {
    $relPath = Resolve-Path -Relative $file.FullName
    $mdContent += "`n----- $relPath -----`n"
    $mdContent += Get-Content $file.FullName -Raw
    $mdContent += "`n"
}

# Step 2: Build the full Codex prompt
$PromptText = @"
# CONTEXT REQUIREMENT
The following Markdown files have been read for context:
$($mdFiles | ForEach-Object { $_.FullName } | Out-String)

$mdContent

# SHELL CONSTRAINTS
- Shell: PowerShell only. Forbidden: bash heredocs, apply_patch, sed -i, ed.
- File ops must use PowerShell-safe commands.

# IDEMPOTENCY
- If feature already exists and matches docs/mission.md and related .mds: DO NOTHING.
- Only add missing or fix incorrect features. Preserve working code.

# TASKS
- Implement requirements from mission.md and related docs under ./src.
- Create CLI stubs (scan, backtest, monitor, health).
- Add scoring skeletons (Hurst estimator, EntryGatesDetailed with late fill guard/cooldown).
- Add network-disabled stubs (Kraken WS, CoinGecko client, historical loader) with //go:build no_net.
- Ensure build passes with: go build -tags no_net ./...
- Respect config files as described (apis.yaml, cache.yaml, circuits.yaml, regimes.yaml).
- NEVER use aggregators for microstructure.

# VERIFICATION
- Print list of .md files read.
- Print structured table: {Created|Modified|Skipped, Path, Reason}.
- Print build result for go build -tags no_net ./...
- Print "DONE" only after idempotency checks and successful build.
"@

# Step 3: Run Codex exec with prompt
codex exec -C C:\wallet\CProtocol --sandbox danger-full-access "$PromptText"
