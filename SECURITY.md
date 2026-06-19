# Security Policy

## Supported Status

MultiSend is under active development and has not yet had a public stable release.

## Reporting Vulnerabilities

When `Codyte/MultiSend` becomes public, report vulnerabilities through GitHub private vulnerability reporting if enabled. Until then, report privately to the repository owner.

Do not open public issues for:

- Remote command execution.
- Arbitrary file write/read.
- Path traversal.
- Authentication or trust-boundary bypass.
- Secret or log exposure.

## Sensitive Data Rules

Never commit:

- Logs from `%LOCALAPPDATA%\MultiSend\logs` or `C:\ProgramData\MultiSend\logs`.
- User config from `%APPDATA%\MultiSend`.
- Runtime state from `%LOCALAPPDATA%\MultiSend`.
- Received/downloaded files.
- Installer backups from `MultiSend/dist/Old`.
- Credentials, tokens, API keys, or local agent/tool configuration.

## Network Model

MultiSend is intended for trusted LAN use. Before a public release, review firewall rules, control API access checks, path validation, and remote-send/pull trust assumptions.
