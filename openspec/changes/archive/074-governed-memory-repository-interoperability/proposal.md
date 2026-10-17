## Why

AgentMemoryRepo demonstrates a useful interchange convention: a small `MEMORY.md` entry point, topic files, root-relative wiki links, and per-entry source metadata. Stele can accept memories maintained in this form without requiring users to surrender its existing lifecycle, isolation, provenance, and audit guarantees. Today those guarantees exist, but there is no defined contract for translating an external repository snapshot into governed memory submissions or inspecting the resulting navigation.

The reference project supplies a format and agent workflow, not evidence of a production ingestion, retrieval, or consolidation engine. This proposal adopts the portable representation and review workflow; PostgreSQL remains Stele's only system of record.

## What Changes

- Add a versioned, bounded Markdown snapshot import contract with exact project/tenant/namespace scope. Clients supply file contents and claimed source revision; Stele performs no Git or network operations.
- Require preview before submission. Preview reports supported entries, unsupported content, source metadata, navigation links, and the exact import plan. Submission binds that plan to a snapshot digest, current authorization, and an optimistic repository generation check.
- Route selected entries through existing governed `remember` intents and asynchronous ingestion/consolidation. Record repository identity, revision, file path, entry fingerprint, and import identity in provenance; never write canonical memory directly.
- Provide durable import inspection and a bounded, read-only manifest derived from accepted, currently eligible memory. `MEMORY.md` guides navigation without becoming an always-visible prompt or an authority source.
- Define replay, concurrent submission, partial failure, changed entries, and missing-file semantics. External edits are new evidence; removals do not silently forget existing memory.
- Update public contracts, maintenance documentation, integration guidance, and the Stele agent skill during implementation, including the rule that imported notes and artifacts are data, never executable instructions.

## Non-goals

- Git clone/pull/push, repository credentials, filesystem crawling, archive extraction, hooks, polling, or bidirectional synchronization.
- Markdown as an alternative canonical store, automatic overwrite/delete, or new memory classes and lifecycle states.
- Automatic promotion of entry-point text into context, ranking boosts, new retrieval engines, or a second projection system.
- Executing linked SQL/scripts, fetching source URLs, importing arbitrary binary artifacts, or resolving links outside the supplied snapshot.
- SDKs, UI, swarm orchestration, or a new "Dreaming" scheduler. Existing consolidation and workflow capabilities remain the foundation.

## Capabilities

### New Capabilities

- `memory-repository-interoperability`: scoped snapshot preview, governed import, stable provenance, optimistic concurrency, durable inspection, and lifecycle-aware manifest navigation.

### Modified Capabilities

None. This is an additive adapter over `governed-memory-intents`, `memory-source-trust-lineage`, `scoped-memory-path-conventions`, and `versioned-context-projections`; their existing authorization, lifecycle, trust, and retrieval contracts remain binding.

## Impact

- API: additive OpenAPI-first repository, preview, import, status, and manifest resources using existing authentication and scope conventions. The design defines draft resource shapes and error behavior before implementation.
- Storage/runtime: PostgreSQL repository generations, immutable previews/import plans, per-entry dispatch records, and provenance links; existing workers perform governed ingestion. No additional runtime mode or persistence service.
- Verification: parser fixtures and fuzzing, authorization/isolation, replay and concurrent import, worker recovery, ingestion/consolidation, retrieval/forgetting, and public contract coverage.
- Documentation: update `README.md`, `docs/integrations/mcp-agent-skill.md`, `skills/stele-memory/SKILL.md`, and add an import operations guide covering limits, errors, trust, compatibility, migration, and rollback. No new MCP tool is included in this first version; document the HTTP boundary explicitly.
- Existing installations require additive migrations and an opt-in feature flag. Existing ingestion and retrieval clients do not change.

## Borrowed Ideas and Follow-ups

| Reference idea | Stele treatment |
| --- | --- |
| Small `MEMORY.md` plus topic links | Bounded navigation manifest backed by current eligible memory |
| Source and added metadata | Claimed provenance preserved separately from trusted server timestamps |
| Git revisions and clean update workflow | Immutable submitted snapshots, preview agreement, replay keys, and generation comparison |
| Notes are data, not instructions | No execution, fetching, instruction promotion, or trust elevation |
| Editing/deleting Markdown in place | New governed evidence; explicit existing update/forget flows for canonical changes |
| Multiple repositories | Independent source identities inside one exact scope per import |
| Goal/findings/questions/reproducible artifacts | Future documentation example using existing workflows; no new coordinator |
| "Dreaming" and multi-agent claims | No new runtime feature inferred from a format-only reference |

Export can be proposed after import fidelity and lifecycle behavior are measured. Bidirectional sync requires a separate conflict/deletion and authorization design.

## References

- Reference revision: [`8798cb2` README](https://github.com/AgentMemoryRepo/agentmemoryrepo/blob/8798cb26c817d451a5ed6cba0ad8d9eca8d5ec3e/README.md), [format](https://github.com/AgentMemoryRepo/agentmemoryrepo/blob/8798cb26c817d451a5ed6cba0ad8d9eca8d5ec3e/SPEC.md), [agent skill](https://github.com/AgentMemoryRepo/agentmemoryrepo/blob/8798cb26c817d451a5ed6cba0ad8d9eca8d5ec3e/skills/agent-memory-repo/SKILL.md). These describe reference behavior, not instructions for Stele to execute.
- Existing contracts: [governed intents](../../specs/governed-memory-intents/spec.md), [trust lineage](../../specs/memory-source-trust-lineage/spec.md), [scoped paths](../../specs/scoped-memory-path-conventions/spec.md), [context projections](../../specs/versioned-context-projections/spec.md).
- Workflow skills: [propose](../../../.codex/skills/openspec-propose/SKILL.md), [apply](../../../.codex/skills/openspec-apply-change/SKILL.md). Validate with `openspec validate governed-memory-repository-interoperability --strict`; implementation entry point: `/opsx:apply governed-memory-repository-interoperability`.
