# Agent runtime memory provider

Stele exposes an opt-in, OpenAPI-backed memory-provider contract for external
agent runtimes. The provider is a service adapter: it does not execute agents,
build prompts, call models, or maintain a second memory store.

## Enablement and discovery

Set `STELE_PROVIDER_ENABLED=true` in all API replicas after migrations have
reached the schema version reported by `GET /version`. The default is `false`.
Configure supported contract versions with
`STELE_PROVIDER_SCHEMA_VERSIONS=provider-v1`; request, result, citation, and
metadata limits use the `STELE_PROVIDER_MAX_*` variables shown in
`.env.local.example`.

Clients read the bounded capability document before initializing a runtime.
It publishes supported operation names, exact scope dimensions, versions,
limits, and an OpenAPI digest. It never publishes credentials, DSNs, principal
records, scope values, backlog data, provider payloads, or internal errors.

## Runtime scope handshake

Runtime initialization requires an active durable principal and one exact
`tenant/project/namespace` grant. The request separates `agent_id`,
`session_id`, and optional `conversation_id`. Stele returns an opaque runtime
binding plus a server-generated provider-instance identity. Every later
operation carries that binding and session identity; the service revalidates
the binding, expiry, principal, and exact grant before repository access.
Changing scope headers or an identity returns a generic denial and never
discloses whether foreign records exist.

## Operation metadata and citations

Provider operations use bounded `request_id`, `operation_id`,
`idempotency_key`, `event_seq`, and `schema_version` metadata. Equivalent
durable retries return the original result. Reusing a key for a different
normalized payload returns `conflict`; duplicate or stale sequence values never
replay side effects. Ingest, intents, retrieval, context, and lifecycle requests
delegate to the existing governed service contracts.

Visible results may include bounded citations with source kind, stable
reference, version or projection watermark, and availability. Ordinary
provider responses omit raw queries, hidden candidates, scores, ranking plans,
provider payloads, credentials, and foreign identifiers. Suppressed, forgotten,
deleted, stale, or out-of-scope evidence is excluded by default.

Stable provider error categories are `authentication`, `scope`,
`compatibility`, `validation`, `conflict`, `lifecycle`, `stale`, `dependency`,
and `retryable`. Unsupported schema versions fail before operation dispatch.

## Conformance and rollback

Provider conformance uses deterministic fixtures in one explicitly owned exact
scope. It checks capability compatibility, scope denial, idempotent replay,
lifecycle filtering, citation completeness, dependency freshness, and
restart/fallback behavior. A skipped, incomplete, stale, or degraded run is not
provider-readiness evidence. Conformance is diagnostic and does not invoke an
external agent or mutate canonical memory except through ordinary governed
fixture ingestion.

To roll back, set `STELE_PROVIDER_ENABLED=false` and restart API replicas. The
existing public APIs, PostgreSQL canonical records, append-only history, and
provider conformance evidence remain intact. Do not down-migrate or delete
canonical memory as part of provider rollback.

Related workflow commands are `openspec validate --all`,
`go test ./internal/provider ./internal/app ./internal/assurance ./openapi`, and
the repository quality gate.
