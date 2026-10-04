# Key components

This page maps the public architecture to the repository's implementation
areas. The linked Go packages and contracts are authoritative when this guide
and an implementation differ.

## API runtime

- `internal/app`: HTTP wiring, dependency composition, health/readiness, admin
  routes, provider routes, and runtime mode startup.
- `openapi`: OpenAPI-first schemas and route publication.
- `internal/auth`: API-key/principal authentication and role checks.
- `internal/provider`: runtime binding, provider capabilities, governed adapter,
  and synchronization cursor contract.

The API resolves identity and exact scope before repository access. It should
not contain a second persistence or authorization implementation for an adapter.

## Storage and memory governance

- `internal/storage/postgres`: PostgreSQL repositories, migrations, projections,
  append-only history, leases, and scope predicates.
- `internal/memory`: memory classes, canonical versions, intents, sessions,
  lifecycle inputs, validation, and service-level contracts.
- `internal/policy`: forgetting and retention policy decisions.
- `internal/governance`: raw-event governance, claims, recovery, and audit.

Storage methods must apply exact `tenant/project/namespace` predicates and return
safe not-found or denied outcomes without confirming foreign records.

## Retrieval and context

- `internal/retrieval`: lexical, semantic, hybrid, fusion, reranking,
  context assembly, citations, temporal constraints, and bounded diagnostics.
- PostgreSQL full-text search supplies the lexical path.
- pgvector supplies semantic retrieval when configured.

Retrieval output is evidence for an agent response. It is not canonical memory
and must not be written back unchanged as if it were a new fact.

## Worker and scheduler

- `internal/jobs`: worker loops, scheduler dispatch, leases, retry/backoff,
  maintenance jobs, and durable run history.
- `internal/workqueue`: durable derived-work queue and checkpoint contracts.

Workers own asynchronous consolidation and recovery. Schedulers decide when
bounded work is dispatched; they do not directly mutate API authorization state.

## MCP adapter

- `internal/mcp/schema.go`: tool names, request shapes, validation limits, and
  tool descriptors.
- `internal/mcp/adapter.go`: Streamable HTTP MCP server, request scope
  resolution, read tools, governed remember, and lifecycle flows.
- `internal/mcp/contract_test.go`: protocol-level behavior and redaction
  coverage.

The current tools are documented in the [MCP integration guide](integrations/mcp-agent-skill.md).
MCP remains optional and disabled by default; OpenAPI remains the primary
service boundary.

## Provider adapter and synchronization

The provider adapter composes existing event, intent, retrieval, context,
lifecycle, and status services. The synchronization contract adds bounded
initial snapshots, ordered replay, cursor acknowledgment, and explicit
`resync_required` recovery. It does not create a client-owned memory store.

See [agent runtime provider](agent-runtime-memory-provider.md) and the
transport-neutral synchronization contract in
`openspec/specs/runtime-capability-and-event-sync-contract/spec.md`.

## Telemetry and assurance

- `internal/telemetry`: low-cardinality metrics and bounded lifecycle events.
- `internal/assurance`: conformance fixtures, health/readiness evidence,
  operational proofs, and recovery verification.

Telemetry must not contain raw queries, payloads, scopes, credentials, cursors,
or unbounded identifiers. Assurance evidence is separate from retrieval quality
metrics.
