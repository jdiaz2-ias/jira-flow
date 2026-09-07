# Workflow contract

In F0 the types `Intent`, `Transition`, `FieldSpec`, `PreparedAction`, and `ApplyResult` exist. The algorithm will be implemented in F3, with the scenarios from the plan.

`start`, `done`, and `close` are not universal state names. The resolver will fetch fresh transitions; it will choose a validated rule or ask for a decision when there is ambiguity. Done can be reached by both Resolve and Cancel. Global IDs are never hardcoded.

The flow will be prepare → validate fields → confirm → revalidate → apply once → verify. A POST that may have reached the server will not be blindly retried. Reserved results are `verified`, `accepted_unverified`, `unknown`, `failed`, and `noop`.

Fixtures `transitions.json` and `transitions-ambiguous.json` include destinations and fields to implement those tests. `transition-required-error.json` represents a confirmed rejection. All are synthetic.
