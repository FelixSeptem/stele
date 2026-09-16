## Context

See `proposal.md` for motivation and scope. Stele already publishes OpenAPI and
version metadata, authenticates durable principals with exact grants, models
memory sessions and governed intents, assembles lifecycle-safe context, and
stores conformance evidence in PostgreSQL. The missing boundary is a coherent
provider contract that lets an external runtime discover those capabilities and
carry server-owned runtime identity across operations.

## Goals / Non-Goals

**Goals:**

- Define one additive provider contract over existing HTTP/OpenAPI behavior.
- Make server-resolved runtime scope and agent/session separation explicit.
- Make operation replay and idempotency metadata consistent across provider operations.
- Preserve bounded citations/provenance while keeping diagnostic internals private.
- Provide deterministic, scope-isolated conformance evidence and readiness outcomes.
- Allow capability negotiation and safe fallback when a client or dependency is incompatible.

**Non-Goals:**

- No streaming replay transport, MCP adapter, SDK, UI, or agent execution.
- No new canonical tables or storage engine; derived provider evidence remains PostgreSQL-backed and rebuildable.
- No change to existing public route authorization, lifecycle defaults, or canonical-memory mutation rules.
- No provider-specific model, embedding, or reasoning dependency.

## Decisions

### 1. Add an OpenAPI provider namespace and typed envelopes

Expose provider discovery and operations as additive, versioned routes (for
example, a capability document and a runtime initialization/operation family)
described in the authoritative OpenAPI document. Use typed request/response
envelopes containing contract version, runtime scope reference, operation
metadata, result, citations, and bounded errors. Reuse existing application
services rather than duplicating event, intent, retrieval, context, or lifecycle
logic. A separate RPC or SDK-first API was rejected because it would create a
second contract and make scope behavior drift from OpenAPI.

### 2. Resolve scope during runtime initialization

Require an authenticated principal with an exact grant to initialize a runtime
binding. The server returns an opaque runtime binding/reference plus the
resolved tenant/project/namespace and agent/session descriptors needed by the
client; callers cannot choose a wider scope. Subsequent requests carry the
binding and a session/conversation identifier, while middleware revalidates the
grant and rejects mismatches before repository access. This preserves the
existing principal model and avoids trusting caller-supplied scope headers.

### 3. Use a common operation metadata envelope

Normalize and validate `request_id`, `operation_id`, `idempotency_key`,
`event_seq`, and `schema_version` with per-field size and character limits.
Persist idempotency decisions through the existing event/session/lifecycle
repositories where available; for read-only operations, use request and
operation identifiers only for bounded evidence correlation. Sequence numbers
are monotonic within a runtime session; duplicate/stale values produce stable
dispositions and never replay side effects. A global sequence was rejected
because it would couple independent sessions and complicate exact-scope replay.

### 4. Return citation references, not diagnostic internals

Map existing context/retrieval/provenance records into a provider citation
shape containing source kind, stable reference, canonical version or projection
watermark, and bounded availability state. Apply the same lifecycle and scope
filters before shaping citations. Keep raw query text, hidden IDs, rank scores,
provider payloads, and SQL out of provider responses; privileged inspection
continues through existing admin surfaces. This lets runtimes explain memory
use without turning diagnostics into a data exfiltration path.

### 5. Implement conformance as deterministic service-side fixtures

Conformance profiles select bounded fixture operations and required evidence
categories. A runner invokes the provider service boundary with an isolated
owned scope, records operation outcomes and safety assertions, and persists a
diagnostic run linked to existing assurance/conformance records. It never calls
an external model or agent and never changes canonical memory except through
ordinary fixture event ingestion. A client-supplied self-attestation was
rejected because it cannot prove scope isolation, lifecycle filtering, or
idempotent replay.

### 6. Compatibility and fallback are fail-closed

Validate schema/provider versions before dispatch. Unsupported versions return
a stable compatibility error and supported-version metadata. If runtime binding
or provider dependencies become unavailable, reads use existing safe behavior
or return a bounded retryable/degraded result; no operation widens scope or
silently bypasses governance. Feature flags/configuration may disable the
provider surface without affecting existing public APIs.

## Risks / Trade-offs

- **[Risk]** A long-lived runtime binding could outlive a revoked grant. → Revalidate the active grant and binding state on every operation; reject revoked or expired bindings.
- **[Risk]** Envelope duplication may diverge from existing route schemas. → Generate/validate OpenAPI from the same typed request/response models and add contract tests against live handlers.
- **[Risk]** Citation references can become stale after lifecycle changes. → Include bounded availability/watermark state and reapply ordinary visibility filters at read time.
- **[Risk]** Conformance fixtures may create noisy derived evidence. → Isolate fixture attribution, bound counts/retention, and retain canonical records only through normal governed ingestion.
- **[Risk]** Client retries can produce ambiguous outcomes around process failure. → Reuse durable idempotency claims and explicit retryable/duplicate dispositions; never report success without a durable result.
- **[Risk]** Provider adoption could accidentally become a default retrieval path. → Keep the adapter additive/opt-in and require conformance and release-gate evidence before enabling it for a scope.

## Migration Plan

1. Add typed provider models, capability metadata, and additive OpenAPI paths behind a disabled-by-default configuration gate.
2. Implement runtime initialization and operation middleware by composing existing principal, session, event, intent, retrieval, context, lifecycle, and provenance services.
3. Add only nullable/defaulted metadata columns or JSON evidence fields if existing records cannot carry operation attribution; migration must be forward/backward compatible and down-scope to this change's objects.
4. Add deterministic contract/conformance fixtures and redaction/isolation tests; run them without provider credentials or ambient production DSNs.
5. Enable the provider per scope only after capability, PostgreSQL, projection freshness, lifecycle, and rollback gates pass. Disable the flag to roll back without deleting canonical or historical records.

## Open Questions

None that alter the specified behavior or task breakdown. Exact route naming and
whether an existing metadata column can be reused are implementation details to
resolve while preserving the envelopes and invariants above.
