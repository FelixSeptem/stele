## Context

Stele already has scoped event ingestion, governed canonical memory, hybrid retrieval, context assembly, worker and scheduler runtimes, embedding lifecycle controls, derived experience insights, insight feedback, admin inspection, and baseline observability. These capabilities are useful individually, but an operator still lacks a single repeatable flow that proves a self-hosted deployment is operational from startup through memory retrieval and insight inspection.

Derived insight derivation also needs an operational replay path. Without replay, operators cannot safely preview or backfill insight changes after policy changes, feedback changes, ingestion fixes, or evaluator improvements. Replay must not become a canonical memory rewrite mechanism; it only re-evaluates derived insights inside explicit scope, time, and limit boundaries.

## Goals / Non-Goals

**Goals:**

- Provide a first-ten-minutes smoke loop that validates `api`, `worker`, and `scheduler` operation end to end.
- Add admin-only derived insight replay with dry-run planning, bounded apply/backfill, replay status, and replay reports.
- Preserve scope isolation, lifecycle governance, evidence history, and audit attribution during replay.
- Route replay apply work through durable background execution rather than executing broad mutation work inline in admin requests.
- Expose enough admin diagnostics, metrics, and documentation for operators to understand smoke and replay failures without querying PostgreSQL directly.

**Non-Goals:**

- No SDK, UI, hosted onboarding product, or MCP adapter.
- No new storage engine or non-PostgreSQL system of record.
- No canonical memory history rewrite, vector history mutation, or in-place provenance replacement.
- No new memory classes or autonomous reasoning-provider inference.
- No general-purpose repair framework beyond derived insight replay/backfill.
- No public replay API.

## Decisions

### Decision: Treat smoke as an operator contract, not a special runtime mode

The smoke loop will be implemented as documented operator steps, fixture inputs, API calls, and tests around existing `api`, `worker`, and `scheduler` modes. This avoids adding a fourth runtime mode or hidden test-only behavior.

Alternative considered: add a dedicated `smoke` mode. That would make demos easy but would not prove the production modes are wired correctly, so it is rejected.

### Decision: Model replay as plan, execute, and report

Replay dry-run creates a deterministic plan/report preview without applying lifecycle or insight mutations. Replay apply creates a durable job request that executes the same bounded evaluation and writes an auditable report. Reports are the operator-facing evidence for both success and failure.

Alternative considered: expose a single synchronous admin endpoint that both evaluates and mutates insights. That is rejected because it would create long admin requests, weak retry semantics, and unclear partial-failure behavior.

### Decision: Bound replay by scope, time window, and limit

Every replay request must include authorized scope and bounded selection controls such as observed time range, evidence limit, insight type filters, and dry-run/apply mode. Replay never expands across tenant, project, or namespace boundaries.

Alternative considered: allow global replay for all scopes. That is rejected because it risks accidental cross-scope work, hard-to-audit side effects, and unbounded operational load.

### Decision: Replay derived insights only

Replay re-evaluates derived insight fingerprints, evidence windows, lifecycle transitions, feedback-influenced decisions, and lesson outputs. It does not rewrite raw events, canonical memories, memory versions, vector revisions, or provenance records.

Alternative considered: create a generic memory replay framework. That is rejected as too broad for this change and inconsistent with the need to preserve canonical memory history.

### Decision: Use existing worker and scheduler ownership semantics

Replay apply/backfill work will be represented as durable background work with idempotency keys, leases, retry state, and failure summaries. Admin requests can enqueue or inspect replay work, but they do not seize worker ownership.

Alternative considered: run replay apply directly inside the admin handler. That is rejected because replay can be expensive and must remain restart-safe.

## Risks / Trade-offs

- Replay can produce confusing results if dry-run and apply see different evidence windows -> Mitigation: report the effective selection window, source counts, policy version or evaluator identity, and idempotency key used for execution.
- Replay may duplicate lifecycle transitions after retries -> Mitigation: key replay output by scope, window, insight fingerprint, replay run identity, and target decision so retries are idempotent.
- Smoke tests can become brittle if they depend on timing -> Mitigation: prefer explicit admin job/status inspection and bounded polling over fixed sleeps.
- Operator reports may expose sensitive insight content -> Mitigation: keep detailed reports admin-only and preserve public context visibility rules.
- Scope-bound replay may not cover cross-namespace operator mistakes -> Mitigation: require separate replay runs per namespace and document that cross-scope replay is intentionally unsupported.

## Migration Plan

1. Add storage for replay run records and reports, including scope, request parameters, mode, status, counters, failure summaries, actor, reason, and timestamps.
2. Add admin OpenAPI contracts and handlers for dry-run planning, apply/backfill enqueue, run inspection, and report reads.
3. Add worker execution for replay apply/backfill using existing derivation and feedback policy services.
4. Add smoke fixtures and docs that exercise startup, ingest, worker, scheduler, retrieval, context assembly, admin inspection, replay dry-run, replay apply, and metrics.
5. Add metrics and tests for replay outcomes and smoke-loop diagnostics.

Rollback is additive: disabling the new admin replay routes and scheduler/worker registration stops new replay execution. Existing replay run records remain audit history and do not affect canonical memory retrieval by themselves.

## Open Questions

- Should replay apply be accepted only as a queued job, or should very small bounded apply requests be allowed to execute synchronously under an explicit limit?
- Should smoke fixtures live under `docs/`, `scripts/`, or a dedicated `examples/` path?
- Should replay reports include sampled insight IDs by default, or only counts unless a detail flag is provided?
