# Contributing

This project is not yet open for public contributions, but the repository is being prepared for that workflow.

## Development Expectations

- Keep changes scoped and testable.
- Run `go test ./...` from `MultiSend` before submitting changes.
- For PowerShell UI or installer changes, run a parser check before shipping.
- Do not commit local logs, generated backups, received files, credentials, or machine-specific configuration.
- Do not introduce paid or external APIs unless they are optional, documented, and disabled by default.

## Pull Requests

When pull requests are enabled, include:

- What changed.
- Why it changed.
- How it was validated.
- Any LAN/manual testing that remains.

## Security

Do not report security issues in public issues. Follow `SECURITY.md`.
