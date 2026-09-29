## Why

Agents need a consistent way to find durable information about their own stated preferences, known capabilities, limitations, and operational lessons across sessions. Without a convention, callers may invent incompatible path layouts or mistake remembered descriptions for authoritative runtime controls. The recently added scoped memory paths make it possible to standardize this organization without adding a new memory class, store, authorization boundary, or agent registry.

## What Changes

- Define an optional `agents/{agent-id}/self/{category}` path convention for self-model memories inside an already authorized tenant/project/namespace.
- Map descriptive self-model facts to existing `profile` memories and operational procedures/lessons to existing `procedural` memories; preserve ordinary provenance, versioning, admission, lifecycle, forgetting, and audit behavior.
- Specify that remembered capabilities and limits are advisory knowledge only. Server-published capabilities, grants, configuration, and runtime enforcement remain authoritative.
- Document how callers can selectively retrieve self-model memories using existing exact-path, prefix, and context contracts; do not add a new endpoint, MCP tool, context section, or default always-on injection behavior.
- Update the roadmap's immediate-next-step bookkeeping to mark P8.2 complete and identify this proposal as the next candidate.

## Capabilities

### New Capabilities

- `scoped-agent-self-model-conventions`: conventions for representing and retrieving agent self-model information as ordinary scoped, governed memories.

### Modified Capabilities

None. This proposal does not change authorization, memory lifecycle, retrieval, context assembly, provider capability discovery, or MCP tool behavior.

## Impact

- Documentation and OpenSpec requirements for self-model path/category conventions.
- Roadmap status and next-step bookkeeping.
- Tests/examples validating category-to-memory-class mapping, exact scope and path use, and the non-authoritative status of remembered capabilities/limits.
- No schema migration, public API change, new dependency, new runtime mode, or canonical storage model.

## Non-goals

- Creating an agent registry, a new scope dimension, a special self-model database, or new canonical memory classes.
- Using an agent ID or memory path as an authorization grant or as a substitute for tenant/project/namespace scope checks.
- Treating remembered capabilities, limits, or preferences as runtime configuration, permission, or a security enforcement input.
- Automatically injecting self-model memories into every retrieval or context response.
- Adding MCP-specific tools or semantics, autonomous self-reflection, or automatic mutation of canonical memory.

## References

- Roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md` (P8 candidate follow-ups and immediate next step).
- Existing path contract: `openspec/specs/scoped-memory-path-conventions/spec.md`.
- Existing governed memory and adapter contracts: `openspec/specs/memory-management-surface/spec.md`, `openspec/specs/mcp-scope-profile-context/spec.md`, and `openspec/specs/runtime-api-contract-publication/spec.md`.
- Workflow: `openspec status --change scoped-agent-self-model-conventions` and `openspec validate --all --strict`.
