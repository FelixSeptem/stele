## Context

Stele already has durable embedding rebuild records, append-only vector revisions, deterministic routing inputs, and a scheduler-driven rebuild job. That foundation closes semantic lineage gaps, but operators still cannot inspect or steer embedding work through the admin surface, and the runtime still wires `EmbeddingRebuildJob` with an empty `embedding.StaticProviderRegistry{}` in scheduler mode. As a result, the system records rebuild intent correctly but remains partially degraded in self-hosted deployments unless downstream code manually injects providers.

This change combines the next two operational layers that were intentionally deferred by the previous embedding governance work:

1. expose embedding-specific admin diagnostics and tightly bounded remediation actions
2. make concrete provider registration a first-class runtime concern for self-hosted operation

The repository constraints remain unchanged: PostgreSQL is the system of record, semantic retrieval uses `pgvector`, hidden memory stays excluded from default retrieval, and hot write paths must remain free of inline embedding generation.

## Goals / Non-Goals

**Goals:**
- Add admin-only inspection for embedding rebuild state, vector revision lineage, active semantic projection, and provider or model drift under full scope isolation.
- Add minimal operator controls that can safely retry or requeue embedding rebuild work without bypassing the existing durable worker and scheduler ownership rules.
- Replace the empty runtime provider stub with config-driven provider registration that is available consistently across `api`, `worker`, and `scheduler` boot paths.
- Make degraded provider configuration explicit through startup validation, admin diagnostics, and self-host smoke-check guidance.
- Preserve the current asynchronous rebuild execution path and append-only vector audit model.

**Non-Goals:**
- Do not add foreground embedding generation to ingest or admin mutation writes.
- Do not add public memory API changes or end-user provider selection.
- Do not remove the transitional `canonical_memories.embedding` mirror in this change.
- Do not add bulk rebuild rotation workflows, fleet-wide cutover orchestration, or arbitrary operator mutation of vector revision records.
- Do not require a specific third-party provider vendor; the runtime contract stays internal and provider-agnostic.

## Decisions

### Decision: Reuse the existing admin route family with embedding-specific resources

Embedding diagnostics and operator controls will live under `/v1/admin/...`, alongside job status, governance raw event recovery, and memory history inspection. The new resources should follow the same operator pattern already present in the repository:

- resource-oriented GET routes for inspection
- narrowly scoped POST action routes for recovery or remediation
- admin API key auth plus required `tenant`, `project`, and `namespace` scope headers

This keeps embedding operations aligned with existing governance admin semantics instead of creating a parallel debug-only surface.

Alternatives considered:
- Put embedding inspection on public memory routes. Rejected because vector lineage and backlog state are operational concerns, not ordinary product reads.
- Expose only aggregate job status and skip per-memory diagnostics. Rejected because rebuild drift and provider misconfiguration often require memory-level investigation.

### Decision: Model operator controls as rebuild-state transitions, not direct vector mutation

Operator actions will target `embedding_rebuilds` records and reuse the rebuild job's normal claim-and-process path. The admin surface may clear retryable failure state, move an eligible record back to `pending`, or refresh its requested target from the current router policy, but it will not directly insert or activate vector revisions.

This preserves the append-only revision contract and the compare-and-promote safety already enforced by the background job. It also mirrors the safety stance used by governance raw-event recovery: admin actions can restore eligibility, but they do not seize leased work or bypass worker invariants.

Alternatives considered:
- Allow operators to force a vector revision active immediately. Rejected because it would bypass source-version validation and weaken auditability.
- Allow operators to delete failed revisions or mutate revision history. Rejected because audit history must stay append-only.

### Decision: Add a dedicated runtime provider registry builder with explicit degraded-state behavior

The runtime will introduce a small registry builder that turns configuration into a concrete `embedding.ProviderResolver`. Boot paths for `api`, `worker`, and `scheduler` will all use the same builder so semantic rebuild behavior is consistent regardless of process mode.

The builder should support three outcomes:

- valid configured providers are registered and usable
- no provider is configured, in which case the service still starts but surfaces degraded semantic rebuild state explicitly
- invalid provider configuration fails fast at startup when the configuration claims a provider route that cannot be constructed or resolved

