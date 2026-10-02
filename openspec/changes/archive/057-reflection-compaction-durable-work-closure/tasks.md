## 1. Queue Contract And Configuration

- [x] 1.1 Define derived work kinds, immutable identity, bounded states, loss dispositions, and queue mode configuration; verify validation rejects unsupported modes, unsafe limits, and unbounded payload references.
- [x] 1.2 Add deterministic work-key and exact-scope idempotency helpers; verify duplicate triggers resolve to one work identity and foreign scopes fail closed.
- [x] 1.3 Add queue mode, capacity, batch, flush, lease, retry, and retention configuration to runtime config; verify defaults and startup validation in config tests.

## 2. PostgreSQL Durable Work Queue

- [x] 2.1 Add forward/down migrations for scoped derived work items, append-only checkpoints, lease/retry state, terminal summaries, and bounded indexes; verify migration manifest and clean upgrade tests.
- [x] 2.2 Implement repository enqueue, duplicate resolution, claim, lease renewal, stale reclaim, retry, exhaustion, cancellation, and governed recovery with CAS ownership; verify pgxmock isolation and lease-conflict tests.
- [x] 2.3 Implement queue detail pagination and retention cleanup; verify stable opaque cursors, exact-scope filtering, idempotent cleanup, and preservation of terminal evidence.
- [x] 2.4 Add PostgreSQL integration coverage for concurrent claims, restart/reclaim, duplicate dispatch, retry exhaustion, migration upgrade, and cross-scope rejection.

## 3. In-Memory Buffer Adapter

- [x] 3.1 Implement bounded memory-buffer enqueue, batch flush, backpressure, eviction, and explicit dropped/flush-failure dispositions; verify capacity and loss counters.
- [x] 3.2 Implement startup-only mode selection and drain reporting; verify durable work is never moved into memory mode and unflushed loss is not reported as durable completion.
- [x] 3.3 Add crash/restart simulation tests for buffered tasks and verify only unflushed derived work may be lost while committed PostgreSQL evidence remains intact.

## 4. Reflection And Transcript Progress

- [x] 4.1 Route reflection triggers through derived work items while preserving existing trigger APIs and idempotency; verify session completion, event threshold, schedule, compaction, and operator triggers.
- [x] 4.2 Make transcript/input watermark and processed-offset advancement monotonic and lease-owned; verify stale workers cannot write checkpoints or completion after reclaim.
- [x] 4.3 Add retry/backoff, exhaustion, governed recovery, and duplicate-fire handling for reflection; verify exhausted runs are not automatically re-enqueued.
- [x] 4.4 Add integration tests for session completion to reflection execution, restart resume, candidate-only output, evidence references, and unchanged canonical memory.

## 5. Compaction And Projection Chaining

- [x] 5.1 Extend compaction execution to persist source coverage, token pressure, summary/derivation identity, recent-tail references, freshness/SLO, and checkpoint linkage; verify stale or divergent evidence fails closed.
- [x] 5.2 Enqueue deterministic follow-up reflection work after successful compaction; verify duplicate compaction triggers create no duplicate follow-up or summary side effects.
- [x] 5.3 Route context projection rebuilds through the queue and preserve append-only projection versions; verify dropped, stale, foreign, or hidden evidence leaves projections ineligible.
- [x] 5.4 Route derived insight maintenance and replay/backfill through the same queue; verify bounded scope, idempotent lifecycle transitions, and non-applied loss dispositions.

## 6. Runtime Wiring And Inspection

- [x] 6.1 Wire dispatcher, queue adapter, flusher, worker claims, and scheduler jobs into api/worker/scheduler runtime modes without adding a second scheduler.
- [x] 6.2 Add authorized exact-scope queue status/detail inspection with bounded mode, depth, lag, flush, drop, retry, freshness, and SLO fields; verify foreign scopes reveal no existence or counts.
- [x] 6.3 Add low-cardinality metrics and structured logs for enqueue, duplicate, claim, lease, checkpoint, flush, drop, retry, exhaustion, completion, and freshness outcomes; verify sensitive fields are absent.
- [x] 6.4 Update OpenAPI and operator documentation for queue modes, allowed loss, drain/switch behavior, recovery, and freshness semantics; verify OpenAPI and docs consistency checks.

## 7. Verification And Release Gates

- [x] 7.1 Add end-to-end tests for session completion -> reflection -> compaction -> follow-up reflection -> projection freshness across both queue modes.
- [x] 7.2 Run focused and full Go tests, race tests, vet, migration/integration tests against disposable PostgreSQL, and git diff --check; verify public retrieval and canonical-memory contracts are unchanged.
- [x] 7.3 Run openspec validate --all --strict, reconcile roadmap status, and record the queue durability/loss evidence needed before archiving.
