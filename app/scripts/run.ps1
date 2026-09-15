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
  [string]$RedisPassword = "",
  [string]$ModelScopeToken = "",
  [string]$AmapApiKey = "8bd762d842f9fc2b808a9d75bd243b56",
  [string]$ModelScopeBaseUrl = "https://api-inference.modelscope.cn/v1",
  [string]$ModelScopeModel = "Qwen/Qwen3-30B-A3B-Instruct-2507"
)

$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$runtimeDir = Join-Path $projectRoot ".runtime"
$toolsDir = Join-Path $projectRoot ".tools"
$goExe = Join-Path $toolsDir "go\bin\go.exe"
$logOut = Join-Path $runtimeDir "gateway.out.log"
$logErr = Join-Path $runtimeDir "gateway.err.log"
$pidFile = Join-Path $runtimeDir "gateway.pid"
$mysqlPidFile = Join-Path $runtimeDir "mysql.pid"
$stopScript = Join-Path $projectRoot "scripts\stop.ps1"
$mysqlExe = Join-Path $projectRoot ".tools\mysql\mariadb-11.4.2-winx64\bin\mariadbd.exe"
$mysqlDataDir = Join-Path $runtimeDir "mysql-data"
$mysqlIni = Join-Path $mysqlDataDir "my.ini"

New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
New-Item -ItemType Directory -Force -Path $toolsDir | Out-Null

function Test-TcpPort {
  param([string]$TargetHost = "127.0.0.1", [int]$Port)
  try {
    $tcp = New-Object System.Net.Sockets.TcpClient
    $tcp.ConnectAsync($TargetHost, $Port).Wait(1500) | Out-Null
    $ok = $tcp.Connected
    $tcp.Close()
    return $ok
  } catch { return $false }
}

function Ensure-MySQL {
  if (Test-TcpPort -TargetHost "127.0.0.1" -Port 3306) {
    Write-Host "MySQL already running on 3306"
    return
  }
  if (-not (Test-Path $mysqlExe)) {
    Write-Host "MariaDB not found, skipping MySQL start."
    return
  }
  New-Item -ItemType Directory -Force -Path $mysqlDataDir | Out-Null
  $pluginDir = Join-Path $projectRoot ".tools\mysql\mariadb-11.4.2-winx64\lib\plugin"
  $iniContent = @"
[mysqld]
datadir=$($mysqlDataDir -replace '\\','/')
port=3306
[client]
port=3306
plugin-dir=$($pluginDir -replace '\\','/')
"@
  Set-Content -Path $mysqlIni -Value $iniContent
  Write-Host "Starting MySQL..."
  $mysqlProc = Start-Process -FilePath $mysqlExe -ArgumentList "--defaults-file=$mysqlIni" -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru
  Set-Content -Path $mysqlPidFile -Value $mysqlProc.Id
  $waitStart = Get-Date
  while (((Get-Date) - $waitStart).TotalSeconds -lt 30) {
    Start-Sleep -Milliseconds 500
    if (Test-TcpPort -TargetHost "127.0.0.1" -Port 3306) {
      Write-Host "MySQL started"
      return
    }
  }
  Write-Host "Warning: MySQL may not have started within 30s."
}

function Ensure-Database {
  $mysqlCli = Join-Path $projectRoot ".tools\mysql\mariadb-11.4.2-winx64\bin\mysql.exe"
  $initSql = Join-Path $projectRoot "scripts\init_db.sql"
  $migrateSql = Join-Path $projectRoot "scripts\migrate_users_for_gorm.sql"
  if (-not (Test-Path $mysqlCli) -or -not (Test-Path $initSql)) { return }
  try {
    $mysqlArgs = @("-h", $MySQLHost, "-P", "$MySQLPort", "-u", $MySQLUser)
    if (-not [string]::IsNullOrEmpty($MySQLPassword)) { $mysqlArgs += "-p$MySQLPassword" }
    Get-Content $initSql -Raw | & $mysqlCli @mysqlArgs 2>$null
    if ($LASTEXITCODE -eq 0) { Write-Host "Database initialized" }
    if (Test-Path $migrateSql) {
      $migrateArgs = $mysqlArgs + "--force"
      Get-Content $migrateSql -Raw | & $mysqlCli @migrateArgs 2>$null
    }
  } catch { }
}

