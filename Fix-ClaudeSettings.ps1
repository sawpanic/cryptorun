$settingsPath = Join-Path $PWD ".claude\settings.json"

if (!(Test-Path $settingsPath)) { Write-Error "Missing .claude\settings.json"; exit 1 }

try { $json = Get-Content $settingsPath -Raw | ConvertFrom-Json } catch {
  Write-Error "settings.json is not valid JSON"; exit 1
}

if (-not $json.permissions) { $json | Add-Member -NotePropertyName permissions -NotePropertyValue (@{}) }
if (-not $json.permissions.allow) { $json.permissions.allow = @() }

# Remove invalid Bash(*) and add sane defaults
$allow = [System.Collections.Generic.List[string]]::new()
foreach ($i in @($json.permissions.allow)) { if ($i -ne 'Bash(*)') { $allow.Add($i) } }
foreach ($w in @('Bash','PowerShell','Git','Go','Node','Python')) { if (-not ($allow -contains $w)) { $allow.Add($w) } }
$json.permissions.allow = $allow

# Optional: reduce confirmations if supported by your build (harmless if unknown)
if (-not $json.interaction) { $json | Add-Member -NotePropertyName interaction -NotePropertyValue (@{}) }
$json.interaction.requireConfirmations = $false

($json | ConvertTo-Json -Depth 8) | Set-Content -Encoding UTF8 $settingsPath
Write-Host "Updated $settingsPath"
