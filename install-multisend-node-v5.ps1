# ====================== BEGIN NAV INDEX ======================
# NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
#   L97    Write-Utf8NoBomFile
#   L104   Write-Log
#   L105   Write-Check
#   L106   Write-Section
#   L107   Write-Step
#   L108   Write-Pass
#   L109   Write-Skip
#   L110   Write-WarnLine
#   L112   Test-IsAdministrator
#   L117   Ensure-Admin
#   L118   Ensure-Mode
#   L123   Get-UserConfigPath
#   L124   Get-DefaultReceiveRoot
#   L130   Ensure-Directories
#   L135   Save-State
#   L136   Read-State
#   L137   Find-Binary
#   L138   Test-AgentRunning
#   L139   Get-AgentProcesses
#   L141   Install-Binaries
#   L149   Install-ExtensionFiles
#   L160   New-NodeConfig
#   L207   Ensure-Firewall
#   L257   Set-AutoStart
#   L271   Ensure-ExplorerContext
#   L358   Ensure-ProtocolRegistration
#   L384   Stop-Agent
#   L385   Start-Agent
#   L387   Remove-RegistrySubKeyTreeSafe
#   L398   Remove-ExplorerContextKeysSafe
#   L409   Remove-ProtocolRegistrationSafe
#   L411   Ensure-Shortcuts
#   L443   Remove-ShortcutsSafe
#   L463   Run-Install
#   L482   Run-Test
#   L524   Run-Uninstall
#   L540   Run-Repair
#   L541   Run-Doctor
#   L542   Run-LabSmoke
# ======================= END NAV INDEX =======================

[CmdletBinding(SupportsShouldProcess=$true)]
param(
    [switch]$Install,
    [switch]$Uninstall,
    [switch]$Repair,
    [switch]$Test,
    [switch]$Doctor,
    [switch]$LabSmoke,

    [switch]$EnableAutoStart,
    [switch]$StartAgentNow,
    [switch]$RestartAgent,

    [string]$AppVersion = "0.1.0",
    [string]$ReceiveRoot,
    [string]$DisplayName
)

$ErrorActionPreference = 'Stop'
$ScriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path

$Paths = @{
    InstallRoot   = 'C:\Program Files\MultiSend'
    BinRoot       = 'C:\Program Files\MultiSend\bin'
    DataRoot      = 'C:\ProgramData\MultiSend'
    ConfigRoot    = 'C:\ProgramData\MultiSend\config'
    LogRoot       = 'C:\ProgramData\MultiSend\logs'
    StateFile     = 'C:\ProgramData\MultiSend\install-state.json'
    InstallLog    = 'C:\ProgramData\MultiSend\logs\install.log'
    ExtensionRoot = 'C:\Program Files\MultiSend\browser-extension'
}

$Reg = @{
    ExplorerSubKey    = '*\shell\MultiSend'
    ExplorerCmdSubKey = '*\shell\MultiSend\command'
    ProtocolSubKey    = 'multisend'
    MachineRunKey     = 'Registry::HKEY_LOCAL_MACHINE\Software\Microsoft\Windows\CurrentVersion\Run'
    LegacyUserRunKey  = 'Registry::HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run'
    RunValueName      = 'MultiSendAgent'
}

$FirewallRules = @(
    'MultiSend Agent TCP 56200-56210',
    'MultiSend Discovery UDP 56211-56220',
    'MultiSend Control TCP 56231-56240'
)

$BrowserExtensionFiles = @(
    'manifest.json',
    'background.js',
    'README.md'
)

function Write-Utf8NoBomFile {
    param([Parameter(Mandatory=$true)][string]$Path,[Parameter(Mandatory=$true)][string]$Value)
    $dir = Split-Path -Parent $Path
    if ($dir) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    [System.IO.File]::WriteAllText($Path, $Value, (New-Object System.Text.UTF8Encoding($false)))
}

function Write-Log { param([string]$Level,[string]$Message) $line = "[{0}] [{1}] {2}" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $Level, $Message; Write-Host $line; try { New-Item -ItemType Directory -Force -Path $Paths.LogRoot | Out-Null; Add-Content -Path $Paths.InstallLog -Value $line -Encoding UTF8 } catch {} }
function Write-Check { param([string]$Status,[string]$Message) Write-Host ("{0,-6} {1}" -f $Status, $Message) }
function Write-Section { param([string]$Message) Write-Host ''; Write-Host ('=== {0} ===' -f $Message) }
function Write-Step { param([string]$Message) Write-Check 'STEP' $Message }
function Write-Pass { param([string]$Message) Write-Check 'PASS' $Message }
function Write-Skip { param([string]$Message) Write-Check 'SKIP' $Message }
function Write-WarnLine { param([string]$Message) Write-Check 'WARN' $Message }

function Test-IsAdministrator {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    $p = New-Object Security.Principal.WindowsPrincipal($id)
    return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}
function Ensure-Admin { if (-not (Test-IsAdministrator)) { throw 'Run this script as Administrator.' } }
function Ensure-Mode {
    $selected = @($Install,$Uninstall,$Repair,$Test,$Doctor,$LabSmoke) | Where-Object { $_ }
    if ($selected.Count -ne 1) { throw 'Select exactly one mode: -Install, -Uninstall, -Repair, -Test, -Doctor, or -LabSmoke.' }
}

