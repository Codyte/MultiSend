# MultiSend

MultiSend is a Windows LAN transfer tool focused on sending files across local network peers using multiple interfaces, plus HTTP/HTTPS downloads with resumable chunk handling.

Target public repository: `Codyte/MultiSend`.

## Current Capabilities

- Send files peer-to-peer over LAN.
- Receive files and folders from another LAN PC through pull requests.
- Download HTTP/HTTPS files using ranged chunks.
- Track progress, retries, manifests, cancel/resume, and per-channel telemetry.
- Use detected interfaces such as Ethernet, Wi-Fi, and USB Ethernet.
- Monitor and start send, receive, and HTTP download operations from one source-driven form in the
  embedded responsive UI at the agent's local `/ui/` route.
- Install as a Windows node with Explorer integration, protocol handling, settings UI, and diagnostics.

## Repository Layout

- `cmd/`: Go entrypoints for the agent, sender, and receiver.
- `internal/`: transfer, download, discovery, configuration, protocol, and support packages.
- `scripts/`: benchmarks, diagnostics, and manual test utilities.
- `browser-extension/`: optional browser protocol helper.
- `dist/`: installer definition and ignored generated outputs.
- Root PowerShell scripts: runtime payload, installer helper, and setup rebuild entrypoint.

The incremental plan for replacing the PowerShell WinForms screens with an embedded web UI is
documented in [`docs/UI_REFORMULATION.md`](docs/UI_REFORMULATION.md).
The redacted configuration contract is documented in [`docs/CONFIG_API.md`](docs/CONFIG_API.md).
Download history recovery and retention are documented in
[`docs/DOWNLOAD_HISTORY.md`](docs/DOWNLOAD_HISTORY.md).
LAN send and pull recovery are documented in
[`docs/OPERATION_HISTORY.md`](docs/OPERATION_HISTORY.md).
Diagnostic and self-cleaning smoke tests are documented in
[`docs/SMOKE_TESTS.md`](docs/SMOKE_TESTS.md).
Installation, repair, firewall scope, persistence, and rollback are documented in
[`docs/INSTALLATION.md`](docs/INSTALLATION.md).
The restricted diagnostic split CLIs are documented in
[`docs/SPLIT_CLI.md`](docs/SPLIT_CLI.md), and the current test/coverage audit is in
[`docs/QUALITY_AUDIT.md`](docs/QUALITY_AUDIT.md).

## Build

Prerequisites:

- Windows.
- Go matching `go.mod`.
- PowerShell 5+.
- Inno Setup 6 when rebuilding the installer.

Run:

```powershell
go test ./...
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\rebuild-multisend-setup.ps1
```

The installer is generated at:

```text
dist/MultiSendSetup.exe
```

## Diagnostics

Runtime logs are written locally, not into the repository:

- Agent: `%LOCALAPPDATA%\MultiSend\logs\agent.log`
- Launcher: `%LOCALAPPDATA%\MultiSend\logs\launcher.log`
- Transfer UI: `%LOCALAPPDATA%\MultiSend\logs\download-ui.log`
- Installer: `C:\ProgramData\MultiSend\logs\install.log`

The agent also exposes `agent_log_path` through `/health` and `runtime.json`.

## Public Release Status

This repository is being prepared for publication at `github.com/Codyte/MultiSend`, but it is not ready to publish until the release checklist is completed.

Before making it public:

- Review tracked binary artifacts and repository history.
- Run the publication checklist in `docs/PUBLICATION_CHECKLIST.md`.
- Verify no logs, local configs, received files, or secrets are present.

## License

MultiSend is licensed under the MIT License. See `LICENSE`.
