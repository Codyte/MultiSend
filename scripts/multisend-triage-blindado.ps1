# ====================== BEGIN NAV INDEX ======================
# NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
#   L89    Save-Utf8NoBomText
#   L102   Save-Json
#   L111   Read-TextUtf8
#   L120   Read-JsonFileSafe
#   L132   Sanitize-ConfigFile
#   L150   Get-AgentExeCandidates
#   L167   Test-AgentProcessRunning
#   L171   Start-Agent
#   L182   Stop-Agent
#   L189   Get-LocalApiPortsFromState
#   L218   Invoke-JsonRequest
#   L280   Test-AgentHealth
#   L290   Resolve-AgentApiBase
#   L313   Get-RemoteHostAndPort
#   L363   Convert-Ipv4ToUInt32
#   L370   Test-SameSubnet
#   L389   Get-LocalInterfaces
#   L404   Select-BestPeer
#   L450   Select-LocalInterfaceForPeer
#   L466   Ensure-ManualInterfaceConfig
#   L482   New-ProbeFile
#   L493   Invoke-SendTransfer
#   L549   Wait-JobTerminal
#   L585   Test-RetryableNetworkError
#   L601   Record-Evidence
# ======================= END NAV INDEX =======================

param()

Set-StrictMode -Version 2
$ErrorActionPreference = 'Stop'

$scriptPath = $null
if ($PSCommandPath) {
    $scriptPath = $PSCommandPath
} elseif ($MyInvocation -and $MyInvocation.MyCommand) {
    $cmdObj = $MyInvocation.MyCommand
    $hasPathProp = $cmdObj.PSObject.Properties.Name -contains 'Path'
    if ($hasPathProp) {
        $candidate = $cmdObj.Path
        if (-not [string]::IsNullOrWhiteSpace($candidate)) {
            $scriptPath = $candidate
        }
    }
}

if (-not [string]::IsNullOrWhiteSpace($scriptPath)) {
    $ScriptRoot = Split-Path -Parent $scriptPath
} else {
    $ScriptRoot = (Get-Location).Path
}
if ([string]::IsNullOrWhiteSpace($ScriptRoot)) {
    $ScriptRoot = $PSScriptRoot
}

$ConfigPath = Join-Path $env:APPDATA 'MultiSend\config.json'
$RuntimePath = Join-Path $env:LOCALAPPDATA 'MultiSend\runtime.json'
$DesktopRoot = [Environment]::GetFolderPath([Environment+SpecialFolder]::DesktopDirectory)
if ([string]::IsNullOrWhiteSpace($DesktopRoot)) {
    $DesktopRoot = Join-Path $env:USERPROFILE 'Desktop'
}
$Stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$ReportDir = Join-Path $DesktopRoot "MultiSend-Blindado-$Stamp"
$null = New-Item -ItemType Directory -Force -Path $ReportDir

$State = [ordered]@{
    started_at = (Get-Date).ToString('o')
    ended_at = $null
    result = 'FAIL'
    report_dir = $ReportDir
    config_path = $ConfigPath
    runtime_path = $RuntimePath
    api_base = $null
    api_port = $null
    peer = $null
    retry_used = $false
    retry_reason = $null
    selected_local_ip = $null
    selected_interface = $null
    probe_file = $null
    send_attempts = @()
    poll_history = @()
    terminal_job = $null
    error = $null
}

function Save-Utf8NoBomText {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Text
    )
    $dir = Split-Path -Parent $Path
    if (-not [string]::IsNullOrWhiteSpace($dir)) {
        $null = New-Item -ItemType Directory -Force -Path $dir
    }
    $enc = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($Path, $Text, $enc)
}

function Save-Json {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)]$Object
    )
    $json = $Object | ConvertTo-Json -Depth 30
    Save-Utf8NoBomText -Path $Path -Text $json
}

function Read-TextUtf8 {
    param([Parameter(Mandatory = $true)][string]$Path)
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
        $bytes = $bytes[3..($bytes.Length - 1)]
    }
    return [System.Text.Encoding]::UTF8.GetString($bytes)
}

