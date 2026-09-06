## 1. Migration Manifest And State Model

- [x] 1.1 Add failing unit tests that define deterministic migration manifest parsing, SHA-256 checksums, and rejection of malformed, duplicate, or unordered embedded migration assets.
- [x] 1.2 Implement the immutable embedded migration manifest and extend the migration state model with integrity facts and a `divergent` state.
- [x] 1.3 Add failing unit tests for current, pending, dirty, incompatible, uninitialized, and divergent driver/integrity ledger combinations.
- [x] 1.4 Implement PostgreSQL integrity-ledger inspection and bounded state classification without exposing SQL, DSNs, or credentials.

## 2. Serialized Reconciliation And Forward Application

- [x] 2.1 Add failing tests for clean legacy-prefix backfill, unsupported legacy rejection, and no integrity write for dirty or future schema state.
- [x] 2.2 Implement idempotent integrity-ledger creation and supported legacy-prefix reconciliation under PostgreSQL-owned serialization.
- [x] 2.3 Add failing tests for concurrent apply/reconciliation and post-apply integrity recording.
- [x] 2.4 Implement locked forward application that validates before and after `golang-migrate` execution and records the verified applied prefix.

## 3. Runtime And CLI Admission

- [x] 3.1 Add failing CLI tests for human and JSON status integrity fields and divergent migration command failure.
- [x] 3.2 Update `stele migrate status|up` to expose the shared integrity state and use the same reconciliation/application contract.
- [x] 3.3 Add failing API, worker, and scheduler startup tests for verified-pending auto admission and divergent fail-closed behavior.
- [x] 3.4 Update runtime migration policy handling so all three modes share integrity-gated admission before traffic or job claims.

## 4. Operations And Real-Stack Evidence

- [x] 4.1 Add real PostgreSQL + pgvector integration coverage for legacy populated upgrade, checksum divergence, dirty/future rejection, and concurrent migration application.
- [x] 4.2 Verify upgraded fixture preservation for principals/grants, idempotency, canonical memory, provenance, history, and scope isolation.
- [x] 4.3 Update self-hosting upgrade/restore guidance and document machine-readable migration integrity output, forward remediation, and no-down-migration policy.
- [x] 4.4 Update documentation contract tests for the migration integrity operator workflow.

## 5. Completion Gate

- [x] 5.1 Run focused storage, CLI, runtime, and documentation tests, including the real-stack suite when PostgreSQL + pgvector prerequisites are available.
- [x] 5.2 Run `go test ./... -count=1 -timeout 15m` and `openspec validate versioned-migrations-and-runtime-hardening --strict`; resolve all failures before completion.