function Get-UserConfigPath { Join-Path (Join-Path $env:APPDATA 'MultiSend') 'config.json' }
function Get-DefaultReceiveRoot {
    if ($ReceiveRoot) { return $ReceiveRoot }
    $base = if ($env:USERPROFILE) { $env:USERPROFILE } else { 'C:\Users\Public' }
    return (Join-Path $base 'Downloads\MultiSend')
}

function Ensure-Directories {
    foreach ($p in @($Paths.InstallRoot,$Paths.BinRoot,$Paths.DataRoot,$Paths.ConfigRoot,$Paths.LogRoot,$Paths.ExtensionRoot)) {
        if ($PSCmdlet.ShouldProcess($p,'Create directory')) { New-Item -ItemType Directory -Force -Path $p | Out-Null; Write-Pass "Directory ready: $p" }
    }
}
function Save-State { param([hashtable]$State) if ($PSCmdlet.ShouldProcess($Paths.StateFile,'Write install state')) { Write-Utf8NoBomFile -Path $Paths.StateFile -Value ($State | ConvertTo-Json -Depth 8); Write-Pass "Install state saved: $($Paths.StateFile)" } }
function Read-State { if (-not (Test-Path -LiteralPath $Paths.StateFile)) { return $null }; try { Get-Content -LiteralPath $Paths.StateFile -Raw -Encoding UTF8 | ConvertFrom-Json } catch { Write-WarnLine "Could not parse install state: $($_.Exception.Message)"; return $null } }
function Find-Binary { param([string]$Name) $p = Join-Path $ScriptRoot $Name; if (Test-Path $p) { return $p }; return $null }
function Test-AgentRunning { return [bool](Get-Process -Name 'multisend-agent' -ErrorAction SilentlyContinue) }
function Get-AgentProcesses { @(Get-Process multisend-agent -ErrorAction SilentlyContinue | Sort-Object Id) }

function Install-Binaries {
    foreach ($name in @('multisend-agent.exe','multisend.exe','multirecv.exe','multisend-launcher.ps1','multisend-download-ui.ps1','multisend-settings-ui.ps1')) {
        $src = Find-Binary -Name $name
        if (-not $src) { throw "CRITICAL: $name not found next to installer script." }
        $dst = Join-Path $Paths.BinRoot $name
        if ($PSCmdlet.ShouldProcess($dst,"Copy $name")) { Copy-Item $src $dst -Force; Write-Pass "Copied $name" }
    }
}
function Install-ExtensionFiles {
    $extSrc = Join-Path $ScriptRoot 'browser-extension'
    if (-not (Test-Path -LiteralPath $extSrc)) { throw 'CRITICAL: browser-extension folder not found next to installer script.' }
    foreach ($name in $BrowserExtensionFiles) {
        $src = Join-Path $extSrc $name
        if (-not (Test-Path -LiteralPath $src)) { throw "CRITICAL: browser-extension file missing: $name" }
        $dst = Join-Path $Paths.ExtensionRoot $name
        if ($PSCmdlet.ShouldProcess($dst,"Copy extension file $name")) { Copy-Item $src $dst -Force; Write-Pass "Copied browser extension file: $name" }
    }
}

function New-NodeConfig {
    $cfgPath = Get-UserConfigPath
    $cfgDir = Split-Path -Parent $cfgPath
    if ($PSCmdlet.ShouldProcess($cfgDir,'Create user config dir')) { New-Item -ItemType Directory -Force -Path $cfgDir | Out-Null; Write-Pass "User config path ready: $cfgDir" }
    $existing = $null
    if (Test-Path $cfgPath) { try { $existing = Get-Content $cfgPath -Raw -Encoding UTF8 | ConvertFrom-Json } catch { Write-WarnLine 'Existing config.json could not be parsed; a new config will be written.' } }
    $nodeID = if ($existing -and $existing.node_id) { [string]$existing.node_id } else { "$env:COMPUTERNAME-$([guid]::NewGuid().ToString('N').Substring(0,8))" }
    $name = if ($DisplayName) { $DisplayName } elseif ($existing -and $existing.display_name) { [string]$existing.display_name } else { $env:COMPUTERNAME }
    $receivePath = if ($ReceiveRoot) { $ReceiveRoot } elseif ($existing -and $existing.receive_path) { [string]$existing.receive_path } else { Get-DefaultReceiveRoot }
    $cfg = [ordered]@{
        schema_version = 1
        app_version = $AppVersion
        node_id = $nodeID
        display_name = $name
        mode = 'node'
        receive_path = $receivePath
        transfer_port = 56200
        discovery_port = 56211
        discovery = 'auto'
        interfaces = 'auto'
        scheduler = 'auto'
        chunk_mode = 'auto'
        chunk_size_mb = 32
        start_agent_on_login = [bool]$EnableAutoStart
        local_api_port = 56221
        transfer_port_range = @{ start = 56200; end = 56210 }
        discovery_port_range = @{ start = 56211; end = 56220 }
        local_api_port_range = @{ start = 56221; end = 56230 }
        selected_ports = @{ transfer = 56200; discovery = 56211; local_api = 56221 }
        interface_policy = 'auto'
        interface_refresh_seconds = 5
        allow_new_interfaces_during_transfer = $true
        ignore_virtual_interfaces = $true
        ignore_vpn_interfaces = $true
        ignore_link_local = $true
        allowed_interface_types = @('ethernet','wifi','usb_ethernet')
        manual_interfaces = @()
        ignored_interfaces = @()
        cleanup_completed_chunks = $false
        keep_manifests = $true
        cleanup_empty_download_dirs = $false
        pull_folder_result = 'zip'
    }
    if ($PSCmdlet.ShouldProcess($cfgPath,'Write user config')) { Write-Utf8NoBomFile -Path $cfgPath -Value ($cfg | ConvertTo-Json -Depth 6); Write-Pass "User config written: $cfgPath" }
    if ($PSCmdlet.ShouldProcess($receivePath,'Create receive path')) { New-Item -ItemType Directory -Force -Path $receivePath | Out-Null; Write-Pass "Receive folder ready: $receivePath" }
}

