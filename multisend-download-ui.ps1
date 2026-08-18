[CmdletBinding()]
param(
    [string]$ProtocolUrl,
    [string]$SendJobId
)

$ErrorActionPreference = 'Stop'
$launcher = Join-Path $PSScriptRoot 'multisend-launcher.ps1'
if (-not (Test-Path -LiteralPath $launcher -PathType Leaf)) {
    throw "MultiSend launcher not found: $launcher"
}

$launch = @{ OpenWebUI = $true }
if (-not [string]::IsNullOrWhiteSpace($ProtocolUrl)) { $launch.ProtocolUrl = $ProtocolUrl }
if (-not [string]::IsNullOrWhiteSpace($SendJobId)) { $launch.JobId = $SendJobId }

& $launcher @launch
exit $LASTEXITCODE
