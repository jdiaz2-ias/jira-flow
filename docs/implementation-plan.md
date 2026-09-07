# Implementation plan: Jira Flow CLI

Date: September 6, 2026. Plan version: 1.0. Language: Spanish.

Work name: **Jira Flow**. Proposed executable: **`jflow`.** The name is provisional; check availability before publishing. This document specifies a product to implement: the commands, screens, and configurations are proposed contracts, not already existing features.

## 1. Objective and main decisions

Build a console tool for Linux and macOS that lets the authenticated user query their assigned Jira issues, review their status and progress, start work, complete or close issues, check their links, and open them in a browser. It must serve both daily interactive work and scripts, and allow adding features without rewriting the interface or Jira client.

The implementation will have two entry points sharing the same use cases: a CLI with subcommands and a TUI, that is, an interactive interface inside the terminal. It does not require its own server or a permanent process for the first version.

Project design decisions:

| Aspect | Decision |
| --- | --- |
| Language | Go; initial toolchain 1.27.1, subject to updating patches before implementing |
| CLI | Cobra for commands, help, and completion |
| TUI | Bubble Tea v2, Bubbles v2, and Lip Gloss v2 |
| Initial integration | Jira Cloud, REST API v3, own small HTTP client |
| Initial authentication | Email and API token, with explicit support for scoped and unscoped tokens |
| Configuration | Versioned JSON, profiles, and configuration commands |
| Credentials | macOS Keychain or Linux Secret Service; environment variables for sessions without keyring |
| Persistence | Atomic JSON files for configuration, preferences, and optional cache |
| Distribution | Binaries for Linux/macOS, amd64/arm64; GoReleaser and later Homebrew |
| Extensibility | Small interfaces, internal action registry, and provider adapters |
| Product language | Messages initially in Spanish; commands and JSON keys in English |

Go makes it easy to deliver an executable without installing a Python or Node runtime. Cobra provides structure and completion. The Charm libraries provide terminal components and styles. This is an engineering choice for this project, not the result of a comparative benchmark. The v2 versions of Charm are already published and their paths are `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, and `charm.land/lipgloss/v2`. Sources: [Go](https://go.dev/doc/devel/release), [Cobra](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md), [Charm v2](https://charm.land/blog/v2/).

## 2. Scope, assumptions, and deliverables

It assumes personal or internal team use with an existing Jira account. There is no domain, project, workflow, permissions, or real credentials yet; the first phase will implement fixtures and real validation will be done in a test project.

Linux/macOS compatibility applies from the first delivery. Jira Data Center compatibility is a separate extension: Data Center support must not be announced until its adapter is implemented and tested. Do not confuse the CLI operating system with the Jira deployment type.

### Delivery A: usable core, mandatory requirement

- Authentication, profiles, current identity, and diagnostics.
- List assigned to me, JQL search, and common filters.
- Query detail, status, dates, subtasks, links, and paginated comments.
- Show verifiable progress with scope and data freshness.
- Print URL and open the issue in the browser.
- Discover transitions and start, complete, and close according to the workflow.
- Request transition fields, confirm, and verify results.
- TUI with listing and detail, keyboard, light/dark theme, and accessible mode.
- Stable text and JSON output, exit codes, and shell completion.
- Binaries and tests for both operating systems.

### Delivery B: productivity

- Create comments, assign to me, edit common fields, and reopen via transitions.
- Saved views, local favorites, activity history, and daily summary.
- Personal board by status category and sprint queries when Jira Software exists.
- Manual time logging, conditioned on configuration and permissions.
- Explicit extraction of a Jira key from the current Git branch.

### Delivery C: extensions

- Create issues from creation metadata and local templates.
- Attachments, issue links, epics, and sprint metrics.
- Batch operations with individual review and partial reporting.
- Jira Data Center adapter and external extension protocol.
- Evaluate OAuth for organizational distribution; local stopwatch with explicit publishing.

Out of the first version: managing workflows or permissions, deleting issues, automations that mutate Jira in the background, bidirectional offline sync, sending to chat/email, and handling Jira Service Management requests/approvals.

## 3. Expected daily experience

Illustrative example of the full journey:

```bash
# Initial setup; the wizard prompts for the token without showing it.
jflow auth login --profile work --site https://company.atlassian.net
jflow auth status
jflow me

# Review what I have pending.
jflow mine
jflow mine --status-category in-progress
jflow show APP-123
jflow progress APP-123

# Start; the concrete transition is shown before sending.
jflow start APP-123

# Check or share the URL.
jflow link APP-123
jflow open APP-123

# Finish; complete fields required by the workflow.
jflow done APP-123
jflow show APP-123 --refresh

