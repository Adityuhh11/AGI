# Builds the launcher and packs it for someone else: dist\agi-<version>-windows-x64.zip holds agi.exe and a
# README.txt, and dist\agi-<version>-windows-x64-setup.exe is the installer. A published release gives its number,
# which is also its tag without the "v" (the update check compares it); a test build is named by the date and the
# commit (with -dirty if the tree has changes).
#   .\scripts\release.ps1 -Version 0.2.0   for the release v0.2.0
#   .\scripts\release.ps1                  for a test build
#   -RequireInstaller                      fails instead of skipping the installer when Inno Setup is not installed
param([string]$Version = '', [switch]$RequireInstaller)
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

if ($Version -ne '') {
    if ($Version -notmatch '^\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$') { throw "version must look like 0.2.0, got $Version" }
    $version = $Version
} else {
    $sha = (git rev-parse --short HEAD).Trim()
    $dirty = if ((git status --porcelain | Measure-Object).Count -gt 0) { '-dirty' } else { '' }
    $version = "$(Get-Date -Format 'yyyy.MM.dd')-$sha$dirty"
}

& "$PSScriptRoot\build.ps1" -Version $version
if ($LASTEXITCODE -ne 0) { throw "build failed" }

$stage = "dist\agi-$version-windows-x64"
if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
New-Item -ItemType Directory -Force $stage | Out-Null
Copy-Item bin\agi.exe $stage
(Get-Content "$PSScriptRoot\release-readme.txt" -Raw).Replace('{VERSION}', $version) | Set-Content "$stage\README.txt" -Encoding utf8

$zip = "$stage.zip"
if (Test-Path $zip) { Remove-Item -Force $zip }
Compress-Archive -Path "$stage\*" -DestinationPath $zip
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
Write-Host "built $zip ($([math]::Round((Get-Item $zip).Length / 1MB, 1)) MB)"
Write-Host "sha256 $hash"

# The installer (installer\agi.iss) is built with Inno Setup's compiler: ISCC in the environment, on the path, or where
# a per-user or a normal install puts it. A machine without it still gets the zip, unless -RequireInstaller.
$iscc = @($env:ISCC, (Get-Command ISCC.exe -ErrorAction SilentlyContinue).Source,
    "$env:LOCALAPPDATA\Programs\Inno Setup 7\ISCC.exe", "${env:ProgramFiles(x86)}\Inno Setup 7\ISCC.exe",
    "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe", "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe") |
    Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
if ($iscc) {
    & $iscc "/DVersion=$version" installer\agi.iss | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "the installer did not compile" }
    $setup = "dist\agi-$version-windows-x64-setup.exe"
    Write-Host "built $setup ($([math]::Round((Get-Item $setup).Length / 1MB, 1)) MB)"
    Write-Host "sha256 $((Get-FileHash $setup -Algorithm SHA256).Hash.ToLower())"
} elseif ($RequireInstaller) {
    throw "Inno Setup's compiler (ISCC.exe) was not found"
} else {
    Write-Warning "Inno Setup's compiler (ISCC.exe) was not found: only the zip was built"
}
