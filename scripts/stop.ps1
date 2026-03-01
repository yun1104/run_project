$ErrorActionPreference = "SilentlyContinue"

$projectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$pidFile = Join-Path $projectRoot ".runtime\gateway.pid"

if (Test-Path $pidFile) {
  $pid = Get-Content $pidFile
  if ($pid) {
    Stop-Process -Id ([int]$pid) -Force
    Write-Host "已停止网关进程 PID=$pid"
  }
  Remove-Item -Force $pidFile
} else {
  Write-Host "未找到运行中的网关进程"
}