# Interactive interface.
jflow ui
```

`jflow` with no arguments starts the TUI if stdin and stdout are terminals and the environment supports interaction. If there is no profile, it shows the setup wizard. In a pipe it prints a short help and exits with code 2; it must never accidentally enter full-screen mode.

`jflow mine` always produces a single-run listing. The explicit choice of `ui` keeps scripts predictable. Read commands do not change assignments or states.

## 4. Command and option contract

### 4.1 Core commands

| Command | Behavior |
| --- | --- |
| `jflow auth login` | Create or update profile credentials and check identity |
| `jflow auth status` | Show profile, site, method, and credential source, never the secret; `--verify` checks network |
| `jflow auth logout` | Remove local credential and profile identity cache; does not revoke the token in Atlassian |
| `jflow me` | Get authenticated identity |
| `jflow profile list` | List profiles and mark the active one |
| `jflow profile use NAME` | Change active profile locally |
| `jflow config path` | Show configuration path |
| `jflow config validate` | Validate schema and consistency without printing secrets |
| `jflow config set KEY VALUE` | Modify an allowed key with safe type conversion |
| `jflow mine` | List current user assignments, excluding Done category by default |
| `jflow list` | Query the default project; require explicit filter if no project |
| `jflow search --jql QUERY` | Run user-supplied JQL |
| `jflow show KEY` | Detail; `--comments`, `--history` enable paginated sections |
| `jflow progress KEY` | Status, resolution, subtasks, and available time data |
| `jflow summary` | Counts by category over an explicit scope or the personal view |
| `jflow link KEY` | Write the browsable URL and newline; does not need to query Jira |
| `jflow open KEY` | Open URL in default browser and report whether the process could launch |
| `jflow transitions KEY` | List IDs, names, destinations, and fields of available transitions |
| `jflow transition KEY --id ID` | Run an explicit transition |
| `jflow start KEY` | Resolve start intent using the rules in section 9 |
| `jflow done KEY` | Resolve complete intent using the rules in section 9 |
| `jflow close KEY` | Resolve close intent using configured rule; not a blind alias of done |
| `jflow workflow map` | Wizard to save a validated mapping per project and issue type |
| `jflow ui` | TUI; accepts `--view` for a saved view when available |
| `jflow doctor` | Diagnose configuration, authentication, connectivity, terminal, and keyring |
| `jflow completion bash\|zsh\|fish` | Generate completion without auto-installing it |
| `jflow version` | Version, commit, and platform; optional JSON format |

`close` may target the same transition as `done` if the user configures it. In workflows that distinguish Resolved and Closed, it preserves the difference. There is no universal state named Closed.

### 4.2 Shared options

| Option | Rule |
| --- | --- |
| `--profile NAME` | Temporary selection; does not modify active profile |
| `--format table\|plain\|json` | Presentation; `table` only for collections, reject incompatible combinations |
| `--no-color`, `--ascii` | Remove color or graphic characters |
| `--no-input` | Forbid questions, editor, selector, and TUI; fail if data is missing |
| `--yes` | Accept confirmation of an already fully resolved operation |
| `--dry-run` | Prepare operation and show intended effect; allows HTTP reads, no Jira mutations |
| `--refresh` | Skip cache and query the server |
| `--offline` | Query local data only; incompatible with mutations |
| `--timeout 30s` | Total command deadline, including waits and verification |
| `--verbose` | Redacted diagnostic to stderr |

`--yes` does not choose an ambiguous transition or invent fields. `--format json` implies `--no-input`. A JSON mutation requires `--yes` or `--dry-run`. The TUI rejects `--format` and requires a suitable terminal. `--offline --refresh` is invalid.

For listings: `--project`, `--status-category todo|in-progress|done`, `--type`, `--priority`, `--include-done`, `--updated-since`, `--sort`, `--limit`, `--all`, `--page-size`, and `--page-token`. The `--status-category done` option automatically removes the default exclusion of completed items.

`--limit` limits the total returned, default 50. `--page-size` limits each request, default 50. `--all` walks until exhausted, with a configurable safety maximum of 5,000 results; reaching that maximum produces a partial result, never an exact total. To resume a page, use an opaque token tied to the profile, query, and its fields. If the limit falls mid-page, request only the remaining elements when possible; the own cursor must preserve the remainder if the provider returns more.

Do not mix `search --jql` with structured filters: reject the combination to avoid unexpected interpretations. `--sort` must map to an allowed list of fields, never interpolate arbitrary text as syntax.

### 4.3 Productivity functions and examples

```bash
jflow comment add APP-123 --body "Tests completed; pending review."
jflow comment add APP-123 --editor
jflow assign APP-123 --me
jflow edit APP-123 --priority-id 2 --due 2026-09-15
jflow reopen APP-123
jflow view save review --jql 'assignee = currentUser() AND status = "Code Review"'
jflow view run review
jflow favorite add APP-123
jflow board --mine
jflow sprint list --board 42
jflow sprint show 86
jflow worklog add APP-123 --time 45m --started 2026-09-06T09:00:00-06:00
jflow context issue --from-git
jflow summary --view review --format json
```

IDs and dates are examples. `--editor` launches a configured executable and arguments, suspends the TUI, and shows a preview before publishing. The draft is saved with private permissions and kept if sending fails. The first version of comments supports plain text converted to ADF; rich Markdown will be an explicit extension.

`context issue --from-git` only reads the branch with `git symbolic-ref --short HEAD`; detached HEAD, no key, or multiple keys produce an explanatory error. Git detection does not happen implicitly during mutations.

## 5. Terminal visual design

### 5.1 Main screen

Reference wireframe, not a requirement of exact dimensions:

```text
╭ Jira Flow · work · company.atlassian.net ─────────── synced 10:32 ╮
│ My pending  |  Favorites  |  Views                  / Search         │
├──────────────────────────────────────┬───────────────────────────────────┤
│ KEY    STATUS        SUMMARY       │ APP-123 · Improve authentication   │
│ APP-123  In progress  Improve auth  │ Status: In progress              │
│ APP-119  To do      Fix UI   │ Resolution: —                    │
│ APP-110  In review  Add logs  │ Assignee: You · Priority: High    │
│                                      │                                   │
│ 3 loaded · more results       │ Subtasks: 2 of 4 · 50%          │
│                                      │ Description and activity…         │
├──────────────────────────────────────┴───────────────────────────────────┤
│ ↑↓ move · Enter detail · s start · d done · o open · ? help  │
╰──────────────────────────────────────────────────────────────────────────╯
```

Use discreet borders, enough spacing, short headers, and a cyan/blue accent. To do is shown with neutral text, In progress in blue, Done in green; errors in red and warnings in amber. Every color always accompanies a label: meaning does not depend on color.

Do not require Nerd Fonts or emojis. Theme `auto`, `dark`, `light`, and `mono`; `NO_COLOR` disables color independently of theme. If it cannot determine the background, use a configurable conservative palette. `--ascii` replaces borders, bars, and symbols with simple characters.

### 5.2 Navigation

| Key | Action |
| --- | --- |
| Arrows or `j/k` | Move selection |
| Enter | Open detail or accept active selector |
| Tab / Shift+Tab | Change focus |
| `/` | Locally filter loaded items, labeled as local filter |
| `Ctrl+f` | Open explicit remote search |
| `s` / `d` / `x` | Prepare start / complete / close |
| `t` | View transitions |
| `o` | Open browser |
| `y` | Show selectable link; copy if backend is available |
| `r` | Refresh view |
| `n` | Load next page |
| `?` | Contextual help |
| Esc | Go back or cancel dialog |
| `q` / Ctrl+C | Exit or cancel; protect unpublished drafts |

Shortcuts do not run while focus is in a text field. A mutation key opens review and confirmation; it does not send directly. Disable the action while a request is pending to avoid duplicates.

Automatic clipboard copy is optional: macOS `pbcopy`, Linux `wl-copy` or `xclip` if available. The displayed URL is the universal alternative, especially via SSH. Do not emit OSC 52 automatically or present copy as successful if the backend fails.

### 5.3 States and sizes

- 110 columns or wider: two panels, approximately 55/45.
- 80 to 109: list and detail alternate, fewer columns.
- Less than 80: compact view; less than 60 or height less than 15: message to use CLI or enlarge terminal.
- Loading: spinner with text; initial error: explanation and retry key.
- Empty list: distinguish a valid query with no results from lack of permissions or an error.
- Stale cache: visible date; partial update: persistent warning.
- Resize: recalculate layout without losing selection or draft.

HTTP calls must run outside `Update`/`View` as asynchronous commands. Each load carries a generation ID: discard old responses after changing filter or profile. Cancel obsolete requests with `context.Context`.

When opening browser or editor, restore/suspend terminal as appropriate. On exit, recover cursor, echo, and screen, even on interruption. `TERM=dumb`, redirected output, and screen reader must have a functional alternative via `plain`.

## 6. Architecture and repository structure

Allowed dependencies:

```text
Cobra CLI ───────┐
                ├── Use cases ── Domain + ports
Bubble Tea TUI ─┘                      ↑
                         HTTP, secrets,
                         cache, clock, browser, and Git adapters
