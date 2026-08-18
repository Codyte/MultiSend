# Public Repository Checklist

Use this before switching the GitHub repository visibility to public.

## Required

- [x] Choose and add a license file.
- [ ] Confirm Codyte accepts MIT as the public project license.
- [ ] Run a fresh secret scan over the full working tree.
- [ ] Review Git history for secrets, logs, local configs, and oversized binary artifacts.
- [ ] Decide whether release binaries should remain in Git or move to GitHub Releases.
- [ ] Remove old installer backups from Git history if repository size matters.
- [ ] Confirm `.gitignore` covers local logs, scratch files, DBs, and tool configs.
- [ ] Run `go test ./...` from the repository root.
- [ ] Run `go vet ./...` and review `docs/QUALITY_AUDIT.md`.
- [ ] Confirm compatibility UI wrappers open the embedded web UI and contain no download WinForms.
- [ ] Rebuild installer from a clean checkout.
- [ ] Test install/uninstall on a non-development Windows machine.
- [ ] Run `--doctor`, `--lab-smoke`, and `--download-smoke`.
- [ ] Confirm both smokes report `artifact cleanup: PASS` and leave no `run-*` directory.
- [ ] Run manual LAN tests on two PCs for send, pull file, pull folder zip, and pull folder extract.

## Recommended

- [ ] Add screenshots or short usage recordings.
- [ ] Create GitHub Releases for installers instead of tracking generated binaries.
- [ ] Enable Dependabot or another dependency review workflow.
- [ ] Enable GitHub secret scanning and private vulnerability reporting.
- [x] Review all public-facing text for owner/project naming.

## Known Public-Readiness Notes

- The current product has no runtime dependency on paid or external APIs.
- Historical binaries may still exist in Git history even if removed from the current tree.
- Current public target is `Codyte/MultiSend`.
