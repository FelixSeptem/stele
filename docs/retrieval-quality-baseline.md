# Retrieval Quality Baseline

The first repository-owned retrieval replay was executed on PostgreSQL 18 with
pgvector using the `retrieval-fixture-v1` fixture.

| Field | Value |
| --- | --- |
| Fixture version | `retrieval-fixture-v1` |
| Representation version | `canonical-v1` |
| Ranking version | `baseline-v1` |
| Compatible embedding revision | `deterministic-v1` (semantic channel inactive without vectors) |
| Policy version | `quality-policy-v1` |
| Replay cases | 13 |
| Safety failures | 0 |
| Database | PostgreSQL 18 / pgvector |

The replay completed all cases and verified append-only raw-event, canonical-version,
provenance, scope, and lifecycle behavior. The initial fixture queries intentionally
include semantic paraphrases; the current lexical-only replay therefore records a
zero candidate pool for this baseline. This is an explicit measurement result for
the pre-chunking, pre-fusion implementation and is the reference point for later
query understanding, chunk representation, and hybrid ranking changes.

Run the same owned-stack check with:

```powershell
$env:STELE_TEST_RETRIEVAL_EVALUATION_DSN = '<owned-pg18-test-dsn>'
pwsh -File scripts/retrieval-evaluation.ps1
```

The command requires an explicitly owned disposable database and never falls back to
`STELE_POSTGRES_DSN` or another ambient operator database.

## Chunk representation shadowing

The hierarchical chunk representation is a derived PostgreSQL projection over raw
events and canonical-memory versions. It is identified by source kind/id/version,
chunk policy, renderer, and deterministic whitespace token-counter versions. Raw
events and canonical memory remain immutable; chunk rows are rebuildable and
append-only.

Chunk materialization and retrieval consumption are controlled independently:

- `default_off` keeps the canonical-memory retrieval path unchanged;
- `shadow` evaluates chunk candidates and emits only bounded diagnostics on an
  authorized evaluation path;
- `active` permits chunk-derived evidence to contribute while retaining the
  canonical parent identity and citations.

Every chunk read or parent/adjacent expansion re-checks exact
`tenant/project/namespace` scope and active lifecycle state. Hidden, stale, or
foreign sources fail closed. Parent and adjacent evidence is bounded by count and
character/token budgets and never broadens a session or user boundary.

Use an explicitly owned PostgreSQL + pgvector DSN for chunk integration checks:

```powershell
$env:STELE_TEST_POSTGRES_CHUNK_DSN = '<owned-test-dsn>'
go test ./internal/storage/postgres -run MemoryChunkPostgres -count=1
```

Disable chunk materialization or consumption to roll back; no destructive migration
or canonical-memory rewrite is required. Derived chunk retention and deletion must
follow the source lifecycle and the operator's PostgreSQL backup/restore policy.
