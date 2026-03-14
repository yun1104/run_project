$timestamp = Get-Date -Format "yyyyMMddHHmmss"
$username = "e2e_pref_once_$timestamp"
$password = "123456"

Write-Host "============================================"
Write-Host "UI自测: $username"
Write-Host "============================================"

$chrome = "C:\Program Files\Google\Chrome\Application\chrome.exe"
if (-not (Test-Path $chrome)) {
    $chrome = "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe"
}

if (Test-Path $chrome) {
    $url = "http://127.0.0.1:8080"
    Start-Process $chrome -ArgumentList "--new-window", $url
    Write-Host "[STEP 1] 已打开Chrome: $url"
    Write-Host "[STEP 2] 用户名: $username"
    Write-Host "[STEP 2] 密码: $password"
    Write-Host ""
    Write-Host "请在浏览器中手动执行以下步骤:"
    Write-Host "1) 点击'注册'按钮"
    Write-Host "2) 输入用户名: $username"
    Write-Host "3) 输入密码: $password"
    Write-Host "4) 提交注册并登录"
    Write-Host "5) 若出现定位授权弹层，点击'不允许'"
    Write-Host "6) 观察偏好问卷弹层是否出现"
    Write-Host "7) 选择偏好并提交"
    Write-Host "8) 访问 http://127.0.0.1:8080/assets/location.html"
    Write-Host "9) 点击'返回聊天'或刷新首页"
    Write-Host "10) 观察偏好问卷是否再次弹出"
} else {
    Write-Host "[ERROR] Chrome未找到"
}
