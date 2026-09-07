# Workflow transitions (F3)

Read available transitions from your issue before choosing an ID:

```bash
jflow transitions APP-123 --format table
jflow start APP-123 --dry-run
jflow done APP-123 --transition-id TRANSITION_ID --fields-file fields.json --dry-run --format json
jflow done APP-123 --transition-id TRANSITION_ID --fields-file fields.json --yes --no-input
```

Replace the issue key and IDs with values returned by your Jira. Transition IDs are not status IDs. `start`, `done`, and `close` are intents, not universal status names. Resolve and Cancel can both lead to the Done category; `--yes` cannot decide between them. `close` always needs an explicit choice or mapping, even with one available transition.

In a terminal, unresolved transitions and missing supported fields are prompted before a preview and confirmation. Optional field editing is opt-in. JSON, `--no-input`, and `--token-stdin` disable questions. `--dry-run` reads and validates without sending a write or creating an action record.

## Fields

A fields file contains only a JSON object of field IDs and values, at most 1 MiB. It is never rewritten or deleted. For example:

```json
{
  "resolution": {"id": "RESOLUTION_ID"},
  "assignee": {"accountId": "ACCOUNT_ID"},
  "duedate": "2026-10-01"
}
```

Only include fields exposed by the selected transition. Required values and allowed IDs are validated from fresh metadata. Supported types include text, numbers, booleans, dates, datetime, account IDs, select IDs, and arrays such as multiselect. Basic ADF text is supported for description and textarea fields; terminal text is wrapped as a paragraph. Plugin-specific schemas and rich ADF nodes are not supported: use Jira's browser interface. The command never inserts a resolution, comment, or status update implicitly. A required field with a server default may be omitted.

## Local mappings and no-ops

```bash
jflow workflow map APP-123 --intent done --transition-id TRANSITION_ID
jflow done APP-123 --dry-run
```

A rule stores the current profile's project ID, issue type ID, intent, source status ID, transition ID, and expected destination ID. The command validates availability and destination on every use. Invalid rules stop with a conflict; there is no automatic fallback. An explicit transition ID overrides the mapping. Rules survive token renewal for the same site and account, and are cleared when that identity changes.

Without a mapping or explicit ID, `start` and `done` return `noop` if their category is already satisfied. A mapped intent requires its exact destination: Cancelled does not satisfy a rule targeting Resolved merely because both are Done. Explicitly selected loop transitions still execute. F3 attempts only one direct transition, never a multi-step path.

## Write and verification contract

The flow is prepare → validate fields → preview → confirm → revalidate → apply once → verify. Revalidation rereads identity, issue, available transitions, and metadata. A changed issue or schema stops before writing. There remains a small race between revalidation and Jira processing the write; this is not an atomic compare-and-set API.

The write uses a dedicated transport path that never retries or follows redirects. After Jira returns HTTP 204, up to three observations compare the destination and all explicitly sent fields. Reads can use bounded transport retries within the command deadline.

| Result | Meaning |
| --- | --- |
| `verified` | Jira accepted the write and requested state/fields were observed |
| `accepted_unverified` | Jira accepted the write but verification did not establish the result; exit 9 |
| `unknown` | A lost response or ambiguous server response prevents establishing acceptance; exit 9 |
| `failed` | Rejected or stopped before a successful write; nonzero exit |
| `noop` | Intent already satisfied; no write sent |

An observed destination never upgrades an unknown write to verified. Inspect the issue before retrying an uncertain result. The in-memory issue cache is invalidated after a write attempt.

Action attempts store a random ID, issue key, intent, transition ID, timestamp, and outcome in a private per-profile/site/account directory under the platform state path. Files contain no fields, issue descriptions, or credentials. Old records are pruned on subsequent saves after seven days; this is not a background deletion service. `--no-record` disables recording. `JFLOW_CONFIG` overrides place state next to that configuration file for isolated installations.

API reference: Atlassian's [issue transitions API](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/) documents `expand=transitions.fields` and the HTTP 204 success response. Automated fixtures are synthetic; real tenant acceptance is tracked separately.
