param(
    [string]$TargetPeerNodeId,
    [string]$TargetPeerAddress,
    [int]$TimeoutSec = 120,
    [switch]$SkipActiveSendTest
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function New-ReportDir {
    $root = Join-Path $env:USERPROFILE "Desktop\MultiSend-Triage"
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $dir = Join-Path $root $stamp
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    return $dir
}

function Save-Text {
    param([string]$Path, [string]$Text)
    [System.IO.File]::WriteAllText($Path, $Text, (New-Object System.Text.UTF8Encoding($false)))
}

function Save-Json {
    param([string]$Path, $Obj)
    $json = $Obj | ConvertTo-Json -Depth 12
    Save-Text -Path $Path -Text $json
}

function Run-CmdCapture {
    param([string]$Name, [scriptblock]$Script)
    try {
        $out = & $Script 2>&1 | Out-String
        return [pscustomobject]@{ name = $Name; ok = $true; output = $out.TrimEnd() }
    } catch {
        return [pscustomobject]@{ name = $Name; ok = $false; output = ($_.Exception.Message) }
    }
}

function Resolve-ApiBase {
    $runtimePath = Join-Path $env:LOCALAPPDATA 'MultiSend\runtime.json'
    $cfgPath = Join-Path $env:APPDATA 'MultiSend\config.json'
    $ports = New-Object System.Collections.Generic.List[int]

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

    $uniq = @{}
    foreach ($p in $ports) {
        if ($uniq.ContainsKey($p)) { continue }
        $uniq[$p] = $true
        $api = "http://127.0.0.1:$p"
        try {
            $h = Invoke-RestMethod -Uri "$api/health" -Method Get -TimeoutSec 2
            if ($h.ok) { return $api }
        } catch {}
    }
    return $null
}

function Wait-JobTerminal {
    param([string]$ApiBase, [string]$JobId, [int]$Timeout)
    $deadline = (Get-Date).AddSeconds($Timeout)
    $last = $null
    while ((Get-Date) -lt $deadline) {
        try {
            $job = Invoke-RestMethod -Uri "$ApiBase/jobs/$JobId" -Method Get -TimeoutSec 3
            $last = $job
            if ($job.status -in @('done', 'failed', 'canceled')) {
                return $job
            }
        } catch {}
        Start-Sleep -Seconds 1
    }
    return $last
}

$reportDir = New-ReportDir
$summary = [ordered]@{
    machine = $env:COMPUTERNAME
    user = $env:USERNAME
    started_at = (Get-Date).ToString("o")
    report_dir = $reportDir
    api_base = $null
    peers_count = 0
    active_send_test = [ordered]@{
        attempted = $false
        success = $false
        message = ""
        job_id = $null
        terminal_status = $null
    }
}

$captures = @()
$captures += Run-CmdCapture -Name "ps_version" -Script { $PSVersionTable | Out-String }
$captures += Run-CmdCapture -Name "installer_test" -Script { powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Program Files\MultiSend\bin\install-multisend-node-v5.ps1" -Test }
$captures += Run-CmdCapture -Name "doctor" -Script { & "C:\Program Files\MultiSend\bin\multisend-agent.exe" --doctor }
$captures += Run-CmdCapture -Name "ipconfig_all" -Script { ipconfig /all }
$captures += Run-CmdCapture -Name "route_print" -Script { route print }
$captures += Run-CmdCapture -Name "net_adapters" -Script { Get-NetAdapter | Format-Table -AutoSize | Out-String }
$captures += Run-CmdCapture -Name "net_ipconfig" -Script { Get-NetIPConfiguration | Format-List | Out-String }
$captures += Run-CmdCapture -Name "firewall_rules" -Script { Get-NetFirewallRule -DisplayName "MultiSend*" | Get-NetFirewallPortFilter | Format-List | Out-String }

foreach ($c in $captures) {
    Save-Text -Path (Join-Path $reportDir ($c.name + ".txt")) -Text $c.output
}

$cfgPath = Join-Path $env:APPDATA 'MultiSend\config.json'
$rtPath = Join-Path $env:LOCALAPPDATA 'MultiSend\runtime.json'
$installLog = 'C:\ProgramData\MultiSend\logs\install.log'
foreach ($p in @($cfgPath, $rtPath, $installLog)) {
    if (Test-Path -LiteralPath $p) {
        Copy-Item -LiteralPath $p -Destination (Join-Path $reportDir ([IO.Path]::GetFileName($p))) -Force
    }
}

$api = Resolve-ApiBase
$summary.api_base = $api
if ($api) {
    try {
        $health = Invoke-RestMethod -Uri "$api/health" -Method Get -TimeoutSec 3
        Save-Json -Path (Join-Path $reportDir "api-health.json") -Obj $health
    } catch {}
    try {
        $peers = @(Invoke-RestMethod -Uri "$api/peers" -Method Get -TimeoutSec 4)
        $summary.peers_count = $peers.Count
        Save-Json -Path (Join-Path $reportDir "api-peers.json") -Obj $peers
    } catch {}
    try {
        $jobs = @(Invoke-RestMethod -Uri "$api/jobs" -Method Get -TimeoutSec 4)
        Save-Json -Path (Join-Path $reportDir "api-jobs.json") -Obj $jobs
    } catch {}
}

if (-not $SkipActiveSendTest -and $api) {
    $summary.active_send_test.attempted = $true
    $peerNode = $TargetPeerNodeId
    $peerAddr = $TargetPeerAddress
    if ([string]::IsNullOrWhiteSpace($peerNode) -and [string]::IsNullOrWhiteSpace($peerAddr)) {
        try {
            $peers = @(Invoke-RestMethod -Uri "$api/peers" -Method Get -TimeoutSec 4)
            if ($peers.Count -gt 0) { $peerNode = [string]$peers[0].node_id }
        } catch {}
    }

    if (-not [string]::IsNullOrWhiteSpace($peerNode) -or -not [string]::IsNullOrWhiteSpace($peerAddr)) {
        $probe = Join-Path $env:TEMP "multisend-triage-probe.bin"
        $bytes = New-Object byte[] (196608)
        (New-Object System.Random).NextBytes($bytes)
        [System.IO.File]::WriteAllBytes($probe, $bytes)

        $body = [ordered]@{ file_path = $probe }
        if (-not [string]::IsNullOrWhiteSpace($peerNode)) { $body.peer_node_id = $peerNode }
        if (-not [string]::IsNullOrWhiteSpace($peerAddr)) { $body.peer_address = $peerAddr }

        try {
            $start = Invoke-RestMethod -Uri "$api/send" -Method Post -ContentType "application/json" -Body ($body | ConvertTo-Json -Depth 8) -TimeoutSec 8
            $jobId = [string]$start.job_id
            $summary.active_send_test.job_id = $jobId
            if (-not [string]::IsNullOrWhiteSpace($jobId)) {
                $terminal = Wait-JobTerminal -ApiBase $api -JobId $jobId -Timeout $TimeoutSec
                if ($terminal) {
                    Save-Json -Path (Join-Path $reportDir "active-send-terminal-job.json") -Obj $terminal
                    $summary.active_send_test.terminal_status = [string]$terminal.status
                    if ($terminal.status -eq 'done') {
                        $summary.active_send_test.success = $true
                        $summary.active_send_test.message = "active probe transfer succeeded"
                    } else {
                        $summary.active_send_test.message = [string]$terminal.message
                    }
                } else {
                    $summary.active_send_test.message = "job did not reach terminal status within timeout"
                }
            } else {
                $summary.active_send_test.message = "send API returned empty job_id"
            }
        } catch {
            $summary.active_send_test.message = $_.Exception.Message
        }
    } else {
        $summary.active_send_test.message = "no peer available for active test"
    }
}

$summary.ended_at = (Get-Date).ToString("o")
Save-Json -Path (Join-Path $reportDir "summary.json") -Obj $summary
Save-Text -Path (Join-Path $reportDir "README.txt") -Text @"
MultiSend triage batch generated this folder.

Key files:
- summary.json
- api-health.json / api-peers.json / api-jobs.json
- active-send-terminal-job.json (if send test ran)
- doctor.txt
- installer_test.txt
- firewall_rules.txt
- ipconfig_all.txt
- route_print.txt

If failure is "chunk exceeded retries", check:
1) api-peers.json (target exists and node_id/address)
2) active-send-terminal-job.json channels.*.last_error
3) firewall_rules.txt + route_print.txt + net_ipconfig.txt
"@

Write-Host "Triage concluído. Relatório: $reportDir"
if ($summary.active_send_test.attempted) {
    Write-Host ("Active send test: success={0} status={1} msg={2}" -f $summary.active_send_test.success, $summary.active_send_test.terminal_status, $summary.active_send_test.message)
}
