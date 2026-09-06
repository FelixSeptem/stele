# Versioned Migrations And Runtime Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Stele reject migration history that is dirty, incompatible, or
checksum-divergent before it applies traffic or background work.

**Architecture:** Retain `golang-migrate` and its `schema_migrations` dirty
state, then add a Stele-owned `stele_schema_migration_ledger` that binds each
numeric embedded migration to its SHA-256 checksum. A deterministic manifest is
the common source for status, legacy reconciliation, CLI output, runtime
admission, and real PostgreSQL verification. PostgreSQL advisory locking spans
ledger backfill, migration apply, and final recording.

**Tech Stack:** Go, `embed.FS`, SHA-256, `database/sql` with pgx, PostgreSQL
advisory locks, `golang-migrate`, pgxmock, and existing real PostgreSQL/pgvector
integration fixtures.

---

### Task 1: Migration Manifest And Pure State Classification

**Files:**
- Modify: `internal/storage/postgres/migrations.go`
- Create: `internal/storage/postgres/migration_manifest_test.go`
- Modify: `internal/storage/postgres/migration_runner.go`
- Modify: `internal/storage/postgres/migration_runner_test.go`

- [ ] **Step 1: Write failing manifest tests**

Add a test that calls `MigrationManifest()` twice and asserts one `0001` entry
with the `.up.sql` asset name and a stable 64-character SHA-256 checksum. Add
table-driven unit tests for `parseMigrationAsset` accepting
`0001_base_schema.up.sql` and rejecting an invalid numeric prefix, a down file,
and a non-SQL file.

```go
manifest, err := MigrationManifest()
if err != nil { t.Fatal(err) }
if len(manifest) != 1 || manifest[0].Version != 1 {
    t.Fatalf("manifest = %+v", manifest)
}
if len(manifest[0].ChecksumSHA256) != 64 {
    t.Fatalf("checksum = %q", manifest[0].ChecksumSHA256)
}
```

- [ ] **Step 2: Verify the manifest tests fail**

Run: `go test ./internal/storage/postgres -run 'TestMigrationManifest|TestParseMigrationAsset' -count=1`

Expected: FAIL because `MigrationManifest` and `parseMigrationAsset` do not
exist.

- [ ] **Step 3: Implement the minimal manifest**

Add `MigrationAsset{Version, Name, ChecksumSHA256}`. Enumerate only embedded
`.up.sql` assets, parse a strictly positive zero-padded numeric filename prefix,
read bytes from `migrationFS`, hash with `sha256.Sum256`, sort by version, and
reject duplicate/non-contiguous versions. Define `CurrentMigrationVersion` from
the final manifest rather than an unrelated literal.

- [ ] **Step 4: Verify the manifest tests pass**

Run: `go test ./internal/storage/postgres -run 'TestMigrationManifest|TestParseMigrationAsset' -count=1`

Expected: PASS.

- [ ] **Step 5: Write failing state tests**

Extend the pgxmock-facing migration tests so a matching integrity record yields
`current`; a name/checksum mismatch, an extra row, or a missing record yields
`MigrationStatusDivergent`. Keep existing dirty and future version cases.

- [ ] **Step 6: Verify the state tests fail**

Run: `go test ./internal/storage/postgres -run 'TestInspectMigrationState' -count=1`

Expected: FAIL because the runner never queries or compares
`stele_schema_migration_ledger`.

- [ ] **Step 7: Implement bounded integrity inspection**

Add `MigrationStatusDivergent`, `IntegrityStatus`, and non-secret state fields.
After the driver ledger query identifies a clean supported version, query the
integrity rows ordered by version, compare them exactly with `MigrationManifest`
through the applied version, and classify the result. Do not include SQL text,
DSNs, checksums, or database errors in the state reason exposed to callers.

- [ ] **Step 8: Verify focused storage tests pass**

Run: `go test ./internal/storage/postgres -run 'TestMigrationManifest|TestParseMigrationAsset|TestInspectMigrationState' -count=1`

Expected: PASS.

### Task 2: Reconciliation And Serialized Application

**Files:**
- Modify: `internal/storage/postgres/migration_runner.go`
- Create: `internal/storage/postgres/migration_integrity_test.go`
- Modify: `internal/storage/postgres/migration_concurrency_test.go`

- [ ] **Step 1: Write failing reconciliation tests**

Use pgxmock or a small injected migration database interface to expect creation
of `stele_schema_migration_ledger`, advisory-lock acquisition, and insertion of
exact manifest records for a clean legacy prefix. Add cases asserting no insert
is attempted when the driver row is dirty or ahead of the manifest.

```go
err := reconcileMigrationLedger(ctx, db, manifest, MigrationState{
    CurrentVersion: 1, Status: MigrationStatusCurrent,
})
if err != nil { t.Fatal(err) }
```

- [ ] **Step 2: Verify reconciliation tests fail**

Run: `go test ./internal/storage/postgres -run 'TestReconcileMigrationLedger' -count=1`

Expected: FAIL because the reconciliation function and table do not exist.

- [ ] **Step 3: Implement ledger creation and supported-prefix backfill**

Create the table with `version bigint primary key`, `migration_name text not
null`, `checksum_sha256 char(64) not null`, and `applied_at timestamptz not
null`. Reconcile only a clean manifest prefix. Use `INSERT ... ON CONFLICT
(version) DO NOTHING`, re-read and compare after write, and return an explicit
bounded error if a conflicting record already exists.

- [ ] **Step 4: Verify reconciliation tests pass**

Run: `go test ./internal/storage/postgres -run 'TestReconcileMigrationLedger' -count=1`

Expected: PASS.

- [ ] **Step 5: Write failing locked-application tests**

