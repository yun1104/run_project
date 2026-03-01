param(
  [string]$Domain = "",
  [int]$HttpPort = 8080,
  [int]$TcpPort = 9091,
  [int]$UdpPort = 9092,
  [switch]$OpenBrowser,
  [string]$MySQLHost = "127.0.0.1",
  [int]$MySQLPort = 3306,
  [string]$MySQLUser = "root",
  [string]$MySQLPassword = "root",
  [string]$MySQLDB = "meituan_db_0",
  [string]$RedisAddrs = "127.0.0.1:6379",
  [string]$RedisPassword = ""
)

$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$runtimeDir = Join-Path $projectRoot ".runtime"
$toolsDir = Join-Path $projectRoot ".tools"
$goExe = Join-Path $toolsDir "go\bin\go.exe"
$logOut = Join-Path $runtimeDir "gateway.out.log"
$logErr = Join-Path $runtimeDir "gateway.err.log"
$pidFile = Join-Path $runtimeDir "gateway.pid"

New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
New-Item -ItemType Directory -Force -Path $toolsDir | Out-Null

function Ensure-Go {
  $systemGo = "C:\Go\bin\go.exe"
  if (Test-Path $systemGo) {
    $script:goExe = $systemGo
    return
  }

  if (Test-Path $goExe) {
    return
  }

  Write-Host "未检测到 Go，开始下载便携版..."
  $goZip = Join-Path $toolsDir "go.zip"
  $goUrl = "https://go.dev/dl/go1.22.6.windows-amd64.zip"
  Invoke-WebRequest -Uri $goUrl -OutFile $goZip -TimeoutSec 90

  if (Test-Path (Join-Path $toolsDir "go")) {
    Remove-Item -Recurse -Force (Join-Path $toolsDir "go")
  }
  Expand-Archive -Path $goZip -DestinationPath $toolsDir -Force
  Remove-Item -Force $goZip

  if (!(Test-Path $goExe)) {
    throw "Go 安装失败: $goExe 不存在。请手动安装 Go 1.22+ 后重试。"
  }
}

function Stop-OldGateway {
  if (Test-Path $pidFile) {
    $oldPid = Get-Content $pidFile -ErrorAction SilentlyContinue
    if ($oldPid) {
      try {
        Stop-Process -Id ([int]$oldPid) -Force -ErrorAction Stop
        Write-Host "已停止旧网关进程 PID=$oldPid"
      } catch {
      }
    }
    Remove-Item -Force $pidFile -ErrorAction SilentlyContinue
  }
}

function Wait-HttpReady {
  param(
    [string]$Url,
    [int]$MaxSeconds = 60
  )

  $start = Get-Date
  while (((Get-Date) - $start).TotalSeconds -lt $MaxSeconds) {
    try {
      $resp = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 3
      if ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500) {
        return $true
      }
    } catch {
    }
    Start-Sleep -Milliseconds 800
  }
  return $false
}

