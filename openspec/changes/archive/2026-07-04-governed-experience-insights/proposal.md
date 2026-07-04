## Why

Stele already preserves governed memory, provenance, recovery history, and operational failure state, but it does not yet turn repeated experience into explicit, reusable lessons for future agent context. The Stash reference shows value in experience-derived concepts, but Stele needs a narrower, evidence-backed version that preserves OpenAPI-first contracts, PostgreSQL-first persistence, scope isolation, and audit history.

## What Changes

- Add a governed derived insight layer for experience-backed operational and agent lessons.
- Introduce `failure_pattern` as the first active insight type, derived from repeated evidence rather than free-form inference.
- Allow `lesson` as an evidence-backed output associated with a failure pattern.
- Persist derived insights with scope, lifecycle state, confidence, evidence citations, derivation metadata, provenance, and audit history.
- Add asynchronous derivation through the existing worker or scheduler model instead of synchronous ingest, mutation, or retrieval paths.
- Add admin inspection for derived insight provenance, lifecycle state, and evidence context.
- Add optional context assembly sections for `known_failures` and `experience_lessons`.
- Reserve vocabulary for `hypothesis`, `goal`, `contradiction`, and `causal_link` without implementing autonomous inference for them in this change.

## Capabilities

### New Capabilities

- `governed-experience-insights`: Defines derived insight records, lifecycle, evidence requirements, failure-pattern derivation, and lesson surfacing.

### Modified Capabilities

- `context-assembly`: Adds optional sections for evidence-backed failure patterns and experience lessons.
- `admin-inspection-surface`: Adds admin-only inspection of derived insights and their evidence/provenance.
- `worker-orchestration-and-maintenance-jobs`: Adds asynchronous derived insight derivation without moving work into foreground request paths.

## Impact

- API: Adds admin inspection endpoints for derived insights and extends context assembly response shape with optional insight sections.
- Storage: Adds PostgreSQL tables or projections for derived insights, evidence links, versions or lifecycle transitions, and derivation metadata.
- Worker/scheduler: Adds a scoped derivation job that scans eligible evidence and upserts derived insight candidates or active insights idempotently.
- Retrieval/context: Context assembly can include `known_failures` and `experience_lessons` when requested and budget allows.
- Docs: Updates self-hosting and operator docs with derived insight behavior, evidence requirements, and non-goals.
- Dependencies: No new graph store, MCP server, SDK, UI, or reasoning provider dependency is required.

## Non-goals

- Implementing MCP tools or changing Stele from OpenAPI-first to MCP-first.
- Implementing autonomous `hypothesis`, `goal`, `contradiction`, or `causal_link` inference.
- Adding a reasoning-provider abstraction in this change.
- Overwriting canonical memory, memory versions, vector revisions, or provenance in place.
- Creating a global agent-self namespace that bypasses `tenant`, `project`, and `namespace` isolation.
- Introducing SDK, UI, hosted-product, or end-user product logic.

## Artifact References

- Proposal workflow: `.codex/skills/openspec-propose/SKILL.md`
- Exploration workflow: `.codex/skills/openspec-explore/SKILL.md`
- Stash reference backlog: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`
