Param(
  [string]$Url
)

if (-not $Url) {
  $stdin = [Console]::In.ReadToEnd().Trim()
  if ($stdin) { $Url = $stdin }
}
if (-not $Url) { Write-Output '{"allow":false,"reason":"no URL provided"}'; exit 2 }

$allow = @(
  'https://api.coingecko.com',
  'https://api.kraken.com',
  'https://api.exchange.coinbase.com',
  'https://api.pro.coinbase.com',
  'https://www.okx.com'
)

$uri = [Uri]$Url
$origin = $uri.Scheme + '://' + $uri.Host

$ok = $false
foreach ($a in $allow) { if ($origin -like "$a*") { $ok = $true; break } }

if ($ok) {
  Write-Output ('{"allow":true,"url":"' + $Url + '"}')
  exit 0
} else {
  Write-Output ('{"allow":false,"reason":"domain not whitelisted","url":"' + $Url + '"}')
  exit 2
}
