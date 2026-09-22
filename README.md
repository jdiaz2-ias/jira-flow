# Jira Flow

A CLI for working with Jira from Linux and macOS. Executable: `jflow`.

Current status: **F6 in progress: distribution candidates and acceptance**. F5's interactive Jira workflows are implemented. Run `jflow` or `jflow ui` for the keyboard interface. `make release-check` builds and verifies four candidate archives; see the [installation guide](docs/install.md) and [F6 validation](docs/validation-f6.md) for delivery status and outstanding acceptance.

## Quick start

Requirements: Go 1.27.1, Git, and Make. GCC/Clang is needed for `make test-race`; normal binaries are built with `CGO_ENABLED=0`.

```bash
cd jira-flow
go mod download
make build
./bin/jflow --help
./bin/jflow version
./bin/jflow version --format json
```

## Connect a Jira account

```bash
./bin/jflow auth login --profile work \
  --site https://company.atlassian.net \
  --email person@example.com
./bin/jflow me
./bin/jflow doctor --format=json
./bin/jflow profile list
```

Login prompts for the token with hidden input and validates identity before saving. For a scoped token add `--method api-token-scoped --cloud-id REAL_ID`. Cloud ID must be provided explicitly; it is not inferred from the hostname.

On macOS, Keychain is used; on Linux, `secret-tool` and Secret Service are required. For automation, `--token-stdin` reads a single line and `--no-store` avoids persisting it. `JFLOW_TOKEN` is always ephemeral: provide it on every invocation. `me` and `doctor` also accept `--token-stdin`. Do not pass the token as an argument.

Configuration is saved in `~/Library/Application Support/jflow/config.json` on macOS and `$XDG_CONFIG_HOME/jflow/config.json` or `~/.config/jflow/config.json` on Linux. `JFLOW_CONFIG` allows another path. `jflow config path` shows the effective path; `jflow config validate` validates the file without showing secrets. Files are created with `0600` permissions and new directories with `0700`.

`--profile` > `JFLOW_PROFILE` > active profile. `--email` > `JFLOW_EMAIL` > profile email > global email. `JFLOW_CA_CERT` adds a PEM CA to the system roots; transport respects the environment proxy.

`auth status` checks the local credential source; `me` and `doctor` verify them with Jira. `auth logout --profile work` removes that profile and its local credentials; it does not revoke the token in Atlassian. A `JFLOW_TOKEN` variable continues to exist in the shell until you remove it.

More details: [commands](docs/commands.md), [authentication](docs/authentication.md), [compatibility](docs/compatibility.md).

Example output:

```text
jflow dev
commit: unknown
Go: go1.27.1
platform: linux/amd64
```

The commit reflects Git when built with Make. `go run ./cmd/jflow version` preserves development values.

## Query Jira

```bash
./bin/jflow mine --format table
./bin/jflow mine --project APP --status-category in-progress
./bin/jflow config set default_project APP
./bin/jflow list --type Bug --limit 20
./bin/jflow search --jql 'project = APP ORDER BY updated DESC' --all --format json
./bin/jflow show APP-123 --comments --history
./bin/jflow link APP-123
./bin/jflow open APP-123
```

Use the active profile or add `--profile ias`. Replace `APP` and `APP-123` with real keys. `mine` excludes Done by default; `--include-done` includes it. Listings indicate truncation and offer a cursor to continue with `--page-token`. `show` loads comments/history only when requested; their offsets are controlled with `--comments-start` and `--history-start`.

`--refresh` bypasses caches. Persistence is off by default; enable it per profile with `jflow config set cache.persist true` to reuse results with `--offline` across invocations. See [progress and scripting](docs/progress-scripting.md) for scope, privacy, and freshness rules. A network query verifies identity once, never per row. The default project is preserved when renewing login.

## Interactive workflow

```bash
./bin/jflow                     # Starts the UI in a capable terminal; setup if no profiles exist
./bin/jflow ui --theme dark     # auto, dark, light, mono
./bin/jflow ui --ascii --offline
./bin/jflow ui --accessible     # Plain listing; no full-screen mode
```

Use arrows or `j/k` to select, Enter for detail, Tab to change focus, `/` for a local filter, and Ctrl+F for a remote JQL query. `s`, `d`, and `x` prepare start, done, and close; `t` lets you choose a transition. Required fields are validated before review. Only `y` in the review dialog confirms a write; Enter does not. `e` edits fields, including optional ones. `o` opens Jira and `y` outside a dialog displays a selectable URL.

The UI preserves drafts on resize and asks before discarding entered fields. A pending write blocks duplicates; uncertain outcomes remain explicit and are never retried automatically. After acknowledging the result, the list and detail refresh. `r` reloads, `n` loads the next page, `?` shows help, and `q` exits. At 110 columns the UI shows two panels; narrower terminals alternate list/detail. The minimum size is 60×15. `NO_COLOR`, `--no-color`, `--ascii`, and the plain CLI provide terminal/accessibility alternatives.

