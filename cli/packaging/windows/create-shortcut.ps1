# this creates a desktop shortcut that opens vogo in the correct manner

$WshShell = New-Object -ComObject WScript.Shell
$ExePath = Join-Path $PSScriptRoot "vogo.exe"
$Shortcut = $WshShell.CreateShortcut("$Home\Desktop\Vogo.lnk")
$Shortcut.TargetPath = "C:\Windows\System32\cmd.exe"
$Shortcut.Arguments = "/k `"PROMPT `$G & $ExePath`""
$Shortcut.WorkingDirectory = $PSScriptRoot
$Shortcut.Save()
