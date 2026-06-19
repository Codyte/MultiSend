# MultiSend

MultiSend is a Windows LAN transfer tool focused on sending files across local network peers using multiple interfaces, plus HTTP/HTTPS downloads with resumable chunk handling.

Target public repository: `Codyte/MultiSend`.

## Current Capabilities

- Send files peer-to-peer over LAN.
- Receive files and folders from another LAN PC through pull requests.
- Download HTTP/HTTPS files using ranged chunks.
- Track progress, retries, manifests, cancel/resume, and per-channel telemetry.
- Use detected interfaces such as Ethernet, Wi-Fi, and USB Ethernet.
- Install as a Windows node with Explorer integration, protocol handling, settings UI, and diagnostics.

## Repository Layout

- `MultiSend/cmd/multisend-agent`: local/control API, receiver, LAN pull orchestration, diagnostics.
- `MultiSend/internal/transfer`: P2P sender engine.
- `MultiSend/internal/download`: HTTP/HTTPS download manager.
- `MultiSend/internal/discovery`: LAN peer discovery.
- `MultiSend/internal/config`: node configuration and migration defaults.
- `MultiSend/*.ps1`: launcher, transfer UI, settings UI, installer helper, triage scripts.
- `MultiSend/dist`: generated installer output; only `MultiSendSetup.iss` is versioned.

## Build

Prerequisites:

- Windows.
- Go matching `MultiSend/go.mod`.
- PowerShell 5+.
- Inno Setup 6 when rebuilding the installer.

Run:

```powershell
cd MultiSend
go test ./...
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\rebuild-multisend-setup.ps1
```

The installer is generated at:

```text
MultiSend/dist/MultiSendSetup.exe
```

## Diagnostics

Runtime logs are written locally, not into the repository:

- Agent: `%LOCALAPPDATA%\MultiSend\logs\agent.log`
- Launcher: `%LOCALAPPDATA%\MultiSend\logs\launcher.log`
- Transfer UI: `%LOCALAPPDATA%\MultiSend\logs\download-ui.log`
- Installer: `C:\ProgramData\MultiSend\logs\install.log`

The agent also exposes `agent_log_path` through `/health` and `runtime.json`.

## Optional External Services

The memory package contains optional OpenAI embedding support. It is disabled by default and only runs when both environment variables are set:

```text
MULTISEND_ENABLE_OPENAI_EMBEDDINGS=true
OPENAI_API_KEY=<key>
```

Do not commit API keys or local environment files.

## Public Release Status

This repository is being prepared for publication at `github.com/Codyte/MultiSend`, but it is not ready to publish until the release checklist is completed.

Before making it public:

- Review tracked binary artifacts and repository history.
- Run the publication checklist in `docs/PUBLICATION_CHECKLIST.md`.
- Verify no logs, local configs, received files, or secrets are present.

## License

MultiSend is licensed under the MIT License. See `LICENSE`.
