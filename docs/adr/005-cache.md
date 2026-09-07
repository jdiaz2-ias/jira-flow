# ADR-005: private opt-in cache

Status: accepted for upcoming phases. Date: 2026-09-06.

Decision: initial cache in memory; optional persistence in F4. Isolate by provider, site/base, profile, identity, query, and schema. Mutations require fresh reads; an authentication failure is not hidden with old data.

Consequence: F0 creates no configuration, token, or cache files when running the binary. TTLs and sizes will follow the plan; later results indicate date, source, and partiality.
