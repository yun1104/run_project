# 自动化测试

```powershell
cd D:\waimai_agent\run_project\app
powershell -ExecutionPolicy Bypass -File .\automation\run_all.ps1
```

运行前确保 MySQL 已启动并监听 `127.0.0.1:3306`，且已安装 Python Selenium：

```powershell
python -m pip install selenium
```
