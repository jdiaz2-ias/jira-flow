# F5 validation: interactive reading increment

Validated locally on macOS arm64, 2026-09-21, using Go 1.27.1. Branch: `feat/f5-tui-reading`, based on the F4 merge `2d000e7`.

This starts F5; it does not meet the full phase's query/start/review/complete/browser exit criterion. No real Jira account or issue was accessed or modified for this increment.

Implemented: explicit `jflow ui`, assigned pending issues, detail scrolling, local filtering, pagination, refresh, offline cache reads, keyboard help, small-terminal guidance, monochrome rendering, async commands, per-request deadlines, cancellation, stale-response rejection, and sanitized terminal content. The CLI and TUI share `app.Reader` and its identity/cache behavior.

Validation:

- `make check`: format, vet, unit/integration tests, and foundation checks pass.
- `make test-race`: passes.
- `make build-all build`: Linux/macOS amd64/arm64 compile without CGO; local binary built.
- `make security`: no vulnerabilities found.
- `go mod verify`: all modules verified. `go mod tidy` promotes the existing ANSI dependency to direct use; no versions changed.
- Model tests cover navigation, pagination cursors, detail rendering, text-field shortcut isolation, cancellation, old-generation rejection, request errors/partial warnings, sanitized output, and terminal resizing.
- CLI and binary tests reject noninteractive UI before Jira access and preserve JSON error output.
- Local macOS pseudoterminal smoke with a temporary synthetic reader: list, Enter to detail, q to exit, alternate-screen entry/exit, and termios restoration passed. The comparison excludes Darwin's transient PENDIN kernel flag. No credentials were used.

The sandbox initially blocked Go's default build cache and local HTTP test listeners. Validation used `GOCACHE=/private/tmp/jflow-go-cache`; the full test suite ran with local-listener permission.

Remaining F5 work: remote search, two-panel layout, themes/focus, workflow fields and confirmation dialogs, browser actions, post-action feedback, root automatic entry/setup, and full keyboard acceptance on Linux/macOS against authorized Jira. The current TUI requires explicit `ui`, displays list/detail separately, and provides an accessible CLI alternative through `mine` and `show --format plain`. Cross-compilation does not certify interactive execution on Linux. Persistent offline details retain the existing F4 omissions.
