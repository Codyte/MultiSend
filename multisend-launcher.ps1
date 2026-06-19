# ====================== BEGIN NAV INDEX ======================
# NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
#   L40    Write-LauncherLog
#   L53    Enter-LauncherCoordinator
#   L64    Exit-LauncherCoordinator
#   L72    Test-AgentApi
#   L82    Get-LocalApiCandidates
#   L112   Resolve-AgentApi
#   L129   Format-Bytes
#   L137   Resolve-InputFilePaths
#   L160   Add-LaunchQueueItem
#   L169   Collect-QueuedPaths
#   L190   New-StagedZip
#   L218   Remove-StagingSafe
#   L230   Open-UnifiedSendMonitor
#   L254   Open-DownloadManager
#   L271   Show-SendConfigDialog
#   L397   Get-TransferPortHint
#   L422   Resolve-ManualPeerAddress
# ======================= END NAV INDEX =======================

param(
    [Parameter(Position=0, ValueFromRemainingArguments=$true)] 
    [string[]]$FilePaths,
    [string]$FilePath
)

$cfgPath = Join-Path $env:APPDATA 'MultiSend\config.json'
$runtimePath = Join-Path $env:LOCALAPPDATA 'MultiSend\runtime.json'
$api = $null
$agentExe = Join-Path $PSScriptRoot 'multisend-agent.exe'
$launcherQueueDir = Join-Path $env:LOCALAPPDATA 'MultiSend\launcher-queue'
$stagingRoot = Join-Path $env:TEMP 'MultiSend\staging'
$script:InvocationLine = $MyInvocation.Line
$launcherLogDir = Join-Path $env:LOCALAPPDATA 'MultiSend\logs'
$launcherLogPath = Join-Path $launcherLogDir 'launcher.log'
$launcherMutexName = 'Global\MultiSendLauncherSingleInstance'
$script:launcherMutex = $null

function Write-LauncherLog {
    param([string]$Message)
    try {
        New-Item -ItemType Directory -Force -Path $launcherLogDir | Out-Null
        $line = "[{0}] {1}" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss.fff'), $Message
        [System.IO.File]::AppendAllText(
            $launcherLogPath,
            $line + [Environment]::NewLine,
            (New-Object System.Text.UTF8Encoding($false))
        )
    } catch {}
}

function Enter-LauncherCoordinator {
    $created = $false
    $m = New-Object System.Threading.Mutex($true, $launcherMutexName, [ref]$created)
    if (-not $created) {
        try { $m.Close() } catch {}
        return $false
    }
    $script:launcherMutex = $m
    return $true
}

function Exit-LauncherCoordinator {
    if ($null -ne $script:launcherMutex) {
        try { $script:launcherMutex.ReleaseMutex() } catch {}
        try { $script:launcherMutex.Close() } catch {}
        $script:launcherMutex = $null
    }
}

function Test-AgentApi {
    param([string]$ApiBase)
    try {
        Invoke-RestMethod -Uri "$ApiBase/health" -Method Get -TimeoutSec 2 | Out-Null
        return $true
    } catch {
        return $false
    }
}

function Get-LocalApiCandidates {
    $ports = New-Object System.Collections.Generic.List[int]
    if (Test-Path $runtimePath) {
        try {
            $rt = Get-Content $runtimePath -Raw | ConvertFrom-Json
            if ($rt.local_api_port) { $ports.Add([int]$rt.local_api_port) }
        } catch {}
    }
    if (Test-Path $cfgPath) {
        try {
            $cfg = Get-Content $cfgPath -Raw | ConvertFrom-Json
            if ($cfg.selected_ports.local_api) { $ports.Add([int]$cfg.selected_ports.local_api) }
            if ($cfg.local_api_port_range.start -and $cfg.local_api_port_range.end) {
                for ($p = [int]$cfg.local_api_port_range.start; $p -le [int]$cfg.local_api_port_range.end; $p++) { $ports.Add($p) }
            } elseif ($cfg.local_api_port) {
                $ports.Add([int]$cfg.local_api_port)
            }
        } catch {}
    }
    if ($ports.Count -eq 0) {
        for ($p = 56221; $p -le 56230; $p++) { $ports.Add($p) }
    }
    $seen = @{}
    $out = @()
    foreach ($p in $ports) {
        if (-not $seen.ContainsKey($p)) { $seen[$p] = $true; $out += $p }
    }
    return ,$out
}

