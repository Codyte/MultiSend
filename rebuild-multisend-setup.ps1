[CmdletBinding()]
param(
    [switch]$SkipTests
)

$ErrorActionPreference = 'Stop'

function Write-Step {
    param([string]$Message)
    Write-Host ("[STEP] {0}" -f $Message)
}

function Resolve-IsccPath {
    $candidates = @(
        (Join-Path $env:LOCALAPPDATA 'Programs\Inno Setup 6\ISCC.exe'),
        'C:\Program Files (x86)\Inno Setup 6\ISCC.exe',
        'C:\Program Files\Inno Setup 6\ISCC.exe'
    )
    foreach ($p in $candidates) {
        if (Test-Path -LiteralPath $p) { return $p }
    }
    throw 'ISCC.exe not found. Install Inno Setup 6.'
}

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$distDir = Join-Path $repoRoot 'dist'
$oldDir = Join-Path $distDir 'Old'
$setupExe = Join-Path $distDir 'MultiSendSetup.exe'
$setupIss = Join-Path $distDir 'MultiSendSetup.iss'

New-Item -ItemType Directory -Force -Path $distDir | Out-Null

if (-not (Test-Path -LiteralPath $setupIss)) {
    throw "Missing ISS file: $setupIss"
}

New-Item -ItemType Directory -Force -Path $oldDir | Out-Null

if (Test-Path -LiteralPath $setupExe) {
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $backup = Join-Path $oldDir ("MultiSendSetup-{0}.exe" -f $stamp)
    Copy-Item -LiteralPath $setupExe -Destination $backup -Force
    Write-Step ("Archived previous setup: {0}" -f $backup)
}

Push-Location $repoRoot
try {
    if (-not $SkipTests) {
        Write-Step 'Running go test ./...'
        go test ./...
    }

    Write-Step 'Building multisend-agent.exe'
    go build -o multisend-agent.exe ./cmd/multisend-agent
    Write-Step 'Building multisend.exe'
    go build -o multisend.exe ./cmd/multisend
    Write-Step 'Building multirecv.exe'
    go build -o multirecv.exe ./cmd/multirecv

    $syncMap = @(
        @{ Src = (Join-Path $repoRoot 'install-multisend-node-v5.ps1'); Dst = (Join-Path $distDir 'install-multisend-node-v5.ps1') },
        @{ Src = (Join-Path $repoRoot 'multirecv.exe'); Dst = (Join-Path $distDir 'multirecv.exe') },
        @{ Src = (Join-Path $repoRoot 'multisend.exe'); Dst = (Join-Path $distDir 'multisend.exe') },
        @{ Src = (Join-Path $repoRoot 'multisend-agent.exe'); Dst = (Join-Path $distDir 'multisend-agent.exe') },
        @{ Src = (Join-Path $repoRoot 'multisend-download-ui.ps1'); Dst = (Join-Path $distDir 'multisend-download-ui.ps1') },
        @{ Src = (Join-Path $repoRoot 'multisend-launcher.ps1'); Dst = (Join-Path $distDir 'multisend-launcher.ps1') },
        @{ Src = (Join-Path $repoRoot 'multisend-settings-ui.ps1'); Dst = (Join-Path $distDir 'multisend-settings-ui.ps1') },
        @{ Src = (Join-Path $repoRoot 'browser-extension\manifest.json'); Dst = (Join-Path $distDir 'browser-extension\manifest.json') },
        @{ Src = (Join-Path $repoRoot 'browser-extension\background.js'); Dst = (Join-Path $distDir 'browser-extension\background.js') },
        @{ Src = (Join-Path $repoRoot 'browser-extension\README.md'); Dst = (Join-Path $distDir 'browser-extension\README.md') }
    )

    $triageSrcPrimary = Join-Path $repoRoot 'multisend-triage-blindado.ps1'
    if (Test-Path -LiteralPath $triageSrcPrimary) {
        $syncMap += @{ Src = $triageSrcPrimary; Dst = (Join-Path $distDir 'multisend-triage-blindado.ps1') }
    }

    foreach ($m in $syncMap) {
        if (-not (Test-Path -LiteralPath $m.Src)) {
            throw ("Missing source file: {0}" -f $m.Src)
        }
        $dstParent = Split-Path -Parent $m.Dst
        if (-not [string]::IsNullOrWhiteSpace($dstParent)) {
            New-Item -ItemType Directory -Force -Path $dstParent | Out-Null
        }
        try {
            $srcFull = (Resolve-Path -LiteralPath $m.Src).Path
            $dstFull = [System.IO.Path]::GetFullPath($m.Dst)
            if ([string]::Equals($srcFull, $dstFull, [System.StringComparison]::OrdinalIgnoreCase)) {
                Write-Step ("Synced (already up to date): {0}" -f $m.Dst)
                continue
            }
        } catch {}
        Copy-Item -LiteralPath $m.Src -Destination $m.Dst -Force
        Write-Step ("Synced: {0}" -f $m.Dst)
    }

    $iscc = Resolve-IsccPath
    Write-Step ("Compiling setup with ISCC: {0}" -f $iscc)
    & $iscc $setupIss

    if (-not (Test-Path -LiteralPath $setupExe)) {
        throw "Setup EXE was not generated: $setupExe"
    }

    $item = Get-Item -LiteralPath $setupExe
    $hash = (Get-FileHash -LiteralPath $setupExe -Algorithm SHA256).Hash
    Write-Host ''
    Write-Host '=== MultiSend Setup Rebuild Complete ==='
    Write-Host ("Path : {0}" -f $item.FullName)
    Write-Host ("Size : {0}" -f $item.Length)
    Write-Host ("Time : {0}" -f $item.LastWriteTime)
    Write-Host ("SHA256: {0}" -f $hash)
}
finally {
    Pop-Location
}
