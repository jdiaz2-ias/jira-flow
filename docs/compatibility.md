# Compatibility

Go 1.27.1. Targets: Linux/macOS, amd64/arm64, `CGO_ENABLED=0`. `make build-all` compiles the four combinations.

F0 ran on Linux amd64; F1 is validated locally on macOS arm64. Compiling for another platform does not prove execution. CI defines Ubuntu 24.04 and macOS 15.

## System credentials

macOS: `/usr/bin/security` and unlocked Keychain. F1 real backend test:

```bash
JFLOW_TEST_KEYCHAIN=1 go test ./internal/secretstore -run TestNativeKeychainRoundTrip -count=1
```

Creates and deletes exactly one random synthetic secret reference under `jflow`. Does not require Jira credentials. Passed on the F1 macOS arm64 machine.

Linux: `secret-tool` executable (libsecret-tools) and a D-Bus session with Secret Service. Real run pending; covered with injected executor. If the store is absent or locked, the CLI returns an error and allows retry with `--no-store` and an ephemeral token. There is no plain-text fallback.

The F0 test under the `foundation` tag remains as an audit of the pinned library; the F1 active backend uses its own cancelable processes.

Reports: [F0](validation-f0.md), [F1](validation-f1.md).

## F6 candidate artifacts

`make release-check` packages all four targets and smoke-tests the archive matching the executing host. On 2026-09-22 the macOS arm64 archive ran `version --format=json` and help successfully. Cross-built Linux amd64/arm64 and macOS amd64 archives passed checksum/content checks locally; that is not execution evidence.

CI is configured for native archive smoke on Ubuntu 24.04 and macOS 15; use the actual run's OS/architecture and result as evidence. Linux arm64 execution, a second stable macOS version, clean-machine installs, human terminal acceptance, and macOS signing/notarization remain pending. See [F6 validation](validation-f6.md).
