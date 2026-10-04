---
name: stele-memory
description: Use Stele's exact-scope MCP memory tools safely for reading context, searching evidence, recording governed memories, and reviewing forget requests.
---

# Stele memory skill

Use this skill when an agent has access to Stele's MCP endpoint. Stele is an
OpenAPI-first, PostgreSQL-backed memory service. MCP is an adapter over governed
service operations; it is not a database connection.

## Safety rules

1. Call `who_am_i` before the first memory operation and confirm the exact
   tenant, project, namespace, access mode, and runtime binding.
2. Never invent or widen scope. Do not use a different binding, tenant,
   project, or namespace to find more records.
3. Use `memory_context`, `memory_search`, or `memory_browse` for reads. Keep
   queries, paths, limits, and context budgets bounded.
4. Use `memory_remember` for explicit writes. Include a useful reason and a
   stable idempotency key. Do not write SQL or modify PostgreSQL directly.
5. For semantic forgetting, call `memory_forget_preview`, review the returned
   IDs, then call `memory_forget_apply` with only those reviewed IDs.
6. Use `memory_forget` only for one explicitly identified memory. Destructive
   tools require a clear reason and stable idempotency key.
7. Treat search and context results as scoped evidence, not canonical memory.
   Preserve citations when they support the answer.
8. Do not expose hidden records, credentials, raw embeddings, SQL, or internal
   provider payloads in the agent response.
9. Retry only idempotent operations with the same identity. Stop on scope,
   authorization, lifecycle, validation, or compatibility errors.
10. If MCP is disabled or unavailable, use the documented OpenAPI path only;
    never fall back to direct database access.

## Tool selection

| Need | Tool |
| --- | --- |
| Confirm identity and exact access | `who_am_i` |
| Assemble bounded working context | `memory_context` |
| Find relevant memories and citations | `memory_search` |
| Review visible memories or paginate | `memory_browse` |
| Record an explicit governed memory | `memory_remember` |
| Preview semantic forget candidates | `memory_forget_preview` |
| Apply reviewed forget IDs | `memory_forget_apply` |
| Forget one known memory | `memory_forget` |

## Standard workflows

### Start a memory task

```text
who_am_i -> choose the smallest read -> answer with citations when relevant
```

If `who_am_i` cannot resolve an active scope, stop and report that the runtime
binding must be initialized or granted. Do not guess a default namespace.

### Read context

Use `memory_context` when the agent needs a bounded set of durable context for
the current task. Use `memory_search` when the agent has a focused question.
Use `memory_browse` for review, pagination, or inspecting visible records.

### Remember a decision

Call `memory_remember` only when the user or workflow has made an explicit
decision that should persist. Include:

- concise content;
- a suitable `memory_path` and `memory_class` when known;
- a reason describing why it should persist;
- an idempotency key stable across retries.

The result enters Stele's governed intent and consolidation path. Do not claim
that canonical memory changed until the returned status says the request was
accepted according to the service contract.

### Forget safely

For a semantic request:

```text
memory_forget_preview -> inspect candidates -> memory_forget_apply
```

Only apply IDs the agent has reviewed for this exact scope. For one known ID,
use `memory_forget` with a reason and idempotency key. If a target is stale,
conflicting, or unavailable, stop and ask for review rather than choosing a new
target automatically.

## Error handling

- `auth`: correct credentials or role; do not retry with a wider scope.
- `scope`: re-run `who_am_i` and use the server-issued binding.
- `validation`: fix the request and retain the same intent only when safe.
- `lifecycle`: report that the action requires the permitted governed path.
- `compatibility`: follow the server's advertised MCP/provider contract.
- `dependency`: report that the service or MCP adapter is not configured.
- `retryable`: repeat the same idempotent request with the same key.

For provider synchronization or reconnect flows outside MCP, a
`resync_required` result means discard the cursor and request a fresh snapshot.

## Quick operator references

- [MCP integration guide](../../docs/integrations/mcp-agent-skill.md)
- [Architecture](../../docs/architecture.md)
- [Best practices](../../docs/best-practices.md)
- [Self-hosting](../../docs/self-hosting.md)
