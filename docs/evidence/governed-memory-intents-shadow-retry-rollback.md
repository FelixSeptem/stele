# Governed Memory Intents Evidence

This record captures bounded verification for `governed-memory-intents`. It is
evidence only and does not enable a production policy or claim contradiction
activation is enabled.

## Owned PostgreSQL + pgvector

- Container: `stele-rq4-evaluation-pgvector`
- Endpoint: local disposable mapping on `localhost:55432`
- Database: `stele_evaluation`
- Migration: `TestMigrationRunnerSerializesConcurrentApply`
- Durable queue: `TestDerivedWorkPostgresRecoveryAndScopeIsolation`
- Intent: `TestGovernedMemoryIntentPostgresReplayQueueHistoryAndScopeIsolation`

Intent verification covered same-scope replay, conflicting idempotency,
durable queue deduplication, append-only transition history, foreign-scope
non-disclosure, and migration-backed transition indexes.

## Bounded policy checks

- Disabled or rolled-back policy decisions reject before intent persistence.
- Resume requires enabled policy, a compatible non-empty policy version, exact
  normalized scope, and `pending` or `accepted` status.
- `failed`, `suppressed`, `rejected`, and `replayed` records do not resume
  automatically.
- Contradiction and feedback outcomes cannot directly become `active`.
- Raw payloads, prompts, claims, credentials, scope values, identifiers, and
  provider/database errors are excluded from lifecycle telemetry and
  transition diagnostics.

## Verification commands

```text
go test ./... -count=1 -timeout 15m
go test -race ./internal/memory ./internal/mcp ./internal/app ./internal/storage/postgres ./openapi -count=1
go vet ./...
openspec validate --all --strict
git diff --check
```

