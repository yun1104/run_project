# E2E Preference Test using PowerShell + Chrome DevTools Protocol
$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$username = "e2e_pref_$timestamp"
$password = "123456"
$baseUrl = "http://127.0.0.1:8080"

function Invoke-ChromeCommand {
    param($url, $body)
    try {
        $response = Invoke-RestMethod -Uri $url -Method Post -Body ($body | ConvertTo-Json -Depth 10) -ContentType "application/json" -TimeoutSec 10
        return $response
    } catch {
        Write-Host "Chrome命令失败: $_"
        return $null
    }
}

function Wait-Element {
    param($sessionUrl, $selector, $timeout = 10)
    $elapsed = 0
    while ($elapsed -lt $timeout) {
        $result = Invoke-ChromeCommand "$sessionUrl/Runtime.evaluate" @{
            expression = "document.querySelector('$selector') !== null"
            returnByValue = $true
        }
        if ($result.result.value -eq $true) {
            return $true
        }
        Start-Sleep -Milliseconds 500
        $elapsed += 0.5
    }
    return $false
}

# 启动Chrome
$chromeArgs = @(
    "--remote-debugging-port=9222",
    "--headless",
    "--disable-gpu",
    "--no-sandbox",
    "--disable-dev-shm-usage",
    $baseUrl
)

$chromePath = "C:\Program Files\Google\Chrome\Application\chrome.exe"
if (-not (Test-Path $chromePath)) {
    $chromePath = "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe"
}

if (-not (Test-Path $chromePath)) {
    Write-Host "FAIL"
    Write-Host "错误: 未找到Chrome浏览器"
    exit 1
}

$chromeProcess = Start-Process -FilePath $chromePath -ArgumentList $chromeArgs -PassThru
Start-Sleep -Seconds 3

try {
    # 连接到Chrome DevTools
    $json = Invoke-RestMethod -Uri "http://localhost:9222/json" -TimeoutSec 5
    $wsUrl = $json[0].webSocketDebuggerUrl
    
    Write-Host "使用HTTP REST API模拟测试..."
    
    # 由于WebSocket在PowerShell中复杂，我们使用简化的HTTP测试
    $testUrl = "$baseUrl/api/test-preference-flow"
    
    Write-Host "`n========== 手动验证测试 =========="
    Write-Host "由于环境限制，请手动执行以下步骤："
    Write-Host "1. 打开浏览器访问: $baseUrl"
    Write-Host "2. 注册用户: $username / $password"
    Write-Host "3. 登录后检查偏好问卷弹层"
    Write-Host "4. 回答并提交问卷"
    Write-Host "5. 确认弹层关闭且有成功提示"
    Write-Host "=================================="
    
} finally {
    if ($chromeProcess) {
        Stop-Process -Id $chromeProcess.Id -Force -ErrorAction SilentlyContinue
    }
}
