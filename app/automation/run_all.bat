@echo off
setlocal

set APP_ROOT=%~dp0..
set REPO_ROOT=%APP_ROOT%\..
set GOEXE=%REPO_ROOT%\.tools\go\bin\go.exe
set MYSQLD=%APP_ROOT%\.tools\mysql\mariadb-11.4.2-winx64\bin\mariadbd.exe
set MYSQLADMIN=%APP_ROOT%\.tools\mysql\mariadb-11.4.2-winx64\bin\mysqladmin.exe
if not exist "%GOEXE%" set GOEXE=C:\Go\bin\go.exe
if not exist "%GOEXE%" (
  echo go.exe not found
  exit /b 1
)

pushd "%APP_ROOT%"

"%MYSQLADMIN%" -uroot -p123456 ping >nul 2>&1
if errorlevel 1 (
  start "" /B "%MYSQLD%" "--defaults-file=%APP_ROOT%\.runtime\mysql-data\my.ini"
  timeout /t 5 /nobreak >nul
)
"%MYSQLADMIN%" -uroot -p123456 ping >nul 2>&1
if errorlevel 1 (
  echo MySQL startup failed
  popd
  exit /b 1
)

"%GOEXE%" test ./automation/account -v -count=1
if errorlevel 1 exit /b 1

"%GOEXE%" test ./automation/preference -v -count=1
if errorlevel 1 exit /b 1

"%GOEXE%" test ./automation/recommend -v -count=1
if errorlevel 1 exit /b 1

if not exist "%APP_ROOT%\.runtime\bin" mkdir "%APP_ROOT%\.runtime\bin"
"%GOEXE%" build -o "%APP_ROOT%\.runtime\bin\user-service.exe" ./cmd/user-service
if errorlevel 1 exit /b 1
"%GOEXE%" build -o "%APP_ROOT%\.runtime\bin\recommend-service.exe" ./cmd/recommend-service
if errorlevel 1 exit /b 1
"%GOEXE%" build -o "%APP_ROOT%\.runtime\bin\app-orchestrator.exe" ./cmd/app-orchestrator
if errorlevel 1 exit /b 1
"%GOEXE%" build -o "%APP_ROOT%\.runtime\bin\gateway.exe" ./cmd/gateway
if errorlevel 1 exit /b 1
start "" /B "%APP_ROOT%\.runtime\bin\user-service.exe"
start "" /B "%APP_ROOT%\.runtime\bin\recommend-service.exe"
start "" /B "%APP_ROOT%\.runtime\bin\app-orchestrator.exe"
start "" /B "%APP_ROOT%\.runtime\bin\gateway.exe"
timeout /t 5 /nobreak >nul

"%GOEXE%" test ./automation/interface -v -count=1
if errorlevel 1 exit /b 1

python .\automation\ui\ui_test.py
set RESULT=%ERRORLEVEL%
popd
exit /b %RESULT%
