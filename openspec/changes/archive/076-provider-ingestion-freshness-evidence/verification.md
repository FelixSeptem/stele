# Provider ingestion freshness evidence verification

Status: implemented, verified, and archived as change 076 on 2026-10-31. This
record contains only command outcomes and
the bounded report artifact; no scopes, raw event IDs, memory IDs, paths,
queries, credentials, request bodies, or ranking details are retained.

## Delivered contract

- `GET /v1/admin/assurance/ingestion-freshness?fixture_reference=<event-uuid>`
  is an exact-scope, admin-authorized inspection surface. Its fixed report has
  four ordered stages: `admitted`, `governance_completed`, `canonical_active`,
  and `public_retrievable`.
- PostgreSQL migration `0041_ingestion_freshness_evidence` persists only scoped
  hashes and the redacted report. Replay uses the scoped fixture hash, returning
  the original logical evidence or a bounded conflict.
- `public_retrievable` is recorded only by the Provider retrieve observer after
  result projection and response-size validation. Canonical state, promotion,
  or a write acknowledgement cannot synthesize this final stage.
- The collector checks exact scope, canonical lifecycle, the latest version
  lifecycle, source metadata snapshots, and prior public-read observations.
  Foreign/missing fixtures share unavailable stages; protected source or
  lifecycle states do not disclose identities.
- Metrics use only the fixed `assurance`, `ingestion_freshness`, `evaluate` and
  result categories. The public Provider result body is unchanged.

## Automated verification

The following commands passed from the isolated implementation worktree:

```text
go test ./internal/assurance ./internal/storage/postgres ./internal/app ./openapi -run 'IngestionFreshness|TestProviderRetrieveAcceptsOpenAPITopKField|TestNewHTTPHandlerServesAssuranceAdminAPIs|SpecYAMLIncludesAssuranceAdminRoutesAndSchemas' -count=1
go test ./internal/storage/postgres -run '^TestIngestionFreshnessPostgres' -count=1
go test ./... -count=1 -timeout 15m
go test -race ./internal/assurance ./internal/telemetry -count=1
go vet ./...
git diff --check
pwsh -NoProfile -File scripts/check-documentation.ps1
openspec validate --all --strict
```

Fresh PostgreSQL 18.6 + pgvector verification used a uniquely named disposable
database on the owned local Docker fixture. It applied migrations, exercised a
complete initial lifecycle, persisted and replayed its evidence through a new
repository/service instance, asserted one durable row, and then removed the
database. The same fresh database run covered pending governance and a
restricted source negative fixture; neither could pass public retrieval.

The retained bounded complete report is
[`evidence/ingestion-freshness-initial-restart.json`](evidence/ingestion-freshness-initial-restart.json).

The official `scripts/stele-provider-context-verify.ps1` also passed on an
isolated fresh compose stack built from this worktree. Docker Hub was unavailable
for the base-image lookup, so the build used the equivalent verified
`docker.1ms.run/library/golang:1.25-bookworm` and
`docker.1ms.run/library/debian:bookworm-slim` mirrors. The verifier completed
public context, durable-reference, and projection checks for both initial and
API/worker restart phases. It removed the owned compose resources on exit. Its
bounded reports are retained as `evidence/provider-context.json`,
`evidence/provider-reference.json`, and `evidence/provider-projection.json`,
with phase copies beside them.
