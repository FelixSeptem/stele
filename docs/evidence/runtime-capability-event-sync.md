# Runtime capability and event synchronization evidence

## Real PostgreSQL + pgvector smoke

- Date: 2026-10-04
- Stack: PostgreSQL 18.6 with the repository's existing `docker.1ms.run/pgvector/pgvector:pg18` image
- Database: isolated disposable database created inside the owned local container
- Test: `TestProviderSyncPostgresPgvectorIntegration`
- Command: `go test ./internal/storage/postgres -run TestProviderSyncPostgresPgvectorIntegration -count=1`
- Result: pass

The smoke inserts one in-scope raw event and one foreign-scope event, applies all
migrations including `0034_runtime_capability_event_sync`, performs an initial
snapshot and cursor continuation, and verifies that the continuation returns
only the in-scope event with a bounded replay identity and payload. The test
also persists and reloads the cursor through PostgreSQL.

An existing evaluation database with a divergent historical migration ledger
was intentionally rejected by the migration runner before any write. The smoke
was rerun in a fresh database in the same pgvector container, preserving the
migration safety boundary.

## Offline recovery evidence

The provider package tests cover malformed and foreign cursors, deterministic
duplicate replay, bounded snapshot continuation, expiry, and a retention floor
that returns `resync required` without emitting partial events. HTTP tests verify
machine-readable HTTP 409 recovery responses and reject mutation-shaped sync
requests before any source access.