function Ensure-Go {
  $systemGo = "C:\Go\bin\go.exe"
  if (Test-Path $systemGo) {
    $script:goExe = $systemGo
    return
  }

  if (Test-Path $goExe) {
    return
  }

  Write-Host "Go not found, downloading portable runtime..."
  $goZip = Join-Path $toolsDir "go.zip"
  $goUrl = "https://go.dev/dl/go1.22.6.windows-amd64.zip"
  Invoke-WebRequest -Uri $goUrl -OutFile $goZip -TimeoutSec 90

  if (Test-Path (Join-Path $toolsDir "go")) {
    Remove-Item -Recurse -Force (Join-Path $toolsDir "go")
  }
  Expand-Archive -Path $goZip -DestinationPath $toolsDir -Force
  Remove-Item -Force $goZip

  if (!(Test-Path $goExe)) {
    throw "Go install failed: $goExe not found. Please install Go 1.22+ and retry."
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

function Stop-PortOwners {
  param(
    [int[]]$Ports
  )
  $pidSet = @{}
  foreach ($port in $Ports) {
    try {
      $tcpLines = cmd /c "netstat -ano -p tcp | findstr :$port"
      foreach ($line in $tcpLines) {
        if ($line -match "LISTENING\s+(\d+)$") {
          $pidSet[$matches[1]] = $true
        }
      }
    } catch {
    }
    try {
      $udpLines = cmd /c "netstat -ano -p udp | findstr :$port"
      foreach ($line in $udpLines) {
        if ($line -match "\s(\d+)$") {
          $pidSet[$matches[1]] = $true
        }
      }
    } catch {
    }
  }
  foreach ($procId in $pidSet.Keys) {
    if ($procId -and $procId -ne "0") {
      try {
        Stop-Process -Id ([int]$procId) -Force -ErrorAction Stop
        Write-Host "Killed process occupying target port PID=$procId"
      } catch {
      }
    }
  }
}

function Stop-BeforeRun {
  if (Test-Path $stopScript) {
    try {
      & powershell -ExecutionPolicy Bypass -File $stopScript | Out-Null
    } catch {
    }
  }
  Stop-OldGateway
  Stop-PortOwners -Ports @($HttpPort, $TcpPort, $UdpPort)
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
  Write-Host "Running API smoke tests..."

  # HTTP 可达性已在 Wait-HttpReady 中验证，这里仅测试 TCP/UDP
  try {
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
      Write-Host "Smoke test warning: TCP PONG not received"
    }
  } catch {
    Write-Host "Smoke test warning: TCP check skipped ($_)"
  }

  try {
    $udp = [System.Net.Sockets.UdpClient]::new()
    $udp.Client.ReceiveTimeout = 2000
    $bytes = [System.Text.Encoding]::UTF8.GetBytes("PING")
    [void]$udp.Send($bytes, $bytes.Length, "127.0.0.1", $UdpPort)
    $remote = New-Object System.Net.IPEndPoint([System.Net.IPAddress]::Any, 0)
    $recv = $udp.Receive([ref]$remote)
    $udp.Close()
    $udpResp = [System.Text.Encoding]::UTF8.GetString($recv)
    if ($udpResp -ne "PONG") {
      Write-Host "Smoke test warning: UDP PONG not received"
    }
  } catch {
    Write-Host "Smoke test warning: UDP check skipped ($_)"
  }

  Write-Host "Smoke tests done."
}

Ensure-Go
Stop-BeforeRun
Ensure-MySQL
Ensure-Database

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
  if ([string]::IsNullOrWhiteSpace($ModelScopeToken)) {
    Remove-Item Env:MODELSCOPE_API_KEY -ErrorAction SilentlyContinue
  } else {
    $env:MODELSCOPE_API_KEY = $ModelScopeToken
  }
  if ([string]::IsNullOrWhiteSpace($AmapApiKey)) {
    Remove-Item Env:AMAP_API_KEY -ErrorAction SilentlyContinue
  } else {
    $env:AMAP_API_KEY = $AmapApiKey
  }
  $env:MODELSCOPE_BASE_URL = $ModelScopeBaseUrl
  $env:MODELSCOPE_MODEL = $ModelScopeModel
  if ([string]::IsNullOrWhiteSpace($Domain)) {
    Remove-Item Env:ALLOWED_HOST -ErrorAction SilentlyContinue
  } else {
    $env:ALLOWED_HOST = $Domain
  }

  & $goExe mod tidy
  if ($LASTEXITCODE -ne 0) {
    throw "go mod tidy failed"
  }

  Write-Host "Starting API gateway..."
  $proc = Start-Process -FilePath $goExe -ArgumentList "run", "./cmd/gateway" -WorkingDirectory $projectRoot -RedirectStandardOutput $logOut -RedirectStandardError $logErr -WindowStyle Hidden -PassThru
  Set-Content -Path $pidFile -Value $proc.Id

  $ready = Wait-HttpReady -Url "http://127.0.0.1:$HttpPort/"
  if (-not $ready) {
    throw "Gateway start timeout, check log: $logErr"
  }

  Run-SmokeTests -HttpPort $HttpPort -TcpPort $TcpPort -UdpPort $UdpPort
  if ($OpenBrowser) {
    Start-Process -FilePath "explorer.exe" -ArgumentList "http://127.0.0.1:$HttpPort/"
  }
  Write-Host "System started: http://127.0.0.1:$HttpPort/"
  if (-not [string]::IsNullOrWhiteSpace($Domain)) {
    Write-Host "Domain restriction enabled: $Domain"
    Write-Host "Point DNS/hosts of the domain to this machine, then visit: http://$Domain`:$HttpPort/"
  }
  Write-Host "TCP service: 0.0.0.0:$TcpPort"
  Write-Host "UDP service: 0.0.0.0:$UdpPort"
  Write-Host "Stop command: powershell -ExecutionPolicy Bypass -File .\scripts\stop.ps1"
} finally {
  Pop-Location
}
