$ErrorActionPreference = "Continue"

$appRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$pidDir = Join-Path $appRoot ".runtime\pids"
$binDir = Join-Path $appRoot ".runtime\bin"

function Kill-Pid {
  param([int]$ProcessId)
  if ($ProcessId -le 0) { return }
  cmd /c "taskkill /PID $ProcessId /T /F >nul 2>nul" | Out-Null
}

if (Test-Path $pidDir) {
  $pidFiles = Get-ChildItem -Path $pidDir -Filter "*.pid" -ErrorAction SilentlyContinue
  foreach ($file in $pidFiles) {
    $raw = Get-Content $file.FullName -ErrorAction SilentlyContinue
    $val = 0
    if ([int]::TryParse(($raw | Select-Object -First 1), [ref]$val)) {
      Kill-Pid -ProcessId $val
    }
  }
  Remove-Item -Path (Join-Path $pidDir "*.pid") -Force -ErrorAction SilentlyContinue
}

$exeNames = @("gateway.exe", "app-orchestrator.exe", "user-service.exe", "recommend-service.exe")
foreach ($name in $exeNames) {
  cmd /c "taskkill /IM $name /T /F >nul 2>nul" | Out-Null
}

if (Test-Path $binDir) {
  $procs = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
    $_.ExecutablePath -and $_.ExecutablePath.StartsWith($binDir, [System.StringComparison]::OrdinalIgnoreCase)
  }
  foreach ($p in $procs) {
    Kill-Pid -ProcessId ([int]$p.ProcessId)
  }
}

$ports = @(8080, 50050, 50051, 50053)
foreach ($port in $ports) {
  $lines = netstat -ano -p tcp | Select-String ":$port"
  foreach ($line in $lines) {
    if ($line.Line -match "LISTENING\s+(\d+)$") {
      Kill-Pid -ProcessId ([int]$matches[1])
    }
  }
}

Write-Host "stopped"
exit 0
