## Why

Stele now has durable embedding lifecycle governance, but two operational gaps remain: operators still lack a first-class admin surface to inspect and steer embedding rebuild activity, and the runtime still treats concrete embedding providers as an optional internal stub rather than a configured self-host feature. Closing both gaps together turns the recent lifecycle work into an operable end-to-end semantic retrieval system.

## What Changes

- Add admin-only embedding inspection endpoints for vector revision history, active revision state, rebuild backlog, and provider or model drift visibility under strict scope isolation.
- Add minimal operator controls for safe rebuild remediation, including retrying or requeueing eligible embedding work without bypassing worker lease and retry contracts.
- Add a concrete provider runtime registration model so `api`, `worker`, and `scheduler` modes can boot with configured embedding providers instead of relying on an empty static registry.
- Add startup and bootstrap guidance for provider configuration, missing-provider failure behavior, and smoke checks that confirm semantic rebuild wiring is active.
- Keep semantic generation internal-only and OpenAPI-first for admin surfaces; do not expand public memory APIs.

## Capabilities

### New Capabilities
- `embedding-provider-runtime`: runtime provider registration, configuration validation, and boot-time behavior for concrete embedding generation backends

### Modified Capabilities
- `admin-inspection-surface`: add embedding-specific inspection and operator remediation requirements
- `embedding-lifecycle-governance`: extend lifecycle visibility and drift attribution expectations for operator diagnostics
- `worker-orchestration-and-maintenance-jobs`: define safe embedding requeue and retry control behavior under durable worker ownership rules
- `self-hosting-bootstrap`: document provider configuration, startup expectations, and semantic rebuild smoke checks for self-host operators

## Non-goals

- Do not add SDK, UI, or end-user product workflows.
- Do not add inline embedding generation to foreground write paths.
- Do not redesign retrieval ranking or remove the transitional `canonical_memories.embedding` mirror in this change.
- Do not expose hidden or suppressed memory through new default retrieval paths.
- Do not add arbitrary operator overrides that can seize in-flight worker ownership or mutate vector lineage outside the governed rebuild path.

## Impact

- Affected areas: runtime bootstrap, scheduler and worker composition, embedding provider integration, admin OpenAPI surface, operator docs, and semantic rebuild tests.
- Affected systems: PostgreSQL-backed embedding rebuild state, background job execution, admin inspection handlers, and self-host deployment configuration.
- Dependencies: concrete provider implementations or adapters must be registered through internal runtime wiring; deployments without a configured provider remain supported but must surface explicit degraded-state diagnostics.

## References

- Related archived change: `openspec/changes/archive/2026-06-19-embedding-lifecycle-and-vector-governance/`
- Related commands: `openspec status --change "embedding-operator-controls-and-provider-runtime"`, `/opsx:apply`
