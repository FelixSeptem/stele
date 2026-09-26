## Context

See `proposal.md` for the motivation and scope. Stele already exposes the
authoritative OpenAPI provider contract, exact principal grants, governed memory
intents, temporal/lifecycle-safe search, context assembly, and versioned context
projections. The new surface must compose those contracts without adding a second
source of record or weakening their safety guarantees.

## Goals / Non-Goals

**Goals:**

- Provide an optional MCP transport with small, schema-validated tools that map
  to existing service interfaces.
- Resolve principal, grants, exact scope, agent/session attribution, and active
  scope on the server before dispatching a tool.
- Preserve the existing search, context, projection, temporal, citation, budget,
  lifecycle, intent, idempotency, and audit semantics.
- Make ordinary agent responses compact and safe while keeping richer diagnostics
  available only through existing authorized surfaces.
- Prove the adapter can be disabled without changing OpenAPI behavior or durable
  PostgreSQL state.

**Non-Goals:**

- A new memory engine, storage layer, MCP-specific cache, SDK, UI, OAuth system,
  connector, or graph database.
- Client-controlled scope expansion or a second mutable profile store.
- Autonomous reasoning, arbitrary profile buckets, or direct canonical mutation.

## Decisions

### 1. Keep OpenAPI as the authority and add an adapter boundary

The MCP server will be an API-mode adapter that invokes existing service
interfaces rather than implementing retrieval or mutation logic a second time.
The adapter will be guarded by an explicit configuration switch and will expose a
bounded MCP protocol endpoint only when enabled.

Alternative considered: implement MCP as a new primary service boundary. Rejected
because it would duplicate authentication, scope resolution, lifecycle filtering,
and OpenAPI contracts, creating drift and a second authorization path.

#### SDK and transport selection

The adapter will use the official Go SDK `github.com/modelcontextprotocol/go-sdk`
at `v1.8.0`. The repository already targets Go `1.25.0`, which matches the
SDK's compatibility envelope. The SDK provides typed tool registration,
structured input/output schemas, tool annotations, and a standard
`NewStreamableHTTPHandler` that can be mounted in the existing `net/http` API
runtime. The adapter will keep the handler behind Stele's existing HTTP
dependencies and principal middleware; the SDK is only a protocol/transport
implementation and is not an authentication or persistence boundary.

`github.com/mark3labs/mcp-go` was reviewed as an alternative (latest available
release during review: `v1.1.1`) but is not selected for this slice. The official
SDK has the canonical protocol ownership, the required streamable HTTP transport,
and typed structured tool support without requiring a second server framework.
The transport remains replaceable at the adapter boundary, so a future stdio
bridge would reuse the same normalized dispatch and scope contracts rather than
introduce another implementation path.

### 2. Resolve scope once, then dispatch against normalized context

Each request first authenticates the durable principal, resolves an explicit exact
grant or a server-owned runtime/session binding, and attaches the normalized scope
and attribution to a request-local dispatch context. Explicit authorized scope
takes precedence over active scope. No tool handler reads raw scope fields after
resolution.

Alternative considered: use a free-form `container_tag` as the MCP scope. Rejected
because it cannot represent Stele's tenant/project/namespace grants and would make
cross-scope mistakes easy to hide behind a friendly tool API.

### 3. Use a small tool taxonomy

The first adapter surface will contain separate operations for:

- identity/access (`who_am_i`);
- query-relevant search;
- assembled context/profile;
- lifecycle-safe memory browsing;
- governed remember/update/forget;
- preview-bound semantic forgetting.

Search will not silently include profile/context, and context will call the same
structured assembler used by OpenAPI. Browsing will use the public memory resource
contract and lifecycle-safe defaults.

Alternative considered: expose one generic `memory` tool with a large action
union. Rejected because it makes destructive behavior and response shape harder
for agents to reason about and harder to annotate and test independently.

