# ADR-002: Jira Cloud as first provider

Status: accepted. Date: 2026-09-06.

Decision: REST v3 with `IssueReader` and `TransitionGateway` ports; routes and DTOs will live in `provider/jiracloud`. Enhanced POST `/search/jql` with cursor. The REST base is derived from the token type and the human link always uses the site.

Consequence: Data Center requires another adapter and tests; there will be no fallback for a 404 nor automatic substitution of v3 by v2. No HTTP call is part of F0.