This choice keeps self-hosting practical while avoiding silent partial configuration. A deployment with lexical-only operation remains valid, but once an operator configures an embedding route, the corresponding provider wiring must be honest and testable.

Alternatives considered:
- Hard fail startup whenever no provider exists. Rejected because the current architecture intentionally permits degraded semantic retrieval and some deployments may choose lexical-first operation.
- Allow each runtime mode to wire providers independently. Rejected because it would create inconsistent drift between scheduler, worker, and API processes.

### Decision: Keep provider implementations internal and configuration-driven

This change should not expose provider plugins as a public extension API. Instead, it will define internal registration points and configuration fields for the supported provider set, then document those fields in self-hosting guidance.

The immediate value is operability, not ecosystem extensibility. An internal contract keeps implementation flexible while still satisfying the AGENTS requirement to prefer mature libraries or SDKs when they reduce risk.

Alternatives considered:
- Introduce a generic external plugin ABI now. Rejected because it adds complexity before the first concrete provider runtime is stable.
- Hardcode a single vendor without configuration. Rejected because the recent lifecycle work already made provider and model routing durable concepts.

### Decision: Expose both memory-level and backlog-level embedding diagnostics

The admin surface should cover two operational views:

- backlog-oriented inspection for scope-level pending, rebuilding, failed, and drifted work
- memory-oriented inspection for one memory's active vector revision, revision history, rebuild record, and last failure context

Both views are required. Backlog visibility finds hot scopes and systemic provider drift; memory-level inspection supports precise remediation and audit review.

Alternatives considered:
- Only expose one-memory inspection. Rejected because it does not help operators detect systemic rebuild backlog.
- Only expose aggregate counts. Rejected because counts alone are not actionable during incident response.

## Risks / Trade-offs

- [Admin embedding endpoints could overexpose hidden memory internals] → Reuse the existing admin auth boundary and preserve explicit scope headers on every query and action; do not broaden public retrieval behavior.
- [Provider registration may become vendor-specific too early] → Keep the runtime contract internal, centered on `embedding.Provider`, and treat vendor SDK choice as an implementation detail.
- [Degraded lexical-only mode could still confuse operators] → Add explicit startup warnings, admin diagnostics, and self-host smoke checks that show whether semantic rebuilds are actionable or blocked by missing providers.
- [New operator actions could conflict with leased background work] → Apply the same lease-safety rule used by governance recovery: reject actions against actively leased rebuild records rather than seizing ownership.
- [Embedding inspection routes may duplicate existing job status reporting] → Keep `/v1/admin/jobs/...` for coarse execution health and reserve embedding routes for semantic lineage, drift, and rebuild-specific diagnostics.

## Migration Plan

1. Add the new admin route family and query contracts without changing existing public APIs.
2. Introduce the runtime provider registry builder and wire it into all runtime mode constructors.
3. Add self-host configuration and smoke-check documentation for deployments with and without semantic providers.
4. Roll out provider-backed deployments by configuring a supported provider and observing embedding backlog inspection before relying on semantic rebuild throughput.
5. If rollout reveals provider failures, operators can revert provider configuration and continue lexical-plus-relation retrieval while preserving rebuild history and failed attempt diagnostics.

## Open Questions

- Which concrete provider should be implemented first in-tree: OpenAI-compatible embeddings, a local self-hosted model adapter, or both?
- Whether embedding admin inspection should expose raw vector payload dimensions only, or also include compact hash-style fingerprints for easier drift debugging without surfacing payload content.
- Whether operator remediation should include a “refresh target from current routing policy” action in v1, or only `retry` and `requeue`.

## References

- Related proposal: [proposal.md](/D:/code/stele/openspec/changes/embedding-operator-controls-and-provider-runtime/proposal.md:1)
- Related archived design: [design.md](/D:/code/stele/openspec/changes/archive/2026-06-19-embedding-lifecycle-and-vector-governance/design.md:1)
- Related commands: `openspec status --change "embedding-operator-controls-and-provider-runtime"`, `/opsx:apply`
