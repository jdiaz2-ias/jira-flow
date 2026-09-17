# Progress and scripting (F4)

```bash
jflow progress APP-123
jflow progress APP-123 --history --timezone America/Monterrey --format json
jflow summary --include-done
jflow summary --project APP --type Bug --format json
jflow summary --jql 'project = APP AND updated >= -7d' --format json
```

Progress never turns a status name into a percentage. Subtask completion is Done-category count divided by the complete, visible set. Zero subtasks means “No subtasks”; an incomplete set or unknown category has no percentage. Hidden issues cannot be counted. Done includes canceled and other resolutions; the parent's resolution remains visible, including when a Done parent has unfinished subtasks.

Time fields refer to the issue itself. Child and aggregate time are not summed. Missing values stay unknown; zero stays zero. Estimate consumption is spent/original estimate and can exceed 100%; it is not work completion. Jira's `timetracking` data supplies the numeric values. [Atlassian REST field example](https://developer.atlassian.com/cloud/jira/platform/rest/v2/api-group-issue-search/).

`--history` traverses changelog pages, up to `--history-limit 5000`. Time in state is known only when history is complete from offset zero and its latest status destination ID matches the current status. No transition history means unknown; creation date is not guessed. Due dates use the system's local calendar or `--timezone`; daylight-saving changes do not change a calendar day's meaning. `due_in_days` is negative for overdue dates, zero today, positive in the future.

`summary` always traverses pages up to `--max-results 5000`. It shows the exact JQL, number processed, category counts, freshness, and completeness. The default is my pending issues; use `--include-done` for a personal completion fraction. `--project APP` includes all categories unless filtered. Project defaults do not silently change the summary's scope. Raw JQL and category-filtered summaries show counts without a completion percentage. No percentage is shown over a partial, empty, or unknown-category population. The command does not query individual issue details per row.

## Persistent cache and offline use

```bash
jflow config set cache.persist true --profile work
jflow summary --include-done --profile work
jflow summary --include-done --offline --profile work
jflow progress APP-123 --profile work
jflow progress APP-123 --offline --profile work
jflow cache status --profile work
jflow cache clear --profile work
jflow config set cache.persist false --profile work
```

Persistence is off by default and stores Jira content locally when enabled. Directory permissions are 0700 and file permissions 0600. The cache excludes description/comment bodies, changes to those bodies in history, and raw transition verification fields. Summaries, status, issue keys, people, links, and other history fields remain Jira content. Cache is not encrypted.

`show --offline` explicitly warns about omitted bodies and marks its detail incomplete. `show --all --offline` returns code 10 if requested content was omitted. Online `show` loads its bodies from Jira. Metric-only detail and listings can reuse fresh disk data after verifying online identity.

The cache root is `~/Library/Caches/jflow` on macOS and `$XDG_CACHE_HOME/jflow` or `~/.cache/jflow` on Linux. `JFLOW_CACHE` can override it with an absolute path. Only the application's hashed JSON entries inside `v1` are managed. Cache is isolated by configuration path, profile, provider/site, identity, email, credential, generation, query/options, fields, and schema. Offline queries must use the same options and credential as the original request; credential lookup stays local and there is no Jira or TLS-client setup. A miss returns exit 5 with instructions to enable/populate the cache.

Lists expire for online reuse after 60 seconds; details after 30 seconds. Offline can use those stale snapshots for less than seven days, with `fetched_at`, `source`, and `stale` shown. Capacity is 25 MiB globally across namespaces, 1,000 detail entries, and 4 MiB per entry; older entries are evicted. No daemon runs: writes and `cache status` prune expired/over-limit files, while reads refuse expired data. Entries may remain physically present until pruning or clearing.

`--refresh` bypasses both caches; it cannot combine with `--offline`. Online auth failures are never disguised by cached success. Login renewal invalidates the prior generation. Confirmed transition attempts invalidate before and after sending, even when the result is unknown. Dry runs and no-ops leave the cache intact. Disabling persistence invalidates reuse but does not delete files; run `cache clear` to remove them. A cache-write warning does not discard a successful Jira read.

## Pipelines and outcomes

JSON always contains one document, including on errors. Capture the exit status before piping so a successful `jq` cannot conceal a failed command:

```bash
# Works in bash and zsh; jq is an optional external consumer.
if jflow summary --include-done --format json > summary.json; then
  jq '{counts: .data.counts, percent: .data.completed_percent, fresh: (.meta.stale | not)}' summary.json
else
  code=$?
  jq '{error, partial: .data, meta}' summary.json
  exit "$code"
fi
```

```bash
if jflow search --jql 'project = APP' --all --format json > issues.json; then
  jq -r '.data.issues[].key' issues.json
else
  code=$?
  case "$code" in
    10) jq '{partial: .data.issues, meta, error}' issues.json ;;
    130) printf '%s\n' 'Canceled' >&2 ;;
    *) jq '.error' issues.json >&2 ;;
  esac
  exit "$code"
fi
```

Exit 0 can include a deliberately limited page with `complete=false`. Full traversal failure/limits return 10 while retaining collected data; cancellation returns 130. Permission/authentication errors retain their distinct codes if no data was collected. A transition returning 9 is uncertain or accepted but unverified: inspect Jira before deciding what to do; never blindly retry it in a pipeline. See the [JSON contract](json-contract.md) and [workflow guide](workflows.md).
