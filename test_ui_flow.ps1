# UI端到端测试脚本
$timestamp = Get-Date -Format "yyyyMMddHHmmss"
$username = "e2e_ui_$timestamp"
$password = "123456"
$url = "http://127.0.0.1:8080"

Write-Host "=== UI端到端测试开始 ===" -ForegroundColor Green
Write-Host "测试用户名: $username" -ForegroundColor Cyan
Write-Host "目标URL: $url" -ForegroundColor Cyan
Write-Host ""

# 打开浏览器
Write-Host "步骤1: 打开浏览器访问 $url" -ForegroundColor Yellow
Start-Process "msedge.exe" $url

Write-Host ""
Write-Host "请在浏览器中手动执行以下步骤：" -ForegroundColor Green
Write-Host "1) 确认登录弹层是否出现（用户名/密码输入框、登录按钮、去注册按钮）"
Write-Host "2) 点击'去注册'，填写："
Write-Host "   - 用户名: $username"
Write-Host "   - 密码: $password"
Write-Host "   - 确认密码: $password"
Write-Host "3) 提交注册，确认注册成功提示"
Write-Host "4) 返回登录，使用上述账号登录"
Write-Host "5) 在聊天输入框输入：预算30元，想吃辣，软件园附近"
Write-Host "6) 点击发送，等待AI回复和推荐卡片"
Write-Host "7) 访问 /account 页面，确认已登录状态"
Write-Host ""
Write-Host "完成后请告知测试结果" -ForegroundColor Green