function Ensure-Firewall {
    $agent = Join-Path $Paths.BinRoot 'multisend-agent.exe'
    
    try {
        if ($PSCmdlet.ShouldProcess('multisend-agent','Remove auto-generated block rules')) {
            Remove-NetFirewallRule -DisplayName "multisend-agent" -ErrorAction SilentlyContinue | Out-Null
            Remove-NetFirewallRule -DisplayName "MultiSend Node*" -ErrorAction SilentlyContinue | Out-Null
        }
    } catch {
        Write-WarnLine "Could not clean up conflicting rules: $($_.Exception.Message)"
    }

    $applied = @()
    foreach ($rule in $FirewallRules) {
        try {
            $existing = Get-NetFirewallRule -DisplayName $rule -ErrorAction SilentlyContinue
            if ($existing -and $PSCmdlet.ShouldProcess($rule,'Remove existing firewall rule')) {
                Remove-NetFirewallRule -DisplayName $rule -ErrorAction Stop | Out-Null
                Write-Pass "Removed old firewall rule: $rule"
            }
        } catch {
            Write-WarnLine "Could not remove old firewall rule ${rule}: $($_.Exception.Message)"
        }
    }
    try {
        if ($PSCmdlet.ShouldProcess($FirewallRules[0],'Create TCP firewall rule')) {
            New-NetFirewallRule -DisplayName $FirewallRules[0] -Direction Inbound -Action Allow -Program $agent -Protocol TCP -LocalPort 56200-56210 -Profile Private,Domain -RemoteAddress LocalSubnet -ErrorAction Stop | Out-Null
            Write-Pass "Created firewall rule: $($FirewallRules[0])"
        }
        if ($PSCmdlet.ShouldProcess($FirewallRules[1],'Create UDP firewall rule')) {
            New-NetFirewallRule -DisplayName $FirewallRules[1] -Direction Inbound -Action Allow -Program $agent -Protocol UDP -LocalPort 56211-56220 -Profile Private,Domain -RemoteAddress LocalSubnet -ErrorAction Stop | Out-Null
            Write-Pass "Created firewall rule: $($FirewallRules[1])"
        }
        if ($PSCmdlet.ShouldProcess($FirewallRules[2],'Create Control TCP firewall rule')) {
            New-NetFirewallRule -DisplayName $FirewallRules[2] -Direction Inbound -Action Allow -Program $agent -Protocol TCP -LocalPort 56231-56240 -Profile Private,Domain -RemoteAddress LocalSubnet -ErrorAction Stop | Out-Null
            Write-Pass "Created firewall rule: $($FirewallRules[2])"
        }
    } catch {
        Write-WarnLine "NetFirewall cmdlet failed: $($_.Exception.Message)"
    }
    foreach ($rule in $FirewallRules) {
        if (Get-NetFirewallRule -DisplayName $rule -ErrorAction SilentlyContinue) {
            Write-Pass "Firewall rule verified: $rule"; $applied += $rule
        } else {
            Write-WarnLine "Firewall rule missing after install attempt: $rule"
        }
    }
    return $applied
}

function Set-AutoStart {
    param([bool]$Enabled)
    $agent = Join-Path $Paths.BinRoot 'multisend-agent.exe'
    $cmd = '"{0}"' -f $agent
    $applied = @()
    if ($Enabled) {
        if ($PSCmdlet.ShouldProcess($Reg.MachineRunKey,'Set machine startup run key')) { New-Item -Path $Reg.MachineRunKey -Force | Out-Null; Set-ItemProperty -Path $Reg.MachineRunKey -Name $Reg.RunValueName -Value $cmd; Write-Pass 'Machine autostart enabled'; $applied += 'HKLM\Software\Microsoft\Windows\CurrentVersion\Run\MultiSendAgent' }
    } else {
        if (Get-ItemProperty -Path $Reg.MachineRunKey -Name $Reg.RunValueName -ErrorAction SilentlyContinue) { if ($PSCmdlet.ShouldProcess($Reg.MachineRunKey,'Remove machine startup run key')) { Remove-ItemProperty -Path $Reg.MachineRunKey -Name $Reg.RunValueName -ErrorAction SilentlyContinue; Write-Pass 'Machine autostart disabled' } } else { Write-Skip 'Machine autostart already disabled' }
    }
    if (Get-ItemProperty -Path $Reg.LegacyUserRunKey -Name $Reg.RunValueName -ErrorAction SilentlyContinue) { if ($PSCmdlet.ShouldProcess($Reg.LegacyUserRunKey,'Remove legacy user startup run key')) { Remove-ItemProperty -Path $Reg.LegacyUserRunKey -Name $Reg.RunValueName -ErrorAction SilentlyContinue; Write-Pass 'Removed legacy user autostart' } }
    return $applied
}

