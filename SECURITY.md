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
- Installer backups from `dist/Old`.
- Credentials, tokens, API keys, or local agent/tool configuration.

## Network Model

MultiSend is intended for trusted LAN use. Before a public release, review firewall rules, control API access checks, path validation, and remote-send/pull trust assumptions.

Diagnostic output redacts node secrets, tokens, and passwords. Use the explicit native settings
flow to copy or regenerate a shared secret; do not rely on `--doctor` or `--print-config` to reveal
it.

The local API and embedded web UI listen only on `127.0.0.1`. Requests with a non-loopback `Host`
are rejected, and browser requests that include `Origin` must match the API origin exactly. The web
UI is served with a restrictive Content Security Policy and does not load remote scripts or styles.
File paths passed by the local launcher are removed from the browser URL immediately after the
form is populated, so they are not retained as query parameters in the current history entry.

The control API is a separate trust boundary because it listens on LAN interfaces. Enabling
`require_auth` requires paired nodes to share the same secret; `remote_send_roots` must remain
restricted to directories intentionally exposed to peers. Pull requests send the shared secret
only in the Bearer header when authentication is enabled, and response bodies from peers are
bounded. Authentication fails closed if a required secret is unavailable.

Remote-send authorization compares filesystem-resolved paths. A symlink or Windows junction below
an allowed root cannot expose a target outside that root. Node secrets are generated exclusively
from the operating-system cryptographic random source; there is no predictable fallback.

Receive destinations use the same canonical-path boundary: existing symlink or junction ancestors
are resolved before accepting a path below `receive_path`. Incoming chunks are written to a
session-local temporary file, synchronized, checked against the declared SHA-256 and only then
published atomically. Corrupt manifests are preserved for diagnosis instead of being silently
replaced.

The diagnostic `multisend.exe` and `multirecv.exe` pair validates chunk hashes and safe transfer
identifiers. Optional CLI authentication reads `MULTISEND_NODE_SECRET` from the environment so the
secret does not appear in the process command line. The embedded agent remains the supported path
for authenticated operational transfers.
