## Context

The repository already contains the authoritative service behavior in Go,
OpenAPI, `internal/mcp`, `docs/self-hosting.md`, and the archived provider and
scope contracts. The public README is intentionally short, but it currently
does not provide a map into those documents or a first-use MCP path. The MCP
adapter already exposes a stable tool set, annotations, exact-scope resolver,
bounded inputs, governed intents, and two-step forget operations. This design
turns those existing facts into a maintainable documentation and agent-onboarding
surface.

## Goals / Non-Goals

**Goals:**

- Make README the short entry point for operators, contributors, and agents.
- Make architecture and component boundaries understandable without reading Go
  internals.
- Give an agent one canonical skill that teaches safe use of the current MCP
  tools and links to deeper operational guidance.
- Keep examples exact, bounded, scope-aware, and consistent with the current
  MCP schemas and annotations.
- Add checks that catch broken links, stale tool names, missing safety rules, and
  invalid example assumptions.

**Non-Goals:**

- Do not duplicate the entire self-hosting manual in README or the skill.
- Do not generate independent copies for each agent host or IDE.
- Do not alter MCP dispatch, OpenAPI contracts, memory lifecycle, or retrieval.

## Decisions

### 1. Use a layered documentation structure

README remains a landing page with one quick-start path and a documentation
table. Detailed material is split into focused documents:

- `docs/architecture.md`: system boundaries, runtime modes, request/data flow,
  PostgreSQL as system of record, pgvector and full-text retrieval, lifecycle,
  and isolation.
- `docs/components.md`: responsibilities and interfaces of API, worker,
  scheduler, storage, retrieval, governance, provider, MCP, telemetry, and
  assurance.
- `docs/best-practices.md`: operational and agent-facing rules, including
  scope-first access, append-only memory behavior, idempotency, redaction,
  citations, retries, backups, and recovery.
- `docs/integrations/mcp-agent-skill.md`: MCP setup, tool matrix, examples,
  error handling, and how the canonical skill maps to the service.

The existing `docs/self-hosting.md` remains the detailed deployment and
verification reference. New documents link to it instead of copying long
configuration sections.

### 2. Make `skills/stele-memory/SKILL.md` the canonical agent skill

The skill is plain Markdown with front matter and no runtime dependency. It is
portable across Codex, Claude, Cursor, Gemini, OpenCode, and other agent hosts.
It contains:

- prerequisites and MCP endpoint/authentication setup;
- a tool-selection table with read-only, idempotent, destructive, and
  preview/apply annotations;
- an exact-scope initialization flow using `who_am_i`;
- read workflows for context, search, and browse;
- governed write workflow using `memory_remember`;
- single-memory and reviewed bulk forget workflows;
- response citation, hidden-state, and scope-isolation handling;
- bounded error/retry rules and a compact troubleshooting section;
- explicit prohibitions against direct database writes, invented scopes, and
  treating retrieval output as canonical memory.

The skill links to `docs/integrations/mcp-agent-skill.md` for operator details.
The integration guide remains explanatory; the skill remains executable
instructional content with short examples.

### 3. Derive the tool matrix from current MCP declarations

The implementation must inspect `internal/mcp/schema.go` and the adapter
annotations as the source of truth. Documentation checks will maintain an
allowlist of expected tool names and required safety properties. New MCP tools
must update the matrix and skill in the same change, but this proposal does not
add a tool registry or code generation dependency.

### 4. Use bounded examples and canonical workflows

Examples use the existing MCP JSON shapes and limits. The primary flow is:

```text
who_am_i -> memory_context/search/browse -> memory_remember
who_am_i -> memory_forget_preview -> memory_forget_apply
```

Examples show placeholders for credentials and scope rather than real secrets
or identifiers. Forget examples emphasize preview review and idempotency. The
documentation must distinguish the active runtime binding from caller-supplied
scope fields and explain that the server enforces exact isolation.

### 5. Add documentation-focused verification without a new dependency

Use existing Go tests and PowerShell checks where available. Add a small
repository-owned checker or docs test that validates:

- README and documentation links resolve to tracked files or documented URLs;
- all current MCP tool names appear in the tool matrix and skill;
- required safety phrases/rules are present;
- examples mention authentication, exact scope, and idempotency where writes
  occur;
- no example contains a credential-looking literal or direct SQL write.

The checker should fail with file and line context, and should not require a
network connection. Existing `go test ./docs`, MCP package tests, and
`openspec validate --all --strict` remain part of the verification path.

### 6. Choose curated Markdown over generated API documentation

Generated OpenAPI pages are useful for exact request/response details but are
not sufficient for architecture, operational boundaries, or agent behavior.
Curated Markdown is selected over a documentation site or a large generator so
the repository remains self-host friendly, reviewable offline, and free of a
new build tool. OpenAPI and MCP source references are linked from the curated
pages to keep contract details discoverable.

## Risks / Trade-offs

- **[Risk]** MCP tool names or request fields can drift from the skill. → **Mitigation:** add a static tool-name and required-rule checker backed by
  `internal/mcp` declarations and run MCP tests in the documentation gate.
- **[Risk]** README becomes a second, incomplete self-hosting manual. → **Mitigation:** keep it as a map plus one shortest path and link detailed
  deployment material to `docs/self-hosting.md`.
- **[Risk]** Portable skill guidance becomes too generic to be useful. →
  **Mitigation:** include concrete current tool names, minimal JSON examples,
  annotations, exact-scope rules, and recovery decisions.
- **[Risk]** Reference projects encourage unsupported product claims. →
  **Mitigation:** document only patterns that match Stele's OpenAPI-first,
  PostgreSQL-only, append-only, governed architecture and label references as
  inspiration rather than compatibility guarantees.
- **[Risk]** Documentation checks reject legitimate external links or examples.
  → **Mitigation:** validate local links and fixed safety invariants offline;
  treat external links as allowlisted references without network fetching.

## Migration Plan

1. Add the focused documentation pages and canonical skill.
2. Rewrite README sections to point to those pages and the MCP first-use path.
3. Add the offline documentation/tool-matrix checker and run existing MCP and
   docs tests.
4. Review all examples against current schemas, limits, and annotations.
5. Roll back by removing the new pages, skill, checker, and README links; no
   database migration or runtime rollback is required.
