param(
  [string]$Domain = "",
  [int]$HttpPort = 8080,
  [int]$TcpPort = 9091,
  [int]$UdpPort = 9092,
  [switch]$OpenBrowser,
  [string]$MySQLHost = "127.0.0.1",
  [int]$MySQLPort = 3306,
  [string]$MySQLUser = "root",
  [string]$MySQLPassword = "123456",
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

  Write-Host "Go was not found. Downloading portable Go 1.22.6..."
  $goZip = Join-Path $toolsDir "go.zip"
  $goUrl = "https://go.dev/dl/go1.22.6.windows-amd64.zip"
  Invoke-WebRequest -Uri $goUrl -OutFile $goZip -TimeoutSec 120

  $goDir = Join-Path $toolsDir "go"
  if (Test-Path $goDir) {
    Remove-Item -Recurse -Force $goDir
  }
  Expand-Archive -Path $goZip -DestinationPath $toolsDir -Force
  Remove-Item -Force $goZip

  if (!(Test-Path $goExe)) {
    throw "Go install failed: $goExe does not exist. Please install Go 1.21+ manually and retry."
  }
}

function Stop-OldGateway {
  if (Test-Path $pidFile) {
    $oldPid = Get-Content $pidFile -ErrorAction SilentlyContinue
    if ($oldPid) {
      try {
        Stop-Process -Id ([int]$oldPid) -Force -ErrorAction Stop
        Write-Host "Stopped old gateway process PID=$oldPid"
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
  Write-Host "Running smoke tests..."
  $uname = "smoke_user_$([DateTimeOffset]::Now.ToUnixTimeMilliseconds())"
  $pwd = "123456"
  try {
    Invoke-RestMethod -Uri "$base/api/v1/user/register" -Method POST -ContentType "application/json" -Body (@{ username = $uname; password = $pwd } | ConvertTo-Json) | Out-Null
  } catch {
  }

  $loginResp = Invoke-RestMethod -Uri "$base/api/v1/user/login" -Method POST -ContentType "application/json" -Body (@{ username = $uname; password = $pwd } | ConvertTo-Json)
  if ($loginResp.code -ne 0 -or -not $loginResp.token) {
    throw "login smoke test failed"
  }

  $headers = @{ "Authorization" = "Bearer $($loginResp.token)"; "Content-Type" = "application/json" }
  $chatBody = @{ requirement = "budget 30, spicy food, quick delivery" } | ConvertTo-Json
  $chatResp = Invoke-RestMethod -Uri "$base/api/v1/chat/send" -Method POST -Headers $headers -Body $chatBody
  if ($chatResp.code -ne 0) {
    throw "chat/send smoke test failed"
  }
  if (-not $chatResp.data.merchants -or $chatResp.data.merchants.Count -eq 0) {
    throw "recommendation smoke test failed: empty merchants"
  }

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
    throw "TCP smoke test failed"
  }

  $udp = [System.Net.Sockets.UdpClient]::new()
  $udp.Client.ReceiveTimeout = 2000
  $bytes = [System.Text.Encoding]::UTF8.GetBytes("PING")
  [void]$udp.Send($bytes, $bytes.Length, "127.0.0.1", $UdpPort)
  $remote = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Any, 0)
  $recv = $udp.Receive([ref]$remote)
  $udp.Close()
  $udpResp = [System.Text.Encoding]::UTF8.GetString($recv)
  if ($udpResp -ne "PONG") {
    throw "UDP smoke test failed"
  }

  Write-Host "Smoke tests passed: HTTP/TCP/UDP are ready."
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
    throw "go mod tidy failed"
  }

  Write-Host "Starting gateway..."
  $proc = Start-Process -FilePath $goExe -ArgumentList "run", "./api-gateway" -WorkingDirectory $projectRoot -RedirectStandardOutput $logOut -RedirectStandardError $logErr -PassThru -WindowStyle Hidden
  Set-Content -Path $pidFile -Value $proc.Id

  $ready = Wait-HttpReady -Url "http://127.0.0.1:$HttpPort/"
  if (-not $ready) {
    throw "gateway startup timed out. Check log: $logErr"
  }

  Run-SmokeTests -HttpPort $HttpPort -TcpPort $TcpPort -UdpPort $UdpPort
  if ($OpenBrowser) {
    Start-Process "http://127.0.0.1:$HttpPort/"
  }

  Write-Host "System started: http://127.0.0.1:$HttpPort/"
  if (-not [string]::IsNullOrWhiteSpace($Domain)) {
    Write-Host "Allowed host enabled: $Domain"
    Write-Host "Point DNS or hosts to this machine, then visit: http://$Domain`:$HttpPort/"
  }
  Write-Host "TCP service: 0.0.0.0:$TcpPort"
  Write-Host "UDP service: 0.0.0.0:$UdpPort"
  Write-Host "Stop service: .\stop.bat"
} finally {
  Pop-Location
}