Introduce a runner seam for lock/reconcile/apply behavior. Assert `Apply`
acquires the fixed PostgreSQL advisory lock, rechecks state under the lock,
backfills a supported legacy prefix, calls `migrator.Up`, and writes every
post-apply manifest record exactly once.

- [ ] **Step 6: Verify locked-application tests fail**

Run: `go test ./internal/storage/postgres -run 'TestMigrationRunnerApply' -count=1`

Expected: FAIL because the current runner delegates directly to `migrator.Up`.

- [ ] **Step 7: Implement locked forward application**

Acquire `pg_advisory_lock` on a repository-defined fixed signed key after the
database ping and release it with `pg_advisory_unlock` through `defer`. Inspect
and reconcile prior to `Up`, call the existing driver, then inspect/reconcile
again and require `current` before returning success. Preserve the original
migration error and never reconcile dirty/incompatible/divergent state.

- [ ] **Step 8: Verify storage and concurrent integration tests pass**

Run: `go test ./internal/storage/postgres -count=1`

Expected: PASS; the real concurrency test remains an explicit skip when
`STELE_TEST_POSTGRES_DSN` is absent.

### Task 3: CLI And Runtime Admission

**Files:**
- Modify: `cmd/stele/main.go`
- Modify: `cmd/stele/main_test.go`
- Modify: `internal/app/app_test.go`
- Modify: `internal/app/app.go` only if the shared `MigrateDatabase` contract
  needs a bounded integrity policy adaptation

- [ ] **Step 1: Write failing CLI tests**

Make `runMigrate` accept an injected migration runner or status function. Assert
JSON status includes `integrity_status` and human output includes that bounded
field. Assert a divergent `up` returns an error without applying SQL.

- [ ] **Step 2: Verify CLI tests fail**

Run: `go test ./cmd/stele -run 'TestRunMigrate' -count=1`

Expected: FAIL because output has no integrity facts and runner injection is
not available.

- [ ] **Step 3: Implement shared CLI output**

Add stable JSON tags to the expanded state and print only status/version,
pending/dirty, and integrity classification. Route `up` through the identical
integrity-gated `Apply` function; retain `STELE_MIGRATION_OUTPUT=json` behavior.

- [ ] **Step 4: Verify CLI tests pass**

Run: `go test ./cmd/stele -run 'TestRunMigrate' -count=1`

Expected: PASS.

- [ ] **Step 5: Write failing runtime policy tests**

In existing API/worker/scheduler dependency-injection tests, make the migration
seam return a divergent error for each policy and assert no server/listener or
worker/scheduler loop is constructed. Add one auto-policy test proving a
verified pending transition invokes migration before runtime construction.

- [ ] **Step 6: Verify runtime policy tests fail**

Run: `go test ./internal/app -run 'TestBuild.*Migration' -count=1`

Expected: FAIL until error categorization/ordering is explicitly asserted.

- [ ] **Step 7: Implement minimal runtime wiring**

Keep API, worker, and scheduler on `postgres.MigrateDatabase`; ensure its
`auto`, `validate`, and `off` branches classify integrity status consistently
and never treat `divergent` as current. Avoid alternate migration logic in
runtime builders.

- [ ] **Step 8: Verify runtime tests pass**

Run: `go test ./internal/app ./cmd/stele -count=1`

Expected: PASS.

### Task 4: Real-Stack Evidence And Documentation

**Files:**
- Modify: `internal/storage/postgres/migration_upgrade_test.go`
- Modify: `internal/storage/postgres/migration_concurrency_test.go`
- Modify: `docs/self-hosting.md`
- Modify: `docs/self_hosting_test.go`

- [ ] **Step 1: Write failing real-stack and docs contract tests**

Extend the opt-in upgrade test to assert every applied integrity ledger row
matches `MigrationManifest`. Add an opt-in checksum corruption case expecting
`divergent`; do not mutate databases without the existing test DSN guards. Add
documentation assertions for `integrity_status`, divergence, and forward
remediation/verified restore wording.

- [ ] **Step 2: Verify the new tests fail**

Run: `go test ./internal/storage/postgres ./docs -run 'TestMigration|TestSelfHosting' -count=1`

Expected: FAIL because the integrity evidence and operator text are absent.

- [ ] **Step 3: Implement test-supported docs and integration checks**

Document `status` before `up`, expected state meanings, JSON integrity output,
and the no-historical-edit/no-automatic-down-migration recovery path. Extend
the real-stack fixture assertions for principal/grant, idempotency, canonical
memory, provenance, history, and scope-safe behavior after upgrade.

- [ ] **Step 4: Verify focused suites pass**

Run: `go test ./internal/storage/postgres ./docs ./cmd/stele ./internal/app -count=1`

Expected: PASS; real PostgreSQL cases run when their explicit DSNs are set,
otherwise report their established skip condition.

### Task 5: Completion Validation

**Files:**
- Modify: `openspec/changes/versioned-migrations-and-runtime-hardening/tasks.md`

- [ ] **Step 1: Mark each OpenSpec task immediately after its evidence is green**

Use the exact task checkbox only after its focused tests pass. Do not mark
real-stack tasks complete if their required test ran neither successfully nor
with its documented prerequisite skip behavior.

- [ ] **Step 2: Run full verification**

Run: `go test ./... -count=1 -timeout 15m`

Expected: PASS.

Run: `openspec validate versioned-migrations-and-runtime-hardening --strict`

Expected: PASS with no validation errors.

- [ ] **Step 3: Commit the verified implementation**

```powershell
git add openspec/changes/versioned-migrations-and-runtime-hardening docs/superpowers/plans internal/storage/postgres internal/app cmd/stele docs
git commit -m "feat: verify immutable migration history"
```
