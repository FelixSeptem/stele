## Why

Stele already has reflection runs, transcript/input watermarks, compaction evidence,
versioned context projections, and durable scheduler history, but they are not yet
one recoverable execution contract. Derived work can be triggered repeatedly,
progress can be difficult to resume across process restarts, and compaction or
projection freshness can lag without one bounded queue/inspection model.

This proposal closes that gap by adding a configurable PostgreSQL-backed durable
queue or bounded in-memory buffer for derived-only background work. It makes
reflection, transcript progress, compaction evidence, follow-up reflection, and
projection rebuilds replay-safe and observable while preserving PostgreSQL as the
only system of record for durable state.

## What Changes

- Add a unified derived work-item contract for reflection triggers, compaction,
  context projection rebuilds, derived insight maintenance, and freshness/retention
  work.
- Support configured queue modes:
  - `postgres_durable` for restart-safe persisted work with claim, lease, retry,
    recovery, and audit state.
  - `memory_buffer` for bounded low-latency buffering with explicit, observable
    loss of unflushed derived work.
- Keep raw event ingestion, canonical memory/version writes, memory intents, review
  decisions, reflection checkpoints, and committed compaction evidence durable;
  these paths cannot rely on the lossy memory buffer.
- Extend reflection execution with append-only transcript checkpoint progression,
  lease-safe resume, retry exhaustion, duplicate trigger handling, and governed
  follow-up scheduling.
- Extend compaction evidence with deterministic source-watermark coverage,
  summary/version identity, token pressure, recent-tail references, freshness/SLO
  eligibility, and follow-up reflection linkage.
- Route context projection rebuilds and derived insight maintenance through the same
  work-item identity, scope isolation, retry, and recovery contract.
- Add queue mode/configuration validation, bounded buffer/flush behavior, drain
  reporting, and explicit startup-only mode switching semantics.
- Add authorized queue inspection and low-cardinality telemetry for queue depth,
  flush lag, dropped work, retry/exhaustion, watermark freshness, and SLO state.
- Preserve existing public retrieval, context response shapes, canonical memory
  lifecycle, and exact tenant/project/namespace authorization boundaries.

## Non-goals

- No external Redis/Kafka queue, second scheduler, hosted control plane, SDK, UI,
  or MCP transport.
- No queue payload copy of raw event content, canonical memory content, credentials,
  provider payloads, query text, or scope values.
- No direct canonical-memory mutation from reflection, compaction, projection, or
  derived insight workers.
- No guarantee that unflushed `memory_buffer` work survives process loss.
- No hot runtime queue-mode switching or automatic migration of in-flight durable
  work into the lossy memory buffer.

## Capabilities

### New Capabilities

- `durable-derived-work-queue`: Configurable PostgreSQL durable queue and bounded
  in-memory buffer semantics for derived background work, including identity,
  claim/lease, retry, loss, drain, and recovery behavior.

### Modified Capabilities

- `worker-orchestration-and-maintenance-jobs`: Extend durable worker orchestration
  to unified derived work items, transcript checkpoints, compaction chaining, and
  queue-mode semantics.
- `summary-compaction`: Add watermark-bound, resumable, evidence-covered
  compaction execution and follow-up reflection scheduling.
- `versioned-context-projections`: Route rebuilds through durable derived work
  and enforce queue/rebuild freshness eligibility.
- `durable-multiscope-maintenance-observability`: Add queue depth, buffer loss,
  flush lag, watermark freshness, and derived-work SLO evidence.
- `derived-insight-replay`: Reuse the unified derived queue for replay/backfill
  while preserving bounded scope, idempotency, and audit behavior.
- `admin-inspection-surface`: Add authorized, bounded inspection of queue mode,
  drain/loss state, and derived-work outcomes.
- `service-observability`: Add low-cardinality queue, flush, drop, checkpoint,
  and compaction lifecycle metrics/log categories.

## Impact

- A new PostgreSQL migration and repository interfaces for derived work items,
  append-only checkpoints, leases, and bounded audit transitions.
- Configuration and runtime wiring for dispatcher, durable queue, memory buffer,
  batch flusher, worker claims, and startup validation.
- Changes to reflection, compaction, projection, derived insight, scheduler, and
  admin inspection code with focused unit, integration, race, isolation, migration,
  and telemetry tests.
- New operator documentation describing durability modes, allowed data loss,
  drain/switch behavior, freshness gates, and recovery procedures.
- No new runtime dependency outside the existing Go service and PostgreSQL stack.
