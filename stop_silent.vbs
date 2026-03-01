Option Explicit

Dim fso, shell, baseDir, cmd
Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("Wscript.Shell")

baseDir = fso.GetParentFolderName(WScript.ScriptFullName)
cmd = "powershell -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File """ & baseDir & "\scripts\stop.ps1"""

shell.Run cmd, 0, False
