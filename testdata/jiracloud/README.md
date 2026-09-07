# Synthetic fixtures

Made-up responses for future tests; never exported from a real Jira. Keys APP, accountId, IDs and URL `example.atlassian.net` are not credentials or data from an account.

| File | Scenario |
| --- | --- |
| myself.json | Personal identity |
| search-page-1.json / search-page-2.json | Opaque cursor and last page |
| issue.json | Status and ADF description |
| transitions.json | Start and resolve with required field |
| transitions-ambiguous.json | Two Done destinations, including Cancel |
| transition-required-error.json | Rejected because resolution is missing |
| authentication-error.json | Failed authentication |
| rate-limit.json | Illustrative 429 body |

The F1/F2 httptest server will add codes/headers: `Retry-After: 2` for rate-limit and HTTP 204 with no body for accepted transition. These fixtures do not replace validating the real provider contract.
