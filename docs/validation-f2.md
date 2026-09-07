# Validation F2

Date: 2026-09-07. Local environment: macOS arm64, Go 1.27.1.

## Implemented scope

- `mine`, `list`, `search`, `show`, `link`, `open`, and `config set default_project`.
- JQL constructor with escaped literals, categories, and allowed ordering; POST `/rest/api/3/search/jql` with selected fields.
- Search pages, deduplication, cursors tied to profile/identity/JQL, retention of leftover rows, and partial results.
- Normalized ADF description, subtasks, and links; paginated comments/history on demand.
- Bounded in-memory cache; refresh, explicit offline diagnosis, total deadline, text/table/JSON without terminal controls.
- macOS/Linux browser adapters with separate arguments and URL preserved if the launcher fails.

## Local evidence

Tests use only synthetic data. The user previously confirmed that F1 connects to their real Jira; F2 did not query their issues or modify that configuration during development.

- `make check`: passes; format, vet, application/CLI/HTTP tests, and foundation audit.
- `make test-race`: passes; race-detector tests.
- `make build-all`: passes; CLI and foundation for Linux/macOS × amd64/arm64.
- `make build`: passes; local binary `bin/jflow`.
- `go mod verify`: all modules verified; `go mod tidy` does not alter go.mod/go.sum.
- `make security`: govulncheck 1.7.0 found no vulnerabilities.
- Remote CI is checked in the PR; previous results correspond to the local machine.

Checked cases: both token methods; field selection without per-row requests; missing states/fields; 400/401/403/404 and invalid responses; 429 retries with Retry-After; rejected redirects; cancellation; empty results, limits, repeated cursors, duplicates, and resumption with remainders; cursor signing and isolation; TTL/refresh/auth failures without fallback; unknown ADF and ANSI/OSC; partial sections; simulated browser; real binary without TTY and a single JSON document even with error.

## Observed limits

- F2 integration with real Jira pending user testing. HTTP tests are local HTTPS; they do not verify project permissions or tenant scopes.
- No real browser was launched in tests: the injected executor is verified, without opening windows.
- Cache is per-process only: 128 entries, up to 16 MiB total and 4 MiB per entry; listing TTL 60 s and detail TTL 30 s. Each normal CLI invocation starts empty. `--offline` does not recover data from previous commands; persistence corresponds to F4.
- Searches: 50 results/page by default; maximum page size 100, safety maximum of 5000 IDs per cursor chain and 1000 requests per invocation. Results are not transactional snapshots of Jira.
- Cursors are signed, not encrypted. They may contain leftover rows from the page (Jira data), but do not include the authentication token or JQL. They become invalid when profile, identity, credential, query, or fields change. They are not saved to files by the application.
- `show --comments/--history` loads up to 50 items per section by default. `--all` raises the limit to 5000, adjustable via `--section-limit`; offsets are reported per section. A list of visible subtasks is not assumed to allow counting hidden subtasks.
- ADF is normalized to blocks/text: it does not reproduce all visual styles. Unknown nodes keep descendants or show a marker; images and attachments are not downloaded. Depth limit 64 and 10000 nodes, with warning.
- `open` confirms launcher success, not browser load or authentication.

Official references rechecked: [enhanced search](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/), [issues and history](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/), [comments](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-comments/), [ADF](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/), [limits and retries](https://developer.atlassian.com/cloud/jira/platform/rate-limiting/).