function Ensure-ExplorerContext {
    $launcher = Join-Path $Paths.BinRoot 'multisend-launcher.ps1'
    $downloadUi = Join-Path $Paths.BinRoot 'multisend-download-ui.ps1'
    $settingsUi = Join-Path $Paths.BinRoot 'multisend-settings-ui.ps1'
    $sendCmd = 'powershell.exe -NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Hidden -File "{0}" -FilePath "%1"' -f $launcher
    $downloadCmd = 'powershell.exe -NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Hidden -File "{0}"' -f $downloadUi
    $settingsCmd = 'powershell.exe -NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Hidden -File "{0}"' -f $settingsUi
    $icon = Join-Path $Paths.BinRoot 'multisend-agent.exe'
    $createMenu = {
        param($root, [string]$baseKey, [string]$hiveLabel)
        $menuKey = $root.CreateSubKey($baseKey)
        if (-not $menuKey) { return $false }
        $menuKey.SetValue('MUIVerb', 'MultiSend', [Microsoft.Win32.RegistryValueKind]::String)
        $menuKey.SetValue('Icon', $icon, [Microsoft.Win32.RegistryValueKind]::String)
        $menuKey.SetValue('SubCommands', '', [Microsoft.Win32.RegistryValueKind]::String)
        $menuKey.Close()

        $sendKey = $root.CreateSubKey("$baseKey\shell\send")
        if ($sendKey) {
            $sendKey.SetValue('MUIVerb', 'Enviar com MultiSend', [Microsoft.Win32.RegistryValueKind]::String)
            $sendKey.SetValue('Icon', $icon, [Microsoft.Win32.RegistryValueKind]::String)
            $sendKey.Close()
            $sendCmdKey = $root.CreateSubKey("$baseKey\shell\send\command")
            if ($sendCmdKey) { $sendCmdKey.SetValue('', $sendCmd, [Microsoft.Win32.RegistryValueKind]::String); $sendCmdKey.Close() }
        }

        $dlKey = $root.CreateSubKey("$baseKey\shell\download")
        if ($dlKey) {
            $dlKey.SetValue('MUIVerb', 'Download Manager', [Microsoft.Win32.RegistryValueKind]::String)
            $dlKey.SetValue('Icon', $icon, [Microsoft.Win32.RegistryValueKind]::String)
            $dlKey.Close()
            $dlCmdKey = $root.CreateSubKey("$baseKey\shell\download\command")
            if ($dlCmdKey) { $dlCmdKey.SetValue('', $downloadCmd, [Microsoft.Win32.RegistryValueKind]::String); $dlCmdKey.Close() }
        }

        $recvKey = $root.CreateSubKey("$baseKey\shell\receive")
        if ($recvKey) {
            $recvKey.SetValue('MUIVerb', 'Receber com MultiSend', [Microsoft.Win32.RegistryValueKind]::String)
            $recvKey.SetValue('Icon', $icon, [Microsoft.Win32.RegistryValueKind]::String)
            $recvKey.Close()
            $recvCmd = 'powershell.exe -NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Hidden -File "{0}" -ProtocolUrl "file://%1"' -f $downloadUi
            $recvCmdKey = $root.CreateSubKey("$baseKey\shell\receive\command")
            if ($recvCmdKey) { $recvCmdKey.SetValue('', $recvCmd, [Microsoft.Win32.RegistryValueKind]::String); $recvCmdKey.Close() }
        }

        $cfgKey = $root.CreateSubKey("$baseKey\shell\settings")
        if ($cfgKey) {
            $cfgKey.SetValue('MUIVerb', 'Configuracoes do MultiSend', [Microsoft.Win32.RegistryValueKind]::String)
            $cfgKey.SetValue('Icon', $icon, [Microsoft.Win32.RegistryValueKind]::String)
            $cfgKey.Close()
            $cfgCmdKey = $root.CreateSubKey("$baseKey\shell\settings\command")
            if ($cfgCmdKey) { $cfgCmdKey.SetValue('', $settingsCmd, [Microsoft.Win32.RegistryValueKind]::String); $cfgCmdKey.Close() }
        }
        Write-Pass "Explorer context menu ready in ${hiveLabel}: MultiSend submenu"
        return $true
    }

    try {
        $root = [Microsoft.Win32.Registry]::LocalMachine
        if ($PSCmdlet.ShouldProcess('HKLM\Software\Classes\*\shell\MultiSend','Create Explorer context menu (HKLM)')) {
            & $createMenu $root 'Software\Classes\*\shell\MultiSend' 'HKLM'
            if ($PSCmdlet.ShouldProcess('HKLM\Software\Classes\Directory\shell\MultiSend','Create Explorer directory menu (HKLM)')) {
                & $createMenu $root 'Software\Classes\Directory\shell\MultiSend' 'HKLM'
            }
            if ($PSCmdlet.ShouldProcess('HKLM\Software\Classes\Directory\Background\shell\MultiSend','Create Explorer background menu (HKLM)')) {
                & $createMenu $root 'Software\Classes\Directory\Background\shell\MultiSend' 'HKLM'
            }
            return 'HKLM\Software\Classes\*\shell\MultiSend'
        }
    } catch { Write-WarnLine "HKLM Explorer menu creation failed: $($_.Exception.Message)" }
    try {
        $root = [Microsoft.Win32.Registry]::CurrentUser
        if ($PSCmdlet.ShouldProcess('HKCU\Software\Classes\*\shell\MultiSend','Create Explorer context menu (HKCU fallback)')) {
            & $createMenu $root 'Software\Classes\*\shell\MultiSend' 'HKCU'
            if ($PSCmdlet.ShouldProcess('HKCU\Software\Classes\Directory\shell\MultiSend','Create Explorer directory menu (HKCU fallback)')) {
                & $createMenu $root 'Software\Classes\Directory\shell\MultiSend' 'HKCU'
            }
            if ($PSCmdlet.ShouldProcess('HKCU\Software\Classes\Directory\Background\shell\MultiSend','Create Explorer background menu (HKCU fallback)')) {
                & $createMenu $root 'Software\Classes\Directory\Background\shell\MultiSend' 'HKCU'
            }
            return 'HKCU\Software\Classes\*\shell\MultiSend'
        }
    } catch { Write-WarnLine "HKCU Explorer menu creation failed: $($_.Exception.Message)" }
    Write-WarnLine 'Explorer context menu could not be created (HKLM/HKCU).'
    return $null
}

