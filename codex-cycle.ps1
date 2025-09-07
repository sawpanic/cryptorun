# codex-cycle.ps1
# Run multi-phase Codex builds with Markdown context injected every time.

function Get-MdContext {
    $mdFiles   = Get-ChildItem -Recurse -Filter *.md
    $mdContent = ($mdFiles | ForEach-Object { "----- $($_.FullName) -----`n" + (Get-Content $_.FullName -Raw) }) -join "`n"

    return @"
# CONTEXT (all Markdown docs)
The following .md files have been read:
$($mdFiles | ForEach-Object { $_.FullName } | Out-String)

$mdContent
"@
}

function Invoke-CodexPhase {
    param(
        [Parameter(Mandatory=$true)]
        [string]$PhaseInstructions
    )

    $Context = Get-MdContext

    $Prompt = @"
$Context

# SHELL CONSTRAINTS
- Shell: PowerShell only. Forbidden: bash heredocs, apply_patch, sed -i, ed.
- File writes: New-Item / Set-Content -Encoding UTF8 / Out-File only.

# IDEMPOTENCY RULES
- If feature already exists and matches requirements in docs: DO NOTHING.
- Only add missing or fix incorrect implementations.
- Preserve working code.

# PHASE INSTRUCTIONS
$PhaseInstructions

# VERIFICATION
- Print list of .md files read.
- Print structured table {Created|Modified|Skipped, Path, Reason}.
- Print build result for `go build -tags no_net ./...`.
- Print "DONE" only after idempotency checks and successful build.
"@

    # Pipe prompt into Codex exec via stdin to avoid Windows argument-length limits
    $Prompt | codex exec -C C:\wallet\CProtocol --sandbox danger-full-access
}

Write-Host "Codex cycle ready. Use: Invoke-CodexPhase 'PHASE 1: scaffold CLI stubs …'"
