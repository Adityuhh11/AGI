# Runs the installer for real, in a scratch folder, and checks what it does: installs, installs over itself, uninstalls
# keeping the data, installs again and uninstalls with /REMOVEDATA. It touches this user's registry (the uninstall entry,
# the Start menu shortcut, and the sign-in entry the uninstaller removes), so it puts the sign-in entry back as it was.
#   .\scripts\test-installer.ps1 -Setup dist\agi-0.6.0-windows-x64-setup.exe
param([Parameter(Mandatory)][string]$Setup)
$ErrorActionPreference = 'Stop'
$Setup = (Resolve-Path $Setup).Path
$scratch = Join-Path ([IO.Path]::GetTempPath()) "agi-installer-test-$([guid]::NewGuid().ToString('N').Substring(0, 8))"
$app = Join-Path $scratch 'app'
$local = Join-Path $scratch 'Local'
$roam = Join-Path $scratch 'Roaming'
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$uninstKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\{7B542CA9-0F89-4334-A83E-BEFE228ECBD8}_is1'
$lnk = Join-Path ([Environment]::GetFolderPath('Programs')) 'AGI.lnk'
$realAppData = $env:APPDATA
$realLocal = $env:LOCALAPPDATA
$savedRun = (Get-ItemProperty $runKey -Name AGI -ErrorAction SilentlyContinue).AGI
$failures = @()

function Check($what, $ok) {
    if ($ok) { Write-Host "ok    $what" } else { Write-Host "FAIL  $what"; $script:failures += $what }
}
function Install { Start-Process $Setup -ArgumentList "/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /DIR=`"$app`"" -Wait }
function Uninstall($more) { Start-Process (Join-Path $app 'unins000.exe') -ArgumentList "/VERYSILENT /SUPPRESSMSGBOXES /NORESTART $more" -Wait; Start-Sleep 2 }

try {
    New-Item -ItemType Directory $local, $roam -Force | Out-Null
    Install
    Check 'installs agi.exe' (Test-Path (Join-Path $app 'agi.exe'))
    Check 'makes the uninstaller' (Test-Path (Join-Path $app 'unins000.exe'))
    Check 'adds a Start menu shortcut' (Test-Path $lnk)
    Check 'adds an Apps and features entry' (Test-Path $uninstKey)
    Install
    Check 'installing over itself keeps one entry' ((Get-ChildItem 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall' | Where-Object { $_.PSChildName -like '*7B542CA9*' }).Count -eq 1)

    # What AGI keeps, in a profile of its own, and something beside it that is not AGI's.
    New-Item -ItemType Directory (Join-Path $local 'AGI\engine'), (Join-Path $local 'AGI\memory'), (Join-Path $roam 'AGI'), (Join-Path $local 'Other') -Force | Out-Null
    'x' | Set-Content (Join-Path $local 'AGI\engine\e.bin')
    'x' | Set-Content (Join-Path $local 'AGI\memory\meta.db')
    'x' | Set-Content (Join-Path $roam 'AGI\config.toml')
    'keep' | Set-Content (Join-Path $local 'Other\keep.txt')
    $env:APPDATA = $roam
    $env:LOCALAPPDATA = $local
    # A clean machine (a CI runner) may have no Run key at all; the person's own always does once a program starts at sign-in.
    if (-not (Test-Path $runKey)) { New-Item -Path $runKey -Force | Out-Null }
    Set-ItemProperty $runKey -Name AGI -Value "`"$(Join-Path $app 'agi.exe')`"" -Type String

    Uninstall ''
    Check 'uninstalls the program' (-not (Test-Path (Join-Path $app 'agi.exe')))
    Check 'a silent uninstall keeps the data' ((Test-Path (Join-Path $local 'AGI\engine\e.bin')) -and (Test-Path (Join-Path $roam 'AGI\config.toml')))
    Check 'removes the Start menu shortcut' (-not (Test-Path $lnk))
    Check 'removes the Apps and features entry' (-not (Test-Path $uninstKey))
    Check 'removes the sign-in entry' (-not (Get-ItemProperty $runKey -Name AGI -ErrorAction SilentlyContinue))

    Install
    Uninstall '/REMOVEDATA'
    Check '/REMOVEDATA deletes the engine and the index' (-not (Test-Path (Join-Path $local 'AGI')))
    Check '/REMOVEDATA deletes the settings' (-not (Test-Path (Join-Path $roam 'AGI')))
    Check '/REMOVEDATA leaves other programs alone' (Test-Path (Join-Path $local 'Other\keep.txt'))
} finally {
    $env:APPDATA = $realAppData
    $env:LOCALAPPDATA = $realLocal
    if ($savedRun) { Set-ItemProperty $runKey -Name AGI -Value $savedRun -Type String } else { Remove-ItemProperty $runKey -Name AGI -ErrorAction SilentlyContinue }
    Remove-Item $lnk -ErrorAction SilentlyContinue
    Remove-Item $scratch -Recurse -Force -ErrorAction SilentlyContinue
}
if ($failures.Count -gt 0) { throw "the installer failed $($failures.Count) check(s): $($failures -join '; ')" }
Write-Host 'the installer does what it should'
