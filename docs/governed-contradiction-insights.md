# Governed Contradiction Insights

Contradiction detection is an offline and shadow capability backed by the
existing reasoning candidate envelope. It groups only exact normalized keys
within one `tenant/project/namespace` scope, compares half open validity
intervals, and records `contradiction`, `temporal_coexistence`, or
`unresolved_temporal`. Missing or invalid temporal evidence never becomes an
active insight.

Candidates retain both source versions, evidence references and digest, source
watermark, provider/schema/policy versions, overlap bounds, uncertainty, review
state, and deterministic replay identity. PostgreSQL remains the system of
record. Candidate persistence is append only and idempotent on scoped replay
identity; source correction marks the derived record stale rather than
rewriting either canonical fact.

The reserved activation policy is disabled by default. An operator must create
an independently versioned exact scope policy, satisfy evidence, overlap,
freshness and uncertainty gates, and confirm the candidate review before an
apply request can activate it. Apply requests use the existing activation
decision and lifecycle ledger. Repeating the same replay identity returns a
duplicate decision without creating another active version. Rollback disables
new admissions while preserving candidates, reviews, and audit history.

Ordinary retrieval and context assembly exclude contradiction candidates,
shadow results, unresolved or coexisting intervals, stale records, and review
required records. An authorized contradiction context request may include only
fresh, active, confirmed overlap insights from the exact scope and source
watermark, bounded by an explicit item budget.

Telemetry and logs expose only fixed categories for temporal disposition,
review, freshness, fallback, and activation. Source claims, content, scopes,
identifiers, provider payloads, prompts, and raw errors are not emitted.

## Verification

Use the repository cache when running tests:

```powershell
$env:GOCACHE='D:\code\stele\.gocache-codex-contradiction'
go test ./...
go test -race ./...
go vet ./...
openspec validate --all --strict
git diff --check
```

The PostgreSQL integration test is opt in through
`STELE_TEST_POSTGRES_REASONING_DSN`. It verifies the pgvector extension and
candidate persistence. Run it only against an explicitly owned disposable
database; a schema at an older migration version must be upgraded before the
test is considered evidence.
