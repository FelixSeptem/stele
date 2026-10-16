# Stele best practices

These practices apply to operators, integrations, and agents using the public
OpenAPI or MCP boundaries.

For durable references, poll bounded status until governance is complete and
then use exact ID/path lookup. Validate scope, UUID lineage, limits, lifecycle,
and disclosure on every response. Never treat HTTP acceptance as canonical
promotion or fall back to ranked search when PC5 is unavailable.

For runtime outcome/feedback evidence, negotiate PC6 and preserve a stable
effect ID plus idempotency key per source revision. Append corrections with a
greater revision and new key; never equate admission or `feedback_state=pending`
with task completion, canonical promotion or ranking/insight activation.
Reauthorize current reference states on inspection. See
[projection maintenance](provider-evidence-projections.md) for bounds and errors.

## Establish scope before memory work

Use an authenticated principal and resolve the exact runtime binding before
reading or writing memory. Treat `tenant`, `project`, and `namespace` as a
security boundary, not a search filter. Do not invent a broader namespace or
reuse a binding from another session.

For an explicitly authorized administrator MCP workflow, call `who_am_i` first
and confirm the active scope. Public credentials cannot use MCP data tools.
Public Provider clients complete runtime initialization and retain the
server-issued binding and session identity.

## Choose the smallest read

- Public clients use profiled Provider context, defaulting to references.
- Administrators use native/MCP context assembly for bounded working context.
- Administrators use search for a focused query and browse for scoped review.
- Request only the result and context limits needed for the current turn.

Read results are lifecycle-filtered evidence. Suppressed, forgotten, deleted,
stale, and foreign records must remain unavailable through ordinary reads.

Provider clients must validate the categorized context response before treating
it as usable or empty. Use canonical snake_case fields and binding-owned scope;
preserve citations rather than depending on ranking scores. See the
[Provider contract and migration notes](agent-runtime-memory-provider.md#provider-context-json-contract)
for disclosure provisioning, independent allocations and pending digest/continuity work.

Discover `context-budget-v1` and `context-packing-v1` explicitly; require
`token_budget_supported=false`. Reserve the agent's other prompt components,
then allocate positive item `budget` and optional content/body/citation limits.
Omission inherits server ceilings (100 items, 100 citations, default 1 MiB body
and inherited content); explicit zero content/citations is meaningful and never
changes the mode. Bytes measure UTF-8 content or the complete serialized body,
not model tokens. Whole-item selection retains required citations and may leave
space unused for deterministic accounting reservation.

Require closed `budget_accounting` even with diagnostics disabled. Check mode,
requested/effective limits, exact captured body bytes, content bytes, items and
unique citations; invalid/missing/version-incompatible accounting is a contract
failure. `empty` and `budget_exhausted` are distinct successful outcomes, while
dependency 503 is retryable failure. 400 malformed allocation needs correction,
403 disclosure denial needs operator review, and 413 means no partial projection
may be consumed; an infeasible envelope requires enough body space. Public section
labels describe unsupported capability, never hidden stored evidence. Avoid local
packing/reranking, byte-to-token guesses, alternate-route fallback or stripped
limits. Before rollback, disable access for consumers requiring PC3; historical
PC1/PC2 evidence is not allocation readiness.

## Write through governed paths

Memory writes should be explicit, attributable, and idempotent. Include a
meaningful reason and a stable idempotency key. Use the memory class and path
when known. Do not write directly to PostgreSQL or copy a retrieval result into
canonical storage without an explicit new-memory decision.

Stele preserves raw events, provenance, candidates, canonical versions, and
lifecycle history. A later correction should create a governed version or
intent rather than silently overwriting the old record.

## Forget safely

In an authorized administrator workflow, use the governed single-memory forget operation with a
reason and idempotency key. For semantic or bulk forgetting:

1. Run `memory_forget_preview`.
2. Review the bounded candidate IDs and reasons.
3. Apply only the reviewed IDs with `memory_forget_apply`.

Never turn a search result into an unreviewed bulk deletion. Retain the preview
identity when applying and treat conflicts or stale targets as a stop-and-review
condition.

## Verify Provider context continuity without rebuilding context locally

Discover and require `context-digest-v1`, `context-canonical-v1`,
`context-selection-v1` and `context-continuity-v1` before consuming the PC4
extension. Capture the exact compact response bytes, verify the closed response
shape and recompute the digest using the documented canonical form. Digest
comparison proves only that two authorized projections are the same under the
advertised policy; it neither grants read access nor proves task completion,
write authority, source truth or business state.

Use continuity only for an equivalent follow-up read. An omitted continuity
input means a normal fresh request. `issue` returns an opaque short-lived
metadata-only reference; `compare` returns another complete current body with
`unchanged` or `changed`. Never treat comparison as ETag behavior: do not accept
304, a bodyless response, a stored snapshot, a watermark, or a historical
projection. On 409 continuity refusal or retryable 503, stop and repair/retry
the Provider operation; do not widen scope, switch to MCP/native retrieval,
infer hidden changes, cache a prior body, or locally rerank/pack alternatives.

Keep comparison references and digest values out of prompts, telemetry,
diagnostics and repositories. They are not credentials, but an opaque reference
can still correlate runtime activity. Preserve citations and use recalled text
only after the agent's own tool/write/business policy has evaluated it.

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

## Context disclosure and source trust

Public runtime credentials require explicit exact-scope disclosure provisioning.
Provider context defaults to structured references; bounded content requires the
content profile and explicit mode. Unsupported native/MCP data routes deny public
credentials before lookup. Source labels persist through governance and history;
missing lineage is untrusted/restricted. Attested source data grants no instruction,
tool, write or business-state authority. See the [disclosure/source guide](context-disclosure-and-source-trust.md)
for endpoint behavior, administration, versioning and upgrade/rollback.
