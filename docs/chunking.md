# Hierarchical Memory Chunking

Stele's hierarchical chunk representation is a derived PostgreSQL projection over
immutable raw events and canonical-memory versions. A chunk records its exact
source kind/id/version, parent memory when applicable, source session/user,
character and token bounds, policy/renderer/counter versions, and lifecycle
snapshot. It is never a replacement for canonical memory and can be rebuilt from
durable source records.

## Policy and rollout

The deterministic chunker prefers message, paragraph, sentence, list, and fenced
code boundaries before applying per-memory-class hard limits. Profile and relation
records favor atomic facts; episodic records favor event units; procedural records
preserve bounded step groups; summaries use larger bounded coverage units.

Chunk materialization and consumption are independent controls:

- `default_off`: canonical retrieval remains unchanged;
- `shadow`: authorized evaluation can compare chunk candidates, but public results
  are unchanged;
- `active`: chunk evidence may affect ranking while the canonical parent identity
  and citations remain in the response.

## Safety and rebuild

Every operation requires an exact `tenant/project/namespace` scope. Parent and
adjacent expansion also validates session/user assertions, active lifecycle state,
and character/token budgets. Hidden, stale, foreign, or unverifiable sources fail closed.
Repeated materialization of the same source/version and policy identity is
idempotent; a changed source or policy creates a new derived history entry.

The rollback path is to disable materialization or chunk consumption. No destructive
down migration and no canonical-memory rewrite is required.

## Owned-stack verification

Use a disposable PostgreSQL + pgvector database explicitly owned by the test
harness:

```powershell
$env:STELE_TEST_POSTGRES_CHUNK_DSN = '<owned-test-dsn>'
go test ./internal/storage/postgres -run MemoryChunkPostgres -count=1
```

Do not use `STELE_POSTGRES_DSN` or an operator database for this integration check.

Safety behavior: unverifiable sources fail closed.
