# Builds the launcher: bindings, frontend, then bin\agi.exe.
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

Run wails3 @('generate', 'bindings', '-clean=true', '-ts', '-i', './cmd/agi')

Push-Location frontend
Run npm @('run', 'build', '--silent')
Pop-Location

$ldflags = if ($Console) { '-w -s' } else { '-w -s -H windowsgui' }
$ldflags += " -X main.version=$Version"
Run go @('build', '-tags', 'production', '-trimpath', '-ldflags', $ldflags, '-o', 'bin\agi.exe', './cmd/agi')
Write-Host "built bin\agi.exe"
