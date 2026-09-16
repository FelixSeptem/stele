## Why

Stele now has the durable memory, scoped access, lifecycle, context, and assurance primitives required by an external agent runtime, but those primitives are not yet exposed as one discoverable provider contract. A small P7 contract layer is needed now so integrations can resolve server-owned scope, replay requests safely, and consume cited memory without inventing a second authorization or storage model.

## What Changes

- Introduce an OpenAPI-backed agent runtime provider surface over the existing service APIs.
- Separate agent identity, memory session, and conversation/turn attribution in provider requests and reports.
- Add bounded capability, version, limit, and canonical-scope discovery for compatible clients.
- Return a server-resolved runtime scope and require subsequent operations to use that scope; reject caller-invented or widened scope values.
- Standardize bounded operation metadata: `request_id`, `operation_id`, `idempotency_key`, `event_seq`, and `schema_version`.
- Define provider request/response envelopes for ingest, governed memory intents, retrieval, context assembly, forgetting, and scoped status/report reads.
- Preserve citations and provenance in provider responses while keeping hidden candidates, raw scores, query text, provider payloads, credentials, and internal plans private.
- Add provider conformance fixtures and tests against the public OpenAPI contract, including exact grants, idempotent retries, lifecycle visibility, restart-safe behavior, and diagnostic-only assurance evidence.
- Keep event replay transports, MCP, SDKs, UI, autonomous reasoning, and any second canonical store out of this change.

## Capabilities

### New Capabilities

- `agent-runtime-provider-adapter`: OpenAPI-backed provider contract for external agent runtimes, including identity/session separation, capability and scope discovery, operation metadata, cited responses, and conformance behavior.

### Modified Capabilities

- None. Existing ingestion, scoped-principal, session, context, retrieval, provenance, and assurance requirements remain authoritative; this change composes them behind a provider-facing contract without weakening their boundaries.

## Impact

- Affected areas include public OpenAPI schemas/routes, runtime configuration/version metadata, request-scope resolution, event/intents/retrieval/context/forgetting handlers, provenance citation shaping, and conformance/evaluation tests.
- No new persistence system is introduced. Provider operation and replay metadata is stored with existing PostgreSQL-backed records or bounded request evidence.
- Existing public routes remain compatible; the provider surface is additive and can be disabled or rejected when capability/version negotiation fails.
- Related workflow commands: `openspec status`, `openspec validate --strict`, and `/opsx:apply` (or the repository's equivalent OpenSpec apply command).

## Non-goals

- No SSE/WebSocket replay adapter in this slice.
- No MCP server, client SDK, UI, hosted control plane, or end-user product logic.
- No agent execution, prompt construction, model invocation, autonomous insight generation, or canonical-memory mutation from the provider layer.
- No cross-scope lookup, namespace widening, raw diagnostic export, or replacement of PostgreSQL as the system of record.
