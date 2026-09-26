## Why

Stele already has a scope-safe OpenAPI provider contract, governed memory intents,
versioned context projections, temporal/lifecycle filtering, and bounded context
assembly, but it has no optional MCP surface that makes those contracts usable by
MCP-compatible agents. A thin adapter is the next roadmap step after RQ1–RQ4,
provided it preserves the existing authorization and persistence boundaries rather
than introducing a second memory system.

## What Changes

- Add an optional OpenAPI-backed MCP adapter with small, explicit tool contracts
  for server-resolved identity/scope, memory search, assembled context/profile,
  governed remember/update/forget intents, and lifecycle-safe memory browsing.
- Resolve an explicit authorized scope before falling back to a server-owned
  runtime/session active scope; never allow an active scope or client field to
  widen tenant, project, or namespace grants.
- Map search and context tools to existing public service contracts, preserving
  temporal selectors, lifecycle visibility, context budgets, citations, and
  redacted ordinary responses.
- Map mutable tools to governed memory intents and lifecycle APIs. Semantic bulk
  forgetting SHALL use a preview followed by execution of the caller-reviewed,
  exact bounded memory-ID set with audit attribution.
- Publish bounded MCP schemas, machine-readable errors, read/write annotations,
  and conformance tests for scope isolation, read-only grants, idempotency,
  lifecycle safety, citation completeness, and rollback/disable behavior.
- Keep the adapter independently disableable and outside the canonical PostgreSQL
  persistence path; no MCP tool may write canonical memory directly.

## Non-goals

- No TypeScript SDK, UI/widget surface, OAuth provider, hosted service, connector,
  local binary, or graph visualization is added to the Stele service repository.
- No MCP-specific memory storage/cache, profile database, graph database, or
  alternate memory source of record is introduced. A minimal PostgreSQL
  operational ledger may retain preview manifests and idempotency outcomes only;
  canonical memory remains exclusively owned by existing repositories.
- No replacement of Stele's explicit `tenant`/`project`/`namespace` model with a
  single client-controlled container tag or unrestricted active-space state.
- No autonomous reasoning, new memory classes, arbitrary profile buckets, or
  direct semantic-text deletion fallback is introduced by this change.

## Capabilities

### New Capabilities

- `mcp-scope-profile-context`: Optional OpenAPI-backed MCP contracts for
  server-resolved scope and identity, search, assembled profile/context,
  governed memory intents, lifecycle-safe browsing, bounded errors, and adapter
  conformance.

### Modified Capabilities

None. The adapter composes the existing provider, scope, search, context,
projection, lifecycle, and mutation-governance contracts without changing their
requirements.

## Impact

- Affected runtime: API mode exposes an optional MCP transport/adapter boundary;
  worker and scheduler behavior remain unchanged.
- Affected APIs: additive MCP tool schemas and adapter-facing documentation;
  existing OpenAPI routes remain authoritative.
- Affected code: new adapter package, request/scope resolution, response
  redaction, governed-intent mapping, and conformance/e2e tests.
- Affected storage: PostgreSQL receives a bounded operational ledger for
  expiring forget-preview manifests and batch idempotency claims/outcomes; it
  stores no canonical memory content.
- Related contracts: [agent-runtime-provider-adapter](../../specs/agent-runtime-provider-adapter/spec.md), [scoped-api-access](../../specs/scoped-api-access/spec.md), [scoped-principal-access](../../specs/scoped-principal-access/spec.md), [context-assembly](../../specs/context-assembly/spec.md), [memory-search-contract](../../specs/memory-search-contract/spec.md), [versioned-context-projections](../../specs/versioned-context-projections/spec.md), [manual-memory-lifecycle-actions](../../specs/manual-memory-lifecycle-actions/spec.md), and [manual-mutation-governance-controls](../../specs/manual-mutation-governance-controls/spec.md).
- Workflow references: use `openspec status --change "openapi-backed-mcp-scope-and-profile-context" --json` during planning and `openspec validate --all --strict` before implementation or archive.
