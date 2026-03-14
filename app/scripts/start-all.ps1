param(
  [int]$GatewayPort = 8080,
  [string]$UserGrpcAddr = "127.0.0.1:50051",
  [string]$RecommendGrpcAddr = "127.0.0.1:50053",
  [string]$AppGrpcAddr = "127.0.0.1:50050",
  [string]$OpenBrowser = "1"
)

$ErrorActionPreference = "Stop"

$appRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$workspaceRoot = Split-Path -Parent $appRoot
$runtimeDir = Join-Path $appRoot ".runtime"
$pidDir = Join-Path $runtimeDir "pids"
$logDir = Join-Path $runtimeDir "logs"
$binDir = Join-Path $runtimeDir "bin"
$stopScript = Join-Path $appRoot "scripts\stop-all.ps1"

New-Item -ItemType Directory -Force -Path $pidDir | Out-Null
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

function Resolve-GoExe {
  $systemGo = "C:\Go\bin\go.exe"
  $portableGo = Join-Path $workspaceRoot ".tools\go\bin\go.exe"
  if (Test-Path $systemGo) { return $systemGo }
  if (Test-Path $portableGo) { return $portableGo }
  $cmd = Get-Command go -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd.Source }
  throw "go.exe not found"
}

function Test-TcpPort {
  param([string]$TargetHost, [int]$Port)
  try {
    $tcp = New-Object System.Net.Sockets.TcpClient
    $ok = $tcp.ConnectAsync($TargetHost, $Port).Wait(1200)
    $connected = $ok -and $tcp.Connected
    $tcp.Close()
    return $connected
  } catch {
    return $false
  }
}

function Parse-Port {
  param([string]$Addr)
  $parts = $Addr.Split(":")
  return [int]$parts[$parts.Length - 1]
}

function Wait-Port {
  param([string]$TargetHost, [int]$Port, [int]$TimeoutSec = 40)
  $start = Get-Date
  while (((Get-Date) - $start).TotalSeconds -lt $TimeoutSec) {
    if (Test-TcpPort -TargetHost $TargetHost -Port $Port) { return $true }
    Start-Sleep -Milliseconds 500
  }
  return $false
}

if (Test-Path $stopScript) {
  & powershell -ExecutionPolicy Bypass -File $stopScript | Out-Null
}

$goExe = Resolve-GoExe
$env:GOPROXY = "https://goproxy.cn,direct"
$env:GOSUMDB = "sum.golang.google.cn"
$env:USER_GRPC_ADDR = $UserGrpcAddr
$env:RECOMMEND_GRPC_ADDR = $RecommendGrpcAddr
$env:APP_GRPC_ADDR = $AppGrpcAddr
$env:GATEWAY_HTTP_ADDR = "0.0.0.0:$GatewayPort"

$services = @(
  @{ Name = "user-service"; Cmd = "./cmd/user-service"; Pid = "user-service.pid"; Bin = "user-service.exe" },
  @{ Name = "recommend-service"; Cmd = "./cmd/recommend-service"; Pid = "recommend-service.pid"; Bin = "recommend-service.exe" },
  @{ Name = "app-orchestrator"; Cmd = "./cmd/app-orchestrator"; Pid = "app-orchestrator.pid"; Bin = "app-orchestrator.exe" },
  @{ Name = "gateway"; Cmd = "./cmd/gateway"; Pid = "gateway.pid"; Bin = "gateway.exe" }
)

Push-Location $appRoot
try {
  foreach ($svc in $services) {
    $binPath = Join-Path $binDir $svc.Bin
    & $goExe build -o $binPath $svc.Cmd
    if ($LASTEXITCODE -ne 0) {
      throw "build failed: $($svc.Name)"
    }
    $outLog = Join-Path $logDir "$($svc.Name).out.log"
    $errLog = Join-Path $logDir "$($svc.Name).err.log"
    $proc = Start-Process -FilePath $binPath -WorkingDirectory $appRoot -RedirectStandardOutput $outLog -RedirectStandardError $errLog -WindowStyle Hidden -PassThru
    Set-Content -Path (Join-Path $pidDir $svc.Pid) -Value $proc.Id
  }
} finally {
  Pop-Location
}

$userReady = Wait-Port -TargetHost "127.0.0.1" -Port (Parse-Port $UserGrpcAddr)
$recReady = Wait-Port -TargetHost "127.0.0.1" -Port (Parse-Port $RecommendGrpcAddr)
$appReady = Wait-Port -TargetHost "127.0.0.1" -Port (Parse-Port $AppGrpcAddr)
$gwReady = Wait-Port -TargetHost "127.0.0.1" -Port $GatewayPort

if (-not ($userReady -and $recReady -and $appReady -and $gwReady)) {
  throw "startup failed, check logs under $logDir"
}

Write-Host "started: http://127.0.0.1:$GatewayPort"
Write-Host "logs: $logDir"
$openFlag = $OpenBrowser.ToLower()
if ($openFlag -ne "0" -and $openFlag -ne "false" -and $openFlag -ne "no") {
  Start-Process "http://127.0.0.1:$GatewayPort/"
}
