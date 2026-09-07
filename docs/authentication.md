# Authentication and reading F1–F2; catalog F3–F4

F1 implements personal Cloud authentication and `myself`; F2 adds enhanced search, detail, comments, and read-only history. Transitions and writes remain as specification for later phases. REST contract and tokens rechecked on 2026-09-07; the user confirmed the F1 connection with their tenant; F2 tests use synthetic data.

## Personal Cloud methods

| Method | Authorization | REST base | Browsable URL |
| --- | --- | --- | --- |
| API token without scopes | Basic email:token | `https://site.atlassian.net` | The site |
| API token with scopes | Basic email:token | `https://api.atlassian.com/ex/jira/{cloudId}` | The site |

Service-account tokens and other integration types are not inferred from this contract. Cloud ID must be real and provided/discovered explicitly. Do not extract it from the hostname or send credentials to a different endpoint to “test”. Source: [personal tokens](https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/).

## Endpoints and permissions

The scopes column lists the **classic OAuth scopes published in the REST reference**, as a technical catalog, not a promise that every token mode accepts an identical list. In F1, document the selection offered by the tested tenant's token console. Project permissions and visibility are still required.

| Command | Method/path | Classic reference scope | Permissions/context |
| --- | --- | --- | --- |
| `auth login`, `me`, `doctor` | GET `/rest/api/3/myself` | `read:jira-user` | Jira access |
| `mine`, `list`, `search`, `summary` | POST `/rest/api/3/search/jql` | `read:jira-work` | Browse Projects and issue security |
| `show`, `progress` | GET `/rest/api/3/issue/{key}` | `read:jira-work` | Issue visible |
| `transitions`, prepare action | GET `/rest/api/3/issue/{key}/transitions` | `read:jira-work` | Available transitions for that identity |
| `start`, `done`, `close`, `transition` | POST `/rest/api/3/issue/{key}/transitions` | `write:jira-work` | Browse Projects and Transition Issues |
| `show --comments` | GET `/rest/api/3/issue/{key}/comment` | `read:jira-work` | Issue/comment visibility |
| `show --history` | GET `/rest/api/3/issue/{key}/changelog` | `read:jira-work` | Issue visible |
| Discover fields | GET `/rest/api/3/field` | `read:jira-work` | Fields visible in context |
| `link`, `open` | No Jira call | None | Local configuration |

Granular scopes documented for the first endpoints:

- `myself`: `read:application-role:jira`, `read:group:jira`, `read:user:jira`, `read:avatar:jira`.
- POST `search/jql`: `read:issue-details:jira`, `read:field.default-value:jira`, `read:field.option:jira`, `read:field:jira`, `read:group:jira`.
- GET transitions: `read:issue.transition:jira`, `read:status:jira`, `read:field-configuration:jira`.
- POST transitions: `write:issue:jira`, `write:issue.property:jira`.

Do not request administrative permissions for a personal query/transition flow. Do not reuse the granular GET search list for POST search; published scopes may differ. Commands that prepare and apply need the union of their reads and write.

Sources: [myself](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-myself/), [search](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/), [issues/transitions](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/), [comments](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-comments/), [fields](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-fields/). Reviewed: 2026-09-07 for F2 reads; revalidate when enabling new operations.

## Credentials and macOS

F1 implements `SecretStore` with cancelable processes: `/usr/bin/security -i` on macOS and `secret-tool` on Linux. Secrets travel via stdin, never via argv. See [ADR-004](adr/004-secrets.md). An available keyring enables persistence; a session without one uses an ephemeral token via environment/stdin with `--no-store`. There is no plain-text file fallback. The configuration contains only a random reference; each profile is associated with the verified site and account ID.
