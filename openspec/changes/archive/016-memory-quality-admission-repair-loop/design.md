## Context

Stele already has durable ingestion, governed retrieval, embedding lifecycle management, replay, worker orchestration, readiness diagnostics, and baseline observability. Those pieces expose important signals, but they do not yet form an operator-facing loop that answers whether memory quality is acceptable, whether ingestion should be protected under pressure, which safe repair actions are available, and whether repair improved the outcome.

This design keeps the loop service-side and self-host friendly. It adds admin APIs, persisted reports, bounded repair plans, worker-executed repair actions, and low-cardinality diagnostics without adding UI, SDK, or end-user product behavior.

## Goals / Non-Goals

**Goals:**

- Provide scoped quality evaluation runs for retrieval/context behavior, lifecycle-safe filtering, semantic projection degradation, and ingestion pressure.
- Provide pressure-aware admission decisions for event ingestion and repair-triggered work.
- Generate durable repair plans from quality and pressure findings using only bounded, safe actions.
- Execute repair plans asynchronously through the existing durable worker/scheduler model.
- Verify repair outcomes with post-repair evaluation runs and auditable before/after reports.
- Export metrics and diagnostics that are useful to operators without high-cardinality labels.

**Non-Goals:**

- No SDK, UI, or client product workflow.
- No automatic canonical memory rewriting, hidden lifecycle bypass, or in-place history mutation.
- No replacement of existing embedding rebuild, governance requeue, replay, retrieval, or worker contracts.
- No requirement that `api` readiness depends on embedding provider reachability.

## Decisions

### Use one quality finding taxonomy for evaluation, admission, and repair

Quality evaluations, ingestion admission pressure checks, and repair planning will use stable finding codes grouped by category, severity, component, and suggested action category. This avoids separate diagnostics vocabularies for retrieval quality, ingestion pressure, and repair planning.

Alternative considered: separate schemas for each subsystem. That would be simpler locally but would make reports harder to correlate and would duplicate metric label governance.

### Persist evaluation and repair records before execution

Evaluation runs, findings, repair plans, repair actions, and verification runs will be durable records scoped by tenant, project, and namespace. Admin requests create intent and inspection state; workers perform bounded execution later.

Alternative considered: execute repair inline from the admin request. That would reduce schema work but would violate the existing hot-write and durable-worker direction, and it would make retries, leases, and audit harder.

### Keep repair actions as transitions back into existing execution paths

Repair actions will not perform bespoke mutation logic. They will requeue governance work, retry embedding rebuild work, schedule replay apply work, or mark manual review state through existing contracts and lifecycle safeguards.

Alternative considered: add a generic repair engine that directly mutates underlying rows. That is too broad and conflicts with provenance and version-history rules.

### Make ingestion admission pressure visible but compatible

Event ingestion can still return a durable event id when accepted. The response metadata will include an admission decision and stable finding codes so clients and operators can distinguish normal acceptance from degraded acceptance or queueing. Rejection remains explicit and occurs before writing the event.

Alternative considered: silently accept all events and rely only on background backlog metrics. That preserves compatibility but hides production pressure until quality has already degraded.

### Verification is a first-class phase, not a log message

Post-repair verification will create a new evaluation run linked to the repair plan and compare it to the baseline report. Verification can pass, fail, or require manual review; it does not imply automatic deletion or rewriting of failed evidence.

Alternative considered: treat successful repair job completion as proof. That only proves actions executed, not that memory quality or pressure improved.

## Risks / Trade-offs

- Broad loop scope could become a generic repair platform -> Mitigation: limit actions to explicit categories backed by existing workflows.
- Evaluation fixtures could become high-cardinality operational data -> Mitigation: store detailed findings in PostgreSQL but export only bounded metric labels.
- Admission pressure decisions may surprise existing ingestion callers -> Mitigation: keep accepted responses compatible and add pressure metadata; only reject before durable write.
- Repair plans may race with active worker leases -> Mitigation: require lease-safe checks and reject actions that would seize active ownership.
- Verification may produce ambiguous outcomes when dependencies remain degraded -> Mitigation: record residual finding codes and `manual_review` status instead of forcing success/failure.

## Migration Plan

1. Add database migrations for evaluation runs, findings, repair plans, repair actions, verification links, and admission pressure audit metadata.
2. Add internal packages for quality finding classification, admission pressure evaluation, repair plan generation, and verification comparison.
3. Extend event ingestion response metadata without removing the stable event identifier contract.
4. Add admin endpoints for evaluation, repair planning, repair execution, and report inspection.
5. Add worker/scheduler execution for repair actions using existing lease and retry semantics.
6. Add metrics, structured logs, and docs for the full operator loop.

Rollback is schema-compatible by leaving durable records unused and disabling the new admin endpoints and worker dispatch paths. Existing ingestion, retrieval, embedding rebuild, replay, and governance flows continue to operate independently.

## Open Questions

- Which initial quality fixtures should be implemented first: retrieval-only probes, context assembly probes, ingestion pressure probes, or a minimal combined suite?
- Should repair planning require an explicit dry-run approval step for every action, or only for actions above a configured risk level?
