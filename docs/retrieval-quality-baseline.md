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