```

The domain does not import Cobra, Bubble Tea, HTTP JSON, or keyring libraries. The CLI/TUI translates interaction into application requests; it does not build API URLs or decide transitions on its own.

Proposed structure:

```text
jira-flow/
  cmd/jflow/main.go
  internal/
    app/                 # Use cases; composition and cancellation
    domain/              # Issue, status, progress, transition, errors
    ports/               # Minimal interfaces consumed by app
    provider/
      jiracloud/         # Cloud DTOs, endpoints, ADF, pagination
      jiradc/            # Only when implementing Delivery C
    transport/           # HTTP, authentication, limits, redaction
    workflow/            # Intent resolution and forms
    cli/                 # Cobra constructors and flags
    tui/                 # Models, messages, components, and themes
    output/              # Text, table, JSON v1
    config/              # Schema, precedence, migrations, and writes
    secrets/             # Keyring and environment
    cache/               # Memory and optional persistence
    platform/            # Browser, clipboard, editor, paths, and terminal
    actions/             # Shared action registry
  testdata/              # Synthetic fixtures without private information
  tests/integration/     # Simulated HTTP server and binary tests
  docs/
    architecture.md
    commands.md
    authentication.md
    workflows.md
    compatibility.md
    json-contract.md
    adr/
  .github/workflows/
  .goreleaser.yaml
  Makefile
  go.mod
  go.sum
  README.md
```

Initial external dependencies: Cobra, the three Charm libraries, `golang.org/x/term` for hidden input/terminal detection, and `github.com/zalando/go-keyring`. Use `net/http`, `encoding/json`, `log/slog`, `testing`, and `httptest` from the standard library. Avoid full Jira SDK, ORM, and database until a requirement justifies them.

Pin exact compatible versions in `go.mod`/`go.sum` during F0; do not mix Charm v1/v2 components. Lint and release tools are also pinned in CI. The target is `CGO_ENABLED=0`; verify it with the chosen dependencies before promising binaries without extra libraries.

### 6.1 Domain contracts

Minimal types; these are conceptual specifications the agent must turn into complete Go types:

```go
type Issue struct {
    ID         string
    Key        string
    Summary    string
    ProjectID  string
    IssueTypeID string
    Status     Status
    Resolution *NamedID
    Assignee   *User
    UpdatedAt  time.Time
    DueDate    *LocalDate
    URL        string
}

type Status struct {
    ID       string
    Name     string
    Category string // todo | in-progress | done | unknown
}

type IssueReader interface {
    Myself(context.Context) (User, error)
    Search(context.Context, SearchRequest) (IssuePage, error)
    GetIssue(context.Context, IssueRef, DetailOptions) (IssueDetail, error)
}

type TransitionGateway interface {
    ListTransitions(context.Context, IssueRef) ([]Transition, error)
    ApplyTransition(context.Context, TransitionRequest) (ApplyResult, error)
}

type SecretStore interface {
    Get(context.Context, CredentialRef) (Secret, error)
    Set(context.Context, CredentialRef, Secret) error
    Delete(context.Context, CredentialRef) error
}
```

Other contracts: `CommentReader`, `CommentWriter`, `IssueEditor`, `WorklogWriter`, `Browser`, `Clipboard`, `Cache`, `Clock`, and `Sleeper`. Add them when implementing their function, not as a monolithic interface with empty methods.

`IssueDetail` contains normalized description, subtasks, links, and paginated sections. `FieldSpec` contains ID, name, required, type, allowed values, and relevant metadata. `Transition` contains ID, name, destination state, and fields. IDs are always strings unless an endpoint explicitly requires numbers; do not infer semantics from a numeric value.

`PreparedAction` contains profile, issue, intent, resolved transition, fields, observed state, and predicted changes; it contains no secrets. `ApplyResult` distinguishes `verified`, `accepted_unverified`, `unknown`, `failed`, and `noop`. Public serializers are independent of internal types so refactoring does not break scripts.

## 7. Configuration, authentication, and profiles

### 7.1 Paths and precedence

Linux: configuration in `$XDG_CONFIG_HOME/jflow/config.json` or `~/.config/jflow/config.json`; cache in `$XDG_CACHE_HOME/jflow` or `~/.cache/jflow`; state in `$XDG_STATE_HOME/jflow` or `~/.local/state/jflow`.

macOS: configuration/state in `~/Library/Application Support/jflow/` and cache in `~/Library/Caches/jflow/`. Implement paths in a single module with tests; do not assume `os.UserConfigDir` and XDG have identical semantics across platforms.

Precedence: explicit flags > `JFLOW_*` variables > selected profile > global values > defaults. Resolve `JFLOW_CONFIG` path first, then the profile, then its options. Do not auto-load configuration from the current repository.

Initial variables: `JFLOW_CONFIG`, `JFLOW_PROFILE`, `JFLOW_TOKEN`, `JFLOW_EMAIL`, `JFLOW_NO_INPUT`. `JFLOW_TOKEN` only affects the selected invocation/profile and is never persisted automatically. Do not accept `--token VALUE` to avoid exposing it in shell history and process arguments.

### 7.2 Configuration example

The following identifiers are illustrative. The wizard must save real discovered values, never copy these IDs as defaults:

```json
{
  "schema_version": 1,
  "active_profile": "work",
  "ui": {"theme": "auto", "ascii": false, "language": "es"},
  "cache": {"persist": false, "list_ttl_seconds": 60, "detail_ttl_seconds": 30},
  "profiles": {
    "work": {
      "provider": "jira-cloud",
      "site_url": "https://company.atlassian.net",
      "cloud_id": "REAL-SITE-ID",
      "auth": {
        "method": "api-token-scoped",
        "email": "person@example.com",
        "credential_ref": "profile-uuid-generated"
      },
      "default_project": "APP",
      "timezone": "America/Monterrey",
      "workflow_rules": [
        {
          "project_id": "10001",
          "issue_type_id": "10002",
          "intent": "start",
          "from_status_id": "1",
          "transition_id": "21",
          "expected_to_status_id": "3"
        }
      ],
      "views": {
        "my-pending": "assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC, key ASC"
      },
      "field_map": {"story_points": null}
    }
  }
}
```

Configure `api-token-unscoped` for unscoped tokens. `api_base_url` is derived from the method and is not saved as a second editable value that could contradict site/cloud ID. Validate HTTPS, origin, base path, and absence of user/password in the URL. In Data Center, preserve the context path when implemented.

Private files `0600` and directories `0700`, temporary write in the same directory followed by rename and locking to avoid losing changes across processes. Migrations by `schema_version`, backup before migrating, and rejection of unknown future versions. Do not print credentials when showing configuration.

### 7.3 Login

1. Prompt for profile name, site URL, email, and token type.
2. Explain in the wizard how to generate the token via an official link.
3. For scoped token, prompt for cloud ID and offer official discovery instructions; do not infer it from the site name or assume OAuth endpoints accept Basic.
4. Receive token with hidden input or `--token-stdin` for automation.
5. Build Basic auth email:token in memory and check `myself`.
6. Show the discovered identity and save reference and secret only after successful validation.
7. Try a small query and, optionally, transitions of a chosen issue to diagnose read/write permissions without making changes.

Jira Cloud supports Basic with email and API token. Scoped tokens use `https://api.atlassian.com/ex/jira/{cloudId}`; unscoped tokens use the site URL. Browsable links still point to the site. Tokens expire; a 401 should suggest checking expiration or revocation, without asserting this is necessarily the cause. Sources: [Basic auth](https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/), [tokens and scopes](https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/), [current identity](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-myself/).

