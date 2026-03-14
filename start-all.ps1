param(
  [int]$GatewayPort = 8080,
  [string]$UserGrpcAddr = "127.0.0.1:50051",
  [string]$RecommendGrpcAddr = "127.0.0.1:50053",
  [string]$AppGrpcAddr = "127.0.0.1:50050",
  [string]$OpenBrowser = "1"
)

$scriptPath = Join-Path $PSScriptRoot "app\scripts\start-all.ps1"
if (!(Test-Path $scriptPath)) {
  throw "script not found: $scriptPath"
}

$openValue = 0
$openFlag = $OpenBrowser.ToLower()
if ($openFlag -ne "0" -and $openFlag -ne "false" -and $openFlag -ne "no") { $openValue = 1 }

powershell -ExecutionPolicy Bypass -File $scriptPath `
  -GatewayPort $GatewayPort `
  -UserGrpcAddr $UserGrpcAddr `
  -RecommendGrpcAddr $RecommendGrpcAddr `
  -AppGrpcAddr $AppGrpcAddr `
  -OpenBrowser $openValue
