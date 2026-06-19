#define AppName "MultiSend"
#define AppVersion "0.1.0"
#define AppPublisher "Codyte"

[Setup]
AppId={{8C4D87D2-5F9E-4B7A-9C4C-2B7A9F01A001}}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL=https://github.com/Codyte/MultiSend
AppSupportURL=https://github.com/Codyte/MultiSend/issues
AppUpdatesURL=https://github.com/Codyte/MultiSend/releases
DefaultDirName={commonpf}\MultiSend
DefaultGroupName=MultiSend
OutputDir=.
OutputBaseFilename=MultiSendSetup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
DisableDirPage=no
DisableProgramGroupPage=yes
UninstallDisplayIcon={app}\bin\multisend-agent.exe

[Languages]
Name: "brazilianportuguese"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"

[Tasks]
Name: "startagent"; Description: "Iniciar o MultiSend Agent após instalar"; GroupDescription: "Opções:"; Flags: unchecked
Name: "autostart"; Description: "Iniciar o MultiSend Agent automaticamente com o Windows"; GroupDescription: "Opções:"; Flags: unchecked
Name: "protocol"; Description: "Registrar o protocolo multisend:// (abre links multisend:// no MultiSend Download Manager)"; GroupDescription: "Opções:"; Flags: checkedonce

[Files]
Source: "install-multisend-node-v5.ps1"; DestDir: "{tmp}\MultiSendPayload"; Flags: deleteafterinstall
Source: "multisend-agent.exe"; DestDir: "{tmp}\MultiSendPayload"; Flags: deleteafterinstall
Source: "multisend.exe"; DestDir: "{tmp}\MultiSendPayload"; Flags: deleteafterinstall
Source: "multirecv.exe"; DestDir: "{tmp}\MultiSendPayload"; Flags: deleteafterinstall
Source: "multisend-launcher.ps1"; DestDir: "{tmp}\MultiSendPayload"; Flags: deleteafterinstall
Source: "multisend-download-ui.ps1"; DestDir: "{tmp}\MultiSendPayload"; Flags: deleteafterinstall
Source: "multisend-settings-ui.ps1"; DestDir: "{tmp}\MultiSendPayload"; Flags: deleteafterinstall
Source: "browser-extension\manifest.json"; DestDir: "{tmp}\MultiSendPayload\browser-extension"; Flags: deleteafterinstall
Source: "browser-extension\background.js"; DestDir: "{tmp}\MultiSendPayload\browser-extension"; Flags: deleteafterinstall
Source: "browser-extension\README.md"; DestDir: "{tmp}\MultiSendPayload\browser-extension"; Flags: deleteafterinstall
Source: "install-multisend-node-v5.ps1"; DestDir: "{app}\bin"; Flags: ignoreversion

[Run]
Filename: "powershell.exe"; Parameters: "{code:GetInstallArgs}"; WorkingDir: "{tmp}\MultiSendPayload"; StatusMsg: "Instalando MultiSend..."; Flags: runhidden waituntilterminated

[UninstallRun]
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\bin\install-multisend-node-v5.ps1"" -Uninstall"; RunOnceId: "MultiSendUninstallScript"; StatusMsg: "Removendo MultiSend..."; Flags: runhidden waituntilterminated

[Icons]
Name: "{group}\MultiSend Doctor"; Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\bin\install-multisend-node-v5.ps1"" -Doctor"
Name: "{group}\MultiSend Test"; Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\bin\install-multisend-node-v5.ps1"" -Test"
Name: "{group}\MultiSend Settings"; Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\bin\multisend-settings-ui.ps1"""
Name: "{group}\MultiSend Download Manager"; Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\bin\multisend-download-ui.ps1"""

[Code]
function GetInstallArgs(Param: String): String;
begin
  Result :=
    '-NoProfile -ExecutionPolicy Bypass -File "' +
    ExpandConstant('{tmp}\MultiSendPayload\install-multisend-node-v5.ps1') +
    '" -Install';

  if WizardIsTaskSelected('startagent') then
    Result := Result + ' -StartAgentNow';

  if WizardIsTaskSelected('autostart') then
    Result := Result + ' -EnableAutoStart';
end;
