# ADR-001: Go and shared CLI/TUI inputs

Status: accepted for F0. Date: 2026-09-06.

Decision: use Go 1.27.1, Cobra, and Charm v2. Domain/use cases do not import presentation frameworks. TUI and CLI share future services.

Consequence: deliver as a binary, cross-compilation, and tests without a terminal. F0 only logs functional commands. The foundation tag checks future TUI dependencies without starting it or including them in the normal executable.

The module is `jira-flow.local/jflow`, provisional until the publication repository/owner is known. No invented remote is assigned.