function Ensure-ProtocolRegistration {
    $downloadUi = Join-Path $Paths.BinRoot 'multisend-download-ui.ps1'
    $cmd = 'powershell.exe -NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Hidden -File "{0}" -ProtocolUrl "%1"' -f $downloadUi
    $appId = 'MultiSend Download'
    $protocolKey = 'Software\Classes\multisend'
    $commandKey = 'Software\Classes\multisend\shell\open\command'
    $root = [Microsoft.Win32.Registry]::LocalMachine
    if ($PSCmdlet.ShouldProcess("HKLM\$protocolKey",'Register multisend protocol')) {
        try {
            $k = $root.CreateSubKey($protocolKey)
            $k.SetValue('', 'URL:MultiSend Protocol', [Microsoft.Win32.RegistryValueKind]::String)
            $k.SetValue('URL Protocol', '', [Microsoft.Win32.RegistryValueKind]::String)
            $k.SetValue('FriendlyTypeName', $appId, [Microsoft.Win32.RegistryValueKind]::String)
            $k.Close()
            $ck = $root.CreateSubKey($commandKey)
            $ck.SetValue('', $cmd, [Microsoft.Win32.RegistryValueKind]::String)
            $ck.Close()
            Write-Pass 'Protocol registration ready: multisend://'
            return 'HKLM\Software\Classes\multisend'
        } catch {
            Write-WarnLine "HKLM protocol registration failed: $($_.Exception.Message)"
        }
    }
    return $null
}

function Stop-Agent { $procs = Get-AgentProcesses; if (-not $procs) { Write-Skip 'MultiSend agent is not running'; return }; foreach ($p in $procs) { Write-Step ("Stopping MultiSend agent PID {0}" -f $p.Id); if ($PSCmdlet.ShouldProcess("PID $($p.Id)",'Stop MultiSend agent')) { try { Stop-Process -Id $p.Id -Force -ErrorAction Stop; Write-Pass "Stop signal sent to PID $($p.Id)" } catch { Write-WarnLine "Could not stop agent PID $($p.Id): $($_.Exception.Message)" } } } }
function Start-Agent { $agent = Join-Path $Paths.BinRoot 'multisend-agent.exe'; if (-not (Test-Path $agent)) { throw "CRITICAL: agent not found: $agent" }; if ((Get-AgentProcesses).Count -gt 0) { Write-Skip 'MultiSend agent already running'; return }; if ($PSCmdlet.ShouldProcess($agent,'Start MultiSend agent')) { Start-Process -FilePath $agent -WindowStyle Hidden | Out-Null; Start-Sleep -Seconds 1; if ((Get-AgentProcesses).Count -gt 0) { Write-Pass 'MultiSend agent started' } else { Write-WarnLine 'Start requested but no multisend-agent process was detected' } } }

function Remove-RegistrySubKeyTreeSafe {
    param([Parameter(Mandatory=$true)][ValidateSet('HKLM','HKCU','HKCR')][string]$Hive,[Parameter(Mandatory=$true)][string]$SubKey)
    $label = "$Hive\$SubKey"
    try {
        switch ($Hive) { 'HKLM' { $root = [Microsoft.Win32.Registry]::LocalMachine } 'HKCU' { $root = [Microsoft.Win32.Registry]::CurrentUser } 'HKCR' { $root = [Microsoft.Win32.Registry]::ClassesRoot } }
        $k = $root.OpenSubKey($SubKey, $false)
        if (-not $k) { Write-Skip "Registry key not found: $label"; return }
        $k.Close()
        if ($PSCmdlet.ShouldProcess($label,'Remove registry tree')) { $root.DeleteSubKeyTree($SubKey, $false); Write-Pass "Removed registry key: $label" }
    } catch { Write-WarnLine "Registry cleanup warning for ${label}: $($_.Exception.Message)" }
}
function Remove-ExplorerContextKeysSafe {
    Remove-RegistrySubKeyTreeSafe -Hive 'HKLM' -SubKey 'Software\Classes\*\shell\MultiSend'
    Remove-RegistrySubKeyTreeSafe -Hive 'HKCU' -SubKey 'Software\Classes\*\shell\MultiSend'
    Remove-RegistrySubKeyTreeSafe -Hive 'HKCR' -SubKey '*\shell\MultiSend'
    Remove-RegistrySubKeyTreeSafe -Hive 'HKLM' -SubKey 'Software\Classes\Directory\shell\MultiSend'
    Remove-RegistrySubKeyTreeSafe -Hive 'HKCU' -SubKey 'Software\Classes\Directory\shell\MultiSend'
    Remove-RegistrySubKeyTreeSafe -Hive 'HKCR' -SubKey 'Directory\shell\MultiSend'
    Remove-RegistrySubKeyTreeSafe -Hive 'HKLM' -SubKey 'Software\Classes\Directory\Background\shell\MultiSend'
    Remove-RegistrySubKeyTreeSafe -Hive 'HKCU' -SubKey 'Software\Classes\Directory\Background\shell\MultiSend'
    Remove-RegistrySubKeyTreeSafe -Hive 'HKCR' -SubKey 'Directory\Background\shell\MultiSend'
}
function Remove-ProtocolRegistrationSafe { Remove-RegistrySubKeyTreeSafe -Hive 'HKLM' -SubKey 'Software\Classes\multisend'; Remove-RegistrySubKeyTreeSafe -Hive 'HKCU' -SubKey 'Software\Classes\multisend'; Remove-RegistrySubKeyTreeSafe -Hive 'HKCR' -SubKey 'multisend' }

