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

The capability document also advertises the transport-neutral synchronization
contract. The first enabled transport is `openapi_pull`; WebSocket and SSE are
reserved for future adapters and are not enabled by this release. The sync
contract publishes event kinds, maximum batch and snapshot sizes, cursor size,
and the replay retention window.

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
`retryable`, and `resync_required`. Unsupported schema versions fail before
operation dispatch. A `resync_required` result is fail-closed: the runtime must
start a fresh snapshot instead of assuming that skipped events were applied.

## Snapshot and reconnect synchronization

After the runtime binding handshake, request an initial bounded snapshot through
the provider boundary:

```bash
curl -sS "$STELE_URL/v1/provider/sync" \
  -H "X-API-Key: $STELE_RUNTIME_API_KEY" \
  -H "X-Stele-Runtime-Binding: $STELE_RUNTIME_BINDING" \
  -H "X-Stele-Runtime-Session: $STELE_RUNTIME_SESSION" \
  -H "Content-Type: application/json" \
  -d '{"schema_version":"sync-v1","max_events":100}'
```

The response contains a snapshot watermark, a bounded `next_cursor`, and a
`sync_complete` flag. Persist that cursor only after the runtime has accepted
the complete response. To reconnect, send the acknowledged cursor in the next
request:

```json
{
  "schema_version": "sync-v1",
  "cursor": "<last-acknowledged-cursor>",
  "max_events": 100
}
```

Events are ordered within the exact runtime scope and carry a sequence, replay
identity, source watermark, schema version, and bounded redacted payload or
reference. Repeating a cursor is deterministic and does not replay a write.
When the cursor is expired, malformed, incompatible, or outside the retained
window, the service returns HTTP 409 with `category=resync_required`. Discard
the cursor and repeat the initial snapshot flow. Synchronization is read/replay
only; event ingest, governed intents, and admin lifecycle actions remain the
only write paths.

## Conformance and rollback

Provider conformance uses deterministic fixtures in one explicitly owned exact
scope. It checks capability compatibility, scope denial, idempotent replay,
lifecycle filtering, citation completeness, dependency freshness, and
restart/fallback behavior. A skipped, incomplete, stale, or degraded run is not
provider-readiness evidence. Conformance is diagnostic and does not invoke an
external agent or mutate canonical memory except through ordinary governed
fixture ingestion.

The self-hosted product verifier extends this evidence to governed memory
intents. It submits through the public OpenAPI boundary, confirms the durable
PostgreSQL `memory_intent` work reference, replays and conflicts an idempotency
key, checks exact-scope inspection and foreign-scope denial, and verifies worker
and scheduler restart recovery. It also stops the worker, verifies that
`STELE_MEMORY_INTENT_POLICY_ENABLED=false` rejects new persistence while the
queued record remains inspectable, then re-enables the same
`STELE_MEMORY_INTENT_POLICY_VERSION` and confirms only the exact-scope pending
work resumes. The resulting report is redacted and uses only bounded phase,
recovery, rollback, cleanup, freshness, and consumability categories. A
missing rollback-policy fixture or other hard gate is recorded as degraded
evidence and never enables a provider or changes default retrieval.

To roll back, set `STELE_PROVIDER_ENABLED=false` and restart API replicas. The
existing public APIs, PostgreSQL canonical records, append-only history, and
provider conformance evidence remain intact. Do not down-migrate or delete
canonical memory as part of provider rollback.

Synchronization can also be disabled independently while provider operations
remain enabled: advertise `synchronization.enabled=false` (with no enabled
transport) and do not register the synchronization source. Synchronization
requests then fail closed with a bounded `provider_unavailable` response;
existing ingest, intent, retrieval, context, and lifecycle routes continue to
use their normal governed contracts. Re-enable synchronization only after the
advertised contract, retention window, and source projection are available.

Related workflow commands are `openspec validate --all`,
`go test ./internal/provider ./internal/app ./internal/assurance ./openapi`, and
the repository quality gate.
