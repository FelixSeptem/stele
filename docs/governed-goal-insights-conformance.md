# Governed goal insight conformance run

This run checks the bounded PostgreSQL and pgvector path for `goal` insight
candidates. It is an offline/shadow and review-only check. It does not claim
readiness, enable goal activation, or change ordinary retrieval visibility.

## Operator run

Provide a disposable PostgreSQL instance with the `vector` extension and an
explicit DSN. The existing pgvector images used by this repository are suitable
when the database is isolated for the run. For example, with a local mapping:

```powershell
$ownedGoalDsn = Read-Host 'Enter the disposable PostgreSQL + pgvector DSN'
pwsh -File scripts/stele-goal-conformance.ps1 -TestDSN $ownedGoalDsn
```

The DSN is consumed only by the test process. Do not put credentials or a DSN
in a committed report, issue, log, or documentation artifact.

If `STELE_TEST_POSTGRES_GOAL_DSN` is absent, the script exits with the stable
skip code `2` and states that no readiness claim was made. A failed prerequisite
or missing pgvector extension is a non-pass result.

## Evidence categories

The conformance matrix verifies:

- the goal metadata JSONB round trip preserves `proposed` and
  `review_required`;
- replay retries are idempotent and preserve one candidate row;
- exact tenant/project/namespace scope is used for candidate and derived reads;
- default derived insight listing omits goals, while explicit experimental
  inspection can request them;
- shadow/review-only processing does not mutate canonical memory;
- the PostgreSQL `vector` extension is present before the run proceeds.

The test retains only bounded state and disposition categories in its output.
It does not print goal text, prompts, evidence payloads, identifiers, foreign
scope values, credentials, or raw database errors.

## Cleanup and failure interpretation

The test uses a unique tenant and removes its candidate, derived insight, and
evidence rows on completion. Run it against a disposable database so an
interrupted process can be cleaned up by deleting the unique conformance tenant.

- `PASS` means the matrix completed for the supplied isolated database.
- `SKIP` means the prerequisite DSN was not supplied; this is not readiness.
- a nonzero failure means the review-only safety, scope, persistence, or
  pgvector prerequisite did not pass and must be investigated before any
  policy change.
