; Inno Setup script for Bookwyrm Phase 21 packaging
#define MyAppName "Bookwyrm"
#ifndef MyAppVersion
  #define MyAppVersion "0.1.0-alpha"
#endif
#define MyPublisher "Bookwyrm"
#define MyExeName "bookwyrm-launcher.exe"

[Setup]
AppId={{B5E5C82B-2CF3-4F8E-9BB2-65D9C4A6F0A4}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyPublisher}
DefaultDirName={commonappdata}\Bookwyrm
DisableProgramGroupPage=yes
OutputDir=.
OutputBaseFilename=bookwyrm-{#MyAppVersion}-setup
Compression=lzma
SolidCompression=yes
PrivilegesRequired=admin
WizardStyle=modern

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "installservice"; Description: "Install Bookwyrm as a Windows service"; Flags: unchecked
Name: "startservice"; Description: "Start Bookwyrm service after install"; Flags: unchecked
Name: "openbrowser"; Description: "Open Bookwyrm in browser after install"; Flags: checkedonce

[Dirs]
Name: "{app}\bin"
Name: "{app}\config"
Name: "{app}\logs"
Name: "{app}\data"

[Files]
Source: "bin\bookwyrm-launcher.exe"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "bin\metadata-service.exe"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "bin\indexer-service.exe"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "bin\backend.exe"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "config\bookwyrm.env"; DestDir: "{app}\config"; Flags: onlyifdoesntexist
Source: "config\metadata-service.yaml"; DestDir: "{app}\config"; Flags: onlyifdoesntexist

[Run]
Filename: "{app}\bin\bookwyrm-launcher.exe"; Parameters: "install-service --base-dir ""{app}"""; Flags: runhidden; Tasks: installservice
Filename: "{app}\bin\bookwyrm-launcher.exe"; Parameters: "start-service --base-dir ""{app}"""; Flags: runhidden; Tasks: installservice and startservice
Filename: "{app}\bin\bookwyrm-launcher.exe"; Parameters: "run --base-dir ""{app}"""; Flags: nowait postinstall skipifsilent; Tasks: not installservice
Filename: "http://localhost:8090"; Flags: shellexec postinstall skipifsilent; Tasks: openbrowser

[UninstallRun]
Filename: "{app}\bin\bookwyrm-launcher.exe"; Parameters: "stop-service --base-dir ""{app}"""; Flags: runhidden
Filename: "{app}\bin\bookwyrm-launcher.exe"; Parameters: "uninstall-service --base-dir ""{app}"""; Flags: runhidden
