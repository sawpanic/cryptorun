#!/usr/bin/env pwsh
command -v git-lfs >$null 2>&1
if ($LASTEXITCODE -ne 0) {
  Write-Host "`nThis repository is configured for Git LFS but 'git-lfs' was not found on your path.`n"
  exit 2
}
git lfs post-checkout @args