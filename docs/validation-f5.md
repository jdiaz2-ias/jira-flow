# F5 validation: interactive workflows

Implementation completed on branch `feat/f5-tui-reading`, starting from the reading increment `137ddee`. Local validation: macOS arm64, Go 1.27.1, 2026-09-21. The Linux/macOS CI matrix runs the same automated suite, including the PTY journey. No real Jira account, issue, keyring, or browser was used by the fixtures.

Pre-merge validation repeated on 2026-09-22: `make check test-race build-all build`, `make security`, `go mod verify`, `go mod tidy -diff`, and `git diff --check` all passed. F6's starting checklist is recorded in [roadmap.md](roadmap.md).

F5 implements its keyboard journey: query → start → review → complete → open browser. It also includes remote JQL search, local filtering, pagination, detail/progress, offline reads, required/optional field forms, draft protection, confirmation, post-action refresh, theme selection, adaptive panels, accessible plain output, automatic root entry, and initial scoped/unscoped setup.

Validation:

- `make check`: format, vet, unit/integration tests, and foundation checks pass.
- `make test-race`: passes. The PTY fixture is itself compiled with `-race`, so its actual asynchronous event loop is instrumented.
- `make build-all build`: Linux/macOS amd64/arm64 compile without CGO; local binary built.
- `make security`: no vulnerabilities found.
- `go mod verify` and tidy consistency: existing dependency versions preserved; `x/sys` is now a direct dependency for cancelable terminal polling.
- Model coverage: list/detail navigation, pagination, local/remote text focus, bracketed paste, obsolete responses, cancellation, metadata/errors, output sanitization, themes, sizes, and draft retention.
- Workflow coverage using the real application resolver: ambiguous choices, required allowed values, explicit review, Enter not confirming, one write, no-op, close selection, stale preparations, unknown outcomes, duplicate blocking, offline rejection, and long/hidden review guards. Existing F3 tests continue to cover HTTP semantics, fresh revalidation, and cache invalidation.
- CLI composition coverage: root/explicit UI entry, shared workflow setup, action logging, plain accessible mode, terminal/environment guards, and JSON error contracts.
- PTY integration in `tests/integration/tui_test.go` / `testdata/tui/terminal_journey.py`: root startup, issue detail, start confirmation, complete with resolution field, browser suspension/resumption, JQL search, resize, scoped/unscoped login, hidden token, exit and signal cancellation, including terminal restoration. Python 3 is required for this test and is available on the CI runners; local execution skips it if Python is absent. On Darwin, termios comparisons exclude the transient PENDIN kernel flag.

The full test suite needs permission to bind synthetic localhost HTTPS servers and create pseudoterminals. Local runs used `GOCACHE=/private/tmp/jflow-go-cache` and permission for those operations. Tests caught and fixed competing signal handlers that could report success on cancellation, and setup input now polls for cancellation without leaving an input-reading goroutine behind.

Acceptance boundaries: automated synthetic PTY coverage is not a human usability session or real Jira tenant acceptance. Authorized real-tenant workflows, Linux/macOS human terminal checks, and distribution remain F6 acceptance work. Persistence retains F4's offline content omissions; no offline mutation or automatic retry is introduced. Clipboard copying remains optional and unimplemented: a selectable URL is always available. Unknown workflow field schemas must be completed in Jira via the browser action.
