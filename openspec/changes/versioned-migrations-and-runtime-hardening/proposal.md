## Why

Stele's current migration runtime serializes forward migrations and rejects
dirty or future versions, but its PostgreSQL ledger records only version and
dirty state. An operator cannot detect that an already-applied embedded
migration has been edited, so a deployment could accept traffic against a
schema whose history no longer matches the release being run. P0 requires a
checksummed, immutable migration history before later retrieval work relies on
the deployment as a reproducible baseline.

## What Changes

- Add a Stele-owned, append-only PostgreSQL migration integrity ledger that
  records every applied embedded migration's version, name, checksum, and
  application time alongside the existing `golang-migrate` state table.
- Compute the ordered embedded migration manifest deterministically and make
  migration status distinguish `current`, `pending`, `dirty`, `divergent`,
  `incompatible`, and `uninitialized` schema states.
- Make `stele migrate status`, forward `stele migrate up`, and API, worker,
  and scheduler startup validate the same integrity state; pending, dirty,
  divergent, and incompatible states fail closed unless `auto` can safely
  apply the pending forward migrations.
- Add controlled legacy-ledger backfill for the currently supported versioned
  schema, PostgreSQL-owned locking for status/apply/backfill serialization,
  and stable operator errors that direct recovery toward forward remediation
  or verified restore rather than automatic down migrations.
- Extend migration status output and self-hosting runbooks with migration
  integrity facts, recovery instructions, and upgrade/restore verification.
- Add focused unit and real PostgreSQL + pgvector integration coverage for
  checksums, divergence, legacy upgrade, dirty/future rejection, concurrent
  application, and preservation of durable scoped records.

## Non-goals

- Do not replace `golang-migrate`, add automatic down migrations, or permit
  schema rewrites during normal service startup.
- Do not change canonical-memory lifecycle, retrieval, chunking, fusion,
  reranking, public memory APIs, MCP, SDK, UI, or model-provider behavior.
- Do not add another system of record; PostgreSQL remains authoritative.
- Do not reimplement existing HTTP resource limits, signal shutdown,
  backup/restore scripts, or product verification outside the migration
  integrity coverage needed by this change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `database-schema-migration-management`: Require immutable migration
  checksums, divergence detection, legacy-ledger compatibility, and shared
  fail-closed startup/CLI validation.
- `service-runtime-foundation`: Require every runtime mode to admit protected
  traffic or job work only after migration integrity validation succeeds.
- `self-hosting-bootstrap`: Publish migration-integrity inspection and
  forward-recovery guidance in the supported upgrade and restore workflow.

## Impact

- Storage/runtime: `internal/storage/postgres/migration_runner.go`, embedded
  migration discovery, migration tests, and the API/worker/scheduler admission
  path in `internal/app`.
- CLI: `cmd/stele` machine-readable and human-readable migration status.
- Operations: `docs/self-hosting.md` and its contract tests.
- Verification: focused PostgreSQL tests plus the existing full Go and OpenSpec
  validation suites.
- Artifact references: `openspec instructions apply --change
  versioned-migrations-and-runtime-hardening --json`; `go test
  ./internal/storage/postgres ./internal/app ./cmd/stele -count=1`; and
  `openspec validate versioned-migrations-and-runtime-hardening --strict`.
