# Builds the launcher and packs it for someone else: dist\agi-<version>-windows-x64.zip holds agi.exe and a
# README.txt. The version is the date and the commit (with -dirty if the tree has changes).
#   .\scripts\release.ps1
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

$sha = (git rev-parse --short HEAD).Trim()
$dirty = if ((git status --porcelain | Measure-Object).Count -gt 0) { '-dirty' } else { '' }
$version = "$(Get-Date -Format 'yyyy.MM.dd')-$sha$dirty"

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
