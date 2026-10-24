# Installs Inno Setup's compiler for the current user into a folder, for the build on a machine that does not have it
# (CI). The installer is a pinned release from the project's GitHub page and is checked against its SHA-256 before it
# runs. Prints the path of ISCC.exe and, on GitHub Actions, sets ISCC for the steps after it.
#   .\scripts\install-inno.ps1 [-Dir C:\tools\inno]
param([string]$Dir = (Join-Path $env:RUNNER_TEMP 'inno'))
$ErrorActionPreference = 'Stop'

$version = '7.1.0'
$sha256 = '0362A383ED217D4C4239B5933866DD96D3EB2102737DA92F80F6057A4B40DF2F'
$url = "https://github.com/jrsoftware/issrc/releases/download/is-$($version.Replace('.', '_'))/innosetup-$version-x64.exe"

$download = Join-Path ([IO.Path]::GetTempPath()) "innosetup-$version-x64.exe"
Invoke-WebRequest -Uri $url -OutFile $download
$got = (Get-FileHash $download -Algorithm SHA256).Hash
if ($got -ne $sha256) { throw "Inno Setup $version has SHA-256 $got, not the pinned $sha256" }

Start-Process $download -ArgumentList '/CURRENTUSER', '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', '/NOICONS', "/DIR=`"$Dir`"" -Wait
$iscc = Join-Path $Dir 'ISCC.exe'
if (-not (Test-Path $iscc)) { throw "Inno Setup did not install: $iscc is missing" }
if ($env:GITHUB_ENV) { "ISCC=$iscc" | Out-File -FilePath $env:GITHUB_ENV -Append -Encoding utf8 }
Write-Host $iscc
