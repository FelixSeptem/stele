## Context

See proposal.md for the motivation. The repository already has PostgreSQL-backed
reflection runs, append-only reflection checkpoints, compaction evidence,
versioned context projections, scheduler run history, and exact-scope worker
leases. The design must connect these existing records without introducing an
external queue or a second canonical store.

## Goals / Non-Goals

**Goals:**

- Provide one derived-work identity and execution contract for reflection,
  compaction, projection rebuild, and derived maintenance.
- Make transcript offset and source-watermark progress resumable and lease-safe.
- Support `postgres_durable` and `memory_buffer` modes with explicit durability
  and loss semantics.
- Preserve append-only evidence, exact scope, lifecycle filtering, idempotency,
  bounded retries, and operator inspection.
- Keep public retrieval, context response shapes, and canonical memory behavior
  compatible.

**Non-Goals:**

- External Redis/Kafka, a second scheduler, a second persistence system, or
  client-facing queue APIs.
- Lossless guarantees for unflushed `memory_buffer` work.
- Direct canonical writes by reflection, compaction, projection, or insight
  workers.
- Hot queue-mode migration or payload replication of raw content.

## Decisions

### 1. PostgreSQL is the durable queue source of truth

Use a new scoped work-item table with immutable identity fields, source-watermark
references, lease/retry state, bounded failure categories, and timestamps.
Claiming uses the existing compare-and-set lease pattern and stable ordering.
This is preferred over Redis/Kafka because queue state, audit evidence, scope
isolation, and canonical source references remain transactionally close in the
only system of record.

### 2. Memory buffering is an explicitly lossy adapter

The in-process mode uses a bounded channel and batch flusher. A task is only
reported as durable after PostgreSQL insert succeeds. Capacity, flush interval,
batch size, and loss counters are bounded configuration. Process loss may drop
unflushed derived work; it cannot drop already committed checkpoints, evidence,
canonical records, or audit decisions.

### 3. One work item links to specialized evidence records

Reflection runs, compaction evidence, and context projections retain their
specialized schemas and APIs. A work item references them and carries the
trigger/source watermark needed for deduplication and recovery. This avoids
duplicating payloads and allows each record to keep its existing validation and
append-only rules.

### 4. Watermark and checkpoint advancement is monotonic

Workers may commit only a non-decreasing transcript offset or source watermark
under the active lease. A lost owner cannot checkpoint, complete, or publish
derived output. Reclaim resumes from the latest committed checkpoint and
revalidates the source watermark before producing the next side effect.

### 5. Compaction chains through durable follow-up work

A successful compaction writes evidence with source coverage, summary/derivation
identity, token estimates, recent-tail references, and freshness/SLO result. Any
follow-up reflection is submitted with a deterministic work key in the same
scope. Duplicate triggers resolve to the existing work item rather than creating
another derived execution.

### 6. Queue mode is process-start configuration

A process runs one mode selected at startup. Durable work is never silently moved
into a lossy buffer. Switching from memory to durable requires a drain report;
switching from durable to memory leaves existing durable work on its normal claim
path. This avoids split-brain ownership and hidden durability changes.

### 7. Inspection and telemetry use bounded categories

Admin inspection exposes only mode, state, depth/lag buckets, loss/flush
categories, retry/exhaustion, freshness, and SLO. Metrics and logs reject or
bucket scope values, identifiers, payloads, raw errors, and worker identities.

## Risks / Trade-offs

- **[Risk] PostgreSQL queue throughput becomes a bottleneck.** -> Batch
  insertion, bounded claim limits, scoped indexes, and memory buffering provide
  a controlled optimization path without changing durable semantics.
- **[Risk] Lossy mode hides missing derived work.** -> Mark the process
  non-durable, expose drop/flush counters, invalidate freshness/conformance
  claims, and document explicit loss behavior.
- **[Risk] Replay creates duplicate derived outputs.** -> Stable work keys,
  source-watermark deduplication, append-only checkpoints, and idempotent
  specialized repositories prevent duplicate durable effects.
- **[Risk] A stale worker publishes after reclaim.** -> Every checkpoint,
  evidence write, projection promotion, and terminal transition includes a
  lease-owner compare-and-set guard.
- **[Risk] Queue inspection leaks operational identifiers.** -> Return bounded
  categories and opaque cursors only; keep IDs and raw payloads in authorized
  detail reads.
- **[Risk] Existing consumers assume direct reflection/compaction invocation.**
  -> Keep current interfaces as compatibility adapters that enqueue work and
  preserve existing response shapes.

## Migration Plan

1. Add the derived work-item and audit/checkpoint migration with exact-scope and
   idempotency indexes.
2. Add queue configuration defaults; default production deployments to
   `postgres_durable`, with `memory_buffer` opt-in for explicitly lossy
   environments.
3. Wire reflection, compaction, projection rebuild, and derived maintenance
   dispatchers through the new work-item contract while retaining compatibility
   entry points.
4. Backfill no historical work items; existing durable reflection/projection
   records remain readable and are only queued on new triggers or explicit
   recovery.
5. Roll back by disabling new dispatchers while retaining already committed
   work/evidence; do not delete queue or evidence history during rollback.
