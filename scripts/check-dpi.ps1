# Fails when the launcher would draw soft at a display scale above 100%: it starts the exe in an empty profile of its own
# (beside the launcher in use) and asks Windows whether the process is per-monitor DPI aware. A process that is not is
# stretched as a picture by Windows, which blurs every window of it (docs/decisions/0006-dpi-awareness.md).
#   .\scripts\check-dpi.ps1 [-Exe bin\omni.exe]
param([string]$Exe = 'bin\omni.exe')
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

Add-Type @"
using System; using System.Runtime.InteropServices;
public static class Dpi { [DllImport("shcore.dll")] public static extern int GetProcessDpiAwareness(IntPtr process, out int awareness); }
"@

$profileDir = Join-Path ([IO.Path]::GetTempPath()) "omni-check-dpi-$PID"
New-Item -ItemType Directory -Force "$profileDir\roaming", "$profileDir\local" | Out-Null
$env:APPDATA = "$profileDir\roaming"; $env:LOCALAPPDATA = "$profileDir\local"; $env:OMNI_INSTANCE = "check-dpi-$PID"
$process = Start-Process (Resolve-Path $Exe) -PassThru
try {
    $perMonitor = 2
    $awareness = -1
    for ($i = 0; $i -lt 40 -and $awareness -ne $perMonitor; $i++) { # the awareness is set while the app starts
        Start-Sleep -Milliseconds 500
        if ($process.HasExited) { throw "$Exe exited while it was being checked" }
        [void][Dpi]::GetProcessDpiAwareness($process.Handle, [ref]$awareness)
    }
    if ($awareness -ne $perMonitor) {
        throw "$Exe is not per-monitor DPI aware (Windows reports $awareness, wanted $perMonitor): it will look blurry above 100% scale. Check build\windows\omni.manifest."
    }
    Write-Host "dpi check passed: per-monitor aware"
} finally {
    Stop-Process $process -Force -ErrorAction SilentlyContinue
    Remove-Item $profileDir -Recurse -Force -ErrorAction SilentlyContinue
}
