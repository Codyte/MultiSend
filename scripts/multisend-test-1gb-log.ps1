[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$FilePath1GB,
    [int]$PollSeconds = 1,
    [int]$TimeoutMinutes = 90
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$lockFile = Join-Path $env:TEMP 'multisend-test-1gb-log.lock'

function Write-Step {
    param([string]$Message)
    Write-Host ("[STEP] {0}" -f $Message)
}

function Get-ApiBase {
    foreach ($p in 56221..56230) {
        try {
            $u = "http://127.0.0.1:$p"
            $h = Invoke-RestMethod -Uri "$u/health" -TimeoutSec 2
            if ($h.ok) { return $u }
        } catch {}
    }
    return $null
}

function Resolve-LauncherPath {
    $candidates = @(
        'C:\Program Files\MultiSend\bin\multisend-launcher.ps1',
        (Join-Path (Split-Path -Parent $PSScriptRoot) 'multisend-launcher.ps1')
    )
    foreach ($p in $candidates) {
        if (Test-Path -LiteralPath $p) { return $p }
    }
    throw 'multisend-launcher.ps1 não encontrado (instalado ou local).'
}

function Save-SystemSnapshot {
    param([string]$Dir)
    $snapDir = Join-Path $Dir 'snapshot'
    New-Item -ItemType Directory -Force -Path $snapDir | Out-Null

    $PSVersionTable | Out-File -Encoding UTF8 (Join-Path $snapDir 'ps_version.txt')
    ipconfig /all | Out-File -Encoding UTF8 (Join-Path $snapDir 'ipconfig_all.txt')
    route print | Out-File -Encoding UTF8 (Join-Path $snapDir 'route_print.txt')
    Get-NetAdapter | Format-Table -AutoSize | Out-String | Out-File -Encoding UTF8 (Join-Path $snapDir 'net_adapter.txt')
    Get-NetIPConfiguration | Format-List | Out-String | Out-File -Encoding UTF8 (Join-Path $snapDir 'net_ipconfig.txt')
    Get-NetRoute -AddressFamily IPv4 | Sort-Object -Property ifIndex,DestinationPrefix | Format-Table -AutoSize | Out-String | Out-File -Encoding UTF8 (Join-Path $snapDir 'net_route_v4.txt')
}

function Get-Jobs {
    param([string]$Api)
    try { return @(Invoke-RestMethod -Uri "$Api/jobs" -TimeoutSec 8) } catch { return @() }
}

function Wait-NewJobId {
    param(
        [string]$Api,
        [hashtable]$BeforeSet,
        [datetime]$Deadline
    )
    while ((Get-Date) -lt $Deadline) {
        $jobs = Get-Jobs -Api $Api
        foreach ($j in $jobs) {
            $id = [string]$j.id
            if ([string]::IsNullOrWhiteSpace($id)) { continue }
            if (-not $BeforeSet.ContainsKey($id)) { return $id }
        }
        Start-Sleep -Seconds 1
    }
    return $null
}

function Wait-JobDone {
    param(
        [string]$Api,
        [string]$JobId,
        [datetime]$Deadline,
        [string]$CsvPath
    )

    $rows = @()
    $last = $null
    while ((Get-Date) -lt $Deadline) {
        $s = Invoke-RestMethod -Uri "$Api/jobs/$JobId" -TimeoutSec 6
        $last = $s
        $rows += [pscustomobject]@{
            timestamp      = (Get-Date).ToString('o')
            status         = [string]$s.status
            percent        = [double]$s.percent
            bytes_sent     = [int64]$s.bytes_sent
            total_bytes    = [int64]$s.total_bytes
            mbps_avg       = [double]$s.mbps_avg
            mbps_now       = [double]$s.mbps_now
            chunks_done    = [int64]$s.chunks_done
            chunks_total   = [int64]$s.chunks_total
            chunks_failed  = [int64]$s.chunks_failed
            chunks_pending = [int64]$s.chunks_pending
            cable_state    = if ($s.channels -and $s.channels.cable) { [string]$s.channels.cable.state } else { '' }
            cable_mbps_now = if ($s.channels -and $s.channels.cable) { [double]$s.channels.cable.mbps_now } else { 0 }
            cable_mbps_avg = if ($s.channels -and $s.channels.cable) { [double]$s.channels.cable.mbps_avg } else { 0 }
            cable_bytes    = if ($s.channels -and $s.channels.cable) { [int64]$s.channels.cable.bytes_sent } else { 0 }
            cable_local_ip = if ($s.channels -and $s.channels.cable) { [string]$s.channels.cable.local_ip } else { '' }
            wifi_state     = if ($s.channels -and $s.channels.wifi) { [string]$s.channels.wifi.state } else { '' }
            wifi_mbps_now  = if ($s.channels -and $s.channels.wifi) { [double]$s.channels.wifi.mbps_now } else { 0 }
            wifi_mbps_avg  = if ($s.channels -and $s.channels.wifi) { [double]$s.channels.wifi.mbps_avg } else { 0 }
            wifi_bytes     = if ($s.channels -and $s.channels.wifi) { [int64]$s.channels.wifi.bytes_sent } else { 0 }
            wifi_local_ip  = if ($s.channels -and $s.channels.wifi) { [string]$s.channels.wifi.local_ip } else { '' }
            manifest_path  = [string]$s.manifest_path
            message        = [string]$s.message
        }
        if ($s.status -in @('done', 'failed', 'canceled')) { break }
        Start-Sleep -Seconds $PollSeconds
    }

    $rows | Export-Csv -NoTypeInformation -Encoding UTF8 -LiteralPath $CsvPath
    return $last
}

$api = Get-ApiBase
if (-not $api) { throw 'MultiSend API não encontrada nas portas 56221-56230.' }

if (-not (Test-Path -LiteralPath $FilePath1GB -PathType Leaf)) {
    throw "Arquivo não encontrado: $FilePath1GB"
}
$resolvedFile = (Resolve-Path -LiteralPath $FilePath1GB).Path
$launcher = Resolve-LauncherPath

if (Test-Path -LiteralPath $lockFile) {
    throw "Script já está em execução (lock: $lockFile)."
}
New-Item -ItemType File -Force -Path $lockFile | Out-Null

$reportRoot = Join-Path $env:USERPROFILE ("Desktop\MultiSend-Test-1GB-" + (Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory -Force -Path $reportRoot | Out-Null

try {
    Write-Step "Salvando snapshot de rede em: $reportRoot"
    Save-SystemSnapshot -Dir $reportRoot

    $before = Get-Jobs -Api $api
    $active = @($before | Where-Object { $_.status -in @('running', 'pending') })
    if ($active.Count -gt 0) {
        $active | ConvertTo-Json -Depth 8 | Out-File -Encoding UTF8 (Join-Path $reportRoot 'active_jobs_before_start.json')
        throw ("Há job(s) ativo(s). Não vou iniciar novo envio. Ativos: {0}" -f $active.Count)
    }
    $beforeSet = @{}
    foreach ($j in $before) {
        $id = [string]$j.id
        if (-not [string]::IsNullOrWhiteSpace($id)) { $beforeSet[$id] = $true }
    }

    Write-Step "Disparando launcher com arquivo: $resolvedFile"
    Start-Process -FilePath 'powershell.exe' -ArgumentList @(
        '-NoProfile',
        '-ExecutionPolicy', 'Bypass',
        '-File', $launcher,
        '-FilePath', $resolvedFile
    ) -WindowStyle Normal | Out-Null

    $jobId = Wait-NewJobId -Api $api -BeforeSet $beforeSet -Deadline ((Get-Date).AddMinutes(2))
    if ([string]::IsNullOrWhiteSpace($jobId)) {
        throw 'Nenhum novo job detectado após executar o launcher.'
    }

    $start = Get-Date
    $timelineCsv = Join-Path $reportRoot 'timeline.csv'
    $final = Wait-JobDone -Api $api -JobId $jobId -Deadline $start.AddMinutes($TimeoutMinutes) -CsvPath $timelineCsv
    $elapsed = (Get-Date) - $start

    $final | ConvertTo-Json -Depth 30 | Out-File -Encoding UTF8 (Join-Path $reportRoot 'final_job.json')
    $summary = [pscustomobject]@{
        report_root   = $reportRoot
        api_base      = $api
        launcher_path = $launcher
        file_path     = $resolvedFile
        job_id        = $jobId
        started_at    = $start.ToString('o')
        elapsed_sec   = [math]::Round($elapsed.TotalSeconds, 2)
        status        = [string]$final.status
        mbps_avg      = [double]$final.mbps_avg
        bytes_sent    = [int64]$final.bytes_sent
        total_bytes   = [int64]$final.total_bytes
        chunks_done   = [int64]$final.chunks_done
        chunks_total  = [int64]$final.chunks_total
        chunks_failed = [int64]$final.chunks_failed
        manifest_path = [string]$final.manifest_path
        timeline_csv  = $timelineCsv
    }
    $summary | ConvertTo-Json -Depth 10 | Out-File -Encoding UTF8 (Join-Path $reportRoot 'summary.json')

    Write-Host ''
    Write-Host '=== MultiSend Teste 1GB (Envio) Finalizado ==='
    Write-Host ("Relatório: {0}" -f $reportRoot)
    Write-Host ("Job: {0}" -f $jobId)
    Write-Host ("Status: {0}" -f [string]$final.status)
    Write-Host ("Tempo: {0}s" -f [math]::Round($elapsed.TotalSeconds, 2))
    Write-Host ("Mbps médio: {0:N2}" -f [double]$final.mbps_avg)
}
finally {
    Remove-Item -LiteralPath $lockFile -ErrorAction SilentlyContinue
}
