# Security Policy

## Capability boundary

Scanning is read-only. State-changing commands support launchd agents and daemons only. Stoat does not invoke `sudo`, modify Background Task Management data, rewrite crontabs, or alter Apple-owned `/System/Library` jobs.

## Defensive controls

- External commands are selected from a hard-coded absolute-path allowlist.
- Commands are executed without a shell and with context deadlines.
- Command output, plist size, cron size, and scanner buffers are bounded.
- Symlinks and non-regular configuration files are skipped.
- Partial failures are surfaced as warnings.
- Risk results always include evidence and never claim malware detection.
- Snapshot and policy files must be bounded regular files; JSON schemas reject unknown fields and trailing values.
- Risk exceptions require an exact item ID and rule ID, expire explicitly, and remain visible in audit output.
- Export and snapshot files use mode `0600`, temporary-file writes, and atomic rename; overwrite requires `--force`.
- launchd actions require confirmation bound to the current item, domain, configuration SHA-256 and observed runtime state.
- Action state uses private directories, pre-operation backups, no-follow file opens, rollback and post-action verification.
- Application uninstall requires an attributed top-level `.app`, binds its `Info.plist` digest, moves it to a collision-safe Trash path, and never deletes related user data.
- System launchd actions require the caller to already be root; Stoat never elevates itself.
- Monitoring snapshots and events are stored in private directories; incomplete scans never advance the baseline.
- Unified Log queries use an escaped predicate argument without a shell, bounded time/output and bounded parsed entries.
- The optional TUI update check requests only the fixed GitHub `latest.txt`
  release asset over HTTPS, uses a short deadline and bounded response, sends
  no scan results or user data, and fails silently when unavailable.
- The release installer requires HTTPS, verifies SHA-256, allowlists archive entries, rejects symlink targets, installs atomically, and never invokes `sudo`.
- A GitHub acceleration proxy is only a transport source; trusted checksums should be delivered independently through `stoat.lighting.pub`.

## Reporting

Do not include credentials, personal paths, complete crontabs, or private application data in a public report. Provide the Stoat version, macOS version, command used, redacted warning, and minimal reproducer.

## State-changing features

See [docs/SAFE_ACTIONS.md](docs/SAFE_ACTIONS.md) for the action threat model, protected paths, confirmation protocol and recovery procedure.

See [docs/INSTALLATION.md](docs/INSTALLATION.md) for installer and mirror trust boundaries.
