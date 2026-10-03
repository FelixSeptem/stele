# Governed goal review and experimental visibility conformance

This change adds an exact-scope, redacted goal review projection and an
independently opt-in `goal_context` section. The section is absent from ordinary
context requests and is emitted only by the PostgreSQL reader when a current,
enabled visibility policy exists for the exact scope. Provider output and replay
do not bypass that boundary.

## Verification matrix

- `go test ./internal/insights ./internal/retrieval ./internal/storage/postgres ./internal/app ./openapi -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- `openspec validate governed-goal-review-and-experimental-visibility --type change`
- `openspec validate --all --strict`

The redacted admin route is `GET /v1/admin/goals/review`. It returns bounded
review and policy categories for the authenticated exact scope and never selects
goal title, summary, prompt, provider payload, or foreign identifiers. The public
context request accepts `include_goal_context`; the default is false and the
ordinary sections remain unchanged when the flag is absent or the policy fails.

## PostgreSQL / pgvector run

Use the existing disposable pgvector image and set
`STELE_TEST_POSTGRES_GOAL_DSN` before running the repository conformance script.
The migration is additive (`0032_goal_visibility`) and can be rolled back with
its paired down migration. Do not commit credentials or DSNs.