function Ensure-Shortcuts {
    $shortcutPath = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\MultiSend.lnk'
    if ($PSCmdlet.ShouldProcess($shortcutPath,'Create Start Menu shortcut')) {
        try {
            $WshShell = New-Object -ComObject WScript.Shell
            $Shortcut = $WshShell.CreateShortcut($shortcutPath)
            $Shortcut.TargetPath = 'powershell.exe'
            $downloadUi = Join-Path $Paths.BinRoot 'multisend-download-ui.ps1'
            $Shortcut.Arguments = "-NoExit -NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Normal -File `"$downloadUi`""
            $Shortcut.IconLocation = "$(Join-Path $Paths.BinRoot 'multisend-agent.exe'),0"
            $Shortcut.Description = 'MultiSend Downloads'
            $Shortcut.Save()
            Write-Pass 'Start Menu shortcut created: MultiSend.lnk'

            $settingsShortcutPath = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\MultiSend Settings.lnk'
            $SettingsShortcut = $WshShell.CreateShortcut($settingsShortcutPath)
            $SettingsShortcut.TargetPath = 'powershell.exe'
            $settingsUi = Join-Path $Paths.BinRoot 'multisend-settings-ui.ps1'
            $SettingsShortcut.Arguments = "-NoExit -NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Normal -File `"$settingsUi`""
            $SettingsShortcut.IconLocation = "$(Join-Path $Paths.BinRoot 'multisend-agent.exe'),0"
            $SettingsShortcut.Description = 'MultiSend Settings'
            $SettingsShortcut.Save()
            Write-Pass 'Start Menu shortcut created: MultiSend Settings.lnk'

            return $shortcutPath
        } catch {
            Write-WarnLine "Could not create shortcut: $($_.Exception.Message)"
        }
    }
    return $null
}

function Remove-ShortcutsSafe {
    $shortcutPath = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\MultiSend.lnk'
    if (Test-Path -LiteralPath $shortcutPath) {
        if ($PSCmdlet.ShouldProcess($shortcutPath, 'Remove Start Menu shortcut')) {
            Remove-Item -Path $shortcutPath -Force -ErrorAction SilentlyContinue
            Write-Pass 'Removed Start Menu shortcut'
        }
    } else {
        Write-Skip 'Start Menu shortcut not found'
    }
    
    $settingsShortcutPath = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\MultiSend Settings.lnk'
    if (Test-Path -LiteralPath $settingsShortcutPath) {
        if ($PSCmdlet.ShouldProcess($settingsShortcutPath, 'Remove Start Menu settings shortcut')) {
            Remove-Item -Path $settingsShortcutPath -Force -ErrorAction SilentlyContinue
            Write-Pass 'Removed Start Menu shortcut: MultiSend Settings'
        }
    }
}

function Run-Install {
    Write-Section 'MultiSend install'
    if (Test-AgentRunning) { Write-Step 'Stopping current agent before binary update'; Stop-Agent }
    Ensure-Directories
    $state = @{ schema_version = 1; app_version = $AppVersion; mode = 'node'; installed_at = (Get-Date).ToString('s'); applied_firewall_rules = @(); applied_registry_keys = @(); applied_run_values = @(); notes = @('node-mode install') }
    Write-Step 'Copying binaries'; Install-Binaries
    Write-Step 'Copying browser extension payload'; Install-ExtensionFiles
    Write-Step 'Writing user configuration'; New-NodeConfig
    Write-Step 'Configuring firewall rules'; $state.applied_firewall_rules = @(Ensure-Firewall)
    Write-Step 'Configuring autostart'; $state.applied_run_values = @(Set-AutoStart -Enabled ([bool]$EnableAutoStart)); if (-not $EnableAutoStart) { $state.notes += 'autostart disabled by installer option'; Write-Skip 'Autostart not enabled. Use -EnableAutoStart to enable it.' }
    Write-Step 'Configuring Explorer context menu'; $k = Ensure-ExplorerContext; if ($k) { $state.applied_registry_keys += $k }
    Write-Step 'Registering multisend protocol'; $pk = Ensure-ProtocolRegistration; if ($pk) { $state.applied_registry_keys += $pk }
    Write-Step 'Creating shortcuts'; $sk = Ensure-Shortcuts; if ($sk) { $state.applied_shortcuts += $sk }
    Write-Step 'Saving install state'; Save-State -State $state
    if ($RestartAgent) { Write-Step 'Starting agent (restart mode)'; Start-Agent }
    elseif ($StartAgentNow) { Write-Step 'Starting agent'; Start-Agent } else { Write-Skip 'Agent not started. Use -StartAgentNow to start it after install.' }
    Write-Log -Level 'INFO' -Message 'Install completed (node mode).'
}

