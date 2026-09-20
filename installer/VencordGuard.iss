#define MyAppName "VencordGuard"
#ifndef MyAppVersion
  #define MyAppVersion "0.1.0"
#endif
#define MyAppPublisher "itousouta15"
#define MyAppURL "https://github.com/itousouta15/VencordGuard"
#define MyAppExeName "VencordGuard.exe"

[Setup]
AppId={{7FE960E8-BADD-41C9-8B95-14C8029553B5}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}/issues
AppUpdatesURL={#MyAppURL}/releases
DefaultDirName={localappdata}\Programs\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputDir=..\dist
OutputBaseFilename=VencordGuard-Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
UninstallDisplayIcon={app}\{#MyAppExeName}
CloseApplications=yes
RestartApplications=no
SetupLogging=yes
VersionInfoVersion={#MyAppVersion}
VersionInfoCompany={#MyAppPublisher}
VersionInfoDescription=Vencord injection monitor and launcher
VersionInfoProductName={#MyAppName}
VersionInfoProductVersion={#MyAppVersion}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "chinesetraditional"; MessagesFile: "languages\ChineseTraditional.isl"

[CustomMessages]
english.GuardTask=Background guard (recommended)
english.GuardTaskDescription=Monitor Discord and repair Vencord after updates
english.ShortcutsTask=Guarded Discord shortcuts
english.ShortcutsTaskDescription=Create launchers that check Vencord before Discord starts
chinesetraditional.GuardTask=背景守護（建議）
chinesetraditional.GuardTaskDescription=監控 Discord，並在更新後自動修復 Vencord
chinesetraditional.ShortcutsTask=Discord 專用防護捷徑
chinesetraditional.ShortcutsTaskDescription=建立啟動 Discord 前先檢查 Vencord 的捷徑

[Tasks]
Name: "guard"; Description: "{cm:GuardTask}"; GroupDescription: "{cm:GuardTaskDescription}"; Flags: checkedonce
Name: "shortcuts"; Description: "{cm:ShortcutsTask}"; GroupDescription: "{cm:ShortcutsTaskDescription}"; Flags: checkedonce

[Files]
Source: "..\build\VencordGuard.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\THIRD_PARTY_NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "VencordGuard"; ValueData: """{app}\{#MyAppExeName}"" --guard"; Tasks: guard; Flags: uninsdeletevalue
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: none; ValueName: "VencordGuard"; Tasks: not guard; Flags: deletevalue

[Icons]
Name: "{group}\VencordGuard"; Filename: "{app}\{#MyAppExeName}"; Parameters: "--guard"
Name: "{group}\Discord Stable via VencordGuard"; Filename: "{app}\{#MyAppExeName}"; Parameters: "--launch stable"; Tasks: shortcuts
Name: "{group}\Discord PTB via VencordGuard"; Filename: "{app}\{#MyAppExeName}"; Parameters: "--launch ptb"; Tasks: shortcuts
Name: "{group}\Discord Canary via VencordGuard"; Filename: "{app}\{#MyAppExeName}"; Parameters: "--launch canary"; Tasks: shortcuts
Name: "{userdesktop}\Discord via VencordGuard"; Filename: "{app}\{#MyAppExeName}"; Parameters: "--launch stable"; Tasks: shortcuts

[Run]
Filename: "{app}\{#MyAppExeName}"; Parameters: "--guard"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent; Tasks: guard

[UninstallRun]
Filename: "{app}\{#MyAppExeName}"; Parameters: "--quit"; Flags: runhidden waituntilterminated; RunOnceId: "StopVencordGuard"

[UninstallDelete]
Type: filesandordirs; Name: "{localappdata}\VencordGuard\tools"
