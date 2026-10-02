# Reflection, Compaction, And Durable Derived Work Design

## Scope

This proposal closes the operational loop for reflection runs, transcript watermarks,
compaction evidence, and derived background work. It extends the existing
reflection, compaction, context projection, scheduler, and run-history contracts;
it does not create a second canonical memory system.

The derived work queue is limited to reflection triggers, compaction requests,
context projection rebuilds, derived insight maintenance, and related
freshness/retention work. Raw event ingestion, canonical memory/version writes,
memory intents, review decisions, reflection checkpoints, and committed
compaction evidence remain durable PostgreSQL operations.

## Queue Modes

The service supports one configured queue mode per process:

- `postgres_durable`: work items are persisted before dispatch. PostgreSQL
  provides the queue source of truth, claim/lease/retry/recovery state, and
  audit history.
- `memory_buffer`: a bounded in-process buffer batches work into PostgreSQL.
  Work is not durable until flush succeeds. Buffer overflow, eviction, flush
  failure, or process loss may drop unflushed derived work and must be exposed as
  bounded operational loss.

Mode changes take effect on process startup. Durable work is never moved into the
memory buffer automatically. Switching from memory to durable requires a drain
or an explicit bounded-loss report.

## Data Model

A unified derived work item carries:

- stable work key and kind;
- normalized exact tenant/project/namespace scope;
- idempotency key and input/source watermark;
- referenced reflection run, compaction evidence, session, or projection;
- priority bucket, attempt, retry time, lease owner/expiry;
- checkpoint offset, bounded failure category, lifecycle timestamps.

Work identity, scope, trigger references, input watermark, and created time are
immutable. Checkpoints and audit transitions are append-only. Current queue
state may advance through `queued`, `claimed`, `running`,
`retry_wait`, `completed`, `exhausted`, `cancelled`, or
`duplicate`. Memory-only states such as `buffered` and `buffer_dropped`
are not represented as durable success.

## Execution Flow

Dispatcher creates or resolves a work item using exact-scope idempotency. Workers
claim with lease-safe compare-and-set semantics. Reflection resumes from its last
committed transcript offset and can produce only governed candidates and
evidence references. Compaction writes derived evidence with source watermark,
summary version, token estimates, recent-tail references, and coverage; it may
enqueue a follow-up reflection through the same work-item path. Projection rebuild
writes a new version and never replaces canonical memory.

A worker that loses its lease cannot write checkpoints, evidence, projection
versions, or terminal completion. Retryable failures use bounded backoff;
exhausted work is inspectable and requires governed recovery to re-enter the
ordinary claim path.

## Inspection And Telemetry

Authorized admin inspection reports queue mode, bounded depth/lag buckets,
dropped and flush-failure counts, retry/exhaustion categories, watermark
freshness, and SLO buckets. It does not expose raw payloads, query text,
credentials, scope values, provider responses, or worker identifiers.

Metrics and logs use fixed operation/result/state/recovery/freshness categories
and duration/size buckets. Public retrieval and context response shapes remain
unchanged.

## Verification

Tests cover work identity and transitions, mode validation, bounded buffer loss,
batch flushing, durable claim/reclaim, checkpoint CAS, retries, exhaustion,
scope isolation, restart and duplicate triggers, session-to-reflection-to-
compaction chaining, projection freshness, migration upgrades, and redacted
admin/telemetry contracts.

