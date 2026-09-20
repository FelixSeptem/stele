# Bi-temporal fact validity: migration, recovery, and rollback

This note records the ownership, recovery, and rollback procedure for migration
`0014_bi_temporal_fact_validity`. It is written to be actionable by an operator
who has access to a scoped PostgreSQL DSN and nothing else: no query text, no
memory identifiers, no raw scores, and no provider payloads appear in any
operator-facing evidence produced by this change.

## What the migration changes

`0014_bi_temporal_fact_validity.up.sql` is additive only. It:

- adds `temporal_fact_id`, `temporal_head_version`, `ingested_at`, `valid_from`,
  `valid_to`, and `validity_source` to `canonical_memories` and
  `memory_versions`;
- backfills pre-existing rows with the legacy current-compatible shape
  (`temporal_fact_id = id::text`, `ingested_at = valid_from = created_at`,
  `validity_source = 'legacy_current_compatible'`, `valid_to` left NULL so the
  row stays current-valid);
- creates `temporal_corrections` as the append-only correction ledger;
- adds temporal columns to `relation_projections`, `memory_chunk_derivations`,
  and `context_projection_items`;
- creates scope-safe indexes on temporal identity and validity bounds.

Every `ALTER` uses `ADD COLUMN IF NOT EXISTS` and every `CREATE` uses
`IF NOT EXISTS`, so re-running the migration is a no-op. The backfill
`UPDATE` statements are guarded by `WHERE` clauses that skip already-populated
rows, so they are idempotent as well.

## Ownership

- The migration is applied by the same ordered runner as every other versioned
  migration; there is no separate temporal bootstrap step.
- The backfill runs inside the migration transaction. It is not a background
  job and there is no partially-backfilled steady state to reason about.
- Correction history is owned by `temporal_corrections`. Rows are append-only;
  the current head of a fact is tracked by
  `canonical_memories.temporal_head_version`, not by deriving
  `MAX(successor_version)`, because a derived maximum cannot detect a competing
  writer.

## Recovery

If the migration fails mid-transaction, the runner reports a dirty version.
Recovery is:

1. Inspect the runner's reported dirty version. Do not edit
   `temporal_corrections` or the backfilled columns by hand.
2. Re-run the migration. Because every statement is `IF NOT EXISTS` guarded and
   the backfill is idempotent, a re-run completes the migration from wherever it
   stopped.
3. Confirm ordinary current retrieval returns the same rows it returned before
   the migration attempt. A legacy current-compatible row remains current-valid,
   so a failed or repeated migration must not make existing memories vanish.

There is no destructive repair step. If a manual repair is ever required, it is
a new forward migration, never an in-place rewrite.

## Rollback

There are two distinct rollback paths, and they are not interchangeable.

### Operational rollback (the supported path)

Operational rollback **disables temporal-aware policy resolution** and returns
the approved current baseline. It:

- stops applying valid-time selectors and falls back to the baseline current
  retrieval path;
- **never deletes temporal history**;
- **never rewrites canonical data**;
- leaves every correction, validity interval, and provenance link readable.

This is the path an operator uses in production. Disabling the policy is
data-preserving by construction: the temporal columns and the correction ledger
are simply not consulted.

### Schema revert (`0014_bi_temporal_fact_validity.down.sql`)

The `down` asset exists for local development and test databases so the schema
can be returned to a pre-`0014` shape. It drops the temporal columns and the
`temporal_corrections` table, and therefore **does** discard temporal history.

It is **not** the production rollback path. Running it against a database whose
temporal history must be retained is a destructive action. Prefer the
operational rollback above.

## Rollback rehearsal

The rehearsal is exercised by
`TestTemporalRollbackRehearsalLeavesHistoryIntactAndEmitsBoundedEvidence` in
`internal/storage/postgres/temporal_rollback_rehearsal_test.go`. It proves,
against a mocked database:

1. disabling the temporal policy returns the baseline current selection — the
   generated SQL carries no valid-time predicate;
2. temporal history remains readable after disablement — the correction ledger
   and version history still return their rows;
3. the emitted audit evidence is bounded — it contains only low-cardinality
   categories, no query text, no memory identifiers, no raw scores, and no DSN.

The rehearsal is repeatable and does not mutate data, so it can be run before
and after a rollout without side effects.
