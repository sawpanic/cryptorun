Param()
$text = [Console]::In.ReadToEnd()
$needTask = ($text -notmatch "(?ms)^TASK:")
$needScope = ($text -notmatch "(?ms)^SCOPE:")
$needAccept = ($text -notmatch "(?ms)^ACCEPTANCE:")
$vague = $text -match "(?i)\b(do everything|fix code|refactor project|make it better|start over)\b"
if($needTask -or $needScope -or $needAccept -or $vague){
  @{ allow=$false; reason="Prompt missing required sections or too vague" } | ConvertTo-Json
  exit 2
}
'{"allow":true}' | Write-Output
exit 0
