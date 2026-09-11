$ErrorActionPreference = "Stop"

$appRoot = Split-Path -Parent $PSScriptRoot
$repoRoot = Split-Path -Parent $appRoot
$portableGo = Join-Path $repoRoot ".tools\go\bin\go.exe"
$goExe = if (Test-Path $portableGo) { $portableGo } else { "C:\Go\bin\go.exe" }

if (!(Test-Path $goExe)) { throw "未找到 go.exe" }
if (!(Test-NetConnection 127.0.0.1 -Port 3306 -InformationLevel Quiet)) {
    throw "MySQL 未启动，请先启动 127.0.0.1:3306"
}

$env:GOPROXY = "https://goproxy.cn,direct"
$env:GOSUMDB = "sum.golang.google.cn"

Push-Location $appRoot
try {
    $hasSelenium = $true
    try {
        python -c "import selenium" 2>$null
        if ($LASTEXITCODE -ne 0) { $hasSelenium = $false }
    } catch {
        $hasSelenium = $false
    }
    if (!$hasSelenium) {
        python -m pip install selenium
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }

    & $goExe test ./automation/account -v -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    & $goExe test ./automation/preference -v -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    & $goExe test ./automation/recommend -v -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    & powershell -ExecutionPolicy Bypass -File .\scripts\start-all.ps1 -OpenBrowser 0

    & $goExe test ./automation/interface -v -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    python .\automation\ui\ui_test.py
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Pop-Location
}