function Run-SmokeTests {
  param(
    [int]$HttpPort,
    [int]$TcpPort,
    [int]$UdpPort
  )

  $base = "http://127.0.0.1:$HttpPort"
  Write-Host "执行接口自测..."
  $uname = "smoke_user"
  $pwd = "123456"
  try {
    Invoke-RestMethod -Uri "$base/api/v1/user/register" -Method POST -ContentType "application/json" -Body (@{ username = $uname; password = $pwd } | ConvertTo-Json) | Out-Null
  } catch {
  }
  $loginResp = Invoke-RestMethod -Uri "$base/api/v1/user/login" -Method POST -ContentType "application/json" -Body (@{ username = $uname; password = $pwd } | ConvertTo-Json)
  if ($loginResp.code -ne 0 -or -not $loginResp.token) {
    throw "login 自测失败"
  }
  $headers = @{ "Authorization" = "Bearer $($loginResp.token)"; "Content-Type" = "application/json" }

  $chatBody = @{ requirement = "预算30，想吃辣，30分钟内送达" } | ConvertTo-Json
  $chatResp = Invoke-RestMethod -Uri "$base/api/v1/chat/send" -Method POST -Headers $headers -Body $chatBody
  if ($chatResp.code -ne 0) {
    throw "chat/send 自测失败"
  }

  $merchant = $chatResp.data.merchants[0]
  if (-not $merchant) {
    throw "推荐为空，自测失败"
  }

  $orderBody = @{
    merchant_id = [int64]$merchant.id
    merchant_name = [string]$merchant.name
    amount = [double]$merchant.avg_price
  } | ConvertTo-Json

  $orderResp = Invoke-RestMethod -Uri "$base/api/v1/order/auto-place-pay" -Method POST -Headers $headers -Body $orderBody
  if ($orderResp.code -ne 0) {
    throw "order/auto-place-pay 自测失败"
  }

  $orderId = $orderResp.data.order_id
  $detailResp = Invoke-RestMethod -Uri ("$base/api/v1/order/detail?order_id=" + $orderId) -Method GET -Headers @{ "Authorization" = "Bearer $($loginResp.token)" }
  if ($detailResp.code -ne 0 -or -not $detailResp.data.paid) {
    throw "order/detail 自测失败"
  }

  # TCP 自测
  $tcp = [System.Net.Sockets.TcpClient]::new()
  $tcp.Connect("127.0.0.1", $TcpPort)
  $stream = $tcp.GetStream()
  $writer = New-Object System.IO.StreamWriter($stream)
  $reader = New-Object System.IO.StreamReader($stream)
  $writer.AutoFlush = $true
  $writer.WriteLine("PING")
  $tcpResp = $reader.ReadLine()
  $reader.Dispose()
  $writer.Dispose()
  $stream.Dispose()
  $tcp.Close()
  if ($tcpResp -ne "PONG") {
    throw "TCP 自测失败"
  }

  # UDP 自测
  $udp = [System.Net.Sockets.UdpClient]::new()
  $udp.Client.ReceiveTimeout = 2000
  $bytes = [System.Text.Encoding]::UTF8.GetBytes("PING")
  [void]$udp.Send($bytes, $bytes.Length, "127.0.0.1", $UdpPort)
  $remote = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Any, 0)
  $recv = $udp.Receive([ref]$remote)
  $udp.Close()
  $udpResp = [System.Text.Encoding]::UTF8.GetString($recv)
  if ($udpResp -ne "PONG") {
    throw "UDP 自测失败"
  }

  Write-Host "自测通过：HTTP/TCP/UDP 链路正常"
}

Ensure-Go
Stop-OldGateway

$env:Path = (Join-Path $toolsDir "go\bin") + ";" + $env:Path
$env:GOPROXY = "https://goproxy.cn,direct"
$env:GOSUMDB = "sum.golang.google.cn"

Push-Location $projectRoot
try {
  $env:HTTP_ADDR = "0.0.0.0:$HttpPort"
  $env:TCP_ADDR = "0.0.0.0:$TcpPort"
  $env:UDP_ADDR = "0.0.0.0:$UdpPort"
  $env:MYSQL_HOST = $MySQLHost
  $env:MYSQL_PORT = "$MySQLPort"
  $env:MYSQL_USER = $MySQLUser
  $env:MYSQL_PASSWORD = $MySQLPassword
  $env:MYSQL_DB = $MySQLDB
  $env:REDIS_ADDRS = $RedisAddrs
  $env:REDIS_PASSWORD = $RedisPassword
  if ([string]::IsNullOrWhiteSpace($Domain)) {
    Remove-Item Env:ALLOWED_HOST -ErrorAction SilentlyContinue
  } else {
    $env:ALLOWED_HOST = $Domain
  }

  & $goExe mod tidy
  if ($LASTEXITCODE -ne 0) {
    throw "go mod tidy 失败"
  }

  Write-Host "启动网关..."
  $proc = Start-Process -FilePath $goExe -ArgumentList "run", "./api-gateway" -WorkingDirectory $projectRoot -RedirectStandardOutput $logOut -RedirectStandardError $logErr -PassThru
  Set-Content -Path $pidFile -Value $proc.Id

  $ready = Wait-HttpReady -Url "http://127.0.0.1:$HttpPort/"
  if (-not $ready) {
    throw "网关启动超时，请查看日志: $logErr"
  }

  Run-SmokeTests -HttpPort $HttpPort -TcpPort $TcpPort -UdpPort $UdpPort
  if ($OpenBrowser) {
    Start-Process "http://127.0.0.1:$HttpPort/"
  }
  Write-Host "系统已启动：http://127.0.0.1:$HttpPort/"
  if (-not [string]::IsNullOrWhiteSpace($Domain)) {
    Write-Host "域名限制已启用：$Domain"
    Write-Host "请将域名 DNS 或 hosts 指向本机IP后访问：http://$Domain`:$HttpPort/"
  }
  Write-Host "TCP 服务：0.0.0.0:$TcpPort"
  Write-Host "UDP 服务：0.0.0.0:$UdpPort"
  Write-Host "停止服务：powershell -ExecutionPolicy Bypass -File .\scripts\stop.ps1"
} finally {
  Pop-Location
}
