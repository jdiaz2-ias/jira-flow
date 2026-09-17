# Public JSON v1

The wrapper is implemented in `internal/output`; domain structs are not serialized directly. Stdout contains a single JSON plus newline. On usage errors with JSON, stderr stays empty. An stdout failure itself emits a diagnostic to stderr and code 1.

```json
{
  "schema_version": 1,
  "ok": true,
  "data": {
    "version": "dev",
    "commit": "unknown",
    "go_version": "go1.27.1",
    "os": "linux",
    "arch": "amd64"
  },
  "meta": {},
  "warnings": [],
  "error": null
}
```

On error, `ok=false`, `data=null`, and `error` contains `code`, `message`, `retryable`, and `details`. Internal `Cause` is never exported. Unclassified errors are presented as `internal_error`, without copying private content. JSON help uses `data.help`.

## Reserved codes

| Exit | Domain code | Use |
| --- | --- | --- |
| 0 | — | Success |
| 1 | `internal_error` | Internal/output error |
| 2 | `invalid_input` | Usage/configuration |
| 3 | `authentication_required` | Identity/credentials |
| 4 | `forbidden` | Access denied |
| 5 | `not_found` | Missing or not visible |
| 6 | `validation_failed` | Fields/ambiguity |
| 7 | `conflict` | Concurrent change |
| 8 | `service_unavailable` | Network/429/read |
| 9 | `write_uncertain` | Uncertain write or accepted unverified |
| 10 | `partial_result` | Complete query/batch incomplete |
| 11 | `capability_unavailable` | Function unavailable in context |
| 130 | `canceled` | Cancellation |

Codes 3–11 are used by the implemented access, read, and workflow use cases. `transition_ambiguous` refines exit 6 without changing its meaning. Do not change a v1 field type: incompatible changes require a new version.


## F4 read and metric results

Read metadata includes `profile`, `fetched_at` (RFC3339), `source` (`network`, `memory`, or `disk`), `stale`, and `complete`. Freshness is independent of completeness: a full cached set can be stale, and a fresh page can be incomplete. Search metadata also includes `returned` and nullable `next_page_token`.

`progress.data` contains:

- `issue`: the existing public issue object, including status, resolution, updated date, and due date.
- `subtasks`: `done`, `visible`, `complete`, nullable `percent`, and `scope="visible_subtasks"`. Completeness requires both a complete visible set and known categories.
- `time`: nullable `spent_seconds`, `remaining_seconds`, `original_estimate_seconds`, and `estimate_consumed_percent`, with `scope="issue_only"`.
- Nullable `state_since` and `due_in_days`; `timezone` names the calendar timezone used.

`summary.data` contains `scope` (the exact JQL), `processed`, `counts` (`todo`, `in_progress`, `done`, `unknown`), nullable `completed_percent`, and `includes_done`. The last field means the structured scope permits Done; false also covers arbitrary JQL, where this property cannot be proven. `meta.method` is `visible_issues_by_status_category`. No global total is invented.

A normal requested page may return `ok=true, complete=false`. An interrupted full traversal or guard limit returns `ok=false`, exit 10, and the collected data. Cancellation remains exit 130 with collected data. Unknown/unverified writes remain exit 9; cache warnings never turn them into success or trigger another write. Unknown metrics are `null`, not zero. A known zero is preserved.

Cached `show` output omits description/comment bodies and marks the detail incomplete with a warning. A cached `show --all` with omitted requested content returns exit 10. Hard failures without an issue have `data=null`; search failures can retain an empty or partial issue list. See [pipelines](progress-scripting.md).
