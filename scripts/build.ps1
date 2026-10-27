# Builds the launcher: bindings, frontend, then bin\omni.exe.
#   .\scripts\build.ps1            release build (no console window)
#   .\scripts\build.ps1 -Console   keeps the console window so the log is visible
#   .\scripts\build.ps1 -Version 2026.10.18-abc1234   stamps the version shown in Settings
param([switch]$Console, [string]$Version = 'dev')

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

function Run($command, [string[]]$arguments) {
    & $command @arguments
    if ($LASTEXITCODE -ne 0) { throw "$command failed with exit code $LASTEXITCODE" }
}

# The engine's source is embedded in the executable; compiled Python files must not go with it.
Get-ChildItem sidecar\src -Recurse -Directory -Filter __pycache__ -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force

# The binding generator and the Svelte build both need the frontend's packages.
Push-Location frontend
Run npm @('install', '--no-audit', '--no-fund')
Pop-Location

Run wails3 @('generate', 'bindings', '-clean=true', '-ts', '-i', './cmd/omni')

Push-Location frontend
Run npm @('run', 'build', '--silent')
Pop-Location

# Windows version information, the icon and a manifest go into the exe as a resource (cmd\omni\rsrc_windows_amd64.syso,
# not kept in git): an exe that says what it is and who ships it is trusted more by Windows and by scanners.
$numeric = if ($Version -match '^(\d+)\.(\d+)\.(\d+)') { "$($Matches[1]).$($Matches[2]).$($Matches[3]).0" } else { '0.0.0.0' }
$info = [ordered]@{
    fixed = [ordered]@{ file_version = $numeric }
    info  = [ordered]@{ '0409' = [ordered]@{
        FileVersion = $numeric; ProductVersion = $Version; CompanyName = 'Omni'; FileDescription = 'Omni launcher'; ProductName = 'Omni'
        Comments = 'https://github.com/Adityuhh11/Omni'
    } }
}
$info | ConvertTo-Json -Depth 5 | Set-Content build\windows\info.json -Encoding ascii
Run wails3 @('generate', 'syso', '-arch', 'amd64', '-info', 'build\windows\info.json', '-manifest', 'build\windows\omni.manifest',
    '-icon', 'build\windows\omni.ico', '-out', 'cmd\omni\rsrc_windows_amd64.syso')

$ldflags = if ($Console) { '-w -s' } else { '-w -s -H windowsgui' }
$ldflags += " -X main.version=$Version"
Run go @('build', '-tags', 'production', '-trimpath', '-ldflags', $ldflags, '-o', 'bin\omni.exe', './cmd/omni')
Write-Host "built bin\omni.exe"

# A build that Windows would stretch as a picture is not a build: it fails here, not on someone's screen.
& "$PSScriptRoot\check-dpi.ps1"