function Resolve-AgentApi {
    $candidates = Get-LocalApiCandidates
    foreach ($p in $candidates) {
        $candidate = "http://127.0.0.1:$p"
        if (Test-AgentApi -ApiBase $candidate) {
            try {
                $payload = @{ pid = $null; local_api_port = $p; transfer_port = $null; discovery_port = $null; started_at = (Get-Date).ToString('o') } | ConvertTo-Json
                $dir = Split-Path $runtimePath -Parent
                New-Item -ItemType Directory -Force -Path $dir | Out-Null
                [System.IO.File]::WriteAllText($runtimePath, $payload, (New-Object System.Text.UTF8Encoding($false)))
            } catch {}
            return $candidate
        }
    }
    return $null
}

function Format-Bytes {
    param([long]$n)
    if ($n -lt 1KB) { return "$n B" }
    if ($n -lt 1MB) { return ('{0:N1} KB' -f ($n / 1KB)) }
    if ($n -lt 1GB) { return ('{0:N1} MB' -f ($n / 1MB)) }
    return ('{0:N2} GB' -f ($n / 1GB))
}

function Resolve-InputFilePaths {
    $out = @()
    # Captura tanto o array FilePaths quanto o parâmetro único FilePath
    $inputs = @($FilePaths)
    if (-not [string]::IsNullOrWhiteSpace($FilePath)) { $inputs += $FilePath }

    foreach ($p in $inputs) {
        if ([string]::IsNullOrWhiteSpace($p)) { continue }
        
        # Correção crucial: Se o Windows passar "C", ":", "\", o PowerShell 
        # as vezes fragmenta. Vamos limpar espaços e quotes, mas NÃO descartar.
        $cleanPath = $p.Trim().Trim('"')
        
        # Se for um caminho absoluto válido, usamos.
        if (Test-Path -LiteralPath $cleanPath) {
            $out += (Get-Item -LiteralPath $cleanPath).FullName
        } else {
            Write-LauncherLog "Aviso: Caminho inválido ou inacessível: $cleanPath"
        }
    }
    return ,$out
}

function Add-LaunchQueueItem {
    param([string[]]$Paths)
    New-Item -ItemType Directory -Force -Path $launcherQueueDir | Out-Null
    $id = [Guid]::NewGuid().ToString('N')
    $p = Join-Path $launcherQueueDir ("pending-" + $id + ".json")
    $payload = [ordered]@{ created_at = (Get-Date).ToString('o'); paths = @($Paths) }
    [System.IO.File]::WriteAllText($p, ($payload | ConvertTo-Json -Depth 5), (New-Object System.Text.UTF8Encoding($false)))
}

function Collect-QueuedPaths {
    $out = @()
    $seen = @{}
    if (-not (Test-Path -LiteralPath $launcherQueueDir)) { return $out }
    $pending = Get-ChildItem -LiteralPath $launcherQueueDir -Filter 'pending-*.json' -File -ErrorAction SilentlyContinue
    foreach ($f in $pending) {
        try {
            $obj = Get-Content -LiteralPath $f.FullName -Raw | ConvertFrom-Json
            foreach ($p in @($obj.paths)) {
                if ([string]::IsNullOrWhiteSpace($p)) { continue }
                if (-not (Test-Path -LiteralPath $p -PathType Leaf)) { continue }
                if (-not $seen.ContainsKey($p)) { $seen[$p] = $true; $out += $p }
            }
        } catch {
        } finally {
            Remove-Item -LiteralPath $f.FullName -Force -ErrorAction SilentlyContinue
        }
    }
    return ,$out
}

function New-StagedZip {
    param([string[]]$Paths)
    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    New-Item -ItemType Directory -Force -Path $stagingRoot | Out-Null
    $session = Join-Path $stagingRoot ("batch-" + (Get-Date -Format "yyyyMMdd-HHmmss") + "-" + [Guid]::NewGuid().ToString("N").Substring(0,8))
    New-Item -ItemType Directory -Force -Path $session | Out-Null
    $zipPath = Join-Path $session ("multisend-batch-" + (Get-Date -Format "yyyyMMdd-HHmmss") + ".zip")
    $fs = [System.IO.File]::Open($zipPath, [System.IO.FileMode]::CreateNew)
    try {
        $zip = New-Object System.IO.Compression.ZipArchive($fs, [System.IO.Compression.ZipArchiveMode]::Create, $false)
        try {
            $nameCount = @{}
            foreach ($src in $Paths) {
                $leaf = [System.IO.Path]::GetFileName($src)
                if ($nameCount.ContainsKey($leaf)) {
                    $nameCount[$leaf] = [int]$nameCount[$leaf] + 1
                    $base = [System.IO.Path]::GetFileNameWithoutExtension($leaf)
                    $ext = [System.IO.Path]::GetExtension($leaf)
                    $leaf = "{0} ({1}){2}" -f $base, $nameCount[$leaf], $ext
                } else { $nameCount[$leaf] = 0 }
                [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $src, $leaf, [System.IO.Compression.CompressionLevel]::Optimal) | Out-Null
            }
        } finally { $zip.Dispose() }
    } finally { $fs.Dispose() }
    return [pscustomobject]@{ ZipPath = $zipPath; SessionDir = $session }
}

