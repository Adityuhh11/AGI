; The installer of Omni: for one user, no administrator rights, nothing outside the folder it is put in and the user's
; own Start menu and uninstall entries. Built by scripts\release.ps1 (ISCC /DVersion=0.6.0 installer\omni.iss) into
; dist\omni-<version>-windows-x64-setup.exe. The zip with the bare omni.exe stays as the portable download.
;
; What it decides is where the program lives and how it is reached. The hotkey, starting at sign-in and file search are
; the setup guide's: it opens on the first start.

#ifndef Version
  #define Version "0.0.0"
#endif

[Setup]
; The id is what tells an upgrade from a second program: it never changes.
AppId={{7B542CA9-0F89-4334-A83E-BEFE228ECBD8}
AppName=Omni
AppVersion={#Version}
AppPublisher=Omni
AppPublisherURL=https://github.com/Adityuhh11/Omni
AppSupportURL=https://github.com/Adityuhh11/Omni/issues
AppUpdatesURL=https://github.com/Adityuhh11/Omni/releases
; Per user: {autopf} is then %LOCALAPPDATA%\Programs, a folder the in-app update can write to.
PrivilegesRequired=lowest
DefaultDirName={autopf}\Omni
DisableProgramGroupPage=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\dist
OutputBaseFilename=omni-{#Version}-windows-x64-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\omni.exe
; A running Omni is closed before its file is replaced, and not started again behind the person's back.
CloseApplications=yes
RestartApplications=no

[Tasks]
Name: "desktopicon"; Description: "A shortcut on the desktop"; Flags: unchecked

[Files]
Source: "..\bin\omni.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\THIRD_PARTY_NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\Omni"; Filename: "{app}\omni.exe"
Name: "{autodesktop}\Omni"; Filename: "{app}\omni.exe"; Tasks: desktopicon

[Run]
; Starting it is what opens the setup guide on a first install.
Filename: "{app}\omni.exe"; Description: "Start Omni"; Flags: nowait postinstall skipifsilent

[Code]
const
  RunKey = 'Software\Microsoft\Windows\CurrentVersion\Run';
  ApprovedKey = 'Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run';
  ValueName = 'Omni';

// HasSwitch says whether the uninstaller was started with a switch such as /REMOVEDATA (a silent uninstall cannot ask).
function HasSwitch(Name: String): Boolean;
var
  I: Integer;
begin
  Result := False;
  for I := 1 to ParamCount do
    if CompareText(ParamStr(I), Name) = 0 then
      Result := True;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Code: Integer;
  Remove: Boolean;
begin
  if CurUninstallStep <> usUninstall then
    Exit;
  // Whatever is answered, nothing may be left that starts a program that is gone when the person signs in.
  RegDeleteValue(HKCU, RunKey, ValueName);
  RegDeleteValue(HKCU, ApprovedKey, ValueName);
  if UninstallSilent then
    Remove := HasSwitch('/REMOVEDATA')
  else
    Remove := MsgBox('Also delete what Omni has saved: the search engine and its models, the index of your files, the clipboard history, your notes ' +
      'and your settings (up to about 1.2 GB)?' + #13#10#13#10 + 'Choose No to keep them for a later install. Your own files are never touched.',
      mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES;
  if Remove then
    Exec(ExpandConstant('{app}\omni.exe'), '--remove-data', '', SW_HIDE, ewWaitUntilTerminated, Code);
end;
