## Why

Stele can ingest, retrieve, replay, and diagnose memory workflows, but operators still lack a closed loop for proving memory quality, protecting ingestion during degradation, and safely repairing known failures. This change connects quality evaluation, admission pressure decisions, durable repair execution, and post-repair verification into one service-side operational workflow.

## What Changes

- Add scoped memory quality evaluation runs that exercise ingestion pressure, retrieval/context quality, lifecycle-safe filtering, and degraded semantic projection behavior.
- Add an admission pressure contract for event ingestion and repair-triggered work so the service can return bounded decisions such as `accept`, `accept_degraded`, `queue`, or `reject` with stable reason codes.
- Add durable repair plans generated from evaluation failures and pressure diagnostics, limited to safe actions such as retrying embeddings, requeueing governance work, replaying derived insights, or marking manual review.
- Add repair execution through the existing worker/scheduler model with leases, retry state, idempotency, and audit history.
- Add post-repair verification runs that compare before/after quality and pressure results without rewriting canonical memory in place.
- Add low-cardinality metrics and admin diagnostics for evaluation outcomes, admission pressure decisions, repair actions, and verification status.
- No breaking API changes are expected.

## Non-goals

- Do not add SDK, UI, or end-user product logic.
- Do not replace existing ingestion, retrieval, replay, embedding rebuild, governance, or worker orchestration contracts.
- Do not introduce automatic canonical memory rewriting as a repair action.
- Do not expose suppressed, forgotten, expired, or deleted memory through default retrieval or non-admin diagnostics.
- Do not make embedding provider health a required dependency for `api` runtime readiness.

## Capabilities

### New Capabilities

- `memory-quality-admission-repair`: Covers scoped quality evaluation runs, pressure-aware admission decisions, bounded repair plans, durable repair execution, and post-repair verification.

### Modified Capabilities

- `admission-readiness-diagnostics`: Extend admission diagnostics to include ingestion and repair pressure decisions with stable finding codes.
- `event-ingestion`: Extend event ingestion responses to surface pressure-aware acceptance, degradation, queueing, or rejection semantics.
- `worker-orchestration-and-maintenance-jobs`: Extend durable worker execution to cover quality repair jobs while preserving lease and idempotency guarantees.
- `service-observability`: Extend metrics and diagnostics to cover quality evaluations, admission pressure, repair actions, and verification outcomes.

## Impact

- API: Adds admin quality evaluation, repair plan, repair execution, and verification inspection endpoints; extends event ingestion response metadata for admission pressure decisions.
- Storage: Adds durable records for evaluation runs, evaluation findings, repair plans, repair actions, verification runs, and audit metadata.
- Workers/scheduler: Adds bounded quality repair job dispatch and execution paths using existing durable orchestration semantics.
- Retrieval/context: Adds internal evaluation probes that verify lifecycle-safe recall and degraded semantic projection behavior without changing default retrieval semantics.
- Observability: Adds low-cardinality metrics, structured logs, and admin diagnostics for quality and admission repair loops.
- Artifact references: proposal/design/spec/tasks are managed through `openspec`; implementation should be driven with the local `openspec-apply-change` workflow.
