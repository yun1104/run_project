Option Explicit

Dim fso, shell, baseDir, cmd, logPath, runtimeDir
Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("Wscript.Shell")

baseDir = fso.GetParentFolderName(WScript.ScriptFullName)
runtimeDir = baseDir & "\.runtime"
If Not fso.FolderExists(runtimeDir) Then
  fso.CreateFolder runtimeDir
End If
logPath = baseDir & "\.runtime\run_silent.log"

Sub AppendLog(msg)
  On Error Resume Next
  Dim ts
  Set ts = fso.OpenTextFile(logPath, 8, True)
  ts.WriteLine Now & " " & msg
  ts.Close
  On Error Goto 0
End Sub

Function IsHttpReady(url)
  On Error Resume Next
  Dim http
  Set http = CreateObject("MSXML2.ServerXMLHTTP")
  http.setTimeouts 1000, 1000, 1000, 1000
  http.open "GET", url, False
  http.send
  IsHttpReady = (Err.Number = 0 And http.status >= 200 And http.status < 500)
  Set http = Nothing
  Err.Clear
  On Error Goto 0
End Function

Function WaitHttpReady(url, timeoutMs)
  Dim startAt
  startAt = Timer
  Do While ((Timer - startAt) * 1000) < timeoutMs
    If IsHttpReady(url) Then
      WaitHttpReady = True
      Exit Function
    End If
    WScript.Sleep 800
  Loop
  WaitHttpReady = False
End Function

cmd = "powershell -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File """ & baseDir & "\scripts\run.ps1"" " & _
      "-MySQLHost ""127.0.0.1"" -MySQLPort 3306 -MySQLUser ""root"" -MySQLPassword ""123456"" -MySQLDB ""meituan_db_0"" " & _
      "-RedisAddrs ""127.0.0.1:6379"" -ModelScopeToken ""ms-dd4cdb20-b7a7-4e39-95ea-ae1b5f412d4d"""

AppendLog "run_silent.vbs start"
shell.Run cmd, 0, False
If WaitHttpReady("http://127.0.0.1:8080/", 90000) Then
  shell.Run "explorer.exe http://127.0.0.1:8080/", 0, False
  AppendLog "browser open triggered"
Else
  AppendLog "http not ready within timeout"
End If
