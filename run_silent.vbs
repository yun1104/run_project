Option Explicit

Dim fso, shell, baseDir, cmd, logPath
Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("Wscript.Shell")

baseDir = fso.GetParentFolderName(WScript.ScriptFullName)
logPath = baseDir & "\.runtime\run_silent.log"

cmd = "powershell -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File """ & baseDir & "\scripts\run.ps1"" " & _
      "-MySQLHost ""127.0.0.1"" -MySQLPort 3306 -MySQLUser ""root"" -MySQLPassword ""123456"" -MySQLDB ""meituan_db_0"" " & _
      "-RedisAddrs ""127.0.0.1:6379"" -OpenBrowser"

shell.Run cmd, 0, False
shell.Run "powershell -NoProfile -WindowStyle Hidden -Command ""Add-Content -Path '" & logPath & "' -Value (Get-Date).ToString('yyyy-MM-dd HH:mm:ss') + ' run_silent.vbs triggered'""", 0, False
