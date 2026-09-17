# ADR-005: private opt-in cache

Status: implemented in F4. Updated: 2026-09-17.

Decision: initial cache in memory; optional persistence in F4. Isolate by provider, site/base, profile, identity, query, and schema. Mutations require fresh reads; an authentication failure is not hidden with old data.

Consequence: F0 creates no configuration, token, or cache files when running the binary. TTLs and sizes will follow the plan; later results indicate date, source, and partiality.


F4 stores JSON in a private `v1` directory under the platform cache path. A configuration-path/profile namespace plus a credential- and generation-bound read scope prevents reuse across profiles, identities, credentials, or renewed logins. `JFLOW_CACHE` optionally supplies an absolute cache root. Files use hash names, 0600 permissions, confined filesystem operations, a process-shared lock, and atomic replacement. Global storage is capped at 25 MiB and 1,000 detail entries; individual entries at 4 MiB; retention at seven days. Accesses enforce expiration and writes/status prune stored files; no background daemon deletes files.

TTL is 60 seconds for lists and 30 seconds for details. Online disk hits verify identity first; read failures do not fall back to stale data. Offline reads require the same locally available credential but make no network request. Renewing login advances the generation even if the token is unchanged. Transitions advance the generation and clear the profile's disk cache before and after sending; dry runs and no-ops do not invalidate. A pre-write cache error prevents sending; a post-write error is a warning and never causes a retry.

Descriptions, comments, their history change bodies, and raw verification values are omitted from persistent detail. The omitted requested sections are visibly incomplete. Online `show` fetches fresh bodies rather than serving a redacted disk entry. Listings and metric-only details may use fresh disk entries. Disabling persistence leaves files intact but invalidates their generation; `cache clear` removes the selected profile/configuration's entries, including previous generations.
