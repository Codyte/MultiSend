# ====================== BEGIN NAV INDEX ======================
# NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
#   L39    Write-UiLog
#   L47    Test-ApiHealth
#   L57    Resolve-ApiBase
#   L103   Invoke-ApiJson
#   L170   Normalize-ProtocolUrl
#   L193   Test-DownloadUrl
#   L204   Test-PullSourceUrl
#   L213   Normalize-DownloadUrlInput
#   L228   Get-OptionalPropValue
#   L241   Get-InitialDownloadUrl
#   L248   Get-DetectedSourceMode
#   L479   Get-SelectedSourceMode
#   L488   Get-EffectiveSourceMode
#   L497   Resolve-SourceMode
#   L513   Set-SourceModeVisual
#   L527   Get-SelectedJobId
#   L538   Set-UiState
#   L634   Update-ActionButtons
#   L685   Update-DownloadList
#   L724   Get-SelectedJobIds
#   L733   Refresh-SelectedJob
#   L815   Refresh-SendJob
# ======================= END NAV INDEX =======================

param(
    [string]$ProtocolUrl,
    [string]$SendJobId
)

$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$uiLogDir = Join-Path $env:LOCALAPPDATA 'MultiSend\logs'
$uiLogPath = Join-Path $uiLogDir 'download-ui.log'
function Write-UiLog {
    param([string]$Message)
    try {
        New-Item -ItemType Directory -Force -Path $uiLogDir | Out-Null
        Add-Content -LiteralPath $uiLogPath -Value ("[{0}] {1}" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss.fff'), $Message) -Encoding UTF8
    } catch {}
}

function Test-ApiHealth {
    param([string]$ApiBase)
    try {
        $h = Invoke-RestMethod -Method Get -Uri "$ApiBase/health" -TimeoutSec 2
        return [bool]$h.ok
    } catch {
        return $false
    }
}

function Resolve-ApiBase {
    $ports = New-Object System.Collections.Generic.List[int]
    $runtimePath = Join-Path (Join-Path $env:LOCALAPPDATA 'MultiSend') 'runtime.json'
    $cfgPath = Join-Path $env:APPDATA 'MultiSend\config.json'

    if (Test-Path $runtimePath) {
        try {
            $rt = Get-Content -Raw $runtimePath | ConvertFrom-Json
            if ($rt.local_api_port) { $ports.Add([int]$rt.local_api_port) }
        } catch {}
    }
    if (Test-Path $cfgPath) {
        try {
            $cfg = Get-Content -Raw $cfgPath | ConvertFrom-Json
            if ($cfg.selected_ports.local_api) { $ports.Add([int]$cfg.selected_ports.local_api) }
            elseif ($cfg.local_api_port) { $ports.Add([int]$cfg.local_api_port) }
            if ($cfg.local_api_port_range.start -and $cfg.local_api_port_range.end) {
                for ($p = [int]$cfg.local_api_port_range.start; $p -le [int]$cfg.local_api_port_range.end; $p++) { $ports.Add($p) }
            }
        } catch {}
    }
    if ($ports.Count -eq 0) {
        for ($p = 56221; $p -le 56230; $p++) { $ports.Add($p) }
    }

    $seen = @{}
    foreach ($p in $ports) {
        if ($seen.ContainsKey($p)) { continue }
        $seen[$p] = $true
        $api = "http://127.0.0.1:$p"
        if (Test-ApiHealth -ApiBase $api) { return $api }
    }

    $agentExe = Join-Path $PSScriptRoot 'multisend-agent.exe'
    if (Test-Path -LiteralPath $agentExe) {
        Start-Process -FilePath $agentExe -WindowStyle Hidden | Out-Null
        Start-Sleep -Milliseconds 1200
        foreach ($p in $ports) {
            $api = "http://127.0.0.1:$p"
            if (Test-ApiHealth -ApiBase $api) { return $api }
        }
    }

    throw 'MultiSend Agent não está respondendo na API local (56221-56230). Inicie o Agent e tente novamente.'
}

function Invoke-ApiJson {
    param(
        [Parameter(Mandatory = $true)] [ValidateSet('GET', 'POST')] [string] $Method,
        [Parameter(Mandatory = $true)] [string] $Uri,
        [Parameter()] $Body
    )
    Write-UiLog ("API {0} {1}" -f $Method, $Uri)
    $req = [System.Net.WebRequest]::Create($Uri)
    $req.Method = $Method
    $req.Timeout = 30000
    if ($Method -eq 'POST' -and $null -ne $Body) {
        $req.ContentType = 'application/json; charset=utf-8'
        $json = $Body | ConvertTo-Json -Depth 8
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $req.ContentLength = $bytes.Length
        $stream = $req.GetRequestStream()
        $stream.Write($bytes, 0, $bytes.Length)
        $stream.Close()
    }
    try {
        $res = $req.GetResponse()
    } catch [System.Net.WebException] {
        $resp = $_.Exception.Response
        $statusCode = 0
        $body = ''
        if ($resp) {
            try { $statusCode = [int]$resp.StatusCode } catch {}
            try {
                $errStream = $resp.GetResponseStream()
                if ($errStream) {
                    $errReader = New-Object System.IO.StreamReader($errStream, [System.Text.Encoding]::UTF8)
                    $body = $errReader.ReadToEnd()
                    $errReader.Close()
                }
                $resp.Close()
            } catch {}
        }
        $msg = $_.Exception.Message
        if (-not [string]::IsNullOrWhiteSpace($body)) {
            try {
                $apiErr = $body | ConvertFrom-Json
                $parts = New-Object System.Collections.Generic.List[string]
                if ($statusCode -gt 0) { $parts.Add("HTTP $statusCode") }
                if ($apiErr.error) { $parts.Add("code=$($apiErr.error)") }
                if ($apiErr.message) { $parts.Add([string]$apiErr.message) }
                if ($apiErr.detail) { $parts.Add("detail=$($apiErr.detail)") }
                if ($apiErr.request_id) { $parts.Add("request_id=$($apiErr.request_id)") }
                if ($parts.Count -gt 0) { $msg = ($parts -join ' | ') }
            } catch {
                if ($statusCode -gt 0) { $msg = "HTTP $statusCode | $body" } else { $msg = $body }
            }
        }
        Write-UiLog ("API ERROR {0} {1} -> {2}" -f $Method, $Uri, $msg)
        throw $msg
    }
    $resStream = $res.GetResponseStream()
    $reader = New-Object System.IO.StreamReader($resStream, [System.Text.Encoding]::UTF8)
    $resText = $reader.ReadToEnd()
    $reader.Close()
    $res.Close()
    Write-UiLog ("API OK {0} {1}" -f $Method, $Uri)
    if (-not [string]::IsNullOrWhiteSpace($resText)) {
        return ($resText | ConvertFrom-Json)
    }
    return $null
}

function Normalize-ProtocolUrl {
    param([string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) { return $null }
    $decoded = $Value.Trim()
    if ($decoded.StartsWith('multisend:', [System.StringComparison]::OrdinalIgnoreCase)) {
        $decoded = $decoded.Substring('multisend:'.Length)
    }
    if ($decoded.StartsWith('//')) { $decoded = $decoded.Substring(2) }
    if ($decoded.StartsWith('download?', [System.StringComparison]::OrdinalIgnoreCase)) {
        $query = $decoded.Substring('download?'.Length)
        foreach ($part in $query -split '&') {
            if ($part -match '^(?i)url=(.*)$') {
                $decoded = $Matches[1]
                break
            }
        }
    }
    try {
        $decoded = [System.Uri]::UnescapeDataString($decoded)
    } catch {}
    return $decoded.Trim()
}

function Test-DownloadUrl {
    param([string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) { return $false }
    $uri = $null
    if (-not [System.Uri]::TryCreate($Value, [System.UriKind]::Absolute, [ref]$uri)) {
        return $false
    }
    if ($uri.Scheme -notin @('http','https')) { return $false }
    return $true
}

function Test-PullSourceUrl {
    param([string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) { return $false }
    $v = $Value.Trim()
    if ($v.StartsWith('file://', [System.StringComparison]::OrdinalIgnoreCase)) { return $true }
    if ($v.StartsWith('\\')) { return $true }
    return $false
}

function Normalize-DownloadUrlInput {
    param([string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) { return $Value }
    $v = $Value.Trim()
    $u = $null
    if (-not [System.Uri]::TryCreate($v, [System.UriKind]::Absolute, [ref]$u)) {
        return $v
    }
    $ub = New-Object System.UriBuilder($u)
    if ($ub.Path -and $ub.Path -ne '/') {
        $ub.Path = $ub.Path.TrimEnd('/')
    }
    return $ub.Uri.AbsoluteUri
}

function Get-OptionalPropValue {
    param(
        [Parameter(Mandatory = $false)] $Obj,
        [Parameter(Mandatory = $true)] [string] $Name,
        [Parameter(Mandatory = $false)] $Default = $null
    )
    if ($null -eq $Obj) { return $Default }
    $prop = $Obj.PSObject.Properties[$Name]
    if ($null -eq $prop) { return $Default }
    if ($null -eq $prop.Value) { return $Default }
    return $prop.Value
}

function Get-InitialDownloadUrl {
    $default = ''
    $candidate = Normalize-ProtocolUrl $ProtocolUrl
    if ($null -eq $candidate) { return $default }
    return $candidate
}

function Get-DetectedSourceMode {
    param([string]$Value)
    if (Test-DownloadUrl $Value) { return 'download' }
    if (Test-PullSourceUrl $Value) { return 'pull' }
    return 'unknown'
}

$defaultOutput = Join-Path $env:USERPROFILE 'Downloads\MultiSend\Downloads'
[System.IO.Directory]::CreateDirectory($defaultOutput) | Out-Null

$apiBase = $null
try {
    Write-UiLog ("UI start protocol={0} send_job_id={1}" -f $ProtocolUrl, $SendJobId)
    $apiBase = Resolve-ApiBase
    $health = Invoke-ApiJson -Method GET -Uri "$apiBase/health"
    if (-not $health.ok) {
        throw "Agent health check failed."
    }
    Write-UiLog ("API ready: {0}" -f $apiBase)
    if ($health.agent_log_path) {
        Write-UiLog ("Agent log: {0}" -f $health.agent_log_path)
    }
} catch {
    $msg = "Falha ao iniciar MultiSend Transfer Manager.`r`n$($_.Exception.Message)`r`n`r`nLog: $uiLogPath"
    Write-UiLog ("Startup error: {0}" -f $_.Exception.Message)
    [System.Windows.Forms.MessageBox]::Show($msg, 'MultiSend', [System.Windows.Forms.MessageBoxButtons]::OK, [System.Windows.Forms.MessageBoxIcon]::Error) | Out-Null
    exit 1
}

$form = New-Object System.Windows.Forms.Form
$form.Text = 'MultiSend Transfer Manager'
$form.Width = 1060
$form.Height = 680
$form.StartPosition = 'CenterScreen'

$lblMode = New-Object System.Windows.Forms.Label
$lblMode.Text = 'Tipo'
$lblMode.Left = 15
$lblMode.Top = 18
$lblMode.Width = 120
$form.Controls.Add($lblMode)

$cmbMode = New-Object System.Windows.Forms.ComboBox
$cmbMode.Left = 15
$cmbMode.Top = 38
$cmbMode.Width = 180
$cmbMode.DropDownStyle = 'DropDownList'
[void]$cmbMode.Items.AddRange(@('Auto','Download internet','Receber de outro PC'))
$initialMode = Get-DetectedSourceMode (Get-InitialDownloadUrl)
if ($initialMode -eq 'download') {
    $cmbMode.SelectedItem = 'Download internet'
} elseif ($initialMode -eq 'pull') {
    $cmbMode.SelectedItem = 'Receber de outro PC'
} else {
    $cmbMode.SelectedItem = 'Auto'
}
$form.Controls.Add($cmbMode)

$lblUrl = New-Object System.Windows.Forms.Label
$lblUrl.Text = 'Origem'
$lblUrl.Left = 210
$lblUrl.Top = 18
$lblUrl.Width = 260
$form.Controls.Add($lblUrl)

$txtUrl = New-Object System.Windows.Forms.TextBox
$txtUrl.Left = 210
$txtUrl.Top = 38
$txtUrl.Width = 815
$txtUrl.Text = Get-InitialDownloadUrl
$form.Controls.Add($txtUrl)

$lblOut = New-Object System.Windows.Forms.Label
$lblOut.Text = 'Destino'
$lblOut.Left = 15
$lblOut.Top = 72
$lblOut.Width = 120
$form.Controls.Add($lblOut)

$txtOut = New-Object System.Windows.Forms.TextBox
$txtOut.Left = 15
$txtOut.Top = 92
$txtOut.Width = 1010
$txtOut.Text = $defaultOutput
$form.Controls.Add($txtOut)

$btnStart = New-Object System.Windows.Forms.Button
$btnStart.Text = 'Iniciar'
$btnStart.Left = 15
$btnStart.Top = 130
$btnStart.Width = 110
$form.Controls.Add($btnStart)

$btnRefresh = New-Object System.Windows.Forms.Button
$btnRefresh.Text = 'Atualizar'
$btnRefresh.Left = 135
$btnRefresh.Top = 130
$btnRefresh.Width = 110
$form.Controls.Add($btnRefresh)

$btnCancel = New-Object System.Windows.Forms.Button
$btnCancel.Text = 'Cancelar'
$btnCancel.Left = 255
$btnCancel.Top = 130
$btnCancel.Width = 100
$btnCancel.Enabled = $false
$form.Controls.Add($btnCancel)

$btnPause = New-Object System.Windows.Forms.Button
$btnPause.Text = 'Pausar'
$btnPause.Left = 365
$btnPause.Top = 130
$btnPause.Width = 100
$btnPause.Enabled = $false
$form.Controls.Add($btnPause)

$btnResume = New-Object System.Windows.Forms.Button
$btnResume.Text = 'Retomar'
$btnResume.Left = 475
$btnResume.Top = 130
$btnResume.Width = 100
$btnResume.Enabled = $false
$form.Controls.Add($btnResume)

$btnCleanup = New-Object System.Windows.Forms.Button
$btnCleanup.Text = 'Limpar'
$btnCleanup.Left = 585
$btnCleanup.Top = 130
$btnCleanup.Width = 100
$btnCleanup.Enabled = $false
$form.Controls.Add($btnCleanup)

$btnOpen = New-Object System.Windows.Forms.Button
$btnOpen.Text = 'Abrir pasta'
$btnOpen.Left = 695
$btnOpen.Top = 130
$btnOpen.Width = 100
$btnOpen.Enabled = $false
$form.Controls.Add($btnOpen)

$btnSelectAll = New-Object System.Windows.Forms.Button
$btnSelectAll.Text = 'Selecionar tudo'
$btnSelectAll.Left = 805
$btnSelectAll.Top = 130
$btnSelectAll.Width = 105
$form.Controls.Add($btnSelectAll)

$btnSelectNone = New-Object System.Windows.Forms.Button
$btnSelectNone.Text = 'Limpar selecao'
$btnSelectNone.Left = 920
$btnSelectNone.Top = 130
$btnSelectNone.Width = 105
$form.Controls.Add($btnSelectNone)

$listDownloads = New-Object System.Windows.Forms.ListView
$listDownloads.Left = 15
$listDownloads.Top = 170
$listDownloads.Width = 1010
$listDownloads.Height = 250
$listDownloads.View = 'Details'
$listDownloads.FullRowSelect = $true
$listDownloads.MultiSelect = $true
$listDownloads.GridLines = $true
[void]$listDownloads.Columns.Add('Job', 120)
[void]$listDownloads.Columns.Add('Status', 90)
[void]$listDownloads.Columns.Add('Percent', 80)
[void]$listDownloads.Columns.Add('Resultado', 320)
[void]$listDownloads.Columns.Add('Bytes', 160)
[void]$listDownloads.Columns.Add('Manifest', 240)
$form.Controls.Add($listDownloads)

$lblStatus = New-Object System.Windows.Forms.Label
$lblStatus.Left = 15
$lblStatus.Top = 430
$lblStatus.Width = 1010
$lblStatus.Text = "Status: idle | API: $apiBase"
$form.Controls.Add($lblStatus)

$lblFile = New-Object System.Windows.Forms.Label
$lblFile.Left = 15
$lblFile.Top = 452
$lblFile.Width = 1010
$lblFile.Text = 'Arquivo/resultado: -'
$form.Controls.Add($lblFile)

$progress = New-Object System.Windows.Forms.ProgressBar
$progress.Left = 15
$progress.Top = 480
$progress.Width = 1010
$progress.Height = 22
$progress.Minimum = 0
$progress.Maximum = 100
$form.Controls.Add($progress)

$lblMetrics = New-Object System.Windows.Forms.Label
$lblMetrics.Left = 15
$lblMetrics.Top = 552
$lblMetrics.Width = 1010
$lblMetrics.Height = 80
$lblMetrics.Anchor = [System.Windows.Forms.AnchorStyles]::Top -bor [System.Windows.Forms.AnchorStyles]::Left -bor [System.Windows.Forms.AnchorStyles]::Right
$lblMetrics.Text = 'Aguardando transferência...'
$form.Controls.Add($lblMetrics)

$lblCable = New-Object System.Windows.Forms.Label
$lblCable.Left = 15
$lblCable.Top = 508
$lblCable.Height = 18
$lblCable.Width = 1010
$lblCable.Anchor = [System.Windows.Forms.AnchorStyles]::Top -bor [System.Windows.Forms.AnchorStyles]::Left -bor [System.Windows.Forms.AnchorStyles]::Right
$lblCable.Text = 'Cable: -'
$form.Controls.Add($lblCable)

$lblWifi = New-Object System.Windows.Forms.Label
$lblWifi.Left = 15
$lblWifi.Top = 530
$lblWifi.Height = 18
$lblWifi.Width = 1010
$lblWifi.Anchor = [System.Windows.Forms.AnchorStyles]::Top -bor [System.Windows.Forms.AnchorStyles]::Left -bor [System.Windows.Forms.AnchorStyles]::Right
$lblWifi.Text = 'Wi-Fi: -'
$form.Controls.Add($lblWifi)

$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 1000
$script:SelectedJobId = $null
$script:JobsCache = @{}
$script:CurrentCollectionPath = "/downloads"
$script:finalOutput = $null
$script:currentStatus = 'idle'
$script:currentManifestPath = $null
$script:SendModeJobId = $null

function Get-SelectedSourceMode {
    $selected = [string]$cmbMode.SelectedItem
    switch ($selected) {
        'Download internet' { return 'download' }
        'Receber de outro PC' { return 'pull' }
        default { return 'auto' }
    }
}

function Get-EffectiveSourceMode {
    param([string]$Value)
    $selected = Get-SelectedSourceMode
    if ($selected -ne 'auto') { return $selected }
    $detected = Get-DetectedSourceMode $Value
    if ($detected -eq 'unknown') { return 'download' }
    return $detected
}

function Resolve-SourceMode {
    param([string]$Value)
    $selected = Get-SelectedSourceMode
    if ($selected -eq 'download') {
        if (-not (Test-DownloadUrl $Value)) { throw 'Origem inválida para Download internet. Use uma URL http:// ou https://.' }
        return 'download'
    }
    if ($selected -eq 'pull') {
        if (-not (Test-PullSourceUrl $Value)) { throw 'Origem inválida para Receber de outro PC. Use file://PC/share/path ou \\PC\share\path.' }
        return 'pull'
    }
    if (Test-DownloadUrl $Value) { return 'download' }
    if (Test-PullSourceUrl $Value) { return 'pull' }
    throw 'Origem inválida. Use http/https para internet ou file:///UNC para receber de outro PC.'
}

function Set-SourceModeVisual {
    param([switch]$UpdateCollection)
    $mode = Get-EffectiveSourceMode $txtUrl.Text
    if ($mode -eq 'pull') {
        $lblUrl.Text = 'Origem LAN (file:// ou UNC)'
        $btnStart.Text = 'Receber LAN'
        if ($UpdateCollection) { $script:CurrentCollectionPath = '/pulls' }
    } else {
        $lblUrl.Text = 'URL HTTP/HTTPS'
        $btnStart.Text = 'Baixar'
        if ($UpdateCollection) { $script:CurrentCollectionPath = '/downloads' }
    }
}

function Get-SelectedJobId {
    $items = @($listDownloads.SelectedItems)
    if ($items.Count -gt 0 -and -not [string]::IsNullOrWhiteSpace($items[0].Text)) {
        return [string]$items[0].Text
    }
    if (-not [string]::IsNullOrWhiteSpace($script:SelectedJobId)) {
        return [string]$script:SelectedJobId
    }
    return $null
}

function Set-UiState {
    param(
        [Parameter(Mandatory = $true)] [string] $State,
        [switch] $HasOutput,
        [switch] $CanResume,
        [switch] $CanCleanup
    )
    $script:currentStatus = $State
    $openAllowed = $HasOutput.IsPresent
    $cleanupAllowed = $CanCleanup.IsPresent

    switch ($State) {
        'idle' {
            $btnStart.Enabled = $true
            $btnRefresh.Enabled = $true
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $false
            $btnCleanup.Enabled = $cleanupAllowed
            $btnOpen.Enabled = $openAllowed
        }
        'starting' {
            $btnStart.Enabled = $false
            $btnRefresh.Enabled = $false
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $false
            $btnCleanup.Enabled = $false
            $btnOpen.Enabled = $false
        }
        'running' {
            $btnStart.Enabled = $false
            $btnRefresh.Enabled = $true
            $btnCancel.Enabled = $true
            $btnPause.Enabled = $true
            $btnResume.Enabled = $false
            $btnCleanup.Enabled = $false
            $btnOpen.Enabled = $openAllowed
        }
        'canceling' {
            $btnStart.Enabled = $false
            $btnRefresh.Enabled = $false
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $false
            $btnCleanup.Enabled = $false
            $btnOpen.Enabled = $openAllowed
        }
        'canceled' {
            $btnStart.Enabled = $true
            $btnRefresh.Enabled = $true
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $true
            $btnCleanup.Enabled = $cleanupAllowed
            $btnOpen.Enabled = $openAllowed
        }
        'resuming' {
            $btnStart.Enabled = $false
            $btnRefresh.Enabled = $false
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $false
            $btnCleanup.Enabled = $false
            $btnOpen.Enabled = $openAllowed
        }
        'done' {
            $btnStart.Enabled = $true
            $btnRefresh.Enabled = $true
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $false
            $btnCleanup.Enabled = $cleanupAllowed
            $btnOpen.Enabled = $openAllowed
        }
        'failed' {
            $btnStart.Enabled = $true
            $btnRefresh.Enabled = $true
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $CanResume.IsPresent
            $btnCleanup.Enabled = $cleanupAllowed
            $btnOpen.Enabled = $openAllowed
        }
        default {
            $btnStart.Enabled = $true
            $btnRefresh.Enabled = $true
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $false
            $btnCleanup.Enabled = $cleanupAllowed
            $btnOpen.Enabled = $openAllowed
        }
    }
}

function Update-ActionButtons {
    $id = Get-SelectedJobId
    if ([string]::IsNullOrWhiteSpace($id)) {
        $btnCancel.Enabled = $false
        $btnPause.Enabled = $false
        $btnResume.Enabled = $false
        $btnCleanup.Enabled = $false
        $btnOpen.Enabled = (-not [string]::IsNullOrWhiteSpace($script:finalOutput))
        return
    }
    
    if (-not $script:JobsCache.ContainsKey($id)) { return }
    $job = $script:JobsCache[$id]
    $canCleanup = ($job.status -in @('done', 'canceled', 'failed'))
    $canResume = ([bool]$job.resume_supported)
    
    switch ($job.status) {
        'running' {
            $btnCancel.Enabled = $true
            $btnPause.Enabled = $true
            $btnResume.Enabled = $false
            $btnOpen.Enabled = $true
        }
        'canceled' {
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $true
            $btnOpen.Enabled = $true
        }
        'failed' {
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $canResume
            $btnOpen.Enabled = $true
        }
        'done' {
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $false
            $btnOpen.Enabled = $true
        }
        default {
            $btnCancel.Enabled = $false
            $btnPause.Enabled = $false
            $btnResume.Enabled = $false
            $btnOpen.Enabled = $true
        }
    }
    $btnCleanup.Enabled = $canCleanup
}

function Update-DownloadList {
    try {
        $raw = Invoke-ApiJson -Method GET -Uri "$apiBase$($script:CurrentCollectionPath)"
        $jobs = @($raw)
        $listDownloads.BeginUpdate()
        $listDownloads.Items.Clear()
        $script:JobsCache = @{}
        foreach ($job in $jobs) {
            if ([string]::IsNullOrWhiteSpace($job.id)) { continue }
            $script:JobsCache[$job.id] = $job
            $item = New-Object System.Windows.Forms.ListViewItem([string]$job.id)
            [void]$item.SubItems.Add([string]$job.status)
            [void]$item.SubItems.Add(('{0:N2}%' -f [double]$job.percent))
            [void]$item.SubItems.Add([string]$job.output_path)
            [void]$item.SubItems.Add(('{0} / {1}' -f [int64]$job.bytes_done, [int64]$job.total_bytes))
            [void]$item.SubItems.Add([string]$job.manifest_path)
            [void]$listDownloads.Items.Add($item)
        }
        $listDownloads.EndUpdate()
        $previousSelectedId = $script:SelectedJobId
        $reselected = $false
        if (-not [string]::IsNullOrWhiteSpace($previousSelectedId)) {
            foreach ($item in $listDownloads.Items) {
                if ($item.Text -eq $previousSelectedId) {
                    $item.Selected = $true
                    $reselected = $true
                    break
                }
            }
        }
        if (-not $reselected) {
            $script:SelectedJobId = $null
        }
        Update-ActionButtons
    } catch {
        $lblStatus.Text = "Status: falha ao atualizar lista | $($_.Exception.Message)"
    }
}

function Get-SelectedJobIds {
    $items = @($listDownloads.SelectedItems)
    $ids = @()
    foreach ($item in $items) {
        $ids += [string]$item.Text
    }
    return ,$ids
}

function Refresh-SelectedJob {
    $id = Get-SelectedJobId
    if ([string]::IsNullOrWhiteSpace($id)) { return }
    try {
        $st = Invoke-ApiJson -Method GET -Uri "$apiBase$($script:CurrentCollectionPath)/$id"
        $pct = [Math]::Max([Math]::Min([double]$st.percent, 100), 0)
        $progress.Value = [int][Math]::Round($pct)
        $lblStatus.Text = "Status: $($st.status)"
        $lblFile.Text = "Resultado: $($st.output_path)"
        $script:finalOutput = [string]$st.output_path
        $script:currentManifestPath = [string]$st.manifest_path
        $pipelineMode = [string]$st.pipeline_mode
        if ([string]::IsNullOrWhiteSpace($pipelineMode)) { $pipelineMode = "legacy" }
        $inCable = 0
        $inWifi = 0
        $qCable = 0
        $qWifi = 0
        if ($st.in_flight) {
            if ($st.in_flight.cable -ne $null) { $inCable = [int]$st.in_flight.cable }
            if ($st.in_flight.wifi -ne $null) { $inWifi = [int]$st.in_flight.wifi }
        }
        if ($st.queue_depth) {
            if ($st.queue_depth.cable -ne $null) { $qCable = [int]$st.queue_depth.cable }
            if ($st.queue_depth.wifi -ne $null) { $qWifi = [int]$st.queue_depth.wifi }
        }
        $lblMetrics.Text = "Percent: {0:N2}%`r`nBytes: {1} / {2}`r`nChunks: {3}/{4} failed={5} pending={6}`r`nVelocidade: avg {7:N2} Mbps`r`nPipeline: {8} | in_flight(cable={9},wifi={10}) | queue(cable={11},wifi={12})`r`nManifest: {13}" -f `
            [double]$st.percent, [int64]$st.bytes_done, [int64]$st.total_bytes, [int64]$st.chunks_done, [int64]$st.chunks_total, [int64]$st.chunks_failed, [int64]$st.chunks_pending, [double]$st.mbps_avg, $pipelineMode, $inCable, $inWifi, $qCable, $qWifi, [string]$st.manifest_path

        if ($st.channels) {
            if ($st.channels.cable) {
                $c = $st.channels.cable
                $cLocalIp = Get-OptionalPropValue -Obj $c -Name 'local_ip' -Default ''
                $lblCable.Text = "Cable: $($c.state) | $('{0:N1}' -f ($c.bytes_sent / 1MB)) MB | avg $('{0:N2}' -f $c.mbps_avg) Mbps | now $('{0:N2}' -f $c.mbps_now) Mbps | $cLocalIp"
            } else {
                $lblCable.Text = "Cable: -"
            }
            if ($st.channels.wifi) {
                $w = $st.channels.wifi
                $wLocalIp = Get-OptionalPropValue -Obj $w -Name 'local_ip' -Default ''
                $lblWifi.Text = "Wi-Fi: $($w.state) | $('{0:N1}' -f ($w.bytes_sent / 1MB)) MB | avg $('{0:N2}' -f $w.mbps_avg) Mbps | now $('{0:N2}' -f $w.mbps_now) Mbps | $wLocalIp"
            } else {
                $lblWifi.Text = "Wi-Fi: -"
            }
        } else {
            $lblCable.Text = "Cable: -"
            $lblWifi.Text = "Wi-Fi: -"
        }

        if ($st.status -eq 'done') {
            $timer.Stop()
            $lblStatus.Text = "Status: done"
            if ($script:CurrentCollectionPath -eq "/pulls") {
                Set-UiState -State 'done' -HasOutput
            } else {
                Set-UiState -State 'done' -HasOutput -CanCleanup
            }
        } elseif ($st.status -eq 'failed' -or $st.status -eq 'canceled') {
            if ($st.status -eq 'failed') {
                $timer.Stop()
                $canResume = [bool]$st.resume_supported
                if ($script:CurrentCollectionPath -eq "/pulls") {
                    Set-UiState -State 'failed' -HasOutput -CanResume:$canResume
                } else {
                    Set-UiState -State 'failed' -HasOutput -CanResume:$canResume -CanCleanup
                }
                $lblStatus.Text = "Status: failed | $($st.message)"
            } else {
                if ($script:CurrentCollectionPath -eq "/pulls") {
                    Set-UiState -State 'canceled' -HasOutput
                } else {
                    Set-UiState -State 'canceled' -HasOutput -CanCleanup
                }
                $lblStatus.Text = "Status: canceled | Transferência cancelada com segurança. Chunks preservados."
            }
        } else {
            Set-UiState -State 'running' -HasOutput
        }
    } catch {
        $lblStatus.Text = "Status: falha ao atualizar job | $($_.Exception.Message)"
    }
}

function Refresh-SendJob {
    param([string]$JobId)
    if ([string]::IsNullOrWhiteSpace($JobId)) { return }
    try {
        $st = Invoke-ApiJson -Method GET -Uri "$apiBase/jobs/$JobId"
        $pct = [Math]::Max([Math]::Min([double]$st.percent, 100), 0)
        $progress.Value = [int][Math]::Round($pct)
        $lblStatus.Text = "Status: $($st.status) | Job envio: $JobId"
        $fp = Get-OptionalPropValue -Obj $st -Name 'file_path' -Default '-'
        $lblFile.Text = "Arquivo: $fp"
        $script:finalOutput = $fp
        $lblMetrics.Text = "Percent: {0:N2}%`r`nBytes: {1} / {2}`r`nChunks: {3}/{4} failed={5} pending={6}`r`nVelocidade: avg {7:N2} Mbps" -f `
            [double]$st.percent, [int64]$st.bytes_sent, [int64]$st.total_bytes, [int64]$st.chunks_done, [int64]$st.chunks_total, [int64]$st.chunks_failed, [int64]$st.chunks_pending, [double]$st.mbps_avg

        if ($listDownloads.Items.Count -gt 0) {
            $itm = $listDownloads.Items[0]
            if ($itm.SubItems.Count -ge 5) {
                $itm.SubItems[1].Text = [string]$st.status
                $itm.SubItems[2].Text = ('{0:N2}%' -f [double]$st.percent)
                $itm.SubItems[3].Text = $fp
                $itm.SubItems[4].Text = ('{0} / {1}' -f [int64]$st.bytes_sent, [int64]$st.total_bytes)
            }
        }

        if ($st.channels) {
            if ($st.channels.cable) {
                $c = $st.channels.cable
                $cLocalIp = Get-OptionalPropValue -Obj $c -Name 'local_ip' -Default ''
                $lblCable.Text = "Cable: $($c.state) | $('{0:N1}' -f ($c.bytes_sent / 1MB)) MB | avg $('{0:N2}' -f $c.mbps_avg) Mbps | now $('{0:N2}' -f $c.mbps_now) Mbps | $cLocalIp"
            } else { $lblCable.Text = "Cable: -" }
            if ($st.channels.wifi) {
                $w = $st.channels.wifi
                $wLocalIp = Get-OptionalPropValue -Obj $w -Name 'local_ip' -Default ''
                $lblWifi.Text = "Wi-Fi: $($w.state) | $('{0:N1}' -f ($w.bytes_sent / 1MB)) MB | avg $('{0:N2}' -f $w.mbps_avg) Mbps | now $('{0:N2}' -f $w.mbps_now) Mbps | $wLocalIp"
            } else { $lblWifi.Text = "Wi-Fi: -" }
        }

        if ($st.status -eq 'done') {
            $timer.Stop(); Set-UiState -State 'done' -HasOutput; $lblStatus.Text = "Status: done | envio concluído"
        } elseif ($st.status -eq 'failed' -or $st.status -eq 'canceled') {
            if ($st.status -eq 'failed') {
                $timer.Stop(); Set-UiState -State 'failed' -HasOutput
                $lblStatus.Text = "Status: failed | $($st.message)"
            } else {
                Set-UiState -State 'canceled' -HasOutput
                $lblStatus.Text = "Status: canceled | Envio pausado/cancelado com segurança."
            }
        } else {
            Set-UiState -State 'running' -HasOutput
        }
    } catch {
        $lblStatus.Text = "Status: refresh send failed | $($_.Exception.Message)"
    }
}

Set-SourceModeVisual -UpdateCollection
Set-UiState -State 'idle'
Update-DownloadList
$timer.Interval = 1000
$timer.Start()

if (-not [string]::IsNullOrWhiteSpace($SendJobId)) {
    $script:SendModeJobId = $SendJobId
    $btnStart.Enabled = $false
    $btnCleanup.Enabled = $false
    $btnSelectAll.Enabled = $false
    $btnSelectNone.Enabled = $false
    $cmbMode.Enabled = $false
    
    $txtUrl.Enabled = $false
    $txtUrl.Text = "Modo de Envio (URL não aplicável)"
    $txtOut.Enabled = $false
    $txtOut.Text = "Destino gerido pelo receptor"
    
    $lblStatus.Text = "Status: monitorando envio | Job: $SendJobId"
    
    $listDownloads.Items.Clear()
    $itm = New-Object System.Windows.Forms.ListViewItem($SendJobId)
    [void]$itm.SubItems.Add('starting')
    [void]$itm.SubItems.Add('0.00%')
    [void]$itm.SubItems.Add('-')
    [void]$itm.SubItems.Add('0 / 0')
    [void]$itm.SubItems.Add('Send Mode')
    [void]$listDownloads.Items.Add($itm)
    $itm.Selected = $true

    Refresh-SendJob -JobId $SendJobId
}

$cmbMode.Add_SelectedIndexChanged({
    try {
        Set-SourceModeVisual -UpdateCollection
        if ([string]::IsNullOrWhiteSpace($script:SendModeJobId)) {
            $script:SelectedJobId = $null
            Update-DownloadList
            Update-ActionButtons
        }
    } catch {
        $lblStatus.Text = "Status: erro ao trocar modo: $($_.Exception.Message)"
    }
})

$txtUrl.Add_TextChanged({
    try {
        if ([string]::IsNullOrWhiteSpace($script:SendModeJobId)) {
            Set-SourceModeVisual -UpdateCollection
        }
    } catch {}
})

$timer.Add_Tick({ 
    try {
        if (-not [string]::IsNullOrWhiteSpace($script:SendModeJobId)) {
            Refresh-SendJob -JobId $script:SendModeJobId
        } else {
            Update-DownloadList
            if (-not [string]::IsNullOrWhiteSpace($script:SelectedJobId)) {
                Refresh-SelectedJob
            }
        }
    } catch {
        $lblStatus.Text = "Status: erro no timer: $($_.Exception.Message)"
    }
})

$btnStart.Add_Click({
    try {
        $rawSource = $txtUrl.Text.Trim()
        $mode = Resolve-SourceMode $rawSource
        $url = $rawSource
        if ($mode -eq 'download') {
            $url = Normalize-DownloadUrlInput $rawSource
            $txtUrl.Text = $url
        }
        $out = $txtOut.Text.Trim()
        if ([string]::IsNullOrWhiteSpace($out)) {
            throw "Destino vazio."
        }
        [System.IO.Directory]::CreateDirectory($out) | Out-Null
        Set-UiState -State 'starting'
        $job = $null
        if ($mode -eq 'download') {
            $script:CurrentCollectionPath = "/downloads"
            $payload = @{
                url = $url
                output_dir = $out
                file_name = ""
                chunk_size_mb = 0
            }
            $job = Invoke-ApiJson -Method POST -Uri "$apiBase/downloads" -Body $payload
        } elseif ($mode -eq 'pull') {
            $script:CurrentCollectionPath = "/pulls"
            $payload = @{
                source_url = $url
                output_dir = $out
                chunk_size_mb = 0
            }
            $job = Invoke-ApiJson -Method POST -Uri "$apiBase/pulls" -Body $payload
        }
        $jobId = [string]$job.id
        if ([string]::IsNullOrWhiteSpace($jobId)) {
            throw "Resposta sem job id."
        }
        $currentManifestPath = [string]$job.manifest_path
        $finalOutput = [string]$job.output_path
        $progress.Value = 0
        $lblStatus.Text = "Status: running ($jobId)"
        $lblFile.Text = "Resultado: $($job.output_path)"
        Set-UiState -State 'running' -HasOutput
        $timer.Start()
        Update-DownloadList
    } catch {
        Set-UiState -State 'idle'
        $errMsg = $_.Exception.Message
        try {
            if ($_.ErrorDetails -and $_.ErrorDetails.Message) {
                $errMsg += "`r`nDetalhe: " + $_.ErrorDetails.Message
            } elseif ($_.Exception.Response) {
                $stream = $_.Exception.Response.GetResponseStream()
                if ($stream.CanSeek) { $stream.Position = 0 }
                $reader = New-Object System.IO.StreamReader($stream)
                $respBody = $reader.ReadToEnd()
                if ($respBody) { $errMsg += "`r`nDetalhe: $respBody" }
            }
        } catch {}
        [System.Windows.Forms.MessageBox]::Show("Falha ao iniciar transferência.`r`n$errMsg", 'Erro', 'OK', 'Error') | Out-Null
    }
})

$btnRefresh.Add_Click({
    try {
        Update-DownloadList
        Refresh-SelectedJob
    } catch {
        $lblStatus.Text = "Status: erro ao atualizar: $($_.Exception.Message)"
    }
})

$btnCancel.Add_Click({
    try {
        $id = Get-SelectedJobId
        if ([string]::IsNullOrWhiteSpace($id)) {
            throw "Selecione uma transferência para cancelar."
        }
        Set-UiState -State 'canceling' -HasOutput
        $null = Invoke-ApiJson -Method POST -Uri "$apiBase$($script:CurrentCollectionPath)/$id/cancel" -Body @{}
        $lblStatus.Text = "Status: canceling..."
        Update-DownloadList
        Refresh-SelectedJob
        if (-not $timer.Enabled) {
            $timer.Start()
        }
    } catch {
        Set-UiState -State 'running' -HasOutput
        [System.Windows.Forms.MessageBox]::Show("Falha ao cancelar transferência.`r`n$($_.Exception.Message)", 'Erro', 'OK', 'Error') | Out-Null
    }
})

$btnResume.Add_Click({
    try {
        $id = Get-SelectedJobId
        if ([string]::IsNullOrWhiteSpace($id)) {
            throw "Selecione uma transferência para retomar."
        }
        Set-UiState -State 'resuming' -HasOutput
        $job = Invoke-ApiJson -Method POST -Uri "$apiBase$($script:CurrentCollectionPath)/$id/resume" -Body @{}
        $lblStatus.Text = "Status: resuming... Retomando do manifesto existente."
        if ($null -ne $job -and $job.output_path) {
            $script:finalOutput = [string]$job.output_path
            $lblFile.Text = "Resultado: $($job.output_path)"
        }
        if (-not $timer.Enabled) {
            $timer.Start()
        }
    } catch {
        Set-UiState -State 'canceled' -HasOutput
        [System.Windows.Forms.MessageBox]::Show("Falha ao retomar transferência.`r`n$($_.Exception.Message)", 'Erro', 'OK', 'Error') | Out-Null
    }
})

$btnCleanup.Add_Click({
    try {
        $ids = @(Get-SelectedJobIds)
        if ($ids.Count -eq 0) {
            $id = Get-SelectedJobId
            if (-not [string]::IsNullOrWhiteSpace($id)) { $ids = @($id) }
        }
        if ($ids.Count -eq 0) {
            throw "Nenhum job selecionado para limpeza."
        }
        foreach ($id in $ids) {
            if ($script:CurrentCollectionPath -eq "/pulls") {
                throw "Limpeza de Receber LAN ainda não é suportada."
            }
            $preview = Invoke-ApiJson -Method GET -Uri "$apiBase/downloads/$id/cleanup/preview"
            $confirm = [System.Windows.Forms.MessageBox]::Show(
                ("Limpar job {0}?`r`nArquivos: {1}`r`nManifesto: {2}`r`nDiretório: {3}" -f $id, $preview.will_delete_files, $preview.manifest_path, $preview.output_dir),
                'MultiSend',
                [System.Windows.Forms.MessageBoxButtons]::OKCancel,
                [System.Windows.Forms.MessageBoxIcon]::Question
            )
            if ($confirm -ne [System.Windows.Forms.DialogResult]::OK) { continue }
            $null = Invoke-ApiJson -Method POST -Uri "$apiBase/downloads/$id/cleanup" -Body @{ delete_files = $true; delete_manifest = $true; delete_empty_dirs = $true }
        }
        Update-DownloadList
    } catch {
        [System.Windows.Forms.MessageBox]::Show("Falha ao limpar transferências.`r`n$($_.Exception.Message)", 'Erro', 'OK', 'Error') | Out-Null
    }
})

$btnPause.Add_Click({
    try {
        $id = Get-SelectedJobId
        if ([string]::IsNullOrWhiteSpace($id)) {
            throw "Selecione uma transferência para pausar."
        }
        Set-UiState -State 'canceling' -HasOutput
        $null = Invoke-ApiJson -Method POST -Uri "$apiBase$($script:CurrentCollectionPath)/$id/cancel" -Body @{}
        $lblStatus.Text = "Status: paused | Transferência pausada com segurança. Use Retomar."
        Update-DownloadList
        Refresh-SelectedJob
        if (-not $timer.Enabled) {
            $timer.Start()
        }
    } catch {
        Set-UiState -State 'running' -HasOutput
        [System.Windows.Forms.MessageBox]::Show("Falha ao pausar transferência.`r`n$($_.Exception.Message)", 'Erro', 'OK', 'Error') | Out-Null
    }
})

$btnOpen.Add_Click({
    try {
        $pathToOpen = $null
        $id = Get-SelectedJobId
        if (-not [string]::IsNullOrWhiteSpace($id) -and $script:JobsCache.ContainsKey($id)) {
            $job = $script:JobsCache[$id]
            if (-not [string]::IsNullOrWhiteSpace($job.output_path)) {
                $pathToOpen = Split-Path -Parent $job.output_path
            }
        }
        if ([string]::IsNullOrWhiteSpace($pathToOpen) -and -not [string]::IsNullOrWhiteSpace($script:finalOutput)) {
            $pathToOpen = Split-Path -Parent $script:finalOutput
        }
        if ([string]::IsNullOrWhiteSpace($pathToOpen)) {
            $pathToOpen = $txtOut.Text.Trim()
        }
        if ([string]::IsNullOrWhiteSpace($pathToOpen) -or -not (Test-Path $pathToOpen)) {
            throw "Pasta não encontrada: $pathToOpen"
        }
        Start-Process explorer.exe $pathToOpen | Out-Null
    } catch {
        [System.Windows.Forms.MessageBox]::Show("Falha ao abrir pasta.`r`n$($_.Exception.Message)", 'Erro', 'OK', 'Error') | Out-Null
    }
})

$btnSelectAll.Add_Click({
    try {
        $listDownloads.MultiSelect = $true
        for ($i = 0; $i -lt $listDownloads.Items.Count; $i++) {
            $listDownloads.Items[$i].Selected = $true
        }

        if (@($listDownloads.SelectedItems).Count -gt 0) {
            $script:SelectedJobId = [string]$listDownloads.SelectedItems[0].Text
            Refresh-SelectedJob
        }

        Update-ActionButtons
    } catch {
        $lblStatus.Text = "Status: erro ao selecionar tudo: $($_.Exception.Message)"
    }
})

$btnSelectNone.Add_Click({
    try {
        for ($i = 0; $i -lt $listDownloads.Items.Count; $i++) {
            $listDownloads.Items[$i].Selected = $false
        }
        $script:SelectedJobId = $null
        $lblStatus.Text = "Nenhuma transferência selecionada"
        $progress.Value = 0
        $lblCable.Text = "Cable: -"
        $lblWifi.Text = "Wi-Fi: -"
        $lblMetrics.Text = ""
        Update-ActionButtons
    } catch {
        $lblStatus.Text = "Status: erro ao limpar selecao: $($_.Exception.Message)"
    }
})

$listDownloads.Add_SelectedIndexChanged({
    try {
        $selected = @($listDownloads.SelectedItems)
        if ($selected.Count -gt 0) {
            $script:SelectedJobId = [string]$selected[0].Text
            Refresh-SelectedJob
        } else {
            $script:SelectedJobId = $null
            $lblStatus.Text = "Nenhuma transferência selecionada"
            $progress.Value = 0
            $lblCable.Text = "Cable: -"
            $lblWifi.Text = "Wi-Fi: -"
        }
        Update-ActionButtons
    } catch {
        $lblStatus.Text = "Status: erro na selecao: $($_.Exception.Message)"
    }
})

[void]$form.ShowDialog()



