# ADR-003: separate intents and transitions

Status: accepted. Date: 2026-09-06.

Decision: represent start/done/close/reopen as intents. Prepare with fresh metadata, resolve by rule or choice, and revalidate before applying. An uncertain result is neither rejection nor confirmed success.

Consequence: explicit forms and errors are needed; close may differ from complete. The domain already expresses those states, but F3 will implement the resolver. Do not arbitrarily update `fields.status` or simulate idempotence.
