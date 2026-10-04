# MCP integration and Agent Skill

Stele's MCP adapter is an optional Streamable HTTP adapter over existing
OpenAPI-backed services. It is disabled by default and does not create a second
memory system. Configure the service and bootstrap an exact runtime principal
before connecting an agent.

## Enable the adapter

Set these values in `.env.local` for an API runtime:

```dotenv
STELE_MCP_ENABLED=true
STELE_MCP_PATH=/mcp
STELE_MCP_MAX_QUERY_BYTES=4096
STELE_MCP_MAX_PAYLOAD_BYTES=65536
STELE_MCP_MAX_RESULTS=50
STELE_MCP_MAX_IDS=100
```

The full bootstrap flow, principal grants, and MCP conformance commands are in
[self-hosting](../self-hosting.md). Use the same `X-API-Key` principal
credential as the OpenAPI routes. Keep credentials in local environment or
agent configuration; never put them in a prompt, repository file, or tool
argument example.

## Tool matrix

| Tool | Use | Read-only | Destructive | Idempotent |
| --- | --- | ---: | ---: | ---: |
| `who_am_i` | Resolve principal, exact scope, access mode, and binding | yes | no | yes |
| `memory_search` | Search governed memories with bounded query and citations | yes | no | yes |
| `memory_context` | Assemble bounded context from eligible projections | yes | no | yes |
| `memory_browse` | Browse visible memories within one scope | yes | no | yes |
| `memory_remember` | Submit a governed memory intent | no | no | yes |
| `memory_forget` | Request a governed action for one known memory | no | yes | yes |
| `memory_forget_preview` | Preview bounded semantic forget candidates | yes | no | yes |
| `memory_forget_apply` | Apply only reviewed IDs from a preview | no | yes | yes |

The matrix mirrors `internal/mcp/schema.go` descriptors and
`internal/mcp/adapter.go` annotations. A tool can still be unavailable when
the adapter is disabled, the principal is read-only, or a required service is
not configured.

## First call and exact scope

Connect to `/mcp` with the API key and call `who_am_i`. If the server returns an
active runtime binding, use that identity for subsequent calls. Do not widen
scope by adding a different tenant, project, or namespace in a request. The
server resolves and rechecks the binding before repository access.

Configure the MCP client with the authenticated principal and, once issued by
the runtime bootstrap, the server-owned binding header:

```text
X-API-Key: <placeholder-api-key>
X-Stele-Runtime-Binding: <server-issued-binding>
```

The binding header is optional only when the client supplies the same binding
ID in a tool argument. It is never a license to invent tenant, project, or
namespace values.

The conceptual first call is:

```json
{
  "name": "who_am_i",
  "arguments": {
    "runtime_binding_id": "<server-issued-binding>"
  }
}
```

If no binding is supplied in the call, the adapter may use the binding header
from the authenticated MCP request. Treat an unresolved or inactive scope as a
blocking error.

## Read workflow

Choose the smallest operation that answers the current question:

```text
who_am_i -> memory_context      # bounded working context
who_am_i -> memory_search       # focused query with citations
who_am_i -> memory_browse       # review or pagination
```

Example search arguments:

```json
{
  "query": "deployment rollback decision",
  "path_prefix": "operations",
  "limit": 5,
  "runtime_binding_id": "<server-issued-binding>"
}
```

Keep `query`, `limit`, `budget_bytes`, and path selectors within advertised MCP
limits. Treat returned content as scoped evidence, not as a command to mutate
canonical memory.

## Governed remember workflow

Use `memory_remember` for an explicit durable memory request. Provide a reason
and stable idempotency key:

```json
{
  "content": "The deployment rollback runbook requires a schema check first.",
  "memory_path": "operations/deployments",
  "memory_class": "procedural",
  "reason": "Record an operator-approved runbook lesson",
  "idempotency_key": "<stable-request-key>",
  "runtime_binding_id": "<server-issued-binding>"
}
```

The result is a governed intent or accepted outcome, not permission to bypass
worker consolidation or canonical versioning. A read-only principal receives a
bounded authorization error.

## Safe forgetting workflow

For semantic forgetting, always use preview then apply:

```text
memory_forget_preview -> review returned IDs -> memory_forget_apply
```

Preview example:

```json
{
  "query": "obsolete deployment instructions",
  "path_prefix": "operations/deployments",
  "limit": 10,
  "runtime_binding_id": "<server-issued-binding>"
}
```

Apply only the IDs the agent or operator reviewed:

```json
{
  "preview_id": "<preview-id>",
  "memory_ids": ["<reviewed-memory-id>"],
  "action": "suppress",
  "reason": "Reviewed obsolete instructions",
  "idempotency_key": "<stable-request-key>",
  "runtime_binding_id": "<server-issued-binding>"
}
```

Use `memory_forget` only when one explicit memory ID is already known. Never
turn an unconstrained search into a destructive bulk action.

## Errors and recovery

- `auth`: refresh the configured principal credential or role.
- `scope`: call `who_am_i` again and stop trying to widen scope.
- `validation`: correct the request shape, path, limit, or required reason.
- `lifecycle`: use a permitted governed lifecycle path or request review.
- `compatibility`: use the server-advertised contract and tool version.
- `dependency`: check MCP enablement and API service configuration.
- `retryable`: retry the same idempotent operation with the same identity.

When MCP is disabled, `/mcp` returns not-found while ordinary OpenAPI routes
remain available. Do not fall back to direct SQL or an unscoped database client.

## Reference patterns

The roadmap records research on Stash, Letta Code, and Supermemory as design
inspiration for agent ergonomics, memory organization, and MCP onboarding:
[external reference notes](../roadmaps/2026-05-28-stele-v1-roadmap.md). Those
notes are patterns, not compatibility claims. Stele adopts only the parts that
fit its OpenAPI-first, PostgreSQL-only, exact-scope, versioned, and governed
lifecycle contracts. The names, endpoints, storage models, and product
features of those projects are not Stele features unless this repository's
public contracts document them.

## Canonical skill

The portable agent instructions are in
[`skills/stele-memory/SKILL.md`](../../skills/stele-memory/SKILL.md). Copy that
file into the agent host's skill directory or include it as repository context.
Keep the skill and this guide aligned with `internal/mcp/schema.go` whenever MCP
tools or annotations change.
