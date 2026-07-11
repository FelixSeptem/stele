## Why

Stele now supports concrete embedding providers, backlog inspection, and single-record retry or requeue controls, but operators still cannot run a provider cutover as a governed rollout or reconstruct remediation activity across that rollout after the fact. As semantic targets become more operationally important, provider migrations without durable orchestration and recovery audit leave self-host deployments hard to reason about and risky to recover.

## What Changes

- Add durable embedding provider cutover plans that capture scope, target provider or model snapshot, rollout pacing, operator attribution, and lifecycle status.
- Add bounded rollout controls for activating, pausing, and cancelling provider cutovers while continuing to reuse the existing embedding rebuild path instead of mutating vector history directly.
- Add admin recovery history reads for embedding remediation at both scope and memory granularity, including actor, action, reason, timing, and optional cutover attribution.
- Extend memory-level embedding diagnostics and plan-level inspection so operators can correlate drift, failure, recovery, and current rollout progress from the admin surface.
- Update self-host operator guidance with provider cutover workflows, audit inspection steps, and rollback guidance based on reverse cutover plans rather than history rewrites.

## Capabilities

### New Capabilities
- `embedding-provider-cutover-governance`: durable cutover plans, rollout controls, plan progress, and cutover-linked audit context for embedding target migrations

### Modified Capabilities
- `admin-inspection-surface`: add embedding cutover plan inspection and embedding recovery history query requirements
- `embedding-lifecycle-governance`: expose cutover attribution and target context in derived embedding lifecycle diagnostics
- `worker-orchestration-and-maintenance-jobs`: define scheduler-driven provider cutover progression through the existing durable rebuild path
- `self-hosting-bootstrap`: document provider cutover rollout, rollback, and recovery-audit workflows for self-host operators

## Non-goals

- Do not introduce per-request dynamic model routing or end-user provider selection.
- Do not allow operator actions to overwrite `vector_revisions` or seize already rebuilding work.
- Do not add automatic semantic quality scoring, provider benchmarking, or shadow inference in this change.
- Do not add bulk rollback by mutating old vector history in place; rollback remains a new forward cutover plan.
- Do not redesign embedding route configuration into a policy versioning system in this proposal.

## Impact

- Affected code: embedding admin services, scheduler orchestration, PostgreSQL schema and repositories, admin HTTP handlers, OpenAPI definitions, and self-host docs
- Affected APIs: new admin embedding cutover and recovery history routes under `/v1/admin/embedding/...`
- Affected systems: `embedding_rebuilds`, `embedding_recovery_ledger`, new cutover persistence tables, scheduler dispatch logic, and operator bootstrap workflows
- Dependencies: runtime-configured provider validation remains the source of truth for whether a cutover target can be activated

## References

- Related archive: `openspec/changes/archive/2026-06-28-embedding-operator-controls-and-provider-runtime/`
- Related commands: `openspec status --change "embedding-provider-cutover-and-recovery-audit"`, `/opsx:explore`, `/opsx:apply`
