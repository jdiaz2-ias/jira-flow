# ADR-004: secrets and Keychain feasibility

Status: accepted for design; macOS runtime validation pending. Date: 2026-09-06.

Decision: keep `SecretStore` injectable and pin `github.com/zalando/go-keyring v0.2.8`. Without persistence available, use a session credential; never a plain-text fallback.

Review of [keyring_darwin.go v0.2.8](https://github.com/zalando/go-keyring/blob/v0.2.8/keyring_darwin.go) shows that Set starts `/usr/bin/security -i` and sends the command with the secret via stdin. Get/Delete carry resource identifiers; the value is not part of Set's argv. The `TestPinnedMacKeyringDoesNotPassSecretInArgv` test verifies the structure of that invocation in the downloaded module. It is not a dynamic Keychain test.

The implementation has a 4096-byte limit for the interactive command and builds/validates that command after starting the process. In F1 the size must be checked with a conservative bound before calling Set, and cancellation/process management must be designed: the library does not accept context. A large token will not be truncated; its persistence will be rejected and it can be used ephemerally. If those requirements are not met by the wrapper, replace the backend via the port.

Do not log stdout/stderr of secret reads; nor responses that may contain values. `Secret` redaction is presentation defense, not memory encryption. The opt-in test `TestMacKeychainRoundTrip` is prepared for macOS and must be run before claiming validated persistent support.

This evidence resolves the feasibility of avoiding secrets in argv for the pinned version, without asserting that Linux has tested the macOS keyring.

## F1 update (2026-09-07)

The library wrapper does not allow canceling its processes. The active backend is replaced by an own executor using `exec.CommandContext`, with a 15-second timeout and without propagating stderr. macOS still uses `/usr/bin/security -i`; the complete command is bounded before starting the process and the secret is hex-encoded so as not to introduce interpretable quotes or newlines. It is validated by later read: interactive mode may exit with code zero after a command failure. Get decodes the hex; identifiers are random 32-character hex strings.

Linux uses `secret-tool` via stdin and requires libsecret-tools/Secret Service. This runtime dependency allows cancellation without goroutines abandoned by the library. It has not been tested against a real Secret Service in F1. go-keyring remains pinned only for historical foundation audit.

The macOS backend passed a real test with a synthetic secret (write/read/delete) on arm64. The persistence limit is 1500 bytes; an oversized credential is rejected without starting the process and can be used ephemerally. The replacement keeps the previous credential until the new configuration is saved; pending cleanup references allow retry with logout if the keyring fails afterward.
