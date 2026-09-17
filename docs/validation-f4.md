# F4 validation

Validated locally on macOS arm64, 2026-09-17, using Go 1.27.1. Implementation branch: `feat/f4-progress-scripting`, based on the F3 merge `ffc910a`. Jira tests use synthetic HTTPS servers or injected ports; no real Jira issue was modified.

For this personal project, the owner authorized publishing and merging F4 using automated validation without real-tenant Jira acceptance. That acceptance remains unperformed and is not a merge gate for this increment.

- `make check`: formatting, vet, unit/integration tests, and foundation checks pass.
- `make test-race`: passes, including concurrent disk-cache writers using separate handles.
- `make build-all build`: Linux/macOS amd64/arm64 builds pass without CGO; local binary built.
- `make security`: no vulnerabilities found.
- `go mod verify`: all modules verified; module dependencies unchanged.
- Binary smoke checks: help for `progress` and `summary`; integration tests cover JSON, no-TTY execution, missing authentication, invalid metrics options, and cache status with a temporary cache root.

Coverage includes unknown/zero time, consumption above 100%, partial/empty/unknown-category subtasks, summary denominators and scope, timezone/DST calendar arithmetic, complete status history and destination mismatch, guard-limit exit 10, and one JSON document per command.

Cache coverage includes persistence across reader/CLI instances, online TTL vs offline staleness, seven-day expiry, profile isolation, private permissions, corrupt entries, symlink rejection, independent concurrent writers, 25 MiB/1,000-detail limits, omitted description bodies, credential-generation changes, auth failures without stale fallback, profile clearing, and invalidation for verified and uncertain transitions. A dry run leaves cached reads intact. Pre-write invalidation failure prevents the send; post-write failure produces a warning without altering the observed write outcome or retrying.

Limitations: F4 metrics and offline acceptance against a real tenant remain unvalidated. Linux Secret Service and real execution of every cross-compiled target remain separate acceptance items. Arbitrary JQL summaries deliberately omit a completion percentage; saved views are a later phase. Persistent details omit description/comment bodies and changes to those bodies in history, so some offline detail/history is incomplete. Cache snapshots cannot reveal permissions or Jira changes while offline. TUI, packaging, and public releases remain F5/F6 work.