function Run-Test {
    param([bool]$IsAdmin)
    Write-Section 'MultiSend test'
    $errors = @(); $warnings = @()
    foreach ($path in @($Paths.BinRoot,$Paths.ExtensionRoot)) { if (Test-Path $path) { Write-Pass "Installed path: $path" } else { Write-WarnLine "Not installed yet: $path"; $warnings += "Missing path: $path" } }
    $userCfgRoot = Join-Path $env:APPDATA 'MultiSend'; if (Test-Path $userCfgRoot) { Write-Pass "User config path: $userCfgRoot" } else { Write-WarnLine "User config path not created yet: $userCfgRoot"; $warnings += "Missing path: $userCfgRoot" }
    foreach ($f in @('multisend-agent.exe','multisend.exe','multirecv.exe','multisend-launcher.ps1','multisend-download-ui.ps1','multisend-settings-ui.ps1')) {
        $installed = Join-Path $Paths.BinRoot $f
        $source = Join-Path $ScriptRoot $f
        if (Test-Path $installed) { Write-Pass "Installed file: $f" }
        elseif (Test-Path $source) { Write-WarnLine "Source exists but not installed: $f" }
        else { Write-Check 'FAIL' "Missing source/installed file: $f"; $errors += "Missing file: $f" }
    }
    foreach ($f in $BrowserExtensionFiles) {
        $installed = Join-Path $Paths.ExtensionRoot $f
        $source = Join-Path (Join-Path $ScriptRoot 'browser-extension') $f
        if (Test-Path $installed) { Write-Pass "Extension file: $f" }
        elseif (Test-Path $source) { Write-WarnLine "Extension source exists but not installed: $f" }
        else { Write-WarnLine "Extension file missing: $f"; $warnings += "Extension file missing: $f" }
    }
    $cfgPath = Get-UserConfigPath
    if (Test-Path $cfgPath) { try { $cfg = Get-Content $cfgPath -Raw -Encoding UTF8 | ConvertFrom-Json; if ($cfg.node_id) { Write-Pass 'config.json node_id present' } else { Write-WarnLine 'node_id missing in config.json'; $warnings += 'node_id missing in config.json' } } catch { Write-Check 'FAIL' 'config.json parse failed'; $errors += 'config.json parse failed' } } else { Write-WarnLine 'config.json not found yet'; $warnings += 'config.json not found' }
    if ($IsAdmin) { foreach ($rule in $FirewallRules) { if (Get-NetFirewallRule -DisplayName $rule -ErrorAction SilentlyContinue) { Write-Pass "Firewall rule found: $rule" } else { Write-WarnLine "Firewall rule not found: $rule"; $warnings += "Firewall rule not found: $rule" } } } else { Write-WarnLine 'Firewall check limited: run elevated for full validation.'; $warnings += 'Firewall check limited.' }
    if (Get-ItemProperty -Path $Reg.MachineRunKey -Name $Reg.RunValueName -ErrorAction SilentlyContinue) { Write-Pass 'Machine autostart found' } else { Write-Skip 'Machine autostart not enabled' }
    $hasExplorer = (Get-Item -LiteralPath 'Registry::HKEY_LOCAL_MACHINE\Software\Classes\*\shell\MultiSend' -ErrorAction SilentlyContinue) -or (Get-Item -LiteralPath 'Registry::HKEY_CURRENT_USER\Software\Classes\*\shell\MultiSend' -ErrorAction SilentlyContinue) -or (Get-Item -LiteralPath 'Registry::HKEY_CLASSES_ROOT\*\shell\MultiSend' -ErrorAction SilentlyContinue)
    if ($hasExplorer) { Write-Pass 'Explorer context menu found' } else { Write-WarnLine 'Explorer context menu not found'; $warnings += 'Explorer context menu not found.' }
    if (Get-Item -LiteralPath 'Registry::HKEY_LOCAL_MACHINE\Software\Classes\multisend' -ErrorAction SilentlyContinue) { Write-Pass 'Protocol registration found' } else { Write-WarnLine 'Protocol registration not found'; $warnings += 'Protocol registration not found.' }
    
    $settingsShortcutPath = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\MultiSend Settings.lnk'
    if (Test-Path -LiteralPath $settingsShortcutPath) { Write-Pass 'Start Menu shortcut found: MultiSend Settings' } else { Write-WarnLine 'Start Menu shortcut not found: MultiSend Settings'; $warnings += 'Start Menu shortcut not found: MultiSend Settings' }

    $apiPort = 56221
    try {
        $cfg = Get-Content -LiteralPath $cfgPath -Raw | ConvertFrom-Json
        if ($cfg.selected_ports.local_api) { $apiPort = [int]$cfg.selected_ports.local_api } elseif ($cfg.local_api_port) { $apiPort = [int]$cfg.local_api_port } elseif ($cfg.local_api_port_range.start) { $apiPort = [int]$cfg.local_api_port_range.start }
    } catch {}
    try { Invoke-RestMethod -Uri ("http://127.0.0.1:{0}/health" -f $apiPort) -TimeoutSec 2 | Out-Null; Write-Pass 'Local agent API /health responding' } catch { Write-WarnLine 'Local agent API /health not responding'; $warnings += 'Local agent API /health not responding.' }
    if ($errors.Count -gt 0) { Write-Log -Level 'ERROR' -Message ($errors -join ' | '); throw 'CRITICAL: test mode found blocking errors.' }
    if ($warnings.Count -gt 0) { Write-Log -Level 'WARN' -Message ($warnings -join ' | ') }
    Write-Log -Level 'INFO' -Message 'Test completed.'
}

