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
- launchd actions require a confirmation token bound to the current item, domain and configuration SHA-256.
- Action state uses private directories, pre-operation backups, no-follow file opens, rollback and post-action verification.
- System launchd actions require the caller to already be root; Stoat never elevates itself.

## Reporting

Do not include credentials, personal paths, complete crontabs, or private application data in a public report. Provide the Stoat version, macOS version, command used, redacted warning, and minimal reproducer.

## State-changing features

See [docs/SAFE_ACTIONS.md](docs/SAFE_ACTIONS.md) for the action threat model, protected paths, confirmation protocol and recovery procedure.
