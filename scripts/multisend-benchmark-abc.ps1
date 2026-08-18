[CmdletBinding()]
param(
    [int]$Iterations = 2,
    [string]$Url = 'https://ash-speed.hetzner.com/1GB.bin',
    [int]$DualGainThresholdPct = 10,
    [ValidateSet('dynamic', 'strict_split')]
    [string]$ChannelStrategy = 'dynamic',
    [int]$ChunkSizeMB = 0
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

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

function Set-DownloadConfig {
    param(
        [string]$Mode = 'pipelined',
        [int]$InFlight = 2,
        [string]$Strategy = 'dynamic'
    )
    $cfgPath = Join-Path $env:APPDATA 'MultiSend\config.json'
    if (-not (Test-Path -LiteralPath $cfgPath)) {
        throw "config.json not found: $cfgPath"
    }
    $cfg = Get-Content -LiteralPath $cfgPath -Raw | ConvertFrom-Json
    if (-not ($cfg.PSObject.Properties.Name -contains 'download_pipeline_mode')) {
        $cfg | Add-Member -NotePropertyName 'download_pipeline_mode' -NotePropertyValue 'legacy'
    }
    if (-not ($cfg.PSObject.Properties.Name -contains 'download_in_flight_per_channel')) {
        $cfg | Add-Member -NotePropertyName 'download_in_flight_per_channel' -NotePropertyValue 2
    }
    if (-not ($cfg.PSObject.Properties.Name -contains 'download_channel_strategy')) {
        $cfg | Add-Member -NotePropertyName 'download_channel_strategy' -NotePropertyValue 'dynamic'
    }
    $cfg.download_pipeline_mode = $Mode
    $cfg.download_in_flight_per_channel = $InFlight
    $cfg.download_channel_strategy = $Strategy
    [System.IO.File]::WriteAllText(
        $cfgPath,
        ($cfg | ConvertTo-Json -Depth 20),
        (New-Object System.Text.UTF8Encoding($false))
    )
}

function Wait-For-Completion {
    param(
        [string]$Api,
        [string]$JobId,
        [datetime]$Deadline,
        [string]$TimeseriesCsv
    )
    $rows = @()
    $last = $null
    while ((Get-Date) -lt $Deadline) {
        $s = Invoke-RestMethod -Uri "$Api/downloads/$JobId" -TimeoutSec 5
        $last = $s
        $rows += [pscustomobject]@{
            timestamp      = (Get-Date).ToString('o')
            status         = [string]$s.status
            percent        = [double]$s.percent
            mbps_avg       = [double]$s.mbps_avg
            cable_now      = if ($s.channels) { [double]$s.channels.cable.mbps_now } else { 0 }
            wifi_now       = if ($s.channels) { [double]$s.channels.wifi.mbps_now } else { 0 }
            cable_bytes    = if ($s.channels) { [int64]$s.channels.cable.bytes_sent } else { 0 }
            wifi_bytes     = if ($s.channels) { [int64]$s.channels.wifi.bytes_sent } else { 0 }
            chunks_done    = [int64]$s.chunks_done
            chunks_total   = [int64]$s.chunks_total
            chunks_failed  = [int64]$s.chunks_failed
            chunks_pending = [int64]$s.chunks_pending
            idle_gap_cable_ms = if ($s.idle_gap_ms -and $s.idle_gap_ms.cable -ne $null) { [int64]$s.idle_gap_ms.cable } else { 0 }
            idle_gap_wifi_ms  = if ($s.idle_gap_ms -and $s.idle_gap_ms.wifi -ne $null) { [int64]$s.idle_gap_ms.wifi } else { 0 }
        }
        if ($s.status -in @('done', 'failed', 'canceled')) {
            break
        }
        Start-Sleep -Seconds 1
    }
    $rows | Export-Csv -NoTypeInformation -Encoding UTF8 -LiteralPath $TimeseriesCsv
    return $last
}

function Start-ScenarioRun {
    param(
        [string]$Api,
        [string]$ScenarioName,
        [string]$RunDir,
        [string]$Url,
        [int]$ChunkSizeMB
    )
    $outDir = Join-Path $env:USERPROFILE 'Downloads\MultiSend\Downloads'
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null

    $payload = @{
        url = $Url
        output_dir = $outDir
        file_name = ''
        chunk_size_mb = $ChunkSizeMB
    } | ConvertTo-Json -Compress

    $job = Invoke-RestMethod -Method Post -Uri "$Api/downloads" -ContentType 'application/json' -Body $payload -TimeoutSec 20
    $jobId = [string]$job.id
    if ([string]::IsNullOrWhiteSpace($jobId)) {
        throw "Missing job id for scenario $ScenarioName"
    }

    $start = Get-Date
    $timeseriesCsv = Join-Path $RunDir 'timeseries.csv'
    $final = Wait-For-Completion -Api $Api -JobId $jobId -Deadline $start.AddMinutes(60) -TimeseriesCsv $timeseriesCsv
    $elapsed = (Get-Date) - $start

    $final | ConvertTo-Json -Depth 30 | Out-File -Encoding UTF8 (Join-Path $RunDir 'final_job.json')

    return [pscustomobject]@{
        scenario       = $ScenarioName
        job_id         = $jobId
        status         = [string]$final.status
        elapsed_sec    = [math]::Round($elapsed.TotalSeconds, 2)
        mbps_avg       = [double]$final.mbps_avg
        chunks_done    = [int64]$final.chunks_done
        chunks_total   = [int64]$final.chunks_total
        chunks_failed  = [int64]$final.chunks_failed
        cable_bytes    = if ($final.channels) { [int64]$final.channels.cable.bytes_sent } else { 0 }
        wifi_bytes     = if ($final.channels) { [int64]$final.channels.wifi.bytes_sent } else { 0 }
        idle_gap_cable_ms = if ($final.idle_gap_ms -and $final.idle_gap_ms.cable -ne $null) { [int64]$final.idle_gap_ms.cable } else { 0 }
        idle_gap_wifi_ms  = if ($final.idle_gap_ms -and $final.idle_gap_ms.wifi -ne $null) { [int64]$final.idle_gap_ms.wifi } else { 0 }
        output_path    = [string]$final.output_path
        timeseries_csv = $timeseriesCsv
        final_json     = (Join-Path $RunDir 'final_job.json')
        pass_functional = (
            ([string]$final.status -eq 'done') -and
            ([int64]$final.chunks_failed -eq 0) -and
            ([int64]$final.bytes_done -eq [int64]$final.total_bytes)
        )
    }
}

if ($Iterations -lt 1) { $Iterations = 1 }

$api = Get-ApiBase
if (-not $api) { throw 'MultiSend API not found (56221-56230).' }

Set-DownloadConfig -Mode 'pipelined' -InFlight 2 -Strategy $ChannelStrategy
Write-Step "Download config set: mode=pipelined in_flight=2 strategy=$ChannelStrategy"

$reportRoot = Join-Path $env:USERPROFILE ("Desktop\MultiSend-Benchmark-ABC-" + (Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory -Force -Path $reportRoot | Out-Null

$scenarios = @(
    @{ Name = 'A_wifi_only'; Prompt = 'Conecte SOMENTE Wi-Fi (desligue cabo) e pressione ENTER' },
    @{ Name = 'B_cable_only'; Prompt = 'Conecte SOMENTE Cabo (desligue Wi-Fi) e pressione ENTER' },
    @{ Name = 'C_dual'; Prompt = 'Conecte Wi-Fi + Cabo e pressione ENTER' }
)

$results = @()
foreach ($s in $scenarios) {
    Read-Host $s.Prompt | Out-Null
    for ($i = 1; $i -le $Iterations; $i++) {
        $runDir = Join-Path $reportRoot ("{0}-run{1}" -f $s.Name, $i)
        New-Item -ItemType Directory -Force -Path $runDir | Out-Null
        Write-Step ("Running {0} iteration {1}/{2}" -f $s.Name, $i, $Iterations)
        $res = Start-ScenarioRun -Api $api -ScenarioName $s.Name -RunDir $runDir -Url $Url -ChunkSizeMB $ChunkSizeMB
        $results += $res
        if ($res.status -ne 'done') {
            Write-Warning ("Scenario {0} run {1} ended with status={2}" -f $s.Name, $i, $res.status)
        }
    }
}

$resultsCsv = Join-Path $reportRoot 'results.csv'
$results | Export-Csv -NoTypeInformation -Encoding UTF8 -LiteralPath $resultsCsv

function Avg([object[]]$items, [string]$prop) {
    if ($items.Count -eq 0) { return 0.0 }
    return ($items | Measure-Object -Property $prop -Average).Average
}

$wifi = @($results | Where-Object { $_.scenario -eq 'A_wifi_only' -and $_.status -eq 'done' })
$cable = @($results | Where-Object { $_.scenario -eq 'B_cable_only' -and $_.status -eq 'done' })
$dual = @($results | Where-Object { $_.scenario -eq 'C_dual' -and $_.status -eq 'done' })

$wifiAvg = [double](Avg $wifi 'mbps_avg')
$cableAvg = [double](Avg $cable 'mbps_avg')
$dualAvg = [double](Avg $dual 'mbps_avg')
$bestSingle = [math]::Max($wifiAvg, $cableAvg)
$required = $bestSingle * (1.0 + ($DualGainThresholdPct / 100.0))
$dualPass = ($dualAvg -ge $required)

$summary = [pscustomobject]@{
    report_root                  = $reportRoot
    api_base                     = $api
    strategy                     = $ChannelStrategy
    iterations                   = $Iterations
    dual_gain_threshold_pct      = $DualGainThresholdPct
    avg_wifi_mbps                = [math]::Round($wifiAvg, 2)
    avg_cable_mbps               = [math]::Round($cableAvg, 2)
    avg_dual_mbps                = [math]::Round($dualAvg, 2)
    best_single_mbps             = [math]::Round($bestSingle, 2)
    required_dual_mbps           = [math]::Round($required, 2)
    pass_functional_all_done      = ($results | Where-Object { -not $_.pass_functional }).Count -eq 0
    pass_dual_vs_single_threshold = $dualPass
    decision                      = if ((($results | Where-Object { -not $_.pass_functional }).Count -eq 0) -and $dualPass) { 'APPROVED' } else { 'REVIEW_REQUIRED' }
}

$summaryPath = Join-Path $reportRoot 'summary.json'
$summary | ConvertTo-Json -Depth 10 | Out-File -Encoding UTF8 $summaryPath

Write-Host ''
Write-Host '=== Benchmark Summary ==='
Write-Host ("Report: {0}" -f $reportRoot)
Write-Host ("Wi-Fi avg : {0} Mbps" -f ([math]::Round($wifiAvg, 2)))
Write-Host ("Cable avg : {0} Mbps" -f ([math]::Round($cableAvg, 2)))
Write-Host ("Dual avg  : {0} Mbps" -f ([math]::Round($dualAvg, 2)))
Write-Host ("Dual pass threshold ({0}%): {1}" -f $DualGainThresholdPct, $dualPass)
