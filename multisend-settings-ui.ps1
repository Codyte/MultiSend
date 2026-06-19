# ====================== BEGIN NAV INDEX ======================
# NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
#   L29    Read-Config
#   L36    Write-Config
#   L44    Backup-Config
#   L53    Restore-Config
#   L60    Join-List
#   L66    Split-List
#   L72    Get-PropValue
#   L95    Add-Page
#   L109   New-NodeSecret
#   L116   New-Label
#   L117   New-Box
#   L118   New-Check
#   L119   Add-LineToMultilineBox
#   L127   Remove-LineFromMultilineBox
#   L257   Get-AgentApiPort
#   L273   Load-Interfaces
#   Segurança tab controls ~L330; save/validation in $btnSave click
# ======================= END NAV INDEX =======================

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$cfgPath = Join-Path $env:APPDATA 'MultiSend\config.json'
$backupDir = Join-Path $env:APPDATA 'MultiSend\backups'

function Read-Config {
    if (-not (Test-Path -LiteralPath $cfgPath)) {
        throw "config.json not found at $cfgPath"
    }
    Get-Content -LiteralPath $cfgPath -Raw -Encoding UTF8 | ConvertFrom-Json
}

function Write-Config {
    param($ConfigObject)
    $dir = Split-Path -Parent $cfgPath
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $json = $ConfigObject | ConvertTo-Json -Depth 10
    [System.IO.File]::WriteAllText($cfgPath, $json, (New-Object System.Text.UTF8Encoding($false)))
}

function Backup-Config {
    if (-not (Test-Path -LiteralPath $cfgPath)) { return $null }
    New-Item -ItemType Directory -Force -Path $backupDir | Out-Null
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $backupPath = Join-Path $backupDir "config-$stamp.json"
    Copy-Item -LiteralPath $cfgPath -Destination $backupPath -Force
    return $backupPath
}

function Restore-Config {
    $latest = Get-ChildItem -LiteralPath $backupDir -Filter 'config-*.json' -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if (-not $latest) { throw "No backups found in $backupDir" }
    Copy-Item -LiteralPath $latest.FullName -Destination $cfgPath -Force
    return $latest.FullName
}

function Join-List {
    param([string[]]$Items)
    if (-not $Items) { return '' }
    return ($Items | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | ForEach-Object { $_.Trim() }) -join "`r`n"
}

function Split-List {
    param([string]$Text)
    if ([string]::IsNullOrWhiteSpace($Text)) { return @() }
    return @($Text -split "(`r`n|`n|,)" | ForEach-Object { $_.Trim() } | Where-Object { $_ })
}

function Get-PropValue {
    param($Object, [string]$Name, $Default)
    if ($null -ne $Object -and $Object.PSObject.Properties.Name -contains $Name -and $null -ne $Object.$Name) {
        return $Object.$Name
    }
    return $Default
}

$cfg = Read-Config

$form = New-Object System.Windows.Forms.Form
$form.Text = 'MultiSend - Configurações'
$form.Width = 1040
$form.Height = 820
$form.StartPosition = 'CenterScreen'
$form.MinimumSize = New-Object System.Drawing.Size(900, 700)
$form.FormBorderStyle = 'Sizable'
$form.MaximizeBox = $true

$tabs = New-Object System.Windows.Forms.TabControl
$tabs.Dock = 'Fill'
$form.Controls.Add($tabs)

function Add-Page {
    param([string]$Name)
    $page = New-Object System.Windows.Forms.TabPage
    $page.Text = $Name
    $tabs.TabPages.Add($page) | Out-Null
    return $page
}

$pageGeneral = Add-Page 'Geral'
$pageInterfaces = Add-Page 'Interfaces'
$pageSecurity = Add-Page 'Segurança'
$pageCleanup = Add-Page 'Limpeza'
$pagePull = Add-Page 'Recebimento LAN'

function New-NodeSecret {
    $bytes = New-Object byte[] 32
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
    return -join ($bytes | ForEach-Object { $_.ToString('x2') })
}

