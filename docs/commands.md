# CLI contract (F1–F4)

| Command | Behavior |
| --- | --- |
| `version` | Version; no network or configuration |
| `auth login` | Validates `myself`, saves profile and optionally credential |
| `auth status` | Checks local token availability; does not validate with Jira |
| `auth logout` | Removes local profile and credentials; does not revoke in Atlassian |
| `profile list` | Lists profiles and marks the persisted active one |
| `profile use NAME` | Changes active profile |
| `me` | Queries and displays Jira identity |
| `doctor` | Checks configuration, credential source, identity, connectivity, and stdin TTY |
| `config path` | Shows configuration path |
| `config validate` | Validates schema without showing content |

`--profile` temporarily selects a profile; `--email` changes the email for that invocation. Precedence: flags > `JFLOW_PROFILE`/`JFLOW_EMAIL` > profile > global email. `auth login` saves the validated profile and activates it. Only the environment email is kept when performing an explicit login.

`auth login` accepts `--site`, `--method=api-token-unscoped|api-token-scoped`, `--cloud-id`, `--token-stdin`, and `--no-store`. In a terminal it can prompt for missing data and hidden token. `--no-input`, `JFLOW_NO_INPUT=1`, or `JFLOW_NO_INPUT=true` disable questions; explicit stdin remains allowed. `me` and `doctor` also accept `--token-stdin`.

Explicit stdin credential takes precedence over `JFLOW_TOKEN`, which takes precedence over the keyring. An environment token is never persisted automatically. Without a keyring, use `--no-store` and supply the credential again on each invocation. `--token VALUE` is never accepted.

`--format=json` produces a single object with `schema_version`, `ok`, `data`, `meta`, `warnings`, and `error`. It can go before or after the subcommand. JSON help is in `data.help`; `jflow help auth login` works. Normal output is plain text, no colors.

- No subcommand: starts the TUI only with terminal stdin/stdout and a capable terminal; otherwise returns code 2 with CLI guidance.
- Invalid arguments/configuration: 2; authentication: 3; permissions: 4; not found: 5; service unavailable: 8; cancellation: 130.
- `plain` or `json`; F2 also supports `table` in listings. Last occurrence of `--format` wins; `--` ends options.
- Write failures: code 1, never success.
- Parse diagnostics do not repeat arbitrary arguments; HTTP errors do not include bodies or Authorization.

`doctor` validates identity permissions, not issue or transition permissions. If it uses environment/stdin, it reports `keyring_checked=false`; it does not claim the keyring works.

## F2: issue reading and links

```bash
jflow mine --format table
jflow mine --project APP --status-category in-progress --sort=-priority,-updated
jflow mine --include-done --updated-since=-7d --limit 20
jflow config set default_project APP --profile work
jflow list --type Bug
jflow search --jql 'project = APP AND priority = High ORDER BY updated DESC' --all --format json
jflow show APP-123
jflow show APP-123 --comments --history --section-limit 20
jflow link APP-123
jflow open APP-123
```

`mine` uses `currentUser()` and excludes Done category unless `--include-done` or an explicit category is used. `list` needs a default project or explicit filter. Project: `--project` > `JFLOW_PROJECT` > profile `default_project`. `search --jql` does not inherit that project nor structured filters. `--sort` accepts `updated`, `created`, `priority`, `key`, `due`, comma-separated; prefix `-` means descending. There is always a key tiebreaker.

`--fields` selects from `summary,status,assignee,priority,issuetype,project,updated,duedate,resolution`; unrequested data remains unknown. Key and ID come from the Jira result. There is no per-row request.

Search pagination: `--limit 50`, `--page-size 50`, `--all`, `--max-results 5000`, `--page-token CURSOR`. `--all` and `--limit` are incompatible. A normal limit succeeds with `meta.complete=false`; later failure or `--all` cap returns code 10, preserving data. JSON includes `meta.returned` and `meta.next_page_token`; the cursor is reused with the same profile, JQL, and fields. There is no inferred global total.

`show` loads description, subtasks, and links. `--comments` and `--history` enable their endpoints; `--page-size` controls the request and `--section-limit` the total per section. `--all` walks requested sections up to 5000 items by default. To continue, use `--comments-start`/`--history-start` with each section's `next_start`. A section failure preserves detail in JSON and returns code 10.

Read flags: `--timeout 30s` (up to 10m), `--token-stdin`, `--refresh`, `--offline`. Cache is in memory by default; `config set cache.persist true` enables private persistence for the selected profile. Offline misses return code 5. Offline uses the same local credential and query options, without contacting Jira. Listings last 60 s and details 30 s. A 401/403 is not replaced by old data. Transport retries transient reads up to three sends and respects Retry-After within the time budget.

`table` is supported in `mine`, `list`, and `search`; other commands accept plain/JSON. `--no-color` and `--ascii` keep output unadorned; they do not alter Unicode content from Jira. `--verbose` prints the command to stderr without tokens, bodies, or JQL. JSON implies `--no-input`.

`link` uses the browsable site URL and does not query Jira or the keyring. In text it prints only the URL and a newline. `open` uses `/usr/bin/open` on macOS or `xdg-open` on Linux without a shell; if there is no graphical session/launcher, it returns code 11 and keeps the URL.

## F3: transitions

| Command | Behavior |
| --- | --- |
| `transitions KEY` | Read fresh transition IDs, destination states, and field requirements |
| `transition KEY --transition-id ID` | Apply an explicitly selected transition, including loops |
| `start KEY` | Resolve a direct transition to In progress |
| `done KEY` | Resolve a direct transition to Done; multiple candidates require selection |
| `close KEY` | Require an explicit ID or validated mapping |
| `workflow map KEY --intent done --transition-id ID` | Save a local mapping for the issue's current project, type, and state |

