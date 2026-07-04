## Context

Stele already has the building blocks for embedding operations: durable rebuild records, append-only vector revisions, provider-aware runtime wiring, admin backlog inspection, and single-record retry or requeue controls with audit ledger writes. That means operators can diagnose one failing memory and restore it to the normal rebuild path, but they still cannot coordinate a provider migration across a scope or explain which remediation actions happened during such a migration.

The next operational gap is not raw embedding generation. It is controlled rollout. A self-host operator needs to declare an exact target, activate that target in waves, observe progress and failure hotspots, pause or cancel the rollout without rewriting history, and later inspect recovery actions with enough context to explain what happened during the cutover.

This design stays aligned with repository constraints:

- PostgreSQL remains the only system of record.
- Semantic rebuild execution continues to flow through the existing durable scheduler and worker path.
- `vector_revisions` stay append-only and are never rewritten by operator actions.
- Scope isolation remains mandatory across plan creation, inspection, rollout progression, and recovery history reads.

## Goals / Non-Goals

**Goals:**
- Introduce a durable provider cutover plan model with immutable target snapshots, operator attribution, and bounded rollout state.
- Add admin inspection for cutover plans and embedding recovery history at both memory and scope granularity.
- Reuse the existing embedding rebuild path for rollout execution so provider cutovers do not become a second execution system.
- Preserve auditability by linking remediation actions to a cutover plan when they occur inside an active rollout.
- Give operators documented rollout and rollback workflows that fit Stele's forward-only vector governance model.

**Non-Goals:**
- Do not implement policy versioning for runtime embedding routes in this change.
- Do not add inline bulk embedding generation or a second worker dedicated only to cutovers.
- Do not introduce automatic semantic quality gates, A/B score comparisons, or shadow vector production.
- Do not support direct rollback of active vectors in place; rollback is modeled as a new cutover plan toward another target.
- Do not broaden any public API surface or weaken admin scope requirements.

## Decisions

### Decision: Model provider migration as a durable cutover plan plus per-memory items

Cutovers will be represented by a plan record and a per-memory membership table rather than by a transient query over current drift state. The plan stores immutable target data such as provider, model, dimensions, scope, optional class filters, rollout wave size, current status, and operator attribution. Per-memory cutover items record which canonical memories belong to that plan and whether they are queued, rebuilding, current, failed, skipped, paused, or cancelled from the plan's perspective.

This is heavier than deriving progress on the fly, but it gives the operator an actual rollout object to inspect, pause, cancel, and audit later. It also avoids ambiguity when runtime defaults change after the plan is created.

Alternatives considered:
- Derive plan membership dynamically from current `embedding_rebuilds`. Rejected because progress and audit become unstable once runtime configuration or canonical visibility changes.
- Rewrite routing configuration directly and let drift detection implicitly drive the rollout. Rejected because operators need a durable operational object with attribution and bounded scope.

### Decision: Persist exact target snapshots on the plan instead of referencing mutable runtime aliases

The plan target will be stored as an immutable snapshot of provider, model, and dimensions. Activation validates that the current runtime can support that target, but the plan does not point at a mutable alias whose meaning may later change.

This keeps audit history truthful. A historical plan should still describe the exact semantic target it intended to roll out even if runtime configuration later evolves.

Alternatives considered:
- Store only provider name and re-resolve model or dimensions at execution time. Rejected because audit history would drift with configuration.
- Reference a named route alias only. Rejected because the repository does not yet have durable route policy versioning.

### Decision: Let the scheduler advance rollout waves, while the existing rebuild worker still owns embedding execution

The scheduler will be responsible for activating the next wave of cutover items by marking their underlying rebuild records claimable through the ordinary embedding rebuild path. The embedding rebuild worker continues to own generation, compare-and-promote, failure recording, and active vector updates.

This preserves one durable execution path instead of inventing a second cutover-specific processing loop. It also keeps lease safety consistent: pausing or cancelling a plan stops future waves, but already rebuilding items keep their current worker ownership.

Alternatives considered:
- Add a dedicated cutover worker that embeds memories directly. Rejected because it duplicates the existing rebuild machinery and weakens safety.
- Mark every plan item pending immediately. Rejected because large scope cutovers need bounded pacing and pause controls.

### Decision: Extend the embedding recovery ledger with optional cutover attribution and expose read-side history

`embedding_recovery_ledger` will gain an optional `cutover_plan_id` so retry or requeue actions during a rollout can be tied back to the plan. New read models will expose scope-level history filters and memory-level history timelines, including actor, reason, before/after snapshots, and cutover linkage when present.

This turns the existing write-only ledger into an operator-grade audit surface and lets plan inspection correlate progress with remediation activity.

Alternatives considered:
- Keep recovery history embedded only inside the cutover plan tables. Rejected because recovery actions must stay useful even outside cutovers.
- Expose only aggregated recovery counters. Rejected because incident review needs exact actor and timing details.

### Decision: Support create, activate, pause, and cancel controls, but keep rollback as a new forward plan

The admin surface will support:

- create cutover plan
- activate cutover plan
- pause active plan
- cancel draft, active, or paused plan

Pause and cancel only affect future waves. They do not rewrite already scheduled history or take over rebuilding work. Rollback remains a new cutover plan toward the prior provider or model target, which preserves the append-only vector lineage model.

Alternatives considered:
- Support direct rollback of active vectors. Rejected because it would mutate semantic history in place.
- Omit pause and cancel in v1. Rejected because provider cutover without a bounded stop control is operationally weak.

## Risks / Trade-offs

- [Plan membership tables increase storage and query complexity] -> Keep item rows scoped to active operator rollouts and design list or detail reads around indexed plan and scope queries.
- [Runtime provider support may change after a plan is activated] -> Validate on activation, preserve immutable target snapshots, and surface runtime degradation plus failure hotspots through admin inspection.
- [Large cutovers could flood the rebuild backlog] -> Advance rollout in waves with explicit batch size and plan state transitions rather than enqueuing the entire scope immediately.
- [Pause or cancel semantics may confuse operators if some work is already rebuilding] -> Document and expose that pause or cancel only stops future waves; current leases continue until they naturally finish or fail.
- [Recovery history queries may expose too much operator detail] -> Keep them under the existing admin auth boundary with full scope isolation and no public surface expansion.

## Migration Plan

1. Add PostgreSQL tables for cutover plans and plan items, plus optional `cutover_plan_id` on `embedding_recovery_ledger`.
2. Add repository, service, and HTTP contracts for creating, listing, reading, activating, pausing, cancelling, and auditing cutover plans.
3. Extend scheduler maintenance flow with a cutover dispatcher that advances eligible plan items in bounded waves through the existing rebuild path.
4. Update admin inspection and docs so operators can monitor rollout progress and recovery history before depending on the feature in production.
5. For rollback, create a new cutover plan toward the previous provider target. Do not mutate vector history in place.

## Open Questions

- Should v1 cutover selection support only scope plus class filters, or also explicit memory id allowlists?
- Should plan detail expose only aggregates and failed samples, or also page through every plan item in the first iteration?
- Do we want plan pause or cancel actions to require an explicit `reason`, separate from the original creation reason?

## References

- Related archive: `openspec/changes/archive/2026-06-28-embedding-operator-controls-and-provider-runtime/`
- Related commands: `openspec status --change "embedding-provider-cutover-and-recovery-audit"`, `/opsx:explore`, `/opsx:apply`
