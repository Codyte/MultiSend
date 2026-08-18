# Support scripts

These scripts are development and diagnostic tools; they are not runtime entrypoints. Run them only against a disposable or known local MultiSend configuration.

- `multisend-benchmark-abc.ps1`: changes download settings, starts repeated downloads, and writes a report to the desktop.
- `multisend-test-1gb-log.ps1`: starts a 1 GB transfer and writes telemetry to the desktop.
- `multisend-triage-batch.ps1`: reads local system/API state and can run an active send test unless skipped.
- `multisend-triage-blindado.ps1`: can sanitize configuration, restart the agent, select an interface, and send a probe.

Run them from the repository root or by absolute path. They do not modify repository files at runtime.
