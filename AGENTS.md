# Stoat Agent Guide

This file defines the shared rules for AI agents and automation working on
Stoat. Repository documentation, code, and tests take precedence over
assumptions. If a rule conflicts with the current implementation, verify the
facts first and explain the conflict in the pull request.

## Product Scope

Stoat is a terminal-first macOS persistence inspector and manager. Its core
job is to answer three questions:

1. What runs automatically at login, boot, on a schedule, or in the
   background?
2. Where did it come from, what is its current state, and why might it need
   review?
3. What can the user safely do under a verifiable, auditable, and recoverable
   workflow?

### What Stoat Should Do

- Collect Login Items, BTM, launchd, and cron data while preserving source
  evidence.
- Provide clear category navigation, explainable details, machine-readable
  output, and stable schemas.
- Keep launchd actions within a plan, confirmation, backup, apply,
  verification, audit, and rollback lifecycle.
- Keep risk rules explainable through stable rule IDs, explicit scores,
  concrete evidence, and expiring exceptions.
- Tolerate unstable macOS output by ignoring unknown fields or reporting
  warnings without hiding valid results.

### What Stoat Should Not Do

- Do not turn Stoat into antivirus software, a general cleanup utility, a
  package manager, or a system optimizer.
- Do not write to the private BTM database, rewrite a complete crontab, or
  modify Apple system jobs.
- Do not infer application ownership or related files from vague names,
  vendor prefixes, or wildcard matches.
- Do not remove application caches, settings, accounts, credentials,
  documents, or other user data.
- Do not elevate privileges, call `sudo`, introduce hidden background
  mutations, or perform destructive work without confirmation.

When a feature does not clearly belong to Stoat, narrow it, keep it read-only,
or explicitly mark it unsupported.

## Repository Map

- `cmd/stoat/`: CLI bootstrap, argument parsing, and command orchestration.
  Domain logic does not belong here.
- `internal/collector/`: invokes bounded macOS interfaces and collects raw
  data.
- `internal/parser/`: converts plist, cron, and BTM input into domain models.
- `internal/app/`: scan orchestration, deduplication, sorting, and module
  composition.
- `internal/model/`: shared domain models, enums, and data structures.
- `internal/runtimeinfo/`: reads live launchctl state.
- `internal/attribution/`: associates applications using path and Info.plist
  evidence.
- `internal/signing/`: inspects file properties and code signatures.
- `internal/risk/`: contains pure risk rules, policies, and exceptions.
- `internal/action/`: the only business layer allowed to mutate persistence
  state.
- `internal/monitor/`: snapshots, change events, and event history.
- `internal/diagnostics/`: runtime state and Unified Log diagnostics.
- `internal/tui/`: category, list, detail, and action UI. It must reuse the
  action layer instead of duplicating business rules.
- `internal/updatecheck/`: bounded, read-only release version lookup used by
  the interactive TUI.
- `internal/executil/`: command allowlist, shell-free execution, deadlines,
  and output limits.
- `schemas/`: public JSON schemas.
- `testdata/`: fixed, redacted, and reproducible fixtures.
- `scripts/`: installation, verification, and release helpers.
- `Formula/`: experimental Homebrew HEAD formula.
- `docs/`: product, security, architecture, compatibility, installation, and
  roadmap documentation.

## Common Commands

```bash
make verify
make build
go test ./internal/risk/...
go test ./internal/action/...
bash scripts/install_test.sh
make release-arm64
make release-amd64
```

Run `make verify` before committing. Outside macOS, at minimum run the full Go
test suite and cross-build both Darwin architectures. Real scans are verified
by macOS CI.

## Critical Safety Rules

- Every state mutation must go through `internal/action`; all other layers
  remain read-only.
- State changes support launchd only. BTM and cron remain read-only in v1.
- Never modify `/System/Library` or Apple-owned jobs.
- Do not use `sh -c`, dynamic command paths, or an implicit shell. System
  commands must use the absolute-path allowlist, deadlines, and output limits
  in `internal/executil`.
- Stoat must never invoke `sudo`. System-level items may be changed only when
  the calling process is already root and the relevant ownership and
  permission checks pass.
- State changes must complete the full lifecycle:
  `Plan → Confirm → Backup → Apply → Verify → Audit`. Stop on any critical
  failure and roll back when required.
- Confirmation tokens must bind the action kind, item ID, launchd domain,
  configuration digest, and observed runtime state. Any change invalidates
  the token.
- Backup and state directories must be private to the current actor. Reject
  symlinks, non-regular files, loose permissions, and untrusted ancestors.
- `remove` deletes only the exact launchd configuration and keeps recovery
  material.
- `uninstall` accepts only an attributed top-level `.app` directly under
  `/Applications` or `~/Applications`. Bind its `Info.plist` digest and move
  it to a collision-safe Trash path.