function New-Label { param($Text,$Left,$Top,$Width=180) $l = New-Object System.Windows.Forms.Label; $l.Text=$Text; $l.Left=$Left; $l.Top=$Top; $l.Width=$Width; return $l }
function New-Box { param($Left,$Top,$Width=300,$Text='') $t = New-Object System.Windows.Forms.TextBox; $t.Left=$Left; $t.Top=$Top; $t.Width=$Width; $t.Text=$Text; return $t }
function New-Check { param($Left,$Top,$Text,$Checked) $c = New-Object System.Windows.Forms.CheckBox; $c.Left=$Left; $c.Top=$Top; $c.Width=360; $c.Text=$Text; $c.Checked=[bool]$Checked; return $c }
function Add-LineToMultilineBox {
    param([System.Windows.Forms.TextBox]$Box,[string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) { return }
    $lines = @($Box.Lines | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | ForEach-Object { $_.Trim() })
    if ($lines -contains $Value) { return }
    $lines += $Value
    $Box.Text = ($lines -join "`r`n")
}
function Remove-LineFromMultilineBox {
    param([System.Windows.Forms.TextBox]$Box,[string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) { return }
    $lines = @($Box.Lines | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | ForEach-Object { $_.Trim() })
    $lines = @($lines | Where-Object { $_ -ne $Value })
    $Box.Text = ($lines -join "`r`n")
}

$pageGeneral.Controls.Add((New-Label 'Nome exibido' 20 20))
$txtDisplay = New-Box 220 16 420 $cfg.display_name
$pageGeneral.Controls.Add($txtDisplay)
$pageGeneral.Controls.Add((New-Label 'Pasta de recebimento' 20 60))
$txtReceive = New-Box 220 56 620 $cfg.receive_path
$pageGeneral.Controls.Add($txtReceive)
$pageGeneral.Controls.Add((New-Label 'Porta API local' 20 100))
$numLocal = New-Object System.Windows.Forms.NumericUpDown
$numLocal.Left = 220; $numLocal.Top = 96; $numLocal.Width = 120; $numLocal.Minimum = 1; $numLocal.Maximum = 65535
$selectedPorts = Get-PropValue -Object $cfg -Name 'selected_ports' -Default $null
$localApi = Get-PropValue -Object $selectedPorts -Name 'local_api' -Default $null
if ($null -ne $localApi) {
    $numLocal.Value = [int]$localApi
} elseif ($null -ne (Get-PropValue -Object $cfg -Name 'local_api_port' -Default $null)) {
    $numLocal.Value = [int](Get-PropValue -Object $cfg -Name 'local_api_port' -Default 56221)
} else {
    $numLocal.Value = 56221
}
$pageGeneral.Controls.Add($numLocal)
$startAgent = Get-PropValue -Object $cfg -Name 'start_agent_on_login' -Default $false
$chkAutoStart = New-Check 20 140 'Iniciar Agent ao entrar' $startAgent
$pageGeneral.Controls.Add($chkAutoStart)

$pageInterfaces.Controls.Add((New-Label 'Política de interfaces' 20 20))
$cmbPolicy = New-Object System.Windows.Forms.ComboBox
$cmbPolicy.Left = 220; $cmbPolicy.Top = 16; $cmbPolicy.Width = 220; $cmbPolicy.DropDownStyle = 'DropDownList'
[void]$cmbPolicy.Items.AddRange(@('auto','manual','all'))
$policy = Get-PropValue -Object $cfg -Name 'interface_policy' -Default 'auto'
if ($cmbPolicy.Items.Contains($policy)) { $cmbPolicy.SelectedItem = $policy } else { $cmbPolicy.SelectedItem = 'auto' }
$pageInterfaces.Controls.Add($cmbPolicy)
$pageInterfaces.Controls.Add((New-Label 'auto: escolhe interfaces utilizaveis | manual: usa somente Manual interfaces | all: permite todas exceto ignoradas.' 460 20 540))
$pageInterfaces.Controls.Add((New-Label 'Atualização (s)' 20 60))
$numRefresh = New-Object System.Windows.Forms.NumericUpDown
$numRefresh.Left = 220; $numRefresh.Top = 56; $numRefresh.Width = 120; $numRefresh.Minimum = 2; $numRefresh.Maximum = 60
$refreshSec = Get-PropValue -Object $cfg -Name 'interface_refresh_seconds' -Default 5
if ($refreshSec -lt 2) { $refreshSec = 5 }
$numRefresh.Value = [int]$refreshSec
$pageInterfaces.Controls.Add($numRefresh)
$pageInterfaces.Controls.Add((New-Label 'Intervalo de atualizacao/deteccao de interfaces (segundos).' 360 60 500))

$allowNew = Get-PropValue -Object $cfg -Name 'allow_new_interfaces_during_transfer' -Default $true
$chkAllowNew = New-Check 20 100 'Permitir novas interfaces durante transferência' $allowNew
$pageInterfaces.Controls.Add($chkAllowNew)
$pageInterfaces.Controls.Add((New-Label 'Permite adicionar nova interface no meio da transferencia.' 380 102 500))

$ignVirtual = Get-PropValue -Object $cfg -Name 'ignore_virtual_interfaces' -Default $true
$chkIgnoreVirtual = New-Check 20 130 'Ignorar interfaces virtuais' $ignVirtual
$pageInterfaces.Controls.Add($chkIgnoreVirtual)
$pageInterfaces.Controls.Add((New-Label 'Ignora Hyper-V/VMware/TAP e interfaces virtuais.' 380 132 500))

$ignVpn = Get-PropValue -Object $cfg -Name 'ignore_vpn_interfaces' -Default $true
$chkIgnoreVpn = New-Check 20 160 'Ignorar interfaces VPN' $ignVpn
$pageInterfaces.Controls.Add($chkIgnoreVpn)
$pageInterfaces.Controls.Add((New-Label 'Ignora adaptadores de VPN para evitar rotas instaveis.' 380 162 500))

$ignLink = Get-PropValue -Object $cfg -Name 'ignore_link_local' -Default $true
$chkIgnoreLink = New-Check 20 190 'Ignorar endereços link-local' $ignLink
$pageInterfaces.Controls.Add($chkIgnoreLink)
$pageInterfaces.Controls.Add((New-Label 'Ignora enderecos APIPA/link-local (169.254.x.x).' 380 192 500))
$pageInterfaces.Controls.Add((New-Label 'Tipos de interface permitidos' 20 230 200))
$allowedArr = Get-PropValue -Object $cfg -Name 'allowed_interface_types' -Default @('ethernet','wifi','usb_ethernet')
$txtAllowed = New-Box 220 226 620 (Join-List $allowedArr)
$txtAllowed.Multiline = $true; $txtAllowed.Height = 80
$pageInterfaces.Controls.Add($txtAllowed)
$pageInterfaces.Controls.Add((New-Label 'Interfaces manuais' 20 320 180))
$manualArr = Get-PropValue -Object $cfg -Name 'manual_interfaces' -Default @()
$txtManual = New-Box 220 316 620 (Join-List $manualArr)
$txtManual.Multiline = $true; $txtManual.Height = 80
$pageInterfaces.Controls.Add($txtManual)
$pageInterfaces.Controls.Add((New-Label 'IPs/Nomes permitidos manualmente (1 por linha).' 20 402 360))
$pageInterfaces.Controls.Add((New-Label 'Interfaces ignoradas' 20 430 180))
$ignoredArr = Get-PropValue -Object $cfg -Name 'ignored_interfaces' -Default @()
$txtIgnored = New-Box 220 426 620 (Join-List $ignoredArr)
$txtIgnored.Multiline = $true; $txtIgnored.Height = 80
$pageInterfaces.Controls.Add($txtIgnored)
$pageInterfaces.Controls.Add((New-Label 'Nomes/IPs para sempre ignorar (1 por linha).' 20 512 360))

$pageInterfaces.Controls.Add((New-Label 'Interfaces detectadas' 20 540 180))

$listInterfaces = New-Object System.Windows.Forms.ListView
$listInterfaces.Left = 20
$listInterfaces.Top = 560
$listInterfaces.Width = 870
$listInterfaces.Height = 160
$listInterfaces.View = 'Details'
$listInterfaces.FullRowSelect = $true
$listInterfaces.GridLines = $true
[void]$listInterfaces.Columns.Add('Usable', 60)
[void]$listInterfaces.Columns.Add('Type', 80)
[void]$listInterfaces.Columns.Add('Name', 300)
[void]$listInterfaces.Columns.Add('IPv4', 120)
[void]$listInterfaces.Columns.Add('Ignore Reason', 180)
$pageInterfaces.Controls.Add($listInterfaces)

$btnRefreshIfaces = New-Object System.Windows.Forms.Button
$btnRefreshIfaces.Text = 'Atualizar'
$btnRefreshIfaces.Left = 900
$btnRefreshIfaces.Top = 560
$btnRefreshIfaces.Width = 110
$pageInterfaces.Controls.Add($btnRefreshIfaces)

$btnUseSelected = New-Object System.Windows.Forms.Button
$btnUseSelected.Text = 'Usar selecionada'
$btnUseSelected.Left = 900
$btnUseSelected.Top = 595
$btnUseSelected.Width = 110
$pageInterfaces.Controls.Add($btnUseSelected)

$btnIgnoreSelected = New-Object System.Windows.Forms.Button
$btnIgnoreSelected.Text = 'Ignorar selecionada'
$btnIgnoreSelected.Left = 900
$btnIgnoreSelected.Top = 630
$btnIgnoreSelected.Width = 110
$pageInterfaces.Controls.Add($btnIgnoreSelected)

$btnRemoveSelected = New-Object System.Windows.Forms.Button
$btnRemoveSelected.Text = 'Remover da lista'
$btnRemoveSelected.Left = 900
$btnRemoveSelected.Top = 665
$btnRemoveSelected.Width = 110
$pageInterfaces.Controls.Add($btnRemoveSelected)

function Get-AgentApiPort {
    $runtimePath = Join-Path (Join-Path $env:LOCALAPPDATA 'MultiSend') 'runtime.json'
    if (Test-Path $runtimePath) {
        try {
            $rt = Get-Content -Raw $runtimePath | ConvertFrom-Json
            if ($rt.local_api_port) { return [int]$rt.local_api_port }
        } catch {}
    }
    $selectedPorts = Get-PropValue -Object $cfg -Name 'selected_ports' -Default $null
    $localApi = Get-PropValue -Object $selectedPorts -Name 'local_api' -Default $null
    if ($null -ne $localApi) { return [int]$localApi }
    $localApiPort = Get-PropValue -Object $cfg -Name 'local_api_port' -Default $null
    if ($null -ne $localApiPort) { return [int]$localApiPort }
    return 56221
}

function Load-Interfaces {
    $listInterfaces.Items.Clear()
    try {
        $port = Get-AgentApiPort
        $uri = "http://127.0.0.1:$port/interfaces"
        $data = Invoke-RestMethod -Uri $uri -Method Get -TimeoutSec 2
        foreach ($iface in $data.interfaces) {
            $item = New-Object System.Windows.Forms.ListViewItem([string]$iface.Usable)
            [void]$item.SubItems.Add([string]$iface.Type)
            [void]$item.SubItems.Add([string]$iface.Name)
            [void]$item.SubItems.Add([string]$iface.IPv4)
            [void]$item.SubItems.Add([string]$iface.IgnoreReason)
            [void]$listInterfaces.Items.Add($item)
        }
    } catch {
        $item = New-Object System.Windows.Forms.ListViewItem("Error")
        [void]$item.SubItems.Add("")
        [void]$item.SubItems.Add("Could not load interfaces")
        [void]$item.SubItems.Add($_.Exception.Message)
        [void]$listInterfaces.Items.Add($item)
    }
}

$btnRefreshIfaces.Add_Click({ Load-Interfaces })
$btnUseSelected.Add_Click({
    try {
        if ($listInterfaces.SelectedItems.Count -eq 0) { throw "Selecione uma interface na lista detectada." }
        $name = [string]$listInterfaces.SelectedItems[0].SubItems[2].Text
        $ip = [string]$listInterfaces.SelectedItems[0].SubItems[3].Text
        $value = if (-not [string]::IsNullOrWhiteSpace($ip)) { $ip } else { $name }
        Add-LineToMultilineBox -Box $txtManual -Value $value
        Remove-LineFromMultilineBox -Box $txtIgnored -Value $value
    } catch {
        [System.Windows.Forms.MessageBox]::Show($_.Exception.Message, 'MultiSend', 'OK', 'Warning') | Out-Null
    }
})
$btnIgnoreSelected.Add_Click({
    try {
        if ($listInterfaces.SelectedItems.Count -eq 0) { throw "Selecione uma interface na lista detectada." }
        $name = [string]$listInterfaces.SelectedItems[0].SubItems[2].Text
        $ip = [string]$listInterfaces.SelectedItems[0].SubItems[3].Text
        $value = if (-not [string]::IsNullOrWhiteSpace($name)) { $name } else { $ip }
        Add-LineToMultilineBox -Box $txtIgnored -Value $value
        Remove-LineFromMultilineBox -Box $txtManual -Value $value
    } catch {
        [System.Windows.Forms.MessageBox]::Show($_.Exception.Message, 'MultiSend', 'OK', 'Warning') | Out-Null
    }
})
$btnRemoveSelected.Add_Click({
    try {
        if ($listInterfaces.SelectedItems.Count -eq 0) { throw "Selecione uma interface na lista detectada." }
        $name = [string]$listInterfaces.SelectedItems[0].SubItems[2].Text
        $ip = [string]$listInterfaces.SelectedItems[0].SubItems[3].Text
        if (-not [string]::IsNullOrWhiteSpace($name)) { Remove-LineFromMultilineBox -Box $txtIgnored -Value $name }
        if (-not [string]::IsNullOrWhiteSpace($ip)) {
            Remove-LineFromMultilineBox -Box $txtIgnored -Value $ip
            Remove-LineFromMultilineBox -Box $txtManual -Value $ip
        }
    } catch {
        [System.Windows.Forms.MessageBox]::Show($_.Exception.Message, 'MultiSend', 'OK', 'Warning') | Out-Null
    }
})

# ---------------------- Segurança ----------------------
$pageSecurity.Controls.Add((New-Label 'Autenticação entre nós' 20 20 220))
$requireAuth = Get-PropValue -Object $cfg -Name 'require_auth' -Default $false
$chkRequireAuth = New-Check 260 16 'Exigir HMAC/token (require_auth)' $requireAuth
$pageSecurity.Controls.Add($chkRequireAuth)
$pageSecurity.Controls.Add((New-Label 'Quando ligado, todos os nós precisam compartilhar o MESMO segredo abaixo. Sem isso, envios entre nós são rejeitados.' 20 46 900))

$pageSecurity.Controls.Add((New-Label 'Segredo do nó (node_secret)' 20 86 220))
$nodeSecretVal = [string](Get-PropValue -Object $cfg -Name 'node_secret' -Default '')
$txtSecret = New-Box 260 82 560 $nodeSecretVal
$txtSecret.UseSystemPasswordChar = $true
$txtSecret.ReadOnly = $true
$pageSecurity.Controls.Add($txtSecret)

$chkShowSecret = New-Object System.Windows.Forms.CheckBox
$chkShowSecret.Left = 260; $chkShowSecret.Top = 108; $chkShowSecret.Width = 120; $chkShowSecret.Text = 'Mostrar'
$pageSecurity.Controls.Add($chkShowSecret)
$chkShowSecret.Add_CheckedChanged({ $txtSecret.UseSystemPasswordChar = -not $chkShowSecret.Checked })

$btnGenSecret = New-Object System.Windows.Forms.Button
$btnGenSecret.Text = 'Gerar novo'; $btnGenSecret.Left = 830; $btnGenSecret.Top = 80; $btnGenSecret.Width = 100
$pageSecurity.Controls.Add($btnGenSecret)
$btnGenSecret.Add_Click({
    $confirm = [System.Windows.Forms.MessageBox]::Show("Gerar novo segredo? Os outros nós precisarão ser atualizados com o mesmo valor.", 'MultiSend', [System.Windows.Forms.MessageBoxButtons]::YesNo, [System.Windows.Forms.MessageBoxIcon]::Warning)
    if ($confirm -eq [System.Windows.Forms.DialogResult]::Yes) { $txtSecret.Text = New-NodeSecret }
})

$btnCopySecret = New-Object System.Windows.Forms.Button
$btnCopySecret.Text = 'Copiar'; $btnCopySecret.Left = 830; $btnCopySecret.Top = 112; $btnCopySecret.Width = 100
$pageSecurity.Controls.Add($btnCopySecret)
$btnCopySecret.Add_Click({
    if ([string]::IsNullOrWhiteSpace($txtSecret.Text)) { return }
    try { [System.Windows.Forms.Clipboard]::SetText($txtSecret.Text); $status.Text = 'Segredo copiado para a área de transferência.' } catch {}
})

$pageSecurity.Controls.Add((New-Label 'Se vazio, um segredo é gerado automaticamente ao salvar. Compartilhe-o por canal seguro com os nós pareados.' 20 142 900))

$pageSecurity.Controls.Add((New-Label 'Raízes de envio permitidas' 20 182 220))
$pageSecurity.Controls.Add((New-Label '(remote_send_roots)' 20 202 220))
$remoteRootsArr = Get-PropValue -Object $cfg -Name 'remote_send_roots' -Default @()
$txtRemoteRoots = New-Box 260 182 660 (Join-List $remoteRootsArr)
$txtRemoteRoots.Multiline = $true; $txtRemoteRoots.Height = 120
$txtRemoteRoots.ScrollBars = 'Vertical'
$pageSecurity.Controls.Add($txtRemoteRoots)
$pageSecurity.Controls.Add((New-Label 'Pastas (1 por linha) que outro PC pode pedir via remote-send. Vazio = apenas a pasta de recebimento. Bloqueia exfiltração de caminhos arbitrários.' 260 308 660))

$pageCleanup.Controls.Add((New-Label 'Limpar chunks concluídos' 20 20 220))
$cleanupChunks = Get-PropValue -Object $cfg -Name 'cleanup_completed_chunks' -Default $false
$chkCleanupChunks = New-Check 260 16 'Ativado' $cleanupChunks
$pageCleanup.Controls.Add($chkCleanupChunks)
$pageCleanup.Controls.Add((New-Label 'Manter manifestos' 20 60 220))
$keepManifests = Get-PropValue -Object $cfg -Name 'keep_manifests' -Default $true
$chkKeepManifests = New-Check 260 56 'Ativado' $keepManifests
$pageCleanup.Controls.Add($chkKeepManifests)
$pageCleanup.Controls.Add((New-Label 'Limpar pastas vazias' 20 100 220))
$cleanupDirs = Get-PropValue -Object $cfg -Name 'cleanup_empty_download_dirs' -Default $false
$chkCleanupDirs = New-Check 260 96 'Ativado' $cleanupDirs
$pageCleanup.Controls.Add($chkCleanupDirs)
$pagePull.Controls.Add((New-Label 'Resultado ao receber pasta' 20 20 220))
$cmbPullFolderResult = New-Object System.Windows.Forms.ComboBox
$cmbPullFolderResult.Left = 260; $cmbPullFolderResult.Top = 16; $cmbPullFolderResult.Width = 160; $cmbPullFolderResult.DropDownStyle = 'DropDownList'
[void]$cmbPullFolderResult.Items.AddRange(@('zip','extract'))
$pullFolderResult = Get-PropValue -Object $cfg -Name 'pull_folder_result' -Default 'zip'
if ($cmbPullFolderResult.Items.Contains($pullFolderResult)) { $cmbPullFolderResult.SelectedItem = $pullFolderResult } else { $cmbPullFolderResult.SelectedItem = 'zip' }
$pagePull.Controls.Add($cmbPullFolderResult)
$pagePull.Controls.Add((New-Label 'zip mantém um arquivo compactado. extract extrai automaticamente após receber.' 260 48 620))

$panel = New-Object System.Windows.Forms.Panel
$panel.Dock = 'Bottom'
$panel.Height = 60
$form.Controls.Add($panel)

$btnBackup = New-Object System.Windows.Forms.Button
$btnBackup.Text = 'Backup'
$btnBackup.Left = 20
$btnBackup.Top = 12
$btnBackup.Width = 100
$panel.Controls.Add($btnBackup)

$btnRestore = New-Object System.Windows.Forms.Button
$btnRestore.Text = 'Restaurar'
$btnRestore.Left = 130
$btnRestore.Top = 12
$btnRestore.Width = 120
$panel.Controls.Add($btnRestore)

$btnDefaults = New-Object System.Windows.Forms.Button
$btnDefaults.Text = 'Padrões'
$btnDefaults.Left = 260
$btnDefaults.Top = 12
$btnDefaults.Width = 80
$panel.Controls.Add($btnDefaults)

$btnRestartAgent = New-Object System.Windows.Forms.Button
$btnRestartAgent.Text = 'Reiniciar Agent'
$btnRestartAgent.Left = 350
$btnRestartAgent.Top = 12
$btnRestartAgent.Width = 120
$panel.Controls.Add($btnRestartAgent)

$btnSave = New-Object System.Windows.Forms.Button
$btnSave.Text = 'Salvar'
$btnSave.Left = 720
$btnSave.Top = 12
$btnSave.Width = 80
$panel.Controls.Add($btnSave)

$btnClose = New-Object System.Windows.Forms.Button
$btnClose.Text = 'Fechar'
$btnClose.Left = 810
$btnClose.Top = 12
$btnClose.Width = 80
$panel.Controls.Add($btnClose)

$status = New-Object System.Windows.Forms.Label
$status.Left = 480
$status.Top = 16
$status.Width = 240
$status.Text = "Configuração: $cfgPath"
$panel.Controls.Add($status)

$btnBackup.Add_Click({
    try {
        $path = Backup-Config
        [System.Windows.Forms.MessageBox]::Show("Backup criado:`r`n$path", 'MultiSend', 'OK', 'Information') | Out-Null
    } catch {
        [System.Windows.Forms.MessageBox]::Show($_.Exception.Message, 'MultiSend', 'OK', 'Error') | Out-Null
    }
})

$btnRestore.Add_Click({
    try {
        $path = Restore-Config
        [System.Windows.Forms.MessageBox]::Show("Restaurado:`r`n$path", 'MultiSend', 'OK', 'Information') | Out-Null
        $form.Close()
    } catch {
        [System.Windows.Forms.MessageBox]::Show($_.Exception.Message, 'MultiSend', 'OK', 'Error') | Out-Null
    }
})

$btnSave.Add_Click({
    try {
        $recv = $txtReceive.Text.Trim()
        if ([string]::IsNullOrWhiteSpace($recv)) { throw 'Pasta de recebimento não pode ficar vazia.' }
        try { [System.IO.Directory]::CreateDirectory($recv) | Out-Null }
        catch { throw "Pasta de recebimento inválida: $recv`r`n$($_.Exception.Message)" }
        $cfg | Add-Member -MemberType NoteProperty -Name 'display_name' -Value $txtDisplay.Text.Trim() -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'receive_path' -Value $recv -Force
        $selectedPorts = Get-PropValue -Object $cfg -Name 'selected_ports' -Default $null
        if ($null -eq $selectedPorts) {
            $cfg | Add-Member -MemberType NoteProperty -Name 'selected_ports' -Value (New-Object PSObject) -Force
        }
        $cfg.selected_ports | Add-Member -MemberType NoteProperty -Name 'local_api' -Value ([int]$numLocal.Value) -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'local_api_port' -Value ([int]$numLocal.Value) -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'start_agent_on_login' -Value $chkAutoStart.Checked -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'interface_policy' -Value ([string]$cmbPolicy.SelectedItem) -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'interface_refresh_seconds' -Value ([int]$numRefresh.Value) -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'allow_new_interfaces_during_transfer' -Value $chkAllowNew.Checked -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'ignore_virtual_interfaces' -Value $chkIgnoreVirtual.Checked -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'ignore_vpn_interfaces' -Value $chkIgnoreVpn.Checked -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'ignore_link_local' -Value $chkIgnoreLink.Checked -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'allowed_interface_types' -Value @(Split-List $txtAllowed.Text) -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'manual_interfaces' -Value @(Split-List $txtManual.Text) -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'ignored_interfaces' -Value @(Split-List $txtIgnored.Text) -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'cleanup_completed_chunks' -Value $chkCleanupChunks.Checked -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'keep_manifests' -Value $chkKeepManifests.Checked -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'cleanup_empty_download_dirs' -Value $chkCleanupDirs.Checked -Force
        $cfg | Add-Member -MemberType NoteProperty -Name 'pull_folder_result' -Value ([string]$cmbPullFolderResult.SelectedItem) -Force

        # --- Segurança ---
        $secret = $txtSecret.Text.Trim()
        if ($chkRequireAuth.Checked -and [string]::IsNullOrWhiteSpace($secret)) {
            $secret = New-NodeSecret
            $txtSecret.Text = $secret
        }
        $cfg | Add-Member -MemberType NoteProperty -Name 'require_auth' -Value $chkRequireAuth.Checked -Force
        if (-not [string]::IsNullOrWhiteSpace($secret)) {
            $cfg | Add-Member -MemberType NoteProperty -Name 'node_secret' -Value $secret -Force
        }
        $roots = @(Split-List $txtRemoteRoots.Text)
        $missing = @($roots | Where-Object { -not (Test-Path -LiteralPath $_) })
        if ($missing.Count -gt 0) {
            $proceed = [System.Windows.Forms.MessageBox]::Show(
                ("Estas raízes de envio não existem:`r`n{0}`r`n`r`nSalvar mesmo assim?" -f ($missing -join "`r`n")),
                'MultiSend', [System.Windows.Forms.MessageBoxButtons]::YesNo, [System.Windows.Forms.MessageBoxIcon]::Warning)
            if ($proceed -ne [System.Windows.Forms.DialogResult]::Yes) { return }
        }
        $cfg | Add-Member -MemberType NoteProperty -Name 'remote_send_roots' -Value $roots -Force

        Backup-Config | Out-Null
        Write-Config $cfg
        [System.Windows.Forms.MessageBox]::Show("Configuração salva.`r`nReinicie o MultiSend Agent para aplicar completamente.", 'MultiSend', 'OK', 'Information') | Out-Null
    } catch {
        [System.Windows.Forms.MessageBox]::Show($_.Exception.Message, 'MultiSend', 'OK', 'Error') | Out-Null
    }
})

$btnDefaults.Add_Click({
    $confirm = [System.Windows.Forms.MessageBox]::Show("Restaurar padrões de interface?", "MultiSend", [System.Windows.Forms.MessageBoxButtons]::YesNo, [System.Windows.Forms.MessageBoxIcon]::Question)
    if ($confirm -eq [System.Windows.Forms.DialogResult]::Yes) {
        $cmbPolicy.SelectedItem = 'auto'
        $numRefresh.Value = 5
        $chkAllowNew.Checked = $true
        $chkIgnoreVirtual.Checked = $true
        $chkIgnoreVpn.Checked = $true
        $chkIgnoreLink.Checked = $true
        $txtAllowed.Text = "ethernet`r`nwifi`r`nusb_ethernet"
        $txtManual.Text = ""
        $txtIgnored.Text = ""
    }
})

$btnRestartAgent.Add_Click({
    try {
        $agentLocal = Join-Path $PSScriptRoot 'multisend-agent.exe'
        $agentInstalled = 'C:\Program Files\MultiSend\bin\multisend-agent.exe'
        $agentExe = if (Test-Path -LiteralPath $agentInstalled) { $agentInstalled } elseif (Test-Path -LiteralPath $agentLocal) { $agentLocal } else { $null }
        if (-not $agentExe) { throw 'multisend-agent.exe nao encontrado (instalado ou local).' }

        $procs = @(Get-Process -Name 'multisend-agent' -ErrorAction SilentlyContinue)
        foreach ($p in $procs) {
            try { Stop-Process -Id $p.Id -Force -ErrorAction Stop } catch {}
        }
        Start-Sleep -Milliseconds 600
        Start-Process -FilePath $agentExe -WindowStyle Hidden | Out-Null
        [System.Windows.Forms.MessageBox]::Show('MultiSend Agent reiniciado com sucesso.', 'MultiSend', 'OK', 'Information') | Out-Null
        Start-Sleep -Milliseconds 800
        Load-Interfaces
    } catch {
        [System.Windows.Forms.MessageBox]::Show("Falha ao reiniciar Agent.`r`n$($_.Exception.Message)", 'MultiSend', 'OK', 'Error') | Out-Null
    }
})

$btnClose.Add_Click({ $form.Close() })
$form.Add_Load({ Load-Interfaces })
[void]$form.ShowDialog()
