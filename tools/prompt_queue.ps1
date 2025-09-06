Param(
  [Parameter(Mandatory=$true)][string]$PromptDir,
  [string]$Cli = "npx @anthropic-ai/claude-code@latest",  # adjust if you use a different entrypoint
  [string]$Log = ".\prompt_queue.log",
  [switch]$DryRun
)

if (-not (Test-Path $PromptDir)) { throw "PromptDir not found: $PromptDir" }
$files = Get-ChildItem $PromptDir -File -Filter *.md | Sort-Object Name
if ($files.Count -eq 0) { throw "No .md files in $PromptDir" }

"===== $(Get-Date -Format s) — QUEUE START ($($files.Count) files) =====" | Tee-Object -FilePath $Log -Append | Out-Null

foreach ($f in $files) {
  $name = $f.Name
  $msg = "[RUN] $name"
  $msg | Tee-Object -FilePath $Log -Append | Write-Host

  if ($DryRun) { continue }

  $cmd = "$Cli run --file `"$($f.FullName)`""
  $sw = [System.Diagnostics.Stopwatch]::StartNew()
  $p = Start-Process -FilePath "pwsh" -ArgumentList "-NoProfile","-Command",$cmd -Wait -PassThru -WindowStyle Hidden
  $sw.Stop()

  $exit = $p.ExitCode
  $line = "[DONE] $name exit=$exit time=${($sw.Elapsed.ToString())}"
  $line | Tee-Object -FilePath $Log -Append | Write-Host

  if ($exit -ne 0) {
    "[HALT] Non-zero exit, stopping queue." | Tee-Object -FilePath $Log -Append | Write-Host
    break
  }
}
"===== $(Get-Date -Format s) — QUEUE END =====" | Tee-Object -FilePath $Log -Append | Out-Null