function Run-Uninstall {
    Write-Section 'MultiSend uninstall'
    Stop-Agent
    $state = Read-State
    $managedFirewall = @(); if ($state -and $state.applied_firewall_rules) { $managedFirewall += @($state.applied_firewall_rules) }; foreach ($r in $FirewallRules) { if ($managedFirewall -notcontains $r) { $managedFirewall += $r } }
    Write-Step 'Removing firewall rules'; foreach ($rule in $managedFirewall) { try { if (Get-NetFirewallRule -DisplayName $rule -ErrorAction SilentlyContinue) { if ($PSCmdlet.ShouldProcess($rule,'Remove firewall rule')) { Remove-NetFirewallRule -DisplayName $rule -ErrorAction Stop | Out-Null; Write-Pass "Removed firewall rule: $rule" } } else { Write-Skip "Firewall rule not found: $rule" } } catch { Write-WarnLine "Could not remove firewall rule ${rule}: $($_.Exception.Message)" } }
    Write-Step 'Removing autostart entries'; foreach ($runKey in @($Reg.MachineRunKey,$Reg.LegacyUserRunKey)) { try { if (Get-ItemProperty -Path $runKey -Name $Reg.RunValueName -ErrorAction SilentlyContinue) { if ($PSCmdlet.ShouldProcess($runKey,'Remove autostart run key')) { Remove-ItemProperty -Path $runKey -Name $Reg.RunValueName -ErrorAction SilentlyContinue; Write-Pass "Removed autostart entry: $runKey" } } else { Write-Skip "Autostart entry not found: $runKey" } } catch { Write-WarnLine "Could not remove autostart entry ${runKey}: $($_.Exception.Message)" } }
    Write-Step 'Removing Explorer context menu'; Remove-ExplorerContextKeysSafe
    Write-Step 'Removing multisend protocol'; Remove-ProtocolRegistrationSafe
    Write-Step 'Removing shortcuts'; Remove-ShortcutsSafe
    Write-Step 'Removing install folder'; if (Test-Path $Paths.InstallRoot) { if ($PSCmdlet.ShouldProcess($Paths.InstallRoot,'Remove install folder')) { Remove-Item -Path $Paths.InstallRoot -Recurse -Force -ErrorAction SilentlyContinue; Write-Pass "Removed install folder: $($Paths.InstallRoot)" } } else { Write-Skip "Install folder not found: $($Paths.InstallRoot)" }
    Write-Check 'KEEP' "User config preserved: $(Join-Path $env:APPDATA 'MultiSend')"
    Write-Check 'KEEP' "Logs/install state preserved: $($Paths.DataRoot)"
    Write-Log -Level 'INFO' -Message 'Uninstall completed. User config, logs, and received files were preserved.'
}

function Run-Repair { Write-Section 'MultiSend repair'; Write-Log -Level 'INFO' -Message 'Repair started.'; Run-Install }
function Run-Doctor { Write-Section 'MultiSend doctor'; $installed = Join-Path $Paths.BinRoot 'multisend-agent.exe'; $local = Join-Path $ScriptRoot 'multisend-agent.exe'; $agent = if (Test-Path -LiteralPath $installed) { Write-Pass "Using installed agent: $installed"; $installed } elseif (Test-Path -LiteralPath $local) { Write-WarnLine "Installed agent not found. Using local fallback: $local"; $local } else { Write-Check 'FAIL' 'multisend-agent.exe not found (installed or local).'; throw 'CRITICAL: doctor could not find multisend-agent.exe.' }; & $agent --doctor; if ($LASTEXITCODE -ne 0) { throw "agent --doctor exited with code $LASTEXITCODE" } Write-Pass 'agent --doctor finished successfully' }
function Run-LabSmoke { Write-Section 'MultiSend lab smoke'; $installed = Join-Path $Paths.BinRoot 'multisend-agent.exe'; $local = Join-Path $ScriptRoot 'multisend-agent.exe'; $agent = if (Test-Path -LiteralPath $installed) { Write-Pass "Using installed agent: $installed"; $installed } elseif (Test-Path -LiteralPath $local) { Write-WarnLine "Installed agent not found. Using local fallback: $local"; $local } else { Write-Check 'FAIL' 'multisend-agent.exe not found (installed or local).'; throw 'CRITICAL: lab smoke could not find multisend-agent.exe.' }; & $agent --lab-smoke; if ($LASTEXITCODE -ne 0) { throw "lab-smoke failed with exit code $LASTEXITCODE" }; Write-Pass 'agent --lab-smoke finished successfully' }

try {
    Ensure-Mode
    $isAdmin = Test-IsAdministrator
    if (($Install -or $Uninstall -or $Repair) -and -not $isAdmin) { Ensure-Admin }
    if (($Test -or $Doctor -or $LabSmoke) -and -not $isAdmin) { Write-Warning 'Running without Administrator. Firewall checks may be limited.' }
    switch ($true) { $Install { Run-Install; break } $Uninstall { Run-Uninstall; break } $Repair { Run-Repair; break } $Test { Run-Test -IsAdmin:$isAdmin; break } $Doctor { Run-Doctor; break } $LabSmoke { Run-LabSmoke; break } }
    Write-Host ''; Write-Check 'DONE' 'MultiSend installer finished.'; Write-Check 'LOG' $Paths.InstallLog; exit 0
} catch {
    Write-Host ''; Write-Check 'FAIL' $_.Exception.Message; Write-Check 'LOG' $Paths.InstallLog; exit 1
}
