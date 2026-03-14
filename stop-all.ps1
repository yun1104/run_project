$scriptPath = Join-Path $PSScriptRoot "app\scripts\stop-all.ps1"
if (!(Test-Path $scriptPath)) {
  throw "script not found: $scriptPath"
}

& powershell -ExecutionPolicy Bypass -File $scriptPath
