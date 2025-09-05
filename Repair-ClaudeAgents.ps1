# Repair-ClaudeAgents.ps1
# Audits and fixes Claude Code subagents so they show up in /agents.
# - Renames to kebab-case
# - Adds YAML front-matter with name/description/tools
# - Creates missing agents (historian, shipwright, analyst-trader)

$ErrorActionPreference = "Stop"
$root = Get-Location
$agentsDir = ".\.claude\agents"
if (-not (Test-Path $agentsDir)) { New-Item -ItemType Directory -Force -Path $agentsDir | Out-Null }

function Kebab([string]$s){
  $s = [System.IO.Path]::GetFileNameWithoutExtension($s)
  $s = $s -replace '[_\s]+','-'
  $s = $s -replace '[^a-zA-Z0-9\-]',''
  $s.ToLower()
}

# Canonical names & default metadata
$catalog = @(
  @{ slug="feature-builder";          file="builder.md";         desc="Feature implementation in ./src with tests-first discipline."; tools="Read, Grep, Glob, Edit, Write, Bash" },
  @{ slug="code-verifier";            file="qa.md";              desc="Builds and runs tests; blocks writes when red.";               tools="Read, Bash" },
  @{ slug="security-risk-officer";    file="security.md";        desc="Secret scanning and dependency hygiene; no edits.";           tools="Read, Bash" },
  @{ slug="market-microstructure-scout"; file="microstructure.md"; desc="Implements/validates spread, ±2% depth, VADR, ADV gates."; tools="Read, Edit, Write" },
  @{ slug="catalyst-harvester";       file="catalyst.md";        desc="Collects catalysts from whitelisted sources; no writes.";     tools="Read, WebFetch" },
  @{ slug="regime-detector-4h";       file="regime.md";          desc="Maintains 4h regime detector and weight blends.";             tools="Read, Edit, Write" },
  @{ slug="historian";                file="historian.md";       desc="Ensures point-in-time integrity; emits invariants report.";   tools="Read, Bash" },
  @{ slug="shipwright";               file="shipwright.md";      desc="Prepares releases/PRs; never merges.";                        tools="Read, Bash" },
  @{ slug="analyst-trader";           file="analyst_trader.md";  desc="Compares venue top gainers vs scanner; explains misses.";     tools="Read, WebFetch, Bash" }
)

# Ensure each agent file exists with valid front-matter
foreach ($agent in $catalog) {
  $intendedSlug = $agent.slug
  $defaultFile  = Join-Path $agentsDir $agent.file
  $existing = Get-ChildItem $agentsDir -Filter "*.md" -File -ErrorAction SilentlyContinue |
              Where-Object {
                (Kebab $_.Name) -in @($intendedSlug, (Kebab $agent.file))
              } | Select-Object -First 1

  $path = $defaultFile
  if ($existing) { $path = $existing.FullName }

  if (-not (Test-Path $path)) {
    # Create minimal body
    $body = @"
---
name: $intendedSlug
description: $($agent.desc)
tools: $($agent.tools)
---

# $intendedSlug
$($agent.desc)
"@
    $dir = Split-Path -Parent $path
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    $body | Out-File -Encoding UTF8 $path
    Write-Host "✔ Created $path"
  } else {
    # Fix filename to kebab-case slug.md
    $targetName = "$intendedSlug.md"
    $targetPath = Join-Path $agentsDir $targetName
    if ((Split-Path -Leaf $path) -ne $targetName) {
      Rename-Item -LiteralPath $path -NewName $targetName -Force
      $path = $targetPath
      Write-Host "✔ Renamed -> $targetName"
    }

    # Ensure YAML front-matter present & correct
    $text = Get-Content -Raw -LiteralPath $path
    if (-not ($text -match '^\s*---\s*[\s\S]*?---')) {
      $front = @"
---
name: $intendedSlug
description: $($agent.desc)
tools: $($agent.tools)
---

"@
      ($front + $text) | Out-File -Encoding UTF8 $path
      Write-Host "✔ Added YAML front-matter to $(Split-Path -Leaf $path)"
    } else {
      # Update name field if needed
      $updated = $text -replace '(?ms)^---\s*([\s\S]*?)\s*---', {
        param($m)
        $yaml = $m.Groups[1].Value
        if ($yaml -notmatch '(?m)^\s*name\s*:\s*') {
          $yaml = "name: $intendedSlug`r`n" + $yaml
        } else {
          $yaml = $yaml -replace '(?m)^\s*name\s*:\s*.*$', "name: $intendedSlug"
        }
        if ($yaml -notmatch '(?m)^\s*description\s*:') {
          $yaml += "`r`ndescription: $($agent.desc)"
        }
        if ($yaml -notmatch '(?m)^\s*tools\s*:') {
          $yaml += "`r`ntools: $($agent.tools)"
        }
        "---`r`n$yaml`r`n---"
      }
      if ($updated -ne $text) {
        $updated | Out-File -Encoding UTF8 $path
        Write-Host "✔ Normalized YAML in $(Split-Path -Leaf $path)"
      }
    }
  }
}

# Final list
Write-Host "`nAgents present:"
Get-ChildItem $agentsDir -Filter "*.md" | ForEach-Object { " - " + (Split-Path -Leaf $_.FullName) }

Write-Host "`nNext:"
Write-Host "  1) Restart Claude Code in this repo or run /agents to refresh."
Write-Host "  2) You should now see all 9 subagents listed under Project agents."
