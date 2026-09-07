# Implementation status

## F0: completed technical base

CLI, domain/ports, JSON v1, pinned dependencies, base tests, fixtures, and CI. Historical evidence: [validation-f0.md](validation-f0.md). The repository is already published on GitHub under `jdiaz2-ias/jira-flow`.

## F1: configuration and access implemented

- [x] Linux/macOS paths, JSON v1, private permissions, locking, and atomic writes.
- [x] Profiles, precedence, and rejection of unknown schemas; F0 had no persisted format to migrate.
- [x] Cancelable SecretStore, random references, limit validation, and ephemeral credentials.
- [x] HTTP with TLS/proxy/corporate CA, cancellation, limits, and redirect rejection.
- [x] Both personal Cloud token methods and `myself` mapping.
- [x] `auth login/logout/status`, `profile list/use`, `me`, `doctor`, `config path/validate`.
- [x] Tests with simulated HTTPS, injected keyring, and real macOS Keychain with synthetic secret.
- [ ] Real validation with authorized tenant and Linux Secret Service; they do not block synthetic tests, but limit compatibility claims.

Evidence and limits: [validation-f1.md](validation-f1.md).

## F2: reading and links implemented

- [x] Escaped JQL, filters, allowed ordering, and listing field selection.
- [x] POST search/jql, cursors tied to profile/identity/query, deduplication, and partial results.
- [x] `mine`, `list`, `search`, `show`, ADF description, subtasks, and links.
- [x] Paginated comments and history on demand, with offsets and per-section limits.
- [x] `link`, `open`, native adapters, and default project.
- [x] Per-process bounded cache, refresh, explicit offline, text/table/JSON output.
- [x] HTTP tests, CLI without TTY, cancellation, retries, limits, and isolation.
- [ ] F2 validation against a real tenant.

Evidence and limits: [validation-f2.md](validation-f2.md). The user confirmed F1's real connection; that confirmation does not yet accredit F2 reads.

## F3: workflow transitions implemented

- [x] Live transition IDs, destinations, field schemas, and allowed values.
- [x] `transitions`, `transition`, `start`, `done`, `close`, and `workflow map`.
- [x] Basic field prompts, fields files, explicit selection, dry runs, and confirmation.
- [x] Exact context mappings, destination validation, no-op and ambiguity handling.
- [x] Fresh revalidation, one write attempt, cache invalidation, and bounded verification.
- [x] Minimal private action records with pruning after seven days and `--no-record`.
- [x] Synthetic HTTP and CLI tests, race detection, and four platform builds.
- [ ] F3 acceptance against an authorized real Jira issue.

Evidence and limits: [validation-f3.md](validation-f3.md).

## Next increment

F4: progress/scripting. F5: TUI. F6: packaging/acceptance. F7: productivity. Criteria in the [full plan](implementation-plan.md).
