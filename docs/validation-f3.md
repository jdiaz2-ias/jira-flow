# F3 validation

Validated locally on macOS arm64, 2026-09-07, using Go 1.27.1. All Jira requests in automated tests use synthetic HTTPS servers or injected ports; no real issue was modified.

- `make check`: formatting, vet, unit/integration tests, and foundation checks pass.
- `make test-race`: passes, including cancellation after a server receives a transition.
- `make security`: no vulnerabilities found.
- `go mod verify` and `go mod tidy`: verified, with no module file changes.
- `make build-all build`: Linux/macOS amd64/arm64 builds pass without CGO; local binary built.

Coverage includes live metadata decoding, scoped/unscoped paths, minimal POST payloads, required field and allowed-value validation, ADF text, ambiguous Done candidates, explicit loops, mapped destinations, stale state/schema conflicts, and preview tampering. CLI tests cover JSON/no-input, missing approval, dry runs, field files, interactive selection/confirmation/cancellation, local mappings, and private action history. Provider tests assert a single write for rejection, throttling, server failure, and lost responses. Verification requires both destination and requested field values; unknown writes remain unknown.

Limitations: real Jira transition acceptance and Linux Secret Service still require environment validation. Custom plugin fields and rich ADF input are unsupported. Revalidation cannot remove the server-side race after the last read. The local action record is metadata, not an authoritative Jira audit or idempotency ledger. F4 progress/scripting and F5 TUI are not part of this increment.