- Before restoring, verify the backup and target state. Do not overwrite
  content created later or trust a modified backup.
- Risk output expresses review priority only. Do not make unsupported claims
  that an item is safe, malicious, or malware.
- Partial scan failures must return explicit warnings. An incomplete scan
  must not advance the monitoring baseline.
- The installer uses HTTPS only, verifies SHA-256, allowlists archive entries,
  rejects symlinks and path traversal, replaces the binary atomically, and
  does not edit shell configuration.

Any change that broadens deletion, attribution matching, or privilege
boundaries requires a branch-by-branch safety review plus failure and rollback
tests.

## Architecture Rules

- Collectors collect, parsers interpret, risk rules score, actions perform
  controlled mutations, and the TUI handles interaction.
- New macOS commands must be centrally wrapped and replaceable in tests. Tests
  must not depend on real authorization dialogs or real system mutations.
- Parsers must tolerate unknown fields and fail closed on malformed,
  oversized, non-regular, or symlinked input.
- Domain-model changes must update JSON output, schemas, fixtures, snapshot
  compatibility, and architecture documentation.
- Risk rules must stay deterministic and pure. Every new rule needs a stable
  ID, score, evidence, and table-driven tests.
- Errors must preserve operation, source, and path context. Use warnings for
  supported degradation; never swallow errors silently.
- Avoid new dependencies for small features. If one is necessary, explain in
  the pull request why the standard library and existing dependencies are
  insufficient.

## Interaction Rules

- Keep the default TUI hierarchy as
  `Category → List → Detail → Action`; do not return to a mixed first screen.
- `Esc` moves back, `Enter` moves forward, and `a` opens actions. New shortcuts
  must not conflict with existing behavior.
- Scanning, failure, empty results, and action outcomes must have explicit
  states. Never leave the terminal waiting without feedback.
- Destructive actions must show the target, effect, recoverability, and
  confirmation requirement. Color cannot be the only carrier of meaning.
- CLI and TUI must share the same action semantics. The TUI must not bypass
  safeguards enforced by the CLI path.

## Test Requirements

- Parser tests use redacted fixtures for valid, unknown-field, malformed,
  oversized, and cross-macOS-version input.
- Action tests cover success, stale confirmation, permission errors, state
  races, audit failure, verification failure, rollback failure, restore
  conflicts, and content tampering.
- Risk tests pin rule IDs, evidence, and scores with table-driven tests and do
  not depend on the current machine.
- TUI tests cover category navigation, back navigation, loading, action
  confirmation, and rescanning after success.
- Installer tests cover offline version resolution, architecture selection,
  checksum failure, malicious archives, atomic replacement, and custom
  directories.
- Run the race detector when changing concurrency, storage, or caching. Run
  Ruby syntax checks, ShellCheck, and actionlint when changing Formula, shell,
  or workflow files respectively.

Tests must never modify real launchd jobs, crontabs, BTM data, or
`/Applications`. Use temporary directories, fixtures, and injectable runners
for system behavior.

## Working and Commit Rules

- Create `agent/<description>` from the latest `main`; never commit directly
  to `main`.
- Inspect the worktree before editing. Commit only task-related files and
  preserve unrelated user changes.
- Keep commits single-purpose and do not add AI attribution trailers.
- Pull requests must explain what changed, why, user impact, and verification.
  Security-boundary changes must also state explicit non-goals.
- Comments explain why, not what the next statement says. Documentation
  commands must match current CLI behavior.
- Fix root causes. Do not hide problems by weakening validation, ignoring
  errors, adding unbounded retries, or introducing fallback deletion.

## Release

- Root `VERSION` is the source of truth for stable releases. Change it only
  when explicitly publishing a new version.
- After a `VERSION` change reaches `main`, the release workflow creates an
  immutable tag, builds both architecture archives, and publishes checksums
  and Sigstore bundles.
- Do not move existing tags, overwrite release assets, or publish with failing
  CI.
- Before release, verify consistency among `VERSION`, `CHANGELOG.md`, installer
  scripts, Formula, and documentation examples.
- Apple Developer ID signing and notarization are not enabled. Never describe
  Sigstore as a replacement for Gatekeeper notarization.
- Until `stoat.lighting.pub`, a mainland China mirror, and a stable Homebrew
  tap are actually online, documentation must label them accordingly.

## GitHub Operations

- Read the latest issue or pull request body, comments, state, and checks
  before replying, closing, or merging.
- Keep pull requests in Draft until the maintainer confirms them. Mark Ready
  and merge only after explicit authorization.
- When CI fails, inspect the actual job logs and identify the root cause rather
  than guessing from workflow status.
- After merging, verify `main` CI. If `VERSION` changed, also confirm that the
  tag, release, and every asset exist.
