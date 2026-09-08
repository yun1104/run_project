param(
  [switch]$Race,
  [switch]$Verbose
)

$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$toolsDir = Join-Path $projectRoot ".tools"
$goExe = Join-Path $toolsDir "go\bin\go.exe"

function Resolve-Go {
  $systemGo = "C:\Go\bin\go.exe"
  if (Test-Path $systemGo) {
    return $systemGo
  }
  if (Test-Path $goExe) {
    return $goExe
  }

  New-Item -ItemType Directory -Force -Path $toolsDir | Out-Null
  Write-Host "未检测到可用 Go，开始下载 Go 1.22.6 便携版..."
  $goZip = Join-Path $toolsDir "go.zip"
  $goUrl = "https://go.dev/dl/go1.22.6.windows-amd64.zip"
  Invoke-WebRequest -Uri $goUrl -OutFile $goZip -TimeoutSec 120

  $target = Join-Path $toolsDir "go"
  if (Test-Path $target) {
    Remove-Item -Recurse -Force $target
  }
  Expand-Archive -Path $goZip -DestinationPath $toolsDir -Force
  Remove-Item -Force $goZip

  if (!(Test-Path $goExe)) {
    throw "Go 安装失败：$goExe 不存在。请手动安装 Go 1.21+ 后重试。"
  }
  return $goExe
}

$resolvedGo = Resolve-Go
$env:Path = (Split-Path -Parent $resolvedGo) + ";" + $env:Path
$env:GOPROXY = "https://goproxy.cn,direct"
$env:GOSUMDB = "sum.golang.google.cn"
$env:GIN_MODE = "test"

$args = @("test", "./...", "-count=1")
if ($Verbose) {
  $args += "-v"
}
if ($Race) {
  $args += "-race"
}

Push-Location $projectRoot
try {
  Write-Host "使用 Go：$resolvedGo"
  Write-Host "执行：go $($args -join ' ')"
  & $resolvedGo @args
  if ($LASTEXITCODE -ne 0) {
    throw "单元测试失败，退出码：$LASTEXITCODE"
  }
  Write-Host "全部单元测试通过。"
} finally {
  Pop-Location
}