## Available commands

| Command | Output |
| --- | --- |
| `jflow` / `jflow ui` | Interactive reading, workflows, search, and browser actions |
| `jflow ui --accessible` | Plain listing for screen readers or redirected output |
| `jflow --help` | Help for implemented commands |
| `jflow help version` | Version help |
| `jflow version` | Version, commit, Go, and platform |
| `jflow version --format=json` | Same data in JSON v1 contract |
| `jflow --help --format=json` | Help inside a single JSON document |
| `jflow auth login/logout/status` | Authentication and local credentials |
| `jflow profile list/use` | Profiles and active selection |
| `jflow me` | Authenticated identity in Jira |
| `jflow doctor` | Configuration and connection diagnostics |
| `jflow config path/validate` | Path and configuration validation |
| `jflow config set default_project APP` | Profile default project |
| `jflow mine/list/search` | Listings with filters, pages, and table/JSON format |
| `jflow show KEY` | Detail with optional sections |
| `jflow link/open KEY` | Browsable URL and browser opening |
| `jflow transitions/transition/start/done/close` | Inspect and apply confirmed workflow transitions |
| `jflow progress KEY` | Subtasks, time, state, and local-calendar due date |
| `jflow summary` | Scoped category counts, with completion only for a complete population |
| `jflow config set cache.persist true` | Enable private persistence for the selected profile |
| `jflow cache status/clear` | Cache usage and profile-scoped removal |

Invalid arguments or configuration return exit code 2. Help, version, profiles, and `link` do not require a Jira connection. Authenticated queries need read permission in projects.

## Development and verification

```bash
make check            # Format, vet, tests, and dependency audit
make test-race        # Race detector
make build-all        # Linux/macOS × amd64/arm64, including future imports
make security         # Pinned govulncheck; needs network the first time
make release-check   # Four candidate archives, checksums and native smoke; never publishes
```

`make build` produces `bin/jflow`. `make build-all` produces `bin/{linux,darwin}-{amd64,arm64}/jflow`. These directories are ignored by Git. Cross-compilation does not certify execution on the target system; see [compatibility and validation](docs/compatibility.md).

## Architecture

Go + Cobra for the CLI; Bubble Tea/Bubbles/Lip Gloss v2 are pinned and checked with the `foundation` build tag. F5 integrates Bubble Tea and Lip Gloss for interactive workflows, layouts, and themes; F1 integrates the system keyring.

```text
cmd/jflow       Process entry and signals
internal/cli    Arguments, help, and exit codes
internal/app    Access, read, cursor, and version use cases
internal/config Paths, profiles, and JSON persistence
internal/provider/jiracloud HTTPS, identity, issues, ADF, and pagination
internal/cache  Bounded in-memory cache
internal/browser Native macOS/Linux launchers
internal/secretstore Cancelable macOS/Linux keyring
internal/domain Issues, progress, workflows, and errors
internal/ports  Boundaries for Jira and secret storage
internal/tui    Reading/workflow models, dialogs, themes, and keyboard navigation
internal/output Public JSON contract independent of domain
internal/foundation Dependency compatibility and keyring audit
tests/integration   Binary and layer separation tests
testdata/jiracloud   Synthetic responses for future adapters
docs/adr        Architecture decisions
```

The `jira-flow.local/jflow` module is deliberately local and provisional. The remote repository is `github.com/jdiaz2-ias/jira-flow`. Change the module before distributing via `go install`; distribution licensing is still pending decision.

## Continue implementation

- [Full implementation plan](docs/implementation-plan.md).
- [Phase status and next increment F6](docs/roadmap.md).
- [Architecture and pinned versions](docs/architecture.md).
- [Authentication and endpoint/scope catalog](docs/authentication.md).
- [JSON contract and errors](docs/json-contract.md).
- [Workflow decisions](docs/workflows.md).
- [F0 validation log](docs/validation-f0.md).
- [F1 validation log](docs/validation-f1.md).
- [F2 validation log](docs/validation-f2.md).

F5 implements the keyboard journey: query, start, review, complete, and open in the browser. F6 now provides candidate packaging, generated help/completion and installation guidance; maintainers, real Jira acceptance and human terminal checks remain pending. See [progress and scripting](docs/progress-scripting.md) for pipelines and offline use, and the [workflow guide](docs/workflows.md) for transition semantics.

## License

Jflow source code and documentation are licensed under the [Apache License, Version 2.0](LICENSE) (SPDX: `Apache-2.0`). Distribution archives include the full license. Third-party dependencies retain their respective licenses; their notices are bundled in `extras/THIRD-PARTY-NOTICES.txt`.

Copyright 2026 Jonathan Emmanuel Diaz Delgadillo. See [NOTICE](NOTICE) for attribution.
