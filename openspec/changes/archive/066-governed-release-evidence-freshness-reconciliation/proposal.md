## Why

Release evidence currently proves eligibility when an evaluation or activation handoff is created, but there is no uniform follow-up that detects freshness expiry, source-watermark advancement, policy drift, fixture or representation mismatch, or expired rollback proof. As a result, an activation can retain an apparently valid historical handoff after its evidence is no longer compatible with the current release policy. This change closes that governance regression gap while preserving the evidence and audit history needed to explain every decision.

## What Changes

- Add a scoped, append-only reconciliation contract for release-evidence eligibility.
- Reconcile freshness, source watermarks, policy and fixture identity, representation compatibility, and rollback proof expiry for each activation scope.
- Revoke current activation eligibility fail-closed when a required gate becomes stale or incompatible, while retaining the original evidence, verdicts, and audit history.
- Allow eligibility to be restored only by a new compatible handoff or a newly completed governed evaluation; reconciliation itself never promotes an activation.
- Run reconciliation through the existing scheduler/worker maintenance path and expose an authorized, bounded admin trigger and inspection surface.
- Emit low-cardinality, redacted telemetry and durable reconciliation history that is deterministic, idempotent, replayable, and isolated by project, tenant, and namespace.
- Keep ordinary retrieval, context assembly, canonical memory, and the existing rollout system unchanged.

## Capabilities

### New Capabilities

- `governed-release-evidence-reconciliation`: Reconciles release evidence against current freshness, source, policy, rollback, and scope requirements and maintains activation eligibility history.

### Modified Capabilities

- `retrieval-release-evidence-run`: Evidence handoffs expose the source and compatibility identities required for later reconciliation and remain immutable after submission.
- `retrieval-release-gate-and-progressive-context-evaluation`: Activation eligibility is revoked or retained according to reconciliation verdicts, and restoration requires a new compatible handoff.
- `worker-orchestration-and-maintenance-jobs`: Scheduler and worker execution support bounded, idempotent, scope-bound reconciliation runs with durable retry and recovery state.
- `service-observability`: Reconciliation outcomes, freshness categories, revocations, and recovery are observable through redacted low-cardinality logs and metrics.
- `admin-inspection-surface`: Authorized operators can trigger and inspect reconciliation runs and current eligibility without exposing evidence payloads or foreign scope data.

## Impact

- A new PostgreSQL-backed reconciliation history and current eligibility projection, plus migrations and retention rules.
- Changes to release-evidence, release-gate, maintenance-job, observability, and admin inspection contracts; implementation will reuse existing ownership, lease, policy-precedence, attestation, rollback, and redaction boundaries.
- New scheduler/worker maintenance work and an admin-only API surface; no new external dependency or second rollout/feature-flag system.
- Documentation and roadmap updates describing the post-activation evidence lifecycle and fail-closed behavior.

## Non-goals

- No automatic rerun of a full benchmark or provider evaluation when evidence becomes stale.
- No mutation or rewriting of canonical memory, raw events, projections, or historical evidence.
- No change to default retrieval visibility, ranking behavior, public APIs, or rollout policy precedence.
- No relaxation of owned PostgreSQL/pgvector, exact-scope, freshness, isolation, attestation, or rollback gates.