### 4. Reuse governed mutation and preview-to-fixed-ID forgetting

Write tools translate to governed memory intents or existing lifecycle APIs. A
semantic forget request first creates a bounded preview. Applying the preview
requires the exact reviewed IDs, scope validation, and an audit batch; the adapter
never reruns an unconstrained semantic deletion at apply time.

Alternative considered: allow the adapter to call a direct repository delete or
repeat the semantic query during apply. Rejected because it would bypass
concurrency, lifecycle, provenance, and operator review guarantees.

### 5. Keep ordinary responses redacted and diagnostics out-of-band

Tool result schemas expose stable citations, bounded lifecycle-safe metadata,
structured sections, and operation outcomes. Raw scores, candidate pools,
feedback history, calibration identities, hidden IDs, query text, scope values,
provider payloads, and trajectory details remain restricted to existing authorized
diagnostic/admin paths.

Alternative considered: return the full internal service response through MCP.
Rejected because MCP clients commonly pass tool output directly into model context
and would turn internal diagnostics into a public data-leak surface.

### 6. Persist only destructive-operation review metadata

The adapter uses the existing provider runtime/session binding and current service
repositories. Active-scope state is request/runtime state, not a new canonical
record. If no compatible binding exists, callers must provide an explicit
authorized scope. Implementation review found that process-local preview and
batch-idempotency maps cannot satisfy the stateless transport, multi-replica, and
durable retry requirements. Add a narrowly scoped PostgreSQL ledger containing
only preview scope/ID manifest/expiry and MCP batch idempotency fingerprint and
outcome; it stores no memory content and is not a memory system of record or
cache. Per-memory lifecycle mutations continue through the provider lifecycle
ledger and existing audit path.

### 7. Test the adapter as a contract, not as a separate engine

Tests will cover unit-level scope precedence, tool schemas, redaction, and error
mapping; integration-level delegation to existing OpenAPI services; and an
owned PostgreSQL conformance run for isolation, lifecycle, idempotency, citations,
preview-bound forgetting, budgets, and disablement. Fixtures will use exact scopes
and governed setup/cleanup, not external agents.

## Risks / Trade-offs

- **[Risk] MCP tool output is reused as model context without caller awareness** →
  Keep schemas compact, redact diagnostics, bound all text, and preserve citations.
- **[Risk] Active-scope convenience hides a scope mistake** → Require server-owned
  bindings, explicit-scope precedence, grant revalidation, and a dedicated
  `who_am_i` contract test.
- **[Risk] Adapter behavior drifts from OpenAPI behavior** → Delegate to service
  interfaces, add contract tests against the published OpenAPI document, and do
  not duplicate repositories or ranking logic.
- **[Risk] Semantic forgetting is destructive or changes between preview/apply** →
  require preview identity and exact reviewed IDs, enforce bounded sets, and record
  one audit batch.
- **[Risk] MCP adoption increases optional attack surface** → Disabled-by-default
  configuration, the same principal/grant middleware, bounded errors, and a
  disablement test keep the critical path unchanged.
- **[Trade-off] The first adapter exposes fewer convenience features than
  Supermemory** → This deliberately favors Stele's auditable scope/lifecycle
  contract; additional tools can be proposed only with independent evidence.

## Migration Plan

1. Add the MCP adapter behind a disabled-by-default configuration gate.
2. Implement tool schemas and dispatch through existing service interfaces.
3. Add scope, lifecycle, redaction, durable idempotency, and preview-bound forget
   tests, including the operational metadata ledger migration.
4. Run the adapter conformance suite against owned PostgreSQL + pgvector and
   record bounded evidence.
5. Enable MCP only for explicitly configured deployments; disabling it requires
   no schema rollback and leaves OpenAPI/provider behavior unchanged.

## Open Questions

- Whether the first deployment should expose streamable HTTP only or also a local
  stdio bridge; either choice remains an adapter detail and must preserve the same
  scope and redaction behavior.