function Read-JsonFileSafe {
    param([Parameter(Mandatory = $true)][string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) {
        return $null
    }
    $text = Read-TextUtf8 -Path $Path
    if ([string]::IsNullOrWhiteSpace($text)) {
        return $null
    }
    return $text | ConvertFrom-Json
}

function Sanitize-ConfigFile {
    if (-not (Test-Path -LiteralPath $ConfigPath)) {
        return $null
    }
    $bytes = [System.IO.File]::ReadAllBytes($ConfigPath)
    $hasBom = $bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF
    if ($hasBom) {
        $cleanBytes = $bytes[3..($bytes.Length - 1)]
    } else {
        $cleanBytes = $bytes
    }
    $text = [System.Text.Encoding]::UTF8.GetString($cleanBytes)
    $cfg = $text | ConvertFrom-Json
    $normalized = $cfg | ConvertTo-Json -Depth 30
    Save-Utf8NoBomText -Path $ConfigPath -Text $normalized
    return $cfg
}

function Get-AgentExeCandidates {
    $candidates = @(
        (Join-Path $ScriptRoot 'multisend-agent.exe'),
        (Join-Path (Split-Path -Parent $ScriptRoot) 'multisend-agent.exe'),
        'C:\Program Files\MultiSend\bin\multisend-agent.exe'
    )
    $out = New-Object System.Collections.Generic.List[string]
    foreach ($p in $candidates) {
        if (-not [string]::IsNullOrWhiteSpace($p) -and (Test-Path -LiteralPath $p)) {
            if (-not $out.Contains($p)) {
                $out.Add($p)
            }
        }
    }
    return $out
}

function Test-AgentProcessRunning {
    return [bool](Get-Process -Name 'multisend-agent' -ErrorAction SilentlyContinue)
}

function Start-Agent {
    $candidates = Get-AgentExeCandidates
    if ($candidates.Count -eq 0) {
        throw 'multisend-agent.exe not found in dist or Program Files'
    }
    if (Test-AgentProcessRunning) {
        return
    }
    Start-Process -FilePath $candidates[0] -WindowStyle Hidden | Out-Null
}

function Stop-Agent {
    $procs = Get-Process -Name 'multisend-agent' -ErrorAction SilentlyContinue
    foreach ($p in @($procs)) {
        try { Stop-Process -Id $p.Id -Force -ErrorAction Stop } catch {}
    }
}

function Get-LocalApiPortsFromState {
    $ports = New-Object System.Collections.Generic.List[int]
    foreach ($path in @($RuntimePath, $ConfigPath)) {
        $obj = Read-JsonFileSafe -Path $path
        if ($null -eq $obj) { continue }
        if ($obj.local_api_port) { $ports.Add([int]$obj.local_api_port) }
        if ($obj.selected_ports -and $obj.selected_ports.local_api) { $ports.Add([int]$obj.selected_ports.local_api) }
        if ($obj.local_api_port_range -and $obj.local_api_port_range.start -and $obj.local_api_port_range.end) {
            for ($p = [int]$obj.local_api_port_range.start; $p -le [int]$obj.local_api_port_range.end; $p++) {
                $ports.Add($p)
            }
        }
    }
    if ($ports.Count -eq 0) {
        for ($p = 56221; $p -le 56230; $p++) {
            $ports.Add($p)
        }
    }
    $seen = @{}
    $unique = New-Object System.Collections.Generic.List[int]
    foreach ($p in $ports) {
        if (-not $seen.ContainsKey($p)) {
            $seen[$p] = $true
            $unique.Add($p)
        }
    }
    return $unique
}

function Invoke-JsonRequest {
    param(
        [Parameter(Mandatory = $true)][ValidateSet('GET','POST')][string]$Method,
        [Parameter(Mandatory = $true)][string]$Url,
        [Parameter()][object]$Body,
        [int]$TimeoutMs = 3000
    )

    $req = [System.Net.HttpWebRequest]::Create($Url)
    $req.Method = $Method
    $req.Accept = 'application/json'
    $req.ContentType = 'application/json; charset=utf-8'
    $req.Timeout = $TimeoutMs
    $req.ReadWriteTimeout = $TimeoutMs
    $req.AllowAutoRedirect = $false

    if ($Method -eq 'POST') {
        $payload = if ($null -eq $Body) { '{}' } else { $Body | ConvertTo-Json -Depth 30 }
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($payload)
        $req.ContentLength = $bytes.Length
        $stream = $req.GetRequestStream()
        try {
            $stream.Write($bytes, 0, $bytes.Length)
        } finally {
            $stream.Close()
        }
    }

    try {
        $resp = $req.GetResponse()
        $code = [int]$resp.StatusCode
    } catch [System.Net.WebException] {
        if ($_.Exception.Response) {
            $resp = $_.Exception.Response
            $code = [int]$resp.StatusCode
        } else {
            throw
        }
    }

    try {
        $reader = New-Object System.IO.StreamReader($resp.GetResponseStream(), [System.Text.Encoding]::UTF8)
        $text = $reader.ReadToEnd()
        $reader.Close()
    } finally {
        $resp.Close()
    }

    if ($code -lt 200 -or $code -ge 300) {
        $msg = $text.Trim()
        if ([string]::IsNullOrWhiteSpace($msg)) {
            throw "HTTP $code"
        }
        throw ("HTTP {0}: {1}" -f $code, $msg)
    }

    if ([string]::IsNullOrWhiteSpace($text)) {
        return $null
    }
    return $text | ConvertFrom-Json
}

function Test-AgentHealth {
    param([Parameter(Mandatory = $true)][string]$ApiBase)
    try {
        $null = Invoke-JsonRequest -Method GET -Url "$ApiBase/health" -TimeoutMs 1500
        return $true
    } catch {
        return $false
    }
}

function Resolve-AgentApiBase {
    $ports = Get-LocalApiPortsFromState
    foreach ($port in $ports) {
        $api = "http://127.0.0.1:$port"
        if (Test-AgentHealth -ApiBase $api) {
            return [pscustomobject]@{ api_base = $api; port = $port }
        }
    }
    Stop-Agent
    Start-Sleep -Milliseconds 500
    Start-Agent
    for ($i = 0; $i -lt 20; $i++) {
        foreach ($port in (56221..56230)) {
            $api = "http://127.0.0.1:$port"
            if (Test-AgentHealth -ApiBase $api) {
                return [pscustomobject]@{ api_base = $api; port = $port }
            }
        }
        Start-Sleep -Milliseconds 500
    }
    return $null
}

function Get-RemoteHostAndPort {
    param([Parameter(Mandatory = $true)]$Peer)
    $host = $null
    $port = $null

    foreach ($prop in @('addr', 'address', 'ip', 'host', 'peer_address')) {
        if ($Peer.PSObject.Properties.Name -contains $prop) {
            $val = [string]$Peer.$prop
            if (-not [string]::IsNullOrWhiteSpace($val)) {
                if ($val -match '^\[(.+)\]:(\d+)$') {
                    $host = $matches[1]
                    $port = [int]$matches[2]
                    break
                }
                if ($val -match '^(.+):(\d+)$' -and $val -notmatch '^\d{1,3}(\.\d{1,3}){3}$') {
                    $host = $matches[1]
                    $port = [int]$matches[2]
                    break
                }
                $host = $val
                break
            }
        }
    }

    foreach ($prop in @('port', 'transfer_port', 'transferPort', 'selected_port', 'selectedPort')) {
        if ($Peer.PSObject.Properties.Name -contains $prop) {
            $tmp = [string]$Peer.$prop
            if ($tmp -match '^\d+$') {
                $port = [int]$tmp
                break
            }
        }
    }

    if ($null -eq $port -and $host -match '^(.+):(\d+)$') {
        $host = $matches[1]
        $port = [int]$matches[2]
    }

    if ($null -eq $port -and $Peer.PSObject.Properties.Name -contains 'selected_ports') {
        $sp = $Peer.selected_ports
        if ($sp -and $sp.transfer) {
            $port = [int]$sp.transfer
        }
    }

    return [pscustomobject]@{ host = $host; port = $port }
}

function Convert-Ipv4ToUInt32 {
    param([Parameter(Mandatory = $true)][string]$Ip)
    $addr = [System.Net.IPAddress]::Parse($Ip).GetAddressBytes()
    [Array]::Reverse($addr)
    return [BitConverter]::ToUInt32($addr, 0)
}

function Test-SameSubnet {
    param(
        [Parameter(Mandatory = $true)][string]$IpA,
        [Parameter(Mandatory = $true)][string]$IpB,
        [Parameter(Mandatory = $true)][int]$PrefixLength
    )
    if ($PrefixLength -le 0 -or $PrefixLength -gt 32) {
        return $false
    }
    try {
        $mask = if ($PrefixLength -eq 32) { [uint32]::MaxValue } else { [uint32](([uint64]::MaxValue) -shr (64 - $PrefixLength)) }
        $a = [uint32](Convert-Ipv4ToUInt32 -Ip $IpA)
        $b = [uint32](Convert-Ipv4ToUInt32 -Ip $IpB)
        return (($a -band $mask) -eq ($b -band $mask))
    } catch {
        return $false
    }
}

function Get-LocalInterfaces {
    $rows = @()
    try {
        $rows = @(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Stop | Where-Object {
            $_.IPAddress -and
            $_.PrefixLength -ne $null -and
            $_.IPAddress -notlike '127.*' -and
            $_.IPAddress -notlike '169.254.*'
        } | Select-Object IPAddress, PrefixLength, InterfaceAlias)
    } catch {
        $rows = @()
    }
    return $rows
}

function Select-BestPeer {
    param(
        [Parameter(Mandatory = $true)]$Peers,
        [Parameter(Mandatory = $true)]$LocalInterfaces
    )

    $candidates = New-Object System.Collections.Generic.List[object]
    foreach ($peer in @($Peers)) {
        if ($null -eq $peer) { continue }
        $hp = Get-RemoteHostAndPort -Peer $peer
        if ([string]::IsNullOrWhiteSpace($hp.host) -or $null -eq $hp.port) { continue }
        $ip = [string]$hp.host.Trim()
        $score = 3
        if ($ip -match '^192\.168\.0\.\d{1,3}$') {
            $score = 0
        } else {
            foreach ($li in $LocalInterfaces) {
                if ([string]::IsNullOrWhiteSpace($li.IPAddress)) { continue }
                if (Test-SameSubnet -IpA $ip -IpB $li.IPAddress -PrefixLength ([int]$li.PrefixLength)) {
                    $score = 1
                    break
                }
            }
        }
        if ($ip -match '^(10|172\.(1[6-9]|2\d|3[0-1]))\.') {
            $score = [Math]::Min($score, 2)
        }
        $name = ''
        foreach ($prop in @('name', 'display_name', 'node_name', 'node_id')) {
            if ($peer.PSObject.Properties.Name -contains $prop) {
                $name = [string]$peer.$prop
                if (-not [string]::IsNullOrWhiteSpace($name)) { break }
            }
        }
        $candidates.Add([pscustomobject]@{
            peer = $peer
            host = $ip
            port = [int]$hp.port
            name = $name
            score = $score
        })
    }
    if ($candidates.Count -eq 0) { return $null }
    return $candidates | Sort-Object @{Expression='score'; Ascending=$true}, @{Expression='host'; Ascending=$true}, @{Expression='name'; Ascending=$true} | Select-Object -First 1
}

function Select-LocalInterfaceForPeer {
    param(
        [Parameter(Mandatory = $true)][string]$PeerHost,
        [Parameter(Mandatory = $true)]$LocalInterfaces
    )
    $matches = @()
    foreach ($li in $LocalInterfaces) {
        if (Test-SameSubnet -IpA $PeerHost -IpB $li.IPAddress -PrefixLength ([int]$li.PrefixLength)) {
            $matches += $li
        }
    }
    if ($matches.Count -eq 0) { return $null }
    $preferred = $matches | Sort-Object @{Expression={ if ($_.IPAddress -match '^192\.168\.0\.') { 0 } else { 1 } }}, @{Expression='InterfaceAlias'; Ascending=$true} | Select-Object -First 1
    return $preferred
}

function Ensure-ManualInterfaceConfig {
    param(
        [Parameter(Mandatory = $true)][string]$InterfaceAlias
    )
    $cfg = Read-JsonFileSafe -Path $ConfigPath
    if ($null -eq $cfg) {
        throw "config unavailable: $ConfigPath"
    }
    $cfg.interface_policy = 'manual'
    $cfg.manual_interfaces = @($InterfaceAlias)
    $cfg.selected_ports = $cfg.selected_ports
    $json = $cfg | ConvertTo-Json -Depth 30
    Save-Utf8NoBomText -Path $ConfigPath -Text $json
    return $cfg
}

function New-ProbeFile {
    $path = Join-Path $ReportDir 'probe.txt'
    $text = @"
MultiSend blindado probe
timestamp: $(Get-Date -Format o)
host: $env:COMPUTERNAME
"@
    Save-Utf8NoBomText -Path $path -Text $text
    return $path
}

function Invoke-SendTransfer {
    param(
        [Parameter(Mandatory = $true)][string]$ApiBase,
        [Parameter(Mandatory = $true)][string]$ProbePath,
        [Parameter(Mandatory = $true)][string]$PeerAddress
    )
    $body = [ordered]@{
        file_path = $ProbePath
        peer_address = $PeerAddress
    }
    $json = $body | ConvertTo-Json -Depth 10
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
    $req = [System.Net.HttpWebRequest]::Create("$ApiBase/send")
    $req.Method = 'POST'
    $req.Accept = 'application/json'
    $req.ContentType = 'application/json; charset=utf-8'
    $req.Timeout = 5000
    $req.ReadWriteTimeout = 5000
    $req.ContentLength = $bytes.Length
    $stream = $req.GetRequestStream()
    try {
        $stream.Write($bytes, 0, $bytes.Length)
    } finally {
        $stream.Close()
    }
    try {
        $resp = $req.GetResponse()
        $code = [int]$resp.StatusCode
    } catch [System.Net.WebException] {
        if ($_.Exception.Response) {
            $resp = $_.Exception.Response
            $code = [int]$resp.StatusCode
        } else {
            throw
        }
    }
    try {
        $reader = New-Object System.IO.StreamReader($resp.GetResponseStream(), [System.Text.Encoding]::UTF8)
        $text = $reader.ReadToEnd()
        $reader.Close()
    } finally {
        $resp.Close()
    }
    if ($code -lt 200 -or $code -ge 300) {
        $detail = $text.Trim()
        if ([string]::IsNullOrWhiteSpace($detail)) {
            throw "send failed HTTP $code"
        }
        throw ("send failed HTTP {0}: {1}" -f $code, $detail)
    }
    if ([string]::IsNullOrWhiteSpace($text)) {
        return $null
    }
    return $text | ConvertFrom-Json
}

function Wait-JobTerminal {
    param(
        [Parameter(Mandatory = $true)][string]$ApiBase,
        [Parameter(Mandatory = $true)][string]$JobId,
        [int]$TimeoutSec = 180
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    $history = New-Object System.Collections.Generic.List[object]
    while ((Get-Date) -lt $deadline) {
        try {
            $job = Invoke-JsonRequest -Method GET -Url "$ApiBase/jobs/$JobId" -TimeoutMs 3000
            $snapshot = [pscustomobject]@{
                at = (Get-Date).ToString('o')
                job = $job
            }
            $history.Add($snapshot)
            if ($job -and $job.status -in @('done', 'failed', 'canceled')) {
                return [pscustomobject]@{
                    terminal = $job
                    history = $history
                }
            }
        } catch {
            $history.Add([pscustomobject]@{
                at = (Get-Date).ToString('o')
                error = $_.Exception.Message
            })
        }
        Start-Sleep -Seconds 1
    }
    return [pscustomobject]@{
        terminal = $null
        history = $history
    }
}

function Test-RetryableNetworkError {
    param([Parameter(Mandatory = $true)][string]$Message)
    $m = $Message.ToLowerInvariant()
    return (
        $m -match 'unreachable' -or
        $m -match 'network is unreachable' -or
        $m -match 'no route' -or
        $m -match 'timed out' -or
        $m -match 'timeout' -or
        $m -match 'wrong source ip' -or
        $m -match 'source ip' -or
        $m -match 'cannot assign requested address' -or
        $m -match 'connectex'
    )
}

function Record-Evidence {
    param(
        [Parameter(Mandatory = $true)]$ConfigBefore,
        [Parameter(Mandatory = $true)]$ConfigAfter,
        [Parameter(Mandatory = $true)]$Peers,
        [Parameter(Mandatory = $true)]$LocalInterfaces,
        [Parameter(Mandatory = $true)]$Attempts,
        [Parameter(Mandatory = $true)]$PollHistory,
        [Parameter(Mandatory = $true)]$Summary
    )
    Save-Json -Path (Join-Path $ReportDir 'summary.json') -Object $Summary
    if ($null -ne $ConfigBefore) {
        Save-Json -Path (Join-Path $ReportDir 'config-before.json') -Object $ConfigBefore
    }
    if ($null -ne $ConfigAfter) {
        Save-Json -Path (Join-Path $ReportDir 'config-after.json') -Object $ConfigAfter
    }
    Save-Json -Path (Join-Path $ReportDir 'peers.json') -Object $Peers
    Save-Json -Path (Join-Path $ReportDir 'local-interfaces.json') -Object $LocalInterfaces
    Save-Json -Path (Join-Path $ReportDir 'attempts.json') -Object $Attempts
    Save-Json -Path (Join-Path $ReportDir 'poll-history.json') -Object $PollHistory
}

try {
    $configBefore = Sanitize-ConfigFile
    $configAfter = $configBefore
    $agentInfo = Resolve-AgentApiBase
    if ($null -eq $agentInfo) {
        throw 'local API unavailable on ports 56221-56230'
    }
    $State.api_base = $agentInfo.api_base
    $State.api_port = $agentInfo.port

    $peersRaw = $null
    try {
        $peersRaw = Invoke-JsonRequest -Method GET -Url "$($State.api_base)/peers" -TimeoutMs 3000
    } catch {
        throw "failed to query peers: $($_.Exception.Message)"
    }
    $peers = @()
    if ($peersRaw -is [System.Array]) {
        $peers = @($peersRaw)
    } elseif ($null -ne $peersRaw) {
        foreach ($prop in @('value', 'peers')) {
            if ($peersRaw.PSObject.Properties.Name -contains $prop -and $peersRaw.$prop) {
                $peers = @($peersRaw.$prop)
                break
            }
        }
        if ($peers.Count -eq 0) {
            $peers = @($peersRaw)
        }
    }
    $localInterfaces = @(Get-LocalInterfaces)
    $bestPeer = Select-BestPeer -Peers $peers -LocalInterfaces $localInterfaces
    if ($null -eq $bestPeer) {
        throw 'no usable peers returned by /peers'
    }

    $peerAddress = '{0}:{1}' -f $bestPeer.host, $bestPeer.port
    $State.peer = [pscustomobject]@{
        host = $bestPeer.host
        port = $bestPeer.port
        address = $peerAddress
        name = $bestPeer.name
    }

    $probeFile = New-ProbeFile
    $State.probe_file = $probeFile
    $attempts = New-Object System.Collections.Generic.List[object]
    $pollHistory = New-Object System.Collections.Generic.List[object]
    $sendSucceeded = $false
    $terminal = $null

    for ($attempt = 1; $attempt -le 2; $attempt++) {
        try {
            $sendResp = Invoke-SendTransfer -ApiBase $State.api_base -ProbePath $probeFile -PeerAddress $peerAddress
            $jobId = $null
            if ($sendResp -and $sendResp.job_id) {
                $jobId = [string]$sendResp.job_id
            }
            $attemptRecord = [ordered]@{
                attempt = $attempt
                api_base = $State.api_base
                peer_address = $peerAddress
                response = $sendResp
                job_id = $jobId
            }
            if ([string]::IsNullOrWhiteSpace($jobId)) {
                throw 'send response did not return job_id'
            }
            $wait = Wait-JobTerminal -ApiBase $State.api_base -JobId $jobId -TimeoutSec 180
            foreach ($item in $wait.history) {
                $pollHistory.Add($item)
            }
            $terminal = $wait.terminal
            $attemptRecord.terminal = $terminal
            $attemptRecord.terminal_status = if ($terminal) { [string]$terminal.status } else { $null }
            $attemptRecord.terminal_message = if ($terminal) { [string]$terminal.message } else { 'timeout' }
            $attempts.Add([pscustomobject]$attemptRecord)
            if ($terminal -and $terminal.status -eq 'done') {
                $sendSucceeded = $true
                break
            }
            $retryableTerminal = $false
            if ($terminal -and $terminal.status -eq 'failed') {
                $retryableTerminal = Test-RetryableNetworkError -Message ([string]$terminal.message)
            }
            if ($attempt -eq 1 -and $terminal -and $retryableTerminal) {
                $State.retry_used = $true
                $State.retry_reason = [string]$terminal.message
                $localPick = Select-LocalInterfaceForPeer -PeerHost $bestPeer.host -LocalInterfaces $localInterfaces
                if ($null -eq $localPick) {
                    throw "retry requested but no local interface matches peer subnet $($bestPeer.host)"
                }
                $State.selected_local_ip = [string]$localPick.IPAddress
                $State.selected_interface = [string]$localPick.InterfaceAlias
                $configAfter = Ensure-ManualInterfaceConfig -InterfaceAlias $State.selected_interface
                Stop-Agent
                Start-Sleep -Milliseconds 300
                $agentInfo = Resolve-AgentApiBase
                if ($null -eq $agentInfo) {
                    throw 'API did not recover after agent restart'
                }
                $State.api_base = $agentInfo.api_base
                $State.api_port = $agentInfo.port
                continue
            }
            break
        } catch {
            $attempts.Add([pscustomobject]@{
                attempt = $attempt
                api_base = $State.api_base
                peer_address = $peerAddress
                error = $_.Exception.Message
            })
            $retryableCatch = Test-RetryableNetworkError -Message ([string]$_.Exception.Message)
            if ($attempt -eq 1 -and $retryableCatch) {
                $State.retry_used = $true
                $State.retry_reason = $_.Exception.Message
                $localPick = Select-LocalInterfaceForPeer -PeerHost $bestPeer.host -LocalInterfaces $localInterfaces
                if ($null -eq $localPick) {
                    throw
                }
                $State.selected_local_ip = [string]$localPick.IPAddress
                $State.selected_interface = [string]$localPick.InterfaceAlias
                $configAfter = Ensure-ManualInterfaceConfig -InterfaceAlias $State.selected_interface
                Stop-Agent
                Start-Sleep -Milliseconds 300
                $agentInfo = Resolve-AgentApiBase
                if ($null -eq $agentInfo) {
                    throw 'API did not recover after agent restart'
                }
                $State.api_base = $agentInfo.api_base
                $State.api_port = $agentInfo.port
                continue
            }
            throw
        }
    }

    $attemptArray = @($attempts | ForEach-Object { $_ })
    $pollArray = @($pollHistory | ForEach-Object { $_ })
    $State.ended_at = (Get-Date).ToString('o')
    $State.poll_history = $pollArray
    $State.send_attempts = $attemptArray
    $State.terminal_job = $terminal
    if ($sendSucceeded) {
        $State.result = 'PASS'
    } else {
        $State.result = 'FAIL'
        if (-not $State.error) {
            if ($terminal) {
                $State.error = [string]$terminal.message
            } else {
                $State.error = 'transfer did not reach done'
            }
        }
    }

    $summary = [ordered]@{
        result = $State.result
        api_base = $State.api_base
        api_port = $State.api_port
        peer = $State.peer
        retry_used = $State.retry_used
        retry_reason = $State.retry_reason
        selected_local_ip = $State.selected_local_ip
        selected_interface = $State.selected_interface
        probe_file = $State.probe_file
        report_dir = $State.report_dir
        started_at = $State.started_at
        ended_at = $State.ended_at
        error = $State.error
    }
    Record-Evidence -ConfigBefore $configBefore -ConfigAfter $configAfter -Peers $peers -LocalInterfaces $localInterfaces -Attempts $attemptArray -PollHistory $pollArray -Summary $summary

    if ($State.result -eq 'PASS') {
        Write-Host ("PASS | api={0} | peer={1} | evidence={2}" -f $State.api_base, $peerAddress, $ReportDir)
        exit 0
    }

    Write-Host ("FAIL | api={0} | peer={1} | evidence={2} | error={3}" -f $State.api_base, $peerAddress, $ReportDir, $State.error)
    exit 1
} catch {
    $State.ended_at = (Get-Date).ToString('o')
    $State.error = $_.Exception.Message
    $summary = [ordered]@{
        result = 'FAIL'
        api_base = $State.api_base
        api_port = $State.api_port
        peer = $State.peer
        retry_used = $State.retry_used
        retry_reason = $State.retry_reason
        selected_local_ip = $State.selected_local_ip
        selected_interface = $State.selected_interface
        probe_file = $State.probe_file
        report_dir = $State.report_dir
        started_at = $State.started_at
        ended_at = $State.ended_at
        error = $State.error
    }
    try {
        Record-Evidence -ConfigBefore $null -ConfigAfter (Read-JsonFileSafe -Path $ConfigPath) -Peers @() -LocalInterfaces @(Get-LocalInterfaces) -Attempts @() -PollHistory @() -Summary $summary
    } catch {}
    Write-Host ("FAIL | evidence={0} | error={1}" -f $ReportDir, $State.error)
    exit 1
}
