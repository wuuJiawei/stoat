# Security Policy

## V1 capability boundary

Stoat V1 is read-only. It does not delete files, disable jobs, invoke `sudo`, edit Background Task Management data, or write launchd/cron configuration.

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

## Reporting

Do not include credentials, personal paths, complete crontabs, or private application data in a public report. Provide the Stoat version, macOS version, command used, redacted warning, and minimal reproducer.

## Future state-changing features

Any disable/restore implementation must have a separate threat model, snapshot format, integrity validation, explicit confirmation, rollback verification, and protected-path policy before merge.
