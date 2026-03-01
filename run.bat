@echo off
set DOMAIN=%1
set OPEN=%2
if "%OPEN%"=="open" (
  set OPEN_FLAG=-OpenBrowser
) else (
  set OPEN_FLAG=
)
if "%DOMAIN%"=="" (
  powershell -ExecutionPolicy Bypass -File ".\scripts\run.ps1" %OPEN_FLAG%
) else (
  powershell -ExecutionPolicy Bypass -File ".\scripts\run.ps1" -Domain "%DOMAIN%" %OPEN_FLAG%
)
