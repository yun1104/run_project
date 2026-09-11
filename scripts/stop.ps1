$ErrorActionPreference = "SilentlyContinue"

$projectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$pidFile = Join-Path $projectRoot ".runtime\gateway.pid"

if (Test-Path $pidFile) {
  $pid = Get-Content $pidFile
  if ($pid) {
    Stop-Process -Id ([int]$pid) -Force
    Write-Host "Stopped gateway process PID=$pid"
  }
  Remove-Item -Force $pidFile
} else {
  Write-Host "No running gateway process was found."
}

Get-Process -Name "api-gateway" -ErrorAction SilentlyContinue | ForEach-Object {
  Stop-Process -Id $_.Id -Force
  Write-Host "Stopped stale gateway process PID=$($_.Id)"
}
