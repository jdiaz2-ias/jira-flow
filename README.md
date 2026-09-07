# Jira Flow

A CLI for working with Jira from Linux and macOS. Executable: `jflow`.

Current status: **F3, workflow transitions**. Includes profiles, Jira Cloud authentication, issue queries, details, links, and confirmed workflow transitions with field validation and result verification. See [validation F3](docs/validation-f3.md) for results and limits.

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

`--refresh` bypasses the in-memory cache. F2 does not save issues to disk, so `--offline` does not recover results from a previous run. A network query verifies identity once, never per row. The default project is preserved when renewing login.

## Available commands

| Command | Output |
| --- | --- |
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

No command returns exit code 2. Help, version, profiles, and `link` do not require a Jira connection. Authenticated queries need read permission in projects.

## Development and verification

```bash
make check            # Format, vet, tests, and dependency audit
make test-race        # Race detector
make build-all        # Linux/macOS × amd64/arm64, including future imports
make security         # Pinned govulncheck; needs network the first time
make release-check   # Build verification, does not publish artifacts
```

`make build` produces `bin/jflow`. `make build-all` produces `bin/{linux,darwin}-{amd64,arm64}/jflow`. These directories are ignored by Git. Cross-compilation does not certify execution on the target system; see [compatibility and validation](docs/compatibility.md).

## Architecture

Go + Cobra for the CLI; Bubble Tea/Bubbles/Lip Gloss v2 are pinned and checked with the `foundation` build tag. The TUI will be implemented in F5; F1 already integrates the system keyring.

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
internal/output Public JSON contract independent of domain
internal/foundation Dependency compatibility and keyring audit
tests/integration   Binary and layer separation tests
testdata/jiracloud   Synthetic responses for future adapters
docs/adr        Architecture decisions
```

The `jira-flow.local/jflow` module is deliberately local and provisional. The remote repository is `github.com/jdiaz2-ias/jira-flow`. Change the module before distributing via `go install`; distribution licensing is still pending decision.

## Continue implementation

- [Full implementation plan](docs/implementation-plan.md).
- [Phase status and next increment F4](docs/roadmap.md).
- [Architecture and pinned versions](docs/architecture.md).
- [Authentication and endpoint/scope catalog](docs/authentication.md).
- [JSON contract and errors](docs/json-contract.md).
- [Workflow decisions](docs/workflows.md).
- [F0 validation log](docs/validation-f0.md).
- [F1 validation log](docs/validation-f1.md).
- [F2 validation log](docs/validation-f2.md).

The next phase is F4: progress and scripting. Workflow commands can now modify issues after confirmation; start with `jflow transitions APP-123` and `jflow start APP-123 --dry-run`. See the [workflow guide](docs/workflows.md) before applying changes.