Mutation commands accept `--transition-id` (alias `--id`), `--fields-file`, `--dry-run`, `--yes`, and `--no-record`. They also support `--timeout`, `--token-stdin`, and global profile, output, and input flags. `transitions` accepts table output; mutations use plain or JSON. JSON implies no-input. Without a terminal, applying requires `--yes`; this flag never resolves ambiguity or supplies missing fields. Mapping writes local configuration and does not transition the issue.

Validation or ambiguity returns 6, stale preparation/mapping returns 7, and an uncertain write or accepted but unverified result returns 9. A no-op returns 0 without sending a write. See [workflows](workflows.md) for examples, field formats, and outcome semantics.


## F4: progress, summaries, and persistent cache

| Command | Behavior |
| --- | --- |
| `progress KEY` | Status, resolution, visible-subtask fraction, issue-only time, updated and due dates |
| `progress KEY --history` | Additionally traverse status history, capped by `--history-limit` (default 5000) |
| `progress KEY --timezone America/Monterrey` | Compare due dates using this IANA timezone; defaults to system local |
| `summary` | Traverse my pending issues and count categories; no completion percentage |
| `summary --include-done` | Include completed assignments; permit percentage when fully loaded and categories known |
| `summary --project APP` | Explicit project scope, including Done by default |
| `summary --jql QUERY` | Counts over arbitrary JQL; no inferred completion percentage |
| `config set cache.persist true\|false` | Toggle persistence per profile; default false |
| `cache status` | Show location, enabled setting, entries, bytes, and stale entries |
| `cache clear --profile work` | Invalidate and remove cached results for this profile/configuration |

`progress` and `summary` accept the common read flags and plain/JSON output. `summary` also accepts `--status-category`, `--type`, `--priority`, `--updated-since`, `--page-size` (default 100), and `--max-results` (default 5000). It always traverses pages, so it has no `--all` or `--limit`. Its scope is personal unless `--project` or `--jql` is explicit; it does not inherit `default_project` or `JFLOW_PROJECT`. Raw JQL cannot combine with structured filters.

Full rules and pipelines: [progress and scripting](progress-scripting.md).

## Interactive interface (F5)

`jflow` or `jflow ui [--profile NAME] [--offline | --refresh] [--timeout 30s] [--theme auto|dark|light|mono] [--ascii] [--no-color] [--no-record]` opens the interactive interface. The timeout bounds each read or workflow operation, not the entire session. Existing profile credentials, project defaults, workflow mappings, caches, and private action records are reused. `--no-record` opts out of action metadata.

Both stdin and stdout must be terminals. `TERM=dumb`, `--no-input`, `JFLOW_NO_INPUT=1|true`, and an explicit `--format` prevent full-screen entry. With no profiles, the existing login wizard runs before full-screen mode, asks whether the token is scoped, and reads the token with echo disabled. Cancellation restores terminal settings. Token stdin is not supported by `ui`.

`jflow ui --accessible` produces a plain, one-shot assigned-issue listing without full-screen mode or workflow initialization. It works with pipes and `--no-input`; use `show KEY --format plain` for detail and ordinary workflow commands for further actions. Any explicit `--format` remains invalid for `ui`; use the corresponding CLI command for JSON.

| Key | Behavior |
| --- | --- |
| Arrows / `j/k` | Select an issue or scroll focused detail/review |
| Enter | Load detail, choose a transition, validate field input, or acknowledge a result; never confirms a write |
| Tab / Shift+Tab | Change list/detail focus |
| `/` | Edit a filter over loaded keys/summaries only; Enter accepts, Esc clears |
| Ctrl+F | Edit explicit remote JQL; Enter queries, Esc cancels |
| `s` / `d` / `x` | Prepare start / done / close using the existing workflow resolver |
| `t` | Choose from available transitions |
| `e` in review | Edit required or optional transition fields |
| `y` in review | Confirm the fully resolved, visible review; long reviews must be scrolled to the end |
| `n` / Esc in review | Cancel; entered fields require explicit discard confirmation |
| `o` | Suspend terminal UI, launch browser, restore UI, and report success/failure |
| `y` outside dialogs | Display a selectable URL; no clipboard or OSC 52 side effect |
| `n` / `r` | Next page (5,000 loaded-item guard) / refresh current view |
| `?` | Keyboard help |
| Esc | Cancel read or return; result acknowledgement refreshes list/detail |
| `q` / Ctrl+C | Exit with draft protection; during writes Ctrl+C cancels the request and awaits its outcome |

Text input owns its keys; letters never trigger workflow shortcuts while typing. Ctrl+U clears input, and bracketed paste is supported. Fields use the same schema conversion and validation as CLI prompts, including allowed IDs, numbers, dates, booleans, arrays, and plain-text ADF. Unsupported fields can be completed in Jira via `o`. Writes are unavailable offline.

During Apply, navigation and duplicate mutations are blocked. The shared use case revalidates the prepared issue and transition, sends at most once, invalidates caches, and verifies the destination/fields. Results distinguish verified, no-op, failed, accepted-but-unverified, and unknown. No uncertain write is automatically retried. Exiting while Apply is pending or an uncertain result remains unacknowledged returns uncertain exit code 9; ordinary cancellation returns 130.

The screen reports source, fetched time, staleness, completeness, partial warnings, errors, and measured progress. At 110+ columns list/detail share the screen with approximately 55/45 widths; 80–109 columns alternate focus, and 60–79 use compact rows. Below 60×15 it shows guidance and disables hidden actions. Resizing preserves selection and drafts. Auto theme uses a terminal background response with a dark fallback; `NO_COLOR`/`--no-color` override theme colors. ASCII replaces the panel separator. Jira text is sanitized before styling.
