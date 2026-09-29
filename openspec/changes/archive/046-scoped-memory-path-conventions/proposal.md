## Why

Stele now has exact tenant/project/namespace isolation and an optional MCP
adapter, but callers have no stable way to organize or retrieve related memory
within one authorized scope. A bounded path convention provides useful topic or
agent-local grouping without weakening the existing scope model or introducing
a second namespace authority.

## What Changes

- Add an optional normalized `memory_path` value to governed memory inputs and
  public memory representations where path-aware filtering is meaningful.
- Define bounded path validation: canonical slash-separated segments, explicit
  length and segment limits, no `..`, empty segments, wildcards, or encoded
  separators, and a stable representation for the root path.
- Add exact path filtering as the default for search, browse, context, and MCP
  operations when a path is supplied.
- Add an explicit `path_prefix` selector for recursive path retrieval; callers
  must opt in to descendant matching, and the selector remains inside the
  already-authorized tenant/project/namespace scope.
- Preserve backward compatibility: omitted paths continue to address the whole
  exact scope, and existing records are treated as root-path records.
- Add PostgreSQL migration/index support and propagate path filters through
  ingestion, retrieval, context assembly, public memory reads, OpenAPI, and MCP.
- Add conformance tests for exact matching, explicit prefix matching, root-path
  compatibility, malformed paths, pagination, hidden/lifecycle-safe records,
  and cross-scope isolation.

## Non-goals

- No path-based authorization, grant expansion, or subtree access across
  tenant/project/namespace boundaries.
- No client-controlled replacement for the existing exact scope model.
- No filesystem, graph, Git, or second persistence layer for path metadata.
- No implicit recursive retrieval, wildcard/glob syntax, or arbitrary query
  language for paths.
- No agent self-model schema, autonomous reasoning, or new memory class in this
  change.

## Capabilities

### New Capabilities

- `scoped-memory-path-conventions`: Defines normalized path metadata, exact and
  explicit prefix selectors, bounded validation, persistence, and isolation
  guarantees inside an existing exact scope.

### Modified Capabilities

- `event-ingestion`: Governed ingest accepts and persists an optional normalized
  memory path without changing scope authorization or idempotency boundaries.
- `memory-search-contract`: Search accepts exact `path` and explicit
  `path_prefix` selectors with bounded result semantics.
- `memory-management-surface`: Public memory reads and browse operations expose
  path metadata and apply the same exact/prefix filtering rules.
- `context-assembly`: Context assembly can restrict eligible evidence to an
  exact path or explicit path prefix while preserving budgets and lifecycle
  visibility.
- `mcp-scope-profile-context`: MCP search, context, browse, remember, and forget
  tools carry the same path contract without exposing path values outside the
  caller's requested operation or widening scope.

## Impact

- Affected domain and API models: memory event/intent/resource inputs, search,
  browse, context, OpenAPI schemas, and MCP schemas.
- Affected persistence: PostgreSQL canonical/event/query indexes and a versioned
  migration; existing rows receive the root-path compatibility value.
- Affected runtime: API, worker, scheduler, retrieval, context assembly, and
  MCP adapter must preserve the same normalized path semantics.
- Affected tests and docs: exact-scope/path isolation fixtures, migration tests,
  OpenAPI/MCP contract tests, self-hosting/API documentation, and release
  evidence.

