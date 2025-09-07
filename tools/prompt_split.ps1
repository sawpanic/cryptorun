Param(
  [Parameter(Mandatory=$true)][string]$InFile,
  [int]$MaxChars = 180000
)
$txt = Get-Content $InFile -Raw
$len = $txt.Length
$chunks = [math]::Ceiling($len / $MaxChars)
$base = [System.IO.Path]::GetFileNameWithoutExtension($InFile)
$outDir = Join-Path (Split-Path $InFile -Parent) "$base.chunks"
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

for ($i = 0; $i -lt $chunks; $i++) {
  $start = $i * $MaxChars
  $size  = [math]::Min($MaxChars, $len - $start)
  $part  = $txt.Substring($start, $size)

  $header = @"
OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS
PROMPT_ID=$base.PART_$($i+1)_OF_$chunks

CONTINUATION RULES
- This is part $($i+1)/$chunks.
- If prior parts referenced WRITE-SCOPE/PATCH-ONLY, keep them identical here.
- Begin where the last part ended; do not repeat previous content.
"@

  $body = $header + "`r`n" + $part
  $outFile = Join-Path $outDir ("{0:D2}-{1}.md" -f ($i+1), $base)
  Set-Content -Path $outFile -Value $body -NoNewline
  Write-Host "Wrote $outFile ($size chars)"
}
Write-Host "Done. Files in: $outDir"