Document a scopes-by-command table from the current reference for each endpoint during F0. Do not indiscriminately copy OAuth scopes as if they were interchangeable with all token types. A read-only profile can be used; write functions must report insufficient permissions/scopes when appropriate.

### 7.4 Secret storage

Use the `jflow` service and a random reference per profile, tied to site and user. Linux requires a session with Secret Service; macOS uses Keychain. The proposed library depends on D-Bus/Secret Service in Linux and `/usr/bin/security` in macOS. Source: [go-keyring](https://github.com/zalando/go-keyring).

In an SSH session without keyring, allow in-memory credentials only via environment or stdin; do not silently fall back to a plain-text file. Evaluate the macOS backend to avoid exposing the token in arguments during save: if the chosen library does not meet the requirement, implement a native backend or document and resolve that point before publishing persistent authentication. The `SecretStore` interface allows replacing it.

`auth logout` removes the local copy and purges profile private data. If an environment variable still provides a token, indicate that the source remains; do not claim all authentication was closed. Remote token revocation is done from Atlassian.

OAuth 3LO is reserved for a later architectural decision. Do not embed a client secret in the binary, or invent support for device-code/PKCE. Check the flows authorized by Atlassian and whether they require a confidential component before designing that delivery. Reference: [integration authentication](https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/).

## 8. Jira client and queries

### 8.1 Reference operations

Relative to the corresponding authenticated base:

| Need | Cloud operation |
| --- | --- |
| Identity | `GET /rest/api/3/myself` |
| Search | `POST /rest/api/3/search/jql` |
| Issue | `GET /rest/api/3/issue/{key}` |
| Transitions | `GET /rest/api/3/issue/{key}/transitions?expand=transitions.fields` |
| Run transition | `POST /rest/api/3/issue/{key}/transitions` |
| Comments | `GET` / `POST /rest/api/3/issue/{key}/comment` |
| History | `GET /rest/api/3/issue/{key}/changelog` |
| Assign | `PUT /rest/api/3/issue/{key}/assignee` |
| Edit | `PUT /rest/api/3/issue/{key}`; query edit metadata when applicable |
| Fields | `GET /rest/api/3/field` |
| Time | `GET` / `POST /rest/api/3/issue/{key}/worklog` |
| Boards/sprints | Optional adapter for `/rest/agile/1.0/...` |

References: [issues and transitions](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/), [comments](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-comments/), [fields](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-fields/), [worklogs](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-worklogs/), [Jira Software](https://developer.atlassian.com/cloud/jira/software/rest/api-group-board/).

Initial search will use the enhanced `/search/jql` endpoint, not the retired `/search`. It must walk `nextPageToken` and tolerate results that may not yet reflect a recent change; Jira documents `reconcileIssues` to reinforce consistency after known changes. Source: [Jira Cloud search](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/).

### 8.2 Query construction

Default `mine` query:

```jql
assignee = currentUser()
AND statusCategory != Done
ORDER BY updated DESC, key ASC
```

Example query with explicit filter:

```jql
assignee = currentUser()
AND project = "APP"
AND statusCategory = "In Progress"
ORDER BY priority DESC, updated DESC, key ASC
```

Implement a small predicate builder with literal escaping and an allowed list of fields/operators. Explicit `--jql` is sent as user text, without shell execution. Translate internal categories `todo`, `in-progress`, `done` to the corresponding JQL literals; translate provider keys `new`, `indeterminate`, `done` to the domain. Free state names are not used to infer categories.

Request only listing fields: key/ID and `summary,status,assignee,priority,issuetype,project,updated,duedate,resolution`. Description, comments, history, and other fields are loaded on demand. The list must not make an extra request per row.

Pagination of each endpoint lives in its adapter; comments and changelog need not follow the search cursor. Detect repeated cursors, deduplicate by ID, and mark `complete=false` on truncation or partial error. Searches over changing data are not transactional snapshots.

### 8.3 Transport and failures

Proposed policy: 5 second connection, default 30 second total deadline, up to 3 read attempts, and initial maximum concurrency of 4. These are product parameters, not Jira-published limits.

- Reads: retry transient network errors, 429, and 502/503/504 with exponential backoff and jitter.
- Respect `Retry-After`; if it exceeds available time, end with an actionable error and suggested time.
- Treat search POST as a read by semantics, not as a mutation because of the verb.
- Writes: a single send in the first version. Faced with ambiguous response, reconcile; do not automatically repeat comments, worklogs, or transitions.
- 400: invalid fields/JQL. 401: authentication. 403: access denied. 404: resource does not exist or is not visible; do not distinguish without evidence. 409: conflict when the provider returns it.
- Unexpected HTML responses: possible proxy/login; return a diagnostic without dumping the full body.

Atlassian documents 429 and `Retry-After`, and distinguishes limits by integration, including API token traffic. The client responds to real headers and errors; it does not assume a universal quota. Source: [rate limiting](https://developer.atlassian.com/cloud/jira/platform/rate-limiting/).

Use validated TLS, environment proxy, and configurable corporate CA. Do not implement `--insecure` by default. Do not forward Authorization to another origin via redirects; reject authenticated redirects outside the authorized base. Limit received bodies and redact errors/logs before displaying them.

### 8.4 Descriptions and ADF

Cloud v3 uses Atlassian Document Format in various rich fields. Source: [API v3 intro](https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/).

The adapter transforms ADF into internal blocks: paragraph, header, list, code, link, and text. The TUI renders that tree with styles; `plain` keeps text and URLs. An unknown node shows descendant text or an unsupported-content marker; it does not block the whole issue.

To publish plain text, generate a valid ADF document with paragraphs and text. Do not accept Markdown pretending to convert it fully. Do not download images or execute HTML; links only open by explicit action.

## 9. Start, complete, close, and reopen

### 9.1 Implementation principle

A user intent and a Jira transition are distinct objects. `start`, `done`, `close`, and `reopen` are local intents. The server defines which transitions exist, which are available to the user, and which data each requires. Never change state via an arbitrary `PUT` to `fields.status`.

The following algorithm is a proposed Jira Flow policy. It relies on the provider's transition catalog, but does not presume Atlassian imposes these selection rules.

### 9.2 Intent resolution

Deterministic order:

1. Get issue and fresh transitions, with their fields. Cache does not authorize a mutation.
2. If `--transition-id` is present, look up that ID among available ones and validate its destination.
3. Otherwise, look for an exact rule by profile, project, issue type, intent, and source status. An invalid rule produces an error; do not silently choose another transition.
4. For `start`, offer transitions to the In progress category, excluding loops to the same state. For `done`, offer those that reach the Done category.
5. If there is only one candidate, prepare and show its name/destination for confirmation. If there are more, ask for a choice; in `--no-input`, return ambiguity with candidates.
6. For `close`, require a rule or explicit choice among available transitions: the Done category does not distinguish resolution from closing. For `reopen`, also require rule/choice, since the destination may be To do or In progress.
7. If there are no candidates, show the current state and available actions. Do not build multi-hop paths to reach the destination.

Example: from In progress the transitions `Send to QA`, `Resolve`, and `Cancel` exist. If Resolve and Cancel both end in Done, `done` presents both and does not assume Cancel means work completed. The configuration can pin Resolve for that project and type.

For `start` with no explicit destination, an issue already in the In progress category returns `noop` and reports its status. For `done` with no rule/explicit destination, the Done category allows `noop` with visible resolution; do not assert the issue was successfully delivered if its resolution is Cancelled. If a rule exists, compare the exact expected destination before deciding `noop`. `close` and `reopen` need to know their specific destination. An explicit loop transition is still executable after confirmation, because it may have additional effects.

### 9.3 Required fields

Create an editor registry according to `FieldSpec`: text, number, date, user, single selection, and multiple selection. Use real IDs, schemas, and allowed values; do not assume priority, resolution, or story points share IDs across sites.

In interactive mode, show only required fields that are missing and allow editing available optional ones. In non-interactive mode:

```bash
jflow done APP-123 --transition-id 31 --fields-file ./close-fields.json --dry-run
jflow done APP-123 --transition-id 31 --fields-file ./close-fields.json --yes --no-input
```

Illustrative file, with IDs obtained from metadata:

```json
{
  "resolution": {"id": "10000"},
  "customfield_10421": "Tests reviewed"
}
```

`--fields-file` is a fields object, not a complete HTTP body. Validate size, root type, field IDs, and known types; the client wraps those data in the request. An alternative `--field-json 'resolution={"id":"10000"}'` is optional, never necessary to complete the delivery.

An unsupported field allows explicit JSON entry if its schema can be validated; if not possible, explain the field and offer `open`. Plugin or workflow validators may require conditions not described in the metadata: show the server rejection and keep the form data.

Do not assign a default resolution, do not clear resolution when reopening unless the workflow allows it, and do not auto-complete subtasks. An issue Done with empty resolution is shown as such, along with a note about possible workflow configuration; the CLI does not “fix” it.

### 9.4 Preparation, confirmation, and execution

```text
Intent → fresh read → resolved transition → valid fields
      → preview → confirmation → revalidation → send
      → issue read → verified or uncertain result
```

Minimum preview:

```text
Site: company.atlassian.net · Profile: work
APP-123 · Improve authentication
Transition: Resolve (ID 31)
State: In progress → Resolved
Requested resolution: Fixed
Apply this change? [y/N]
```

Before POST, re-query state/transitions if there was interaction. If the relevant state or transition changed, invalidate the preparation and ask for a new review; in automation return conflict. This control reduces races, but does not guarantee compare-and-swap: the server remains the authority.

Send only the transition and the requested fields. Do not add a separate comment as a hidden side effect of `done`. If later supporting comment with transition, define whether it travels in a single Jira-supported operation or as two operations with partial reporting.

### 9.5 Verification and uncertain responses

After an HTTP acceptance, invalidate related detail/listings and query the issue directly. Up to three spaced reads are allowed within the command deadline. Compare destination and relevant fields; automations may change state again and must be reflected.

- `verified`: acceptance received and expected destination observed.
- `accepted_unverified`: Jira accepted the request, but the final state could not be confirmed. Inform that it must not be blindly repeated.
- `unknown`: the send may have reached the server and there was no conclusive response. The later observed state may be shown, but do not attribute the change to this CLI without evidence.
- `failed`: confirmed rejection; include field information when available.
- `noop`: the intent is already satisfied per the described rules, without sending POST.

“Exactly-once” semantics are not guaranteed. Do not use an invented idempotency header: without endpoint support it does not prevent duplicates. On a comment/worklog timeout, show uncertain state and allow inspecting activity; do not resend automatically.

Save only minimal metadata of recent actions, per profile: local UUID, key, action, time, and result. Never token, description, or comment body. Allow disabling this registry; default TTL seven days. The local UUID aids diagnosis, but is not a server transaction identifier.

## 10. Precise progress definition

The word “progress” can mean several things. The CLI must present metrics with numerator and denominator; do not automatically convert In progress to 50% or assign arbitrary percentages to states.

| Indicator | Calculation or source | Presentation |
| --- | --- | --- |
| State | State and category returned by Jira | `In review · In progress` |
| Resolution | Current field value | `Fixed`, `Cancelled`, or `No resolution` |
| Subtasks | Done category completed / queried visible subtasks | `2 of 4 subtasks · 50%` |
| Logged time | Seconds reported by Jira | `3 h logged` |
| Remaining time | Reported remaining estimate | `2 h estimated remaining` |
| Estimate consumption | Logged time / original estimate, if positive | `150% of estimate consumed`, not capped at 100 |
| Updated | `updated` date | `Updated 2 days ago` |
| Time in state | Fully queried state change history | `In this state since ...`; unknown if it cannot be determined |
| Due date | Local due date and configured timezone | `Due today` or `Overdue 2 days ago` |

Additional rules:

1. Zero subtasks means `No subtasks`, not 0% or 100%.
2. Only show subtask percentage when the denominator of the visible set is known. If pages are missing, show partial counts and omit global percentage.
3. Clarify that issues visible to the account are considered; do not promise to detect items hidden by security.
4. Completed subtasks include any resolution within Done. Show resolution breakdown if needed to distinguish delivered from canceled.
5. A parent Done with open subtasks shows both; it does not alter the workflow.
6. Do not simultaneously sum aggregated parent time values and individual child times.
7. Story points are not time and are not a progress percentage. If added, discover the field via validated configuration, do not hardcode `customfield_10016`.
8. Due dates without a time are compared as local date; do not arbitrarily convert them to midnight UTC.

`summary` returns counts by category and, optionally, completed fraction of a defined population. Since `mine` excludes completed by default, a completeness percentage requires `--include-done` and appropriate scope; the interface will not show a misleading bar over just pending items. A valid example is a specific sprint or defined project/type.

Each summary includes the query or scope description, update date, number processed, `complete` indicator, and measurement method. Do not use an approximate count as an exact denominator. A truncated query may produce `7 completed of 20 loaded; total unknown`, with no global percentage.

## 11. URL, browser, and system integration

Build URL as `site_url` plus `/browse/{key}`, preserving context path when it exists. Do not use `api.atlassian.com/ex/jira/...` for human links. Validate the key and escape path segments using `net/url`.

`jflow link APP-123` works without network if the site is configured. In normal mode write only the URL to ease pipes. `--format json` returns an object with `key` and `url`. It does not guarantee the user has access without querying Jira.

`jflow open` calls `/usr/bin/open` on macOS and `xdg-open` on Linux with separate arguments. Do not use `sh -c`, shell concatenation, or execute issue text. If there is no graphical environment or launcher, explain the reason and show the URL; return capability unavailable code.

A successful launch means the launcher process started, not that the browser loaded or authenticated the page. Optional browser/editor configuration via arrays of executable and arguments, never a shell string to evaluate.

## 12. Cache, privacy, and refresh

In the first version the cache is per-process memory. Allow persistence with `cache.persist=true` for offline use; explain that it stores Jira content on the machine. Offline mode fails with a clear message if it was never enabled or the entry does not exist.

Proposed TTLs: listings 60 seconds, details 30 seconds, identity 15 minutes, and field metadata 1 hour. Transitions are fetched fresh when preparing/applying actions. The TUI allows manual refresh and optional polling every 60 seconds, without overlapping requests or a hidden daemon.

Cache key: provider, authenticated origin/base, profile, identity, normalized query, fields, and schema version. Do not share by issue key alone. Invalidate on credential, user, or site change. A credential renewal increments a local cache generation.

Optional persistence stores JSON files with total limit 25 MB, maximum 1,000 details, and maximum seven-day expiration. Use names derived from a hash, not issue summaries. Exclude comment/description bodies from default persistence; an additional option may enable them. Cache configuration must not contain tokens.

Implement `jflow cache status` and `jflow cache clear --profile work`; document exactly which files the application controls. Local results always include capture time and `stale`. An online 401/403 is not silently turned into success using stale cache.

Sanitize remote text before rendering: remove ANSI escapes, terminal controls, and OSC from summaries, comments, and names. JSON may keep text as escaped data without literal control sequences. Images and attachments are not downloaded automatically.

Logs without authentication headers, cookies, tokens, full bodies, or private JQL by default. No telemetry in the core. Tests use synthetic data and no secret is stored in fixtures, snapshots, or the repository.

## 13. Script output and errors

Stdout is reserved for the result; stderr for diagnostics and progress. With redirected output, no ANSI, spinners, or pager. `--format json` emits exactly one JSON document per invocation, even on error. For JSON, structured errors go in that stdout document and the exit code remains non-zero; stderr may contain additional redacted diagnostics.

Proposed search contract:

```json
{
  "schema_version": 1,
  "ok": true,
  "data": {
    "issues": [
      {
        "id": "100123",
        "key": "APP-123",
        "summary": "Improve authentication",
        "status": {"id": "3", "name": "In progress", "category": "in-progress"},
        "resolution": null,
        "url": "https://company.atlassian.net/browse/APP-123"
      }
    ]
  },
  "meta": {
    "profile": "work",
    "fetched_at": "2026-09-06T16:32:00Z",
    "source": "network",
    "stale": false,
    "complete": false,
    "returned": 1,
    "next_page_token": "example-opaque-cursor"
  },
  "warnings": [],
  "error": null
}
```

`ok=true, complete=false` is normal when the user requested a page or a limit. If `--all` was requested and a failure or safety limit prevents completion, use partial error and code 10 while preserving available data.

Proposed error contract:

```json
{
  "schema_version": 1,
  "ok": false,
  "data": null,
  "meta": {"profile": "work"},
  "warnings": [],
  "error": {
    "code": "transition_ambiguous",
    "message": "More than one transition is possible to complete APP-123.",
    "retryable": false,
    "details": {"candidate_transition_ids": ["31", "41"]}
  }
}
```

For mutations, `data.result` uses the states from 9.5 and presents previous state, observed state, and transition. `accepted_unverified`/`unknown` have `ok=false` and code 9, even if there was initial acceptance; the message distinguishes that case from a rejection. Keep `null` for unknown values, RFC3339 times, and numeric fields with explicit units.

| Exit | Meaning |
| --- | --- |
| 0 | Success, valid empty list, or reported noop |
| 1 | Unclassified internal error |
| 2 | Invalid arguments/configuration |
| 3 | Authentication missing or rejected |
| 4 | Insufficient permissions |
| 5 | Resource not found or not visible |
| 6 | Ambiguity, missing fields, or validation rejected |
| 7 | Conflict detected on revalidation |
| 8 | Network/rate limit before mutation, or read failed |
| 9 | Write accepted unverified or result uncertain |
| 10 | Result requested as complete but incomplete, or future partial batch |
| 11 | Capability unavailable: browser, offline without data, incompatible provider |
| 130 | User cancellation |

Cancellation after sending does not necessarily cancel the remote change: print that the result may be uncertain before exiting. In future batch operations, stop new sends and keep already received results.

Automation examples:

```bash
jflow mine --format json | jq -r '.data.issues[].key'
jflow search --jql 'project = APP AND statusCategory != Done' --all --format json
jflow transition APP-123 --id 21 --yes --no-input --format json
jflow link APP-123 | pbcopy  # macOS only; not required to use the CLI.
```

Version the output schema separately from the binary version. In v1, allow new fields and preserve existing names/types; incompatible changes require a new schema version and migration guide.

## 14. Design to extend features

### 14.1 Internal action registry

Each action declares a stable ID, visible name, intent, whether it mutates Jira, required capabilities, and a preparer/executor. Example IDs: `issue.start`, `issue.done`, `issue.comment.add`.

The registry lets the TUI build a contextual palette and the CLI expose commands while keeping use cases shared. Forms are input metadata; Bubble Tea components are not embedded in the domain. Validate duplicate IDs at startup.

To add a feature:

1. Define the use case and its input/output contract.
2. Add a small port if a provider capability is missing.
3. Implement adapter and HTTP fixtures.
4. Register the action and expose the command.
5. Incorporate into the TUI if useful interactively.
6. Update help, JSON contract, permissions, and tests.

Do not add reflection or a plugin framework for the MVP. Also do not use Go's `plugin` package as a distributed ABI; a compiled registry is enough for the first deliveries.

### 14.2 Provider capabilities

`Capabilities` expresses adapter support, e.g. comments, editing, sprints, or worklogs. Distinguish `supported`, `unsupported`, and `unknown` when not tested. Capabilities do not guarantee permissions on a specific issue; a 403 response affects the tested context and must not globally disable the whole profile.

Jira Data Center will require its own adapter: PAT authentication by version, corresponding routes and pagination, identities different from `accountId`, rich text, and own metadata. Do not implement an automatic fallback from Cloud v3 to Data Center v2 after a 404. PAT and API availability must be checked in the deployed version. Source: [Data Center PAT](https://developer.atlassian.com/server/jira/platform/personal-access-token/).

### 14.3 Future external extensions

If real use justifies extensions, use explicitly registered processes with versioned JSON over stdin/stdout. Invocation will be `jflow extension run NAME ...`; do not execute unknown binaries because of a typo in a command. Define maximum time, response size, and exit codes.

The extension does not receive tokens in the environment by default. An extension that needs data can consume explicit JSON output from the CLI or request capabilities through a later protocol. Do not promise sandboxing of local processes: an installed extension runs code with the user's permissions.

## 15. Verification strategy

### 15.1 Meaningful unit tests

- JQL constructor: quotes, special characters, categories, and stable ordering.
- Workflow resolver: valid/invalid rule, two Done destinations, close distinct from done, noop, and explicit loop transition.
- Progress: zero denominator, partial data, canceled, missing estimate, and consumption above 100%.
- Cloud mapping: unknown categories, missing user, dates, and missing custom fields.
- ADF: lists/code, unknown nodes, outbound plain text, and ANSI/OSC sanitization.
- Configuration: precedence, migrations, future version, and isolation between profiles.
- JSON serialization: public contracts and null fields, not full snapshots of internal structures.

### 15.2 Integration tests with `httptest`

Mandatory cases:

| Case | Expected evidence |
| --- | --- |
| Scoped token | Correct gateway base; browsable URL keeps site |
| Unscoped token | Site base; valid Basic header and never logged |
| Cursor pagination | Recovers pages, keeps filters, and detects repeated cursor |
| 429 with Retry-After | Waits via fake clock or ends by deadline; no long real wait |
| Transition with required field | No POST if value missing; valid payload when completed |
| Two Done transitions | Nothing sent in non-interactive mode without choice |
| Change between preview and send | Preparation invalidated and old one not applied |
| Timeout after send | Does not duplicate POST; result unknown and later query |
| 204 followed by read failure | Result accepted_unverified; not confirmed success |
| Automation moves issue again | Shows observed state and destination difference |
| 401, 403, and 404 | Distinct codes, no misleading cache fallback |
| Redirect to another origin | Credential does not reach the destination |
| Partial read | Keeps obtained items and marks incompleteness |

### 15.3 CLI, TUI, and operating systems

Run the binary from tests with controlled inputs and outputs. Confirm parseable JSON, separate stderr, behavior without TTY, help/completion, and exit codes. Use mocks for Browser/SecretStore in common tests.

TUI tests: navigation, focus, form, cancellation, resize, late HTTP response, and theme without color. Screen snapshots must use fixed clock and width. Add a pseudo-terminal test to verify cursor/echo restoration after exit; do not rely only on snapshots.

Minimum release manual tests: Linux with desktop and SSH session; macOS with Terminal/iTerm2; narrow terminal and `TERM=dumb`; keyring available/locked/absent; browser opening; Unicode and long text. Building arm64 does not substitute for running the arm64 binary: document which combinations were actually executed.

### 15.4 Validation against real Jira

Before tagging v1, use a test project with explicit permission to create/change test issues. Prepare at least: a normal issue, one with required fields, one with two transitions to Done, one closed, and a parent with subtasks.

Check login, assigned to me, reading, linking, starting, finishing, mapped closing, and state confirmation in Jira web. Do not use production issues as tests. Real credentials and bodies must not be stored as fixtures. Clean only the test resources that have been authorized for cleanup.

If there is no test site, a candidate release with simulated integration may be delivered; it must be declared that it has not been validated with a real tenant and that criterion must not be marked as met.

## 16. Work plan by phases

Orientative estimates in working days for one person familiar with Go; not a calendar commitment. They include implementation, tests, and documentation per phase. Work can be organized as successive PRs; delegation between agents is not required to follow this plan.

| Phase | Estimated duration | Dependencies | Result |
| --- | --- | --- | --- |
| F0: technical base | 1–2 days | None | Repo, versions, contracts, and decisions pinned |
| F1: configuration and access | 2–3 days | F0 | Profiles, secrets, login, me, and doctor |
| F2: CLI reading | 3–4 days | F1 | mine, search, show, link, and open |
| F3: workflows | 4–6 days | F2 | Verifiable transitions and start/done/close |
| F4: progress and scripting | 2–3 days | F2; F3 for write results | Metrics, JSON, and errors stabilized |
| F5: TUI | 4–6 days | F2 and F3; integrate F4 | Navigable interface and complete actions |
| F6: distribution and acceptance | 3–4 days | F1–F5 | Binaries, documentation, and real validation |
| F7: productivity | 5–8 days | Delivery A | Delivery B features |

Delivery A: approximately 19–28 working days, 4–6 weeks, subject to tenant access and workflow complexity. Data Center, OAuth, and extensions are estimated after specific investigation and are not included.

### F0. Technical base and contracts

Tasks:

- Create repository and Go module under the real owner; use provisional local name until known.
- Pin toolchain, Cobra/Charm/keyring versions, and CI tools; compile for four platforms.
- Test feasibility of storing secrets on macOS without exposing token in arguments; record decision.
- Define domain types, errors, initial ports, and JSON v1 format.
- Create ADRs for Go/CLI+TUI, Cloud-first, transitions, secrets, and cache.
- Build synthetic fixtures and endpoint/scope catalog by command.

Exit criterion: `jflow version` and `--help` work; project compiles; minimal CI passes; no core function depends on a TUI API.

### F1. Configuration, secrets, and authentication

Tasks:

- Implement paths, JSON schema, migration, and precedence.
- Implement credential backends and session mode without persistence.
- Create HTTP client with cancellation, derived URLs, and redaction.
- Implement login/logout/status, profile list/use, me, and doctor.
- Cover both API token types, wrong credential, and locked keyring.

Exit criterion: a profile can be configured and the account identified; the token does not appear in configuration files, output, or logs; changing profile isolates identity and cache.

### F2. Reading and links

Tasks:

- JQL constructor, enhanced endpoint, pagination, and field selection.
- Implement mine/list/search/show; normalize ADF and paginated sections.
- Implement link/open and their Linux/macOS adapters.
- In-memory cache, `--refresh`, common options, and text/table output.
- HTTP and binary tests without TTY.

Exit criterion: the user queries their pending items, filters, reviews an issue, and opens its URL; there are no extra requests per row; truncated results are identified.

### F3. Workflow engine and mutations

Tasks:

- Get transitions and field metadata; basic forms.
- Implement prepare/confirm/revalidate/apply/verify.
- Create mapped rules, commands transitions/transition/start/done/close, and `workflow map`.
- Support `--transition-id`, `--fields-file`, `--yes`, `--no-input`, and `--dry-run`.
- Cover ambiguity, validation, races, and uncertain response without duplicating send.

Exit criterion: start/complete/close works with arbitrary names and required fields; no global IDs hardcoded nor automatic multi-step transitions.

### F4. Progress and automation contracts

Tasks:

- Implement progress/summary with defined indicators and explicit scope.
- Complete JSON v1 and exit codes for reads and mutations.
- Add opt-in persistent cache, cache commands, and offline mode.
- Document pipelines and unknown/partial metrics.

Exit criterion: a script can process queries and distinguish success, partiality, and uncertainty; no bar invents progress from the state name.

### F5. Interactive interface

Tasks:

- Main model, list/detail, search, pagination, forms, and dialogs.
- Connect only to existing use cases; do not duplicate the workflow resolver.
- Add styles, sizes, help, focus, cancellation, and stale-response controls.
- Implement refresh and post-action feedback.
- Validate pseudo-terminal and manual experience on both systems.

Exit criterion: the query → start → review → complete → open browser journey is done entirely by keyboard, with terminal restored on exit.

### F6. Distribution and delivery

Tasks:

- Configure GoReleaser, four-artifact CI, checksums, and changelog.
- Document install/uninstall, keyring, SSH, proxies, profiles, and workflows.
- Generate help and completion from Cobra.
- Run acceptance in test Jira and record tested versions/systems.
- Review dependencies, licenses, and secret leaks; resolve core defects.

Exit criterion: satisfy the acceptance list in section 18 and deliver verifiable artifacts, not just source code.

### F7. Productivity

Implement in order: comments → assign to me → edit allowed fields → reopen → views/favorites → personal board → sprints → worklogs → Git context. Each feature adds its error/permission tests and output contract. Do not leave Delivery B commands visible as if they worked before implementing them.

To edit, get capabilities/metadata and validate fields; for worklogs, make explicit the policy for updating remaining estimate and show it before sending. `start` changes state, it does not start a clock. A future stopwatch will have independent local state and will publish time only under an explicit action.

## 17. Distribution, CI, and operational targets

Artifacts: `jflow_<version>_linux_amd64.tar.gz`, `linux_arm64`, `darwin_amd64`, and `darwin_arm64`, each with binary, license, and install README. Generate checksums and, when signing infrastructure exists, verifiable signature/provenance. GoReleaser is the proposed tool to automate builds and packaging. Source: [GoReleaser](https://goreleaser.com/getting-started/).

Homebrew will have a tap under the real repository owner; do not document a `brew install` as already available. Before publishing, define license and maintainers. A public macOS release must evaluate signing/notarization and document the real Gatekeeper status; do not recommend disabling its controls as normal installation.

The initial validation matrix will focus on Ubuntu LTS and the two most recent stable macOS versions available at publish time. Register specific versions and architectures in `compatibility.md`; check the chosen toolchain minimums and do not promise support for all historical Linux/macOS.

Proposed CI:

```text
Format + vet + lint
        ↓
Unit/integration tests + race where supported
        ↓
Build Linux/macOS × amd64/arm64
        ↓
Smoke tests on available runners + JSON contract review
        ↓
Release tag → packaging → checksums → authorized publish
```

Makefile development commands to implement:

```bash
make fmt-check
make lint
make test
make test-race
make build
make release-check
```

Verification base: `go test ./...`, `go vet ./...`, `go test -race ./...` where supported, and scan with pinned `govulncheck`. Pin CI actions by reviewed commit and give minimum permissions to publish jobs. Common tests do not require Jira credentials.

Product performance targets, to be measured and recorded on known hardware/fixture:

- `version`/`help`: less than 150 ms on reference development machine.
- First TUI visual response: less than 200 ms, showing loading if network is missing.
- Navigation over 1,000 loaded items: input latency below 50 ms.
- Initial read of 50 issues: target less than 2 s with controlled-latency test server; real Jira may exceed this.
- Memory target: less than 100 MB with 1,000 listing items and one detail open.

These figures are targets to verify, not guarantees or already-taken measurements. Prioritize correctness and cancellation over speculative optimization.

## 18. First-version acceptance criteria

Delivery A is only considered finished when the following are met:

- [ ] Linux and macOS binaries for amd64/arm64 exist, with a documented matrix of real execution.
- [ ] The user can authenticate a profile and query their own identity.
- [ ] Scoped and unscoped tokens use their correct bases.
- [ ] `mine` returns current-user assignments with correct filters and pagination.
- [ ] `show` presents state, resolution, description, and additional sections without breaking on missing fields.
- [ ] `progress` distinguishes real, partial, and unknown data.
- [ ] `link` generates a correct URL without network and `open` works or gives a useful per-platform diagnostic.
- [ ] `start`, `done`, and `close` respect workflow transitions and rules.
- [ ] An ambiguity does not produce any change without resolving it.
- [ ] Common required fields and explicit automation data are supported.
- [ ] `--dry-run` never emits a Jira mutation.
- [ ] A timeout after sending does not cause a blind retry.
- [ ] The TUI allows the main journey by keyboard and restores the terminal.
- [ ] JSON output is stable, valid, and script-friendly.
- [ ] Secrets do not appear in configuration, own history, CLI arguments, or logs.
- [ ] A session without keyring works via ephemeral credential.
- [ ] Cache and configuration are isolated by profile/identity.
- [ ] Defined tests pass and there is an install and usage guide.
- [ ] A validation against test Jira is recorded, or it is explicitly delivered as a candidate without that criterion met.

## 19. Concrete risks and bounded pending decisions

| Risk or unknown | Treatment |
| --- | --- |
| Workflow with external validators | Keep form, show server error, and offer to open browser |
| “Close” distinct from “complete” | Resolve via transition/rule; never assume equivalence |
| Late indexing after writing | Verify detail, invalidate cache, and reconcile searches when appropriate |
| Expired token or limited permissions | Precise diagnosis without requesting general administrative permissions |
| Terminal/SSH without clipboard or browser | Printable link and plain CLI as independent functions |
| macOS keyring exposes argument on save | Investigate backend in F0 and resolve before public persistence |
| Custom fields differ by site | Explicit discovery and mapping, site-local IDs in profile |
| Jira type still unknown | Cloud as initial delivery; if real tenant is Data Center, prioritize its adapter before validating there |
| Name/license/owner to define | Use provisional local name; resolve before publishing |

No unknown prevents building the core with fixtures. The agent must not invent credentials, cloud IDs, or permissions to close those decisions. If the target Jira turns out to be Data Center, preserve the use cases and replace the planned provider; communicate schedule impact.

## 20. Delivery instructions for the implementing agent

Start with F0 and advance by dependencies. The purpose of this document is that common decisions are already fixed; it does not require re-asking which language, interface, or command model to use. Resolve minor details with the rules here and record architecture changes via ADR.

In each phase deliver runnable code, tests proportional to risk, and usage documentation. Keep a list of pending items and actually available functions. An attractive interface with simulated mutations does not satisfy Delivery A; nor does a functional CLI that omits Linux/macOS or the requested TUI.

Recommended order of the first vertical increment: `auth login` → `me` → `mine` → `show` → `link/open`. The second increment: `transitions` → preparation → `start` → `done/close` → verification. Only then complete the TUI on top of those same services.

At the end, deliver repository, release artifacts, quick-start guide, example configuration without secrets, test report, compatibility matrix, and observed limitations. Every claim of compatibility with a tenant or system must be backed by recorded tests.

## 21. References and plan maintenance

Technical references are linked next to the decision or behavior they support. Checked on September 6, 2026. Revalidate authentication, endpoints, scopes, and versions before starting F0 or after any relevant Atlassian change.

The architecture, TTL values, commands, exit codes, screens, estimates, and acceptance criteria are proposals of this document. Official sources support the capabilities of the APIs/libraries, but they do not guarantee that the plan has been implemented or that a specific tenant allows all of its operations.
