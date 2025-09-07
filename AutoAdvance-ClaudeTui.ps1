Add-Type @"
using System;
using System.Runtime.InteropServices;
public class Win32 {
  [DllImport("user32.dll")] public static extern IntPtr FindWindow(string lpClassName, string lpWindowName);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
}
"@

param(
  [string]$WindowTitleLike = "Claude",   # part of window title
  [int]$EveryMs = 2000,                  # how often to send Enter
  [int]$MaxMinutes = 120
)

$deadline = (Get-Date).AddMinutes($MaxMinutes)
Write-Host "Auto-advance armed. Looking for window like: '$WindowTitleLike'"

while ((Get-Date) -lt $deadline) {
  Start-Sleep -Milliseconds $EveryMs
  $procs = Get-Process | Where-Object { $_.MainWindowTitle -like "*$WindowTitleLike*" }
  if ($procs) {
    $hWnd = [Win32]::FindWindow($null, $procs[0].MainWindowTitle)
    if ($hWnd -ne [IntPtr]::Zero) {
      [Win32]::SetForegroundWindow($hWnd) | Out-Null
      # Send Enter (and occasionally Space) to pass any approval/bypass prompts
      $ws = New-Object -ComObject WScript.Shell
      $ws.SendKeys("{ENTER}")
      Start-Sleep -Milliseconds 150
      $ws.SendKeys(" ")
      Write-Host "[tick] Sent ENTER to '$($procs[0].MainWindowTitle)'"
    }
  }
}
Write-Host "Auto-advance finished (deadline or no window)."
