# F2 Architecture

Domain and ports depend only on the Go standard library. CLI and future TUI consume use cases; adapters implement ports. The integration test `TestCoreHasNoUIOrProviderDependencies` verifies this separation using the real import graph.

Present domain types: issues, identity, local dates, normalized blocks, query/page, progress, transition fields, intent, prepared action, apply result, and errors. F1 adds `config` for versioned persistence, `app.Access` for access use cases, `provider/jiracloud` for HTTP, and `secretstore` for system credentials. CLI composes dependencies and tests can inject them.

`Secret` redacts format, JSON, and slog; `Reveal` is an explicit output for authentication/storage adapters. Redaction avoids common logging mistakes; it does not encrypt memory or guarantee string erasure in Go.

## Pinned dependencies

| Component | Initial version | Use |
| --- | --- | --- |
| Go | 1.27.1 | Compiler, gofmt, and go vet |
| Cobra | 1.10.2 | CLI |
| Bubble Tea | 2.0.9 | F0 compatibility; TUI in F5 |
| Bubbles | 2.2.1 | TUI components |
| Lip Gloss | 2.0.6 | TUI styles |
| go-keyring | 0.2.8 | Historical foundation audit; active backend uses own processes |
| x/term | 0.45.0 | Hidden input/terminal detection in F1 |
| govulncheck | 1.7.0 | Analysis run by `make security` |
| GoReleaser | 2.18.1 | Reserved version in Makefile; packaging in F6 |

`go.mod` and `go.sum` are the authority on the actually resolved graph, including transitive dependencies. `foundation` keeps imports of future dependencies present when running `go mod tidy`, and allows compiling them in the matrix without incorporating premature functionality into the binary. It is a deliberate exception from F0: remove those imports when real adapters/TUI use them.

CI pins checkout/setup-go by SHA, uses explicit OS/toolchain versions, and does not publish. The initial lint is `go vet`, pinned by Go; add another linter only when it provides needed rules.

## F2 reading

`jiracloud.Session` implements `ports.IssueReader` with one profile and credential per invocation. `app.Reader` verifies identity, walks pages, signs cursors, and keeps an isolated cache. `output` converts domain to public DTOs and text; it does not export internal structs directly to the JSON contract. `browser` implements the opening port with separate executables and arguments.

Listings do not load details per row. Comments/history sections use their own offsets. Cache lives only in memory; persistence is reserved for F4.

## Future organization

Extend `provider/jiracloud` and `workflow` in F3; persistent `cache` in F4; `tui` in F5. Do not create empty packages to pretend implementation. The public format will remain in `output` even when internal types change.

References checked on 2026-09-06: [Go](https://go.dev/dl/?mode=json), [Cobra](https://github.com/spf13/cobra/releases/tag/v1.10.2), [Charm](https://charm.land/blog/v2/), [keyring](https://github.com/zalando/go-keyring/tree/v0.2.8).
