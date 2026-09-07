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

Codes 3–11 are reserved in contracts and will be activated with the corresponding use cases. Detailed keys such as `transition_ambiguous` will be added as refinements in F3 without changing the meaning of exit 6. Do not change a v1 field type: incompatible changes require a new version.
