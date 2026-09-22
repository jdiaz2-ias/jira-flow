# F6 validation: distribution candidates (in progress)

Started 2026-09-22 from F5 merge `efa362d`. This increment prepares candidate artifacts; it does not finish Delivery A or publish a stable release.

## Implemented

- GoReleaser 2.18.1 configuration builds Linux/macOS × amd64/arm64 without CGO, with version and full commit embedded, tar.gz archives, SHA-256 manifest and changelog configuration.
- `make release-check` validates configuration, builds a snapshot without publishing, verifies exactly four archive checksums and required contents, and executes native archived `version --format=json` and help.
- `make release-tool` downloads the pinned upstream tool and verifies its upstream checksum manifest. This is integrity checking, not an independent signature verification.
- Cobra generates bash/zsh/fish completions with plain and JSON v1 contracts. Invalid shell/missing arguments and output failures are tested.
- Build-generated recursive help, completions and dependency license texts are included in each archive. Dependency inventory includes the four target platforms, tests and foundation dependencies. The project license remains undecided.
- CI is configured to package on Linux/macOS, smoke-test each native archive and retain the four Linux-built archives plus manifest for 14 days.
- Installation/removal, profile/keyring, SSH, proxies, completion setup and macOS signing limitations are documented in [install.md](install.md).

## Local evidence

macOS arm64, Go 1.27.1, 2026-09-22: `make check test-race release-check`, `make security`, `go mod verify`, `go mod tidy -diff`, and `git diff --check` passed. Four-archive checksum/content verification and native archive smoke passed; an intentionally altered checksum was rejected. GoReleaser configuration check and the tool installer checksum verification passed. Bash/zsh generated-script syntax checks passed; fish runtime validation remains pending. Remote CI evidence is attached to the increment's PR.

The first full run exposed an intermittent PTY harness shutdown timeout. The harness now keeps draining terminal output while awaiting process exit, preventing its own unread buffer from blocking a final render. The keyboard journey passed three consecutive uncached runs after this change, followed by the full normal/race suites; exit codes and terminal restoration assertions remain enforced.

No real Jira API, keyring credential, browser, or production issue is used by packaging or automated acceptance. Snapshot builds include working-tree contents; only clean committed CI candidates should be used as shareable evidence tied to a commit.

## Acceptance still required

| Check | Status / evidence needed |
| --- | --- |
| Project license and maintainers | Owner decision pending; no license invented |
| Real Jira Cloud reads | Authorized tenant, profile method, date, issue visibility/pagination, missing fields, progress/summary and sanitized results |
| Real Jira writes | Explicitly designated test issue; dry-run, ambiguity, required fields, start/done/close and verified final state; never retry an uncertain outcome blindly |
| Human Linux/macOS keyboard journey | Terminal/version, architecture, query → start → review → complete → open, resize, cancellation and restoration |
| Linux Secret Service | Synthetic secret roundtrip against an actual session service |
| Execution on all four targets | Native or documented emulated execution; compilation alone is insufficient |
| Clean-machine install/uninstall | Checksum, PATH, completion, upgrade/rollback and preserved configuration |
| macOS delivery | Developer ID/notarization decision and observed Gatekeeper behavior |
| Dependency/license/security review | Generated notices available; review obligations and secret exposure before public release |
| Published artifacts | No release tag, public release, signing/provenance or Homebrew tap created |

Record each real acceptance run with commit/version, OS/architecture, terminal, token method (never token), authorized test scope, commands/actions, observed outcomes, and remaining limitations. Do not upload raw issue payloads, environment dumps, or credentials as evidence. An external acceptance result must be supplied or actually observed before marking its criterion passed.

## References

Configuration follows the official [GoReleaser Go builds](https://goreleaser.com/customization/builds/builders/go/), [archives](https://goreleaser.com/customization/package/archives/) and [release configuration](https://goreleaser.com/customization/publish/scm/) documentation, checked 2026-09-22. The configured draft release behavior is preparation only; this increment runs snapshots exclusively.
