## 1. Contract and domain models

- [x] 1.1 Define transport-neutral synchronization capabilities, schema versions, event kinds, limits, cursor, snapshot, completion, and recovery models; verify validation rejects missing, oversized, malformed, and unsupported values.
- [x] 1.2 Extend provider capability discovery with synchronization metadata and disabled future transport declarations; verify capability unit tests cover limits, redaction, and unknown build metadata.
- [x] 1.3 Add bounded provider error categories for synchronization retry, compatibility, scope, and `resync_required` outcomes; verify error serialization contains no raw scope, payload, or stack data.

## 2. Durable synchronization state and event projection

- [x] 2.1 Add PostgreSQL migration and repository support for exact-scope synchronization watermarks, cursor ownership, retention metadata, and replay identity; verify migration manifest, apply, rollback, and scope-isolation tests pass.
- [x] 2.2 Implement snapshot watermark capture and lifecycle-safe snapshot loading from existing PostgreSQL projections; verify snapshot tests prove hidden and foreign records are omitted.
- [x] 2.3 Implement ordered durable event projection with monotonic per-scope sequence, source watermark, schema version, replay identity, bounded payload/reference, and retention checks; verify deterministic ordering and retention-window tests pass.
- [x] 2.4 Implement cursor validation, continuation, acknowledgment boundary, expiry, and `resync_required` decisions without mutating canonical memory; verify duplicate, stale, malformed, cross-binding, and expired-cursor tests pass.

## 3. OpenAPI pull transport

- [x] 3.1 Add the provider synchronization request/response schemas and route shape under the existing provider boundary; verify OpenAPI generation and route inventory tests pass.
- [x] 3.2 Implement initial synchronization with bounded snapshot continuation and terminal `sync_complete`; verify an in-scope runtime receives a stable watermark, cursor, and completion marker.
- [x] 3.3 Implement cursor-based continuation and deterministic retry behavior; verify reconnect tests show no skipped, reordered, duplicated, or cross-scope events.
- [x] 3.4 Implement bounded compatibility, validation, scope, retry, and `resync_required` HTTP responses; verify clients receive machine-readable errors without hidden-record disclosure.

## 4. Provider adapter and runtime integration

- [x] 4.1 Wire synchronization through the existing runtime binding middleware and resolved agent/session/provider-instance identity; verify caller-supplied widened scope is rejected before storage access.
- [x] 4.2 Ensure synchronized payloads reuse lifecycle filtering, provenance references, redaction, and citation limits; verify suppressed, forgotten, deleted, stale, and out-of-scope transitions do not leak content.
- [x] 4.3 Confirm synchronization requests cannot perform direct canonical-memory or lifecycle mutation; verify unsupported mutation payloads fail and existing governed intent/admin routes remain the only write paths.
- [x] 4.4 Add provider client/runtime documentation for initial sync, cursor acknowledgment, reconnect, retention gaps, and full resynchronization; verify self-hosting smoke documentation checks pass.

## 5. Conformance and offline replay

- [x] 5.1 Extend offline provider fixtures for capabilities, initial snapshot, ordered replay, cursor continuation, `sync_complete`, duplicate retry, and `resync_required`; verify the provider-contract report remains separate from retrieval metrics.
- [x] 5.2 Add exact-scope, lifecycle, redaction, schema-compatibility, restart, and retention-gap conformance cases; verify failures make the provider readiness result incomplete or ineligible.
- [x] 5.3 Add deterministic replay and transport-neutral equivalence assertions that future adapters can reuse; verify identical cursor inputs produce identical event identities, ordering, and completion outcomes.

## 6. Observability and operational safeguards

- [x] 6.1 Add low-cardinality synchronization metrics and bounded diagnostics for initial, resumed, completed, retried, and resync-required outcomes; verify telemetry tests reject raw queries, scope values, identifiers, and payloads.
- [x] 6.2 Add retention, cursor-expiry, and synchronization backlog health signals to existing readiness/maintenance surfaces; verify degraded synchronization evidence cannot claim provider readiness.
- [x] 6.3 Document rollback and disable behavior so synchronization can be disabled while existing provider operations continue; verify capability discovery and route behavior fail closed when disabled.

## 7. Verification and release evidence

- [x] 7.1 Add OpenAPI, app, provider, storage, and conformance regression coverage for the full synchronization lifecycle; verify focused package tests pass.
- [x] 7.2 Run full Go tests, race tests, vet, strict OpenSpec validation, and diff checks; verify all pass with synchronization enabled in the test configuration.
- [x] 7.3 Run an explicitly owned PostgreSQL + pgvector real-stack synchronization smoke/replay test and record bounded evidence for initial sync, reconnect, retention gap, and scope isolation.
