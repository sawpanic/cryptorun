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
