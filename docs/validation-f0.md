# Validation F0

Date: 2026-09-06. Execution environment: Linux Mint 22.3, amd64. Toolchain: Go 1.27.1 installed in the user's local directory, without modifying shell startup files. The official archive was verified by SHA-256 before extraction.

## Recorded results

| Check | Result |
| --- | --- |
| `make check` | Passes: format, vet, CLI/contract/process/layer tests, and keyring audit |
| `make test-race` | Passes on Linux amd64; CLI test was repeated after its final adjustment |
| `go mod verify` | All modules verified |
| `make build` | Produces `bin/jflow`, executed on this machine |
| `make build-all` | Compiles the four platforms, both CLI and `foundation` imports |
| `jflow --help` | Only shows help and version as current commands |
| `jflow version --format=json` | Valid JSON v1 with binary metadata |
| govulncheck 1.7.0 | No vulnerabilities found; also run with `-tags=foundation` |
| Fixtures | Nine synthetic JSONs parseable |

Ten test functions were executed on Linux, one of them with seventeen argument/output cases. Checks include a real `jflow` process, exit states, a single JSON document, secret redaction, write failures, and absence of UI/provider imports in the core.

The first run of the source audit needed to complete module metadata via `go mod download`; afterward it passed without calling the keyring. Security results correspond to the database consulted on this date, not a future guarantee.

## Observed matrix

| Target | CLI + foundation compilation | Binary execution | Real keyring |
| --- | --- | --- | --- |
| Linux amd64 | Yes | Yes | Not used in F0 |
| Linux arm64 | Yes | No | No |
| macOS amd64 | Yes | No | No |
| macOS arm64 | Yes | No | No |

`file` identifies the two Linux binaries as static ELF and the macOS ones as Mach-O for their architecture. CGO is disabled for these builds; macOS still depends on operating-system components.

## Explicit limits

- The CI workflow is created, but has not been run on GitHub: the repository is local.
- macOS SecretStore viability was reviewed in source and via a structural test. The opt-in real test is prepared for a Mac with unlocked keychain.
- No Jira tenant or credentials are configured; F0 does not send Jira requests.
- Authentication, TUI, cache, executable workflows, and release packaging belong to later phases.

## Reproduction

```bash
export PATH="$HOME/.local/share/jira-flow/toolchains/go1.27.1/bin:$PATH"
go mod download
make check test-race build-all build
make security
./bin/jflow --help
./bin/jflow version --format=json
```

On other machines, install the pinned Go version and omit the export specific to this installation.
