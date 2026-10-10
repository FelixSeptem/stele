# Stele best practices

These practices apply to operators, integrations, and agents using the public
OpenAPI or MCP boundaries.

## Establish scope before memory work

Use an authenticated principal and resolve the exact runtime binding before
reading or writing memory. Treat `tenant`, `project`, and `namespace` as a
security boundary, not a search filter. Do not invent a broader namespace or
reuse a binding from another session.

For MCP, call `who_am_i` first and confirm the returned access mode and active
scope. For provider/OpenAPI clients, complete runtime initialization and retain
the server-issued binding and session identity.

## Choose the smallest read

- Use context assembly when the agent needs bounded working context.
- Use search for a focused query and citations.
- Use browse for review or pagination within one exact scope.
- Request only the result and context limits needed for the current turn.

Read results are lifecycle-filtered evidence. Suppressed, forgotten, deleted,
stale, and foreign records must remain unavailable through ordinary reads.

Provider clients must validate the categorized context response before treating
it as usable or empty. Use canonical snake_case fields and binding-owned scope;
preserve citations rather than depending on ranking scores. See the
[Provider contract and migration notes](agent-runtime-memory-provider.md#provider-context-json-contract)
for item-budget limits and pending disclosure, digest, and continuity work.

## Write through governed paths

Memory writes should be explicit, attributable, and idempotent. Include a
meaningful reason and a stable idempotency key. Use the memory class and path
when known. Do not write directly to PostgreSQL or copy a retrieval result into
canonical storage without an explicit new-memory decision.

Stele preserves raw events, provenance, candidates, canonical versions, and
lifecycle history. A later correction should create a governed version or
intent rather than silently overwriting the old record.

## Forget safely

For one known memory, use the governed single-memory forget operation with a
reason and idempotency key. For semantic or bulk forgetting:

1. Run `memory_forget_preview`.
2. Review the bounded candidate IDs and reasons.
3. Apply only the reviewed IDs with `memory_forget_apply`.

Never turn a search result into an unreviewed bulk deletion. Retain the preview
identity when applying and treat conflicts or stale targets as a stop-and-review
condition.

## Preserve evidence quality

Keep citations attached to agent-facing answers when a result depends on memory.
Use source references and projection/version watermarks where available. Do not
expose hidden content, raw embeddings, ranking internals, credentials, or
provider payloads to an agent merely for debugging convenience.

## Retry and recover deliberately

Retry idempotent reads and writes with the same request or idempotency identity.
Do not retry a destructive action with a new key until the prior outcome is
understood. Treat scope, authorization, validation, lifecycle, and compatibility
errors as corrective actions, not transient failures.

For provider synchronization, persist an acknowledged cursor only after the
batch is accepted. On `resync_required`, discard the cursor and start a fresh
snapshot. Do not assume skipped events were applied.

## Operate the self-hosted service

- Keep PostgreSQL as the only system of record.
- Run migrations before enabling new API/provider capabilities.
- Keep MCP and provider adapters disabled until exact-scope smoke and
  conformance checks pass.
- Use explicit owned PostgreSQL + pgvector databases for integration evidence.
- Monitor `/health`, `/ready`, worker leases, scheduler backlog, retention, and
  migration status.
- Back up PostgreSQL and rehearse restore verification before relying on a
  deployment.

Detailed configuration and recovery procedures are in
[self-hosting](self-hosting.md), while the architecture and component roles are
documented in [architecture](architecture.md) and [components](components.md).
