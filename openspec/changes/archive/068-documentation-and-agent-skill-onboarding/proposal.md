## Why

Stele has mature provider, memory-governance, retrieval, self-hosting, and MCP
contracts, but the public entry point still makes the service harder to adopt
than necessary. Architecture decisions are distributed across long roadmap and
self-hosting documents, while an agent has no repository-provided skill that
explains how to select MCP tools, establish exact scope, write governed memory,
or perform safe forgetting. This change turns the existing contracts into a
coherent open-source onboarding path without changing service behavior.

## What Changes

- Add a documentation map and public product overview to `README.md`.
- Add an architecture guide covering API, worker, scheduler, PostgreSQL,
  pgvector, memory lifecycle, retrieval, governance, provider, and MCP layers.
- Add a key-components guide describing responsibilities, boundaries, data flow,
  operational dependencies, and extension points.
- Add a best-practices guide covering exact scope, lifecycle-safe writes,
  retrieval/context usage, idempotency, citations, redaction, observability,
  deployment, backups, and recovery.
- Add an MCP integration guide with the current tool matrix, authentication and
  runtime-binding setup, request examples, error recovery, and safe read/write
  workflows.
- Add a repository-provided `skills/stele-memory/SKILL.md` that an agent can
  copy or install to use the existing MCP methods safely.
- Define the skill workflow around `who_am_i`, `memory_search`,
  `memory_context`, `memory_browse`, `memory_remember`, `memory_forget_preview`,
  `memory_forget_apply`, and `memory_forget`.
- Add documentation checks for README links, MCP tool-name drift, required
  safety rules, and examples that remain within documented limits.
- Include a short Docker-to-first-MCP-call path in the README and link deeper
  material instead of duplicating the full self-hosting manual.

### Non-goals

- No new MCP tools, request fields, service routes, storage tables, or lifecycle
  behavior.
- No SDK, UI, hosted product, or generated client package.
- No second memory store, agent runtime, prompt orchestration, or model call.
- No automatic synchronization of copies into every IDE's private skill format;
  the repository skill is the canonical source.
- No change to OpenAPI-first authorization, PostgreSQL system-of-record policy,
  or tenant/project/namespace isolation.

## Capabilities

### New Capabilities

None. This is a documentation and onboarding change; it does not introduce a
new service contract.

### Modified Capabilities

None. Existing MCP and provider requirements remain unchanged. The repository
skill documents those requirements rather than changing them.

This change opts out of delta specs through `skip_specs: true`.

## Impact

- Documentation: `README.md`, new architecture/components/best-practices/MCP
  guides, and the repository Agent Skill.
- Verification: documentation consistency checks plus existing MCP package
  tests and OpenSpec validation.
- Contributor workflow: future MCP changes must update the tool matrix and skill
  guidance in the same change.
- Runtime code, database schema, public routes, dependencies, and deployment
  behavior remain unchanged.

Related workflow references: `openspec propose`, `openspec apply`,
`openspec validate --all --strict`, and the repository's existing MCP
conformance commands documented in `docs/self-hosting.md`.
