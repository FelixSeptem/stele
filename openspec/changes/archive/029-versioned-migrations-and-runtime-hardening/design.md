## Context

The current migration runner embeds ordered SQL and delegates application and
`schema_migrations(version, dirty)` bookkeeping to `golang-migrate`. Runtime
admission already calls that runner for API, worker, and scheduler modes, and
the standalone CLI exposes `stele migrate status|up`. This is sufficient for
pending, dirty, and future-version detection, but it cannot prove that the SQL
which established an applied version is still the SQL shipped by the current
binary. The P0 runtime gate requires that divergence fail closed before traffic
or job claims begin.

The design must preserve PostgreSQL as the only system of record, maintain
forward-only upgrade behavior, retain compatibility with the supported release
ledger, and reuse the current PostgreSQL coordination and `golang-migrate`
application path.

## Goals / Non-Goals

**Goals:**

- Produce a deterministic manifest from the embedded numbered migrations.
- Persist a Stele-owned append-only integrity record for each clean applied
  migration and verify it before migration application or runtime admission.
- Surface bounded, actionable current/pending/dirty/divergent/incompatible and
  uninitialized migration state consistently to CLI and all runtime modes.
- Safely backfill supported legacy databases, serialize legacy backfill and
  apply using PostgreSQL, and test the contract against a real database.
- Document upgrade, drift recovery, and restore behavior without automatic
  destructive repair.

**Non-Goals:**

- Changing the migration framework, adding down migrations, or accepting an
  edited historical migration.
- Modifying request limits, shutdown behavior, memory semantics, or retrieval.
- Supporting unknown historical schemas beyond an explicit supported migration
  state or creating external backup orchestration.

## Decisions

### Decision 1: Keep `golang-migrate`; add an independent integrity ledger

The runner will retain `golang-migrate` for SQL execution and its dirty-state
mechanism. A `stele_schema_migration_ledger` table will add `version`,
`migration_name`, `checksum_sha256`, and `applied_at`, keyed by version. The
runner obtains its expected records from the embedded files, ordered by parsed
numeric migration version. The ledger is an integrity assertion owned by
Stele, rather than a modification of the third-party `schema_migrations`
table.

Alternatives considered:

- Add checksum columns to `schema_migrations`: rejected because that table is
  owned by the migration driver and its schema/version assumptions are not our
  public contract.
- Replace the migration framework: rejected because the existing application,
  dirty-state, and real-stack coverage are mature and a framework replacement
  creates avoidable bootstrap compatibility risk.

### Decision 2: Verify a complete ordered prefix, then apply only forward

`Status` reads the driver state and then validates the integrity ledger against
the expected manifest for every applied version. A missing integrity row is
`uninitialized` only when no driver ledger exists; it is legacy-compatible when
the driver ledger is clean and exactly equals a supported manifest prefix. Any
checksum/name mismatch, extra row, missing non-legacy row, or non-contiguous
history is `divergent`. A version above the binary's latest remains
`incompatible`; driver dirty state remains `dirty`; a clean compatible prefix
below latest is `pending`.

`Apply` takes a PostgreSQL advisory lock, rechecks state, creates/backfills the
integrity ledger only for a clean supported prefix, executes `Up`, then records
the complete clean resulting prefix in the same serialized critical section.
It never writes checksums for a dirty or incompatible database. This makes
legacy upgrade deterministic while preventing an edited release asset from
being silently blessed.

Alternatives considered:

- Treat all missing ledger rows as divergent: rejected because it would make
  every already-supported deployment unrecoverably unavailable on upgrade.
- Hash the aggregate schema: rejected because it cannot identify which
  historical migration drifted and is unstable across equivalent database
  representations.

### Decision 3: Use PostgreSQL advisory locking around ledger reconciliation

The existing driver serializes application. The Stele runner will additionally
acquire a transaction/session-scoped PostgreSQL advisory lock that covers
integrity-table creation, legacy backfill, pre-apply validation, driver apply,
and post-apply recording. Every standalone or runtime call uses the same
runner, so concurrent API, worker, scheduler, and operator invocations see one
serialized ledger transition.

The runner may retry its status read after another holder releases the lock;
it must never infer integrity from partially written ledger records.

### Decision 4: State is a bounded public operational contract

`MigrationState` adds integrity facts without returning DSNs, SQL text, or raw
driver errors: ledger status, applied records count, and an optional bounded
reason category. Human output remains concise; JSON output adds stable fields.
`auto` accepts only clean current/pending states that verify, and `validate` or
externally managed `off` require clean current state. Dirty, divergent,
incompatible, and uninitialized-without-auto state fail before service work.

### Decision 5: Upgrade data, not schema history, is the real-stack invariant

Integration tests start with the prior supported populated state, apply the
new binary's migrations, and verify principals/grants, idempotency records,
canonical-memory versions, provenance, and history remain accessible only in
their authorized scope. They also exercise checksum divergence and concurrent
application against harness-owned PostgreSQL + pgvector where configured.

## Risks / Trade-offs

- [Legacy databases have a clean driver version but unknown altered SQL] ->
  Backfill recognizes only an exact supported version prefix and records the
  current embedded checksum once; operators must use a verified backup/restore
  or a forward remediation migration for schemas outside that support window.
- [The ledger is created outside migration SQL] -> It is an operational
  metadata table, created idempotently under the same PostgreSQL lock, so it
  cannot race application and does not change application data semantics.
- [A process fails between SQL application and integrity recording] -> The
  next process observes either dirty driver state or a clean supported prefix
  with missing integrity rows and backfills only after validating that prefix;
  no traffic is admitted until reconciliation succeeds.
- [Checksum algorithms or migration file parsing change] -> SHA-256 and strict
  numeric filename parsing are fixed in tests; malformed, duplicate, or
  unordered embedded assets fail startup/build-time manifest construction.

## Migration Plan

1. Release the binary with the integrity ledger logic and legacy-prefix
   backfill support.
2. Operators run `stele migrate status`; a clean supported legacy database
   reports pending/current with a ledger reconciliation requirement rather than
   being treated as drift.
3. `stele migrate up` obtains the lock, backfills the clean prefix, applies
   forward SQL, and records the checksummed final prefix.
4. Start API, worker, and scheduler under `auto` or validate explicitly with
   `status`; all modes reject dirty, divergent, and incompatible states.
5. For failed/edited history, stop the rollout, preserve a backup, inspect
   bounded status, then use a forward remediation release or restore a verified
   backup. No automatic downgrade is attempted.

Rollback is binary-only when the previous binary remains compatible with the
applied schema. Otherwise the documented path is forward remediation or a
verified restore to an explicit target.

## Open Questions

None. The supported legacy input is the clean ordered prefix represented by
the current embedded migration manifest; unknown historical schema states fail
closed rather than receiving heuristic repair.