function Remove-StagingSafe {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return $true }
    for ($i = 0; $i -lt 5; $i++) {
        try {
            if (Test-Path -LiteralPath $Path) { Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction Stop }
            return $true
        } catch { Start-Sleep -Milliseconds 300 }
    }
    return $false
}

function Open-UnifiedSendMonitor {
    param([Parameter(Mandatory=$true)][string]$JobId)
    $ui = Join-Path $PSScriptRoot 'multisend-download-ui.ps1'
    if (-not (Test-Path -LiteralPath $ui)) {
        Write-Host "UI not found: $ui"
        return $false
    }
    try {
        Write-LauncherLog "Open-UnifiedSendMonitor job_id=$JobId"
        Start-Process -FilePath 'powershell.exe' -ArgumentList @(
            '-NoExit',
            '-NoProfile',
            '-STA',
            '-ExecutionPolicy','Bypass',
            '-File', "`"$ui`"",
            '-SendJobId', $JobId
        ) -WindowStyle Normal | Out-Null
        return $true
    } catch {
        Write-Host ("Could not open unified monitor UI: {0}" -f $_.Exception.Message)
        return $false
    }
}

function Open-DownloadManager {
    $ui = Join-Path $PSScriptRoot 'multisend-download-ui.ps1'
    if (-not (Test-Path -LiteralPath $ui)) { return $false }
    try {
        Write-LauncherLog "Open-DownloadManager"
        Start-Process -FilePath 'powershell.exe' -ArgumentList @(
            '-NoProfile',
            '-STA',
            '-ExecutionPolicy','Bypass',
            '-File', $ui
        ) -WindowStyle Normal | Out-Null
        return $true
    } catch {
        return $false
    }
}

function Show-SendConfigDialog {
    param(
        [Parameter(Mandatory=$true)][string]$FilePathToSend,
        [Parameter(Mandatory=$true)]
        [AllowEmptyCollection()]
        [array]$Peers
    )
    Add-Type -AssemblyName System.Windows.Forms
    Add-Type -AssemblyName System.Drawing

    $form = New-Object System.Windows.Forms.Form
    $form.Text = 'MultiSend - Enviar arquivo'
    $form.Width = 820
    $form.Height = 300
    $form.StartPosition = 'CenterScreen'
    $form.FormBorderStyle = 'FixedDialog'
    $form.MaximizeBox = $false
    $form.MinimizeBox = $false

    $lblFile = New-Object System.Windows.Forms.Label
    $lblFile.Text = 'Arquivo'
    $lblFile.Left = 15; $lblFile.Top = 18; $lblFile.Width = 80
    $form.Controls.Add($lblFile)

    $txtFile = New-Object System.Windows.Forms.TextBox
    $txtFile.Left = 95; $txtFile.Top = 14; $txtFile.Width = 690
    $txtFile.ReadOnly = $false
    $txtFile.Text = $FilePathToSend
    $form.Controls.Add($txtFile)

    $lblPeer = New-Object System.Windows.Forms.Label
    $lblPeer.Text = 'Destino detectado'
    $lblPeer.Left = 15; $lblPeer.Top = 56; $lblPeer.Width = 120
    $form.Controls.Add($lblPeer)

    $cmbPeers = New-Object System.Windows.Forms.ComboBox
    $cmbPeers.Left = 140; $cmbPeers.Top = 52; $cmbPeers.Width = 645
    $cmbPeers.DropDownStyle = 'DropDownList'
    $peerMap = @{}
    foreach ($p in $Peers) {
        $label = "{0} ({1}) [{2}]" -f [string]$p.name, [string]$p.addr, [string]$p.node_id
        [void]$cmbPeers.Items.Add($label)
        $peerMap[$label] = [string]$p.node_id
    }
    if ($cmbPeers.Items.Count -gt 0) { $cmbPeers.SelectedIndex = 0 }
    $form.Controls.Add($cmbPeers)

    $lblManual = New-Object System.Windows.Forms.Label
    $lblManual.Text = 'Ou IP:porta manual'
    $lblManual.Left = 15; $lblManual.Top = 92; $lblManual.Width = 120
    $form.Controls.Add($lblManual)

    $txtManual = New-Object System.Windows.Forms.TextBox
    $txtManual.Left = 140; $txtManual.Top = 88; $txtManual.Width = 645
    $txtManual.Text = ''
    $form.Controls.Add($txtManual)

    $lblHelp = New-Object System.Windows.Forms.Label
    $lblHelp.Text = 'Se preencher IP:porta manual, ele tera prioridade sobre destino detectado.'
    $lblHelp.Left = 140; $lblHelp.Top = 116; $lblHelp.Width = 645
    $form.Controls.Add($lblHelp)

    $btnSend = New-Object System.Windows.Forms.Button
    $btnSend.Text = 'Enviar'
    $btnSend.Left = 590; $btnSend.Top = 180; $btnSend.Width = 95
    $form.Controls.Add($btnSend)

    $btnCancel = New-Object System.Windows.Forms.Button
    $btnCancel.Text = 'Cancelar'
    $btnCancel.Left = 690; $btnCancel.Top = 180; $btnCancel.Width = 95
    $form.Controls.Add($btnCancel)

    $result = [pscustomobject]@{
        confirmed    = $false
        peer_node_id = $null
        peer_address = $null
        file_path    = $null
    }

    $btnCancel.Add_Click({
        $form.DialogResult = [System.Windows.Forms.DialogResult]::Cancel
        $form.Close()
    })

    $btnSend.Add_Click({
        $manual = [string]$txtManual.Text
        if (-not [string]::IsNullOrWhiteSpace($manual)) {
            $result.confirmed = $true
            $result.peer_address = $manual.Trim()
            $result.file_path = $txtFile.Text
            $form.DialogResult = [System.Windows.Forms.DialogResult]::OK
            $form.Close()
            return
        }

        if ($cmbPeers.SelectedIndex -lt 0) {
            [System.Windows.Forms.MessageBox]::Show(
                'Selecione um destino detectado ou preencha IP:porta manual.',
                'MultiSend',
                [System.Windows.Forms.MessageBoxButtons]::OK,
                [System.Windows.Forms.MessageBoxIcon]::Warning
            ) | Out-Null
            return
        }
        $label = [string]$cmbPeers.SelectedItem
        $nid = $peerMap[$label]
        if ([string]::IsNullOrWhiteSpace($nid)) {
            [System.Windows.Forms.MessageBox]::Show(
                'Destino selecionado sem node_id válido.',
                'MultiSend',
                [System.Windows.Forms.MessageBoxButtons]::OK,
                [System.Windows.Forms.MessageBoxIcon]::Warning
            ) | Out-Null
            return
        }
        $result.confirmed = $true
        $result.peer_node_id = $nid
        $result.file_path = $txtFile.Text
        $form.DialogResult = [System.Windows.Forms.DialogResult]::OK
        $form.Close()
    })

    [void]$form.ShowDialog()
    return $result
}

function Get-TransferPortHint {
    param([array]$Peers)
    foreach ($p in @($Peers)) {
        $addr = [string]$p.addr
        if (-not [string]::IsNullOrWhiteSpace($addr) -and $addr.Contains(':')) {
            $parts = $addr.Split(':')
            if ($parts.Count -ge 2) {
                $portText = $parts[$parts.Count - 1]
                [int]$port = 0
                if ([int]::TryParse($portText, [ref]$port) -and $port -ge 1 -and $port -le 65535) {
                    return $port
                }
            }
        }
    }
    try {
        if (Test-Path -LiteralPath $cfgPath) {
            $cfg = Get-Content -LiteralPath $cfgPath -Raw | ConvertFrom-Json
            if ($cfg.selected_ports.transfer) { return [int]$cfg.selected_ports.transfer }
            if ($cfg.transfer_port) { return [int]$cfg.transfer_port }
        }
    } catch {}
    return 56200
}

function Resolve-ManualPeerAddress {
    param(
        [string]$ManualText,
        [array]$Peers
    )
    $v = [string]$ManualText
    if ([string]::IsNullOrWhiteSpace($v)) { return $null }
    $v = $v.Trim()
    if ($v.Contains(':')) { return $v }
    $port = Get-TransferPortHint -Peers $Peers
    return ('{0}:{1}' -f $v, $port)
}

Write-LauncherLog ("Launcher start file_path={0} file_paths_count={1} line={2}" -f $FilePath, @($FilePaths).Count, $script:InvocationLine)

if (-not (Enter-LauncherCoordinator)) {
    $initialOnly = Resolve-InputFilePaths
    if ($initialOnly.Count -gt 0) {
        Add-LaunchQueueItem -Paths $initialOnly
        Write-LauncherLog ("Secondary instance queued paths count={0} and exited" -f $initialOnly.Count)
    } else {
        Write-LauncherLog "Secondary instance had no valid paths and exited"
    }
    exit 0
}

$api = Resolve-AgentApi
if (-not $api) {
    Add-Type -AssemblyName System.Windows.Forms
    $msg = "MultiSend Agent não está em execução.`n`nDeseja iniciar agora?"
    $title = 'MultiSend'
    $choice = [System.Windows.Forms.MessageBox]::Show($msg,$title,[System.Windows.Forms.MessageBoxButtons]::YesNo,[System.Windows.Forms.MessageBoxIcon]::Question,[System.Windows.Forms.MessageBoxDefaultButton]::Button1)
    if ($choice -ne [System.Windows.Forms.DialogResult]::Yes) { Write-Host 'Envio cancelado: MultiSend Agent não foi iniciado.'; Exit-LauncherCoordinator; exit 1 }
    if (-not (Test-Path -LiteralPath $agentExe)) { Write-LauncherLog "Agent binary not found: $agentExe"; Write-Host "MultiSend Agent binary not found: $agentExe"; Exit-LauncherCoordinator; exit 1 }
    $running = Get-Process multisend-agent -ErrorAction SilentlyContinue
    if (-not $running) {
        try { Start-Process -FilePath $agentExe -WindowStyle Hidden | Out-Null; Write-LauncherLog "Agent start requested" } catch { Write-LauncherLog "Failed to start agent: $($_.Exception.Message)"; Write-Host "Failed to start MultiSend Agent: $($_.Exception.Message)"; Exit-LauncherCoordinator; exit 1 }
    }
    $ready = $false
    for ($i = 0; $i -lt 12; $i++) { Start-Sleep -Milliseconds 500; $api = Resolve-AgentApi; if ($api) { $ready = $true; break } }
    if (-not $ready) { Write-LauncherLog "Agent API not responding after start attempt"; Write-Host 'MultiSend Agent is not responding on local API range after start attempt.'; Exit-LauncherCoordinator; exit 1 }
}

$initialPaths = Resolve-InputFilePaths
if ($initialPaths.Count -eq 0) { Write-LauncherLog "No valid files were provided"; Exit-LauncherCoordinator; exit 1 }
Add-LaunchQueueItem -Paths $initialPaths
Start-Sleep -Milliseconds 350
$selectedPaths = Collect-QueuedPaths
if ($selectedPaths.Count -eq 0) { $selectedPaths = $initialPaths }
Write-LauncherLog ("Resolved consolidated paths count={0}" -f $selectedPaths.Count)

$sendFilePath = $null
$stagingSessionDir = $null
$cleanupPending = $false
if ($selectedPaths.Count -eq 0) {
    Exit-LauncherCoordinator
    exit 1
}
if ($selectedPaths.Count -eq 1) {
    $sendFilePath = $selectedPaths[0]
    Write-LauncherLog ("Single file selected: {0}" -f $sendFilePath)
} else {
    Write-LauncherLog ("Packing files count={0}" -f $selectedPaths.Count)
    $bundle = New-StagedZip -Paths $selectedPaths
    $sendFilePath = $bundle.ZipPath
    $stagingSessionDir = $bundle.SessionDir
    $cleanupPending = $true
    $zipSize = (Get-Item -LiteralPath $sendFilePath).Length
    Write-LauncherLog ("Prepared staged zip: {0} size={1}" -f $sendFilePath, (Format-Bytes $zipSize))
}

$peers = @()
for ($i = 0; $i -lt 6; $i++) {
    try {
        $res = Invoke-RestMethod -Uri "$api/peers" -Method Get -TimeoutSec 2
        if ($res) { $peers = @($res) }
        if ($peers.Count -gt 0) { break }
    } catch {}
    if ($i -eq 0) { Write-Host 'Searching cached MultiSend peers on local network...' }
    Start-Sleep -Milliseconds 500
}

if ($peers.Count -eq 0) {
    Write-LauncherLog "No peers found - opening config with manual mode only"
}

Write-LauncherLog ("Opening send config dialog peers_count={0}" -f $peers.Count)
$cfgResult = Show-SendConfigDialog -FilePathToSend $sendFilePath -Peers $peers
if (-not $cfgResult.confirmed) {
    Write-LauncherLog "Send canceled by user in config dialog"
    if ($cleanupPending) {
        $clean = Remove-StagingSafe -Path $stagingSessionDir
        Write-LauncherLog ("Cleanup staging: {0}" -f ($(if($clean){'ok'}else{'failed'})))
    }
    Exit-LauncherCoordinator
    exit 0
}

$payload = @{ file_path = $cfgResult.file_path }
if (-not [string]::IsNullOrWhiteSpace($cfgResult.peer_address)) {
    $resolvedPeerAddress = Resolve-ManualPeerAddress -ManualText $cfgResult.peer_address -Peers $peers
    $payload.peer_address = $resolvedPeerAddress
    Write-LauncherLog ("Send confirmed with manual peer_address={0}" -f $resolvedPeerAddress)
} elseif (-not [string]::IsNullOrWhiteSpace($cfgResult.peer_node_id)) {
    $payload.peer_node_id = $cfgResult.peer_node_id
    Write-LauncherLog ("Send confirmed with peer_node_id={0}" -f $cfgResult.peer_node_id)
} else {
    Write-LauncherLog "Config confirmed but missing destination"
    if ($cleanupPending) {
        $clean = Remove-StagingSafe -Path $stagingSessionDir
        Write-LauncherLog ("Cleanup staging: {0}" -f ($(if($clean){'ok'}else{'failed'})))
    }
    Exit-LauncherCoordinator
    exit 1
}

$sendFilePath = $cfgResult.file_path

$body = $payload | ConvertTo-Json
$bytes = [System.Text.Encoding]::UTF8.GetBytes($body)
try {
    Write-LauncherLog ("Sending file_path={0}" -f $sendFilePath)
    $res = Invoke-RestMethod -Uri "$api/send" -Method Post -ContentType 'application/json; charset=utf-8' -Body $bytes -ErrorAction Stop
    if (-not $res.job_id) { Write-LauncherLog "No job_id returned by /send"; Write-Host 'Transfer request returned no job ID.'; $null = Open-DownloadManager; Exit-LauncherCoordinator; exit 1 }
    Write-Host ("Transfer started. Job ID: {0}" -f $res.job_id)
    Write-LauncherLog ("Transfer started job_id={0}" -f $res.job_id)
    $ok = Open-UnifiedSendMonitor -JobId $res.job_id
    if ($cleanupPending) {
        $clean = Remove-StagingSafe -Path $stagingSessionDir
        Write-LauncherLog ("Cleanup staging: {0}" -f ($(if($clean){'ok'}else{'failed'})))
    }
    if (-not $ok) { Write-LauncherLog "Failed to open unified send monitor; fallback to download manager"; $null = Open-DownloadManager; Exit-LauncherCoordinator; exit 1 }
} catch {
    $errMsg = $_.Exception.Message
    if ($_.ErrorDetails -and $_.ErrorDetails.Message) {
        $errMsg += " - " + $_.ErrorDetails.Message
    } elseif ($_.Exception.Response) {
        try {
            $stream = $_.Exception.Response.GetResponseStream()
            if ($stream.CanSeek) { $stream.Position = 0 }
            $reader = New-Object System.IO.StreamReader($stream)
            $errMsg += " - " + $reader.ReadToEnd()
        } catch {}
    }
    Write-LauncherLog ("Send failed: {0}" -f $errMsg)
    Write-Host ("Failed to start transfer: {0}" -f $errMsg)
    $null = Open-DownloadManager
    if ($cleanupPending) {
        $clean = Remove-StagingSafe -Path $stagingSessionDir
        Write-LauncherLog ("Cleanup staging: {0}" -f ($(if($clean){'ok'}else{'failed'})))
    }
    Exit-LauncherCoordinator
    exit 1
}

Exit-LauncherCoordinator
exit 0

