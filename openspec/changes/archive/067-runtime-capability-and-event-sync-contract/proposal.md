## Why

Stele's provider contract can already advertise capabilities, resolve an exact runtime scope, accept idempotent operations, and replay individual requests. It does not yet define how an external runtime obtains a consistent state snapshot, resumes an interrupted event stream, or detects that its replay position has fallen outside the retained history. Each adapter would otherwise invent its own reconnect and synchronization rules, making recovery behavior inconsistent and difficult to verify.

This change defines one transport-independent synchronization contract and implements its first OpenAPI pull-based transport. It gives runtimes deterministic snapshot and event replay behavior now while leaving WebSocket and SSE as future adapters over the same cursor, event, and recovery semantics.

## What Changes

- Add a bounded, versioned runtime synchronization contract covering capability discovery, snapshot identity, event sequence, cursor acknowledgment, batch limits, retention windows, and recovery outcomes.
- Add server-resolved runtime synchronization state bound to the authenticated tenant, project, namespace, agent, session, and provider instance.
- Define an OpenAPI pull flow that returns a consistent snapshot, ordered durable events, and a terminal `sync_complete` marker.
- Support reconnect from a previously acknowledged cursor without replaying side effects or returning events from another scope.
- Return an explicit `resync_required` outcome when a cursor is too old, incompatible, invalid, or outside the retained event window.
- Define bounded event envelopes with schema version, event sequence, event kind, source watermark, replay identity, and redacted payload references; synchronization never permits direct canonical-memory mutation.
- Extend provider conformance fixtures and diagnostics for initial sync, resume, duplicate suppression, retention gaps, scope rejection, schema incompatibility, and deterministic replay.
- Publish synchronization limits, supported event kinds, retention behavior, and transport-neutral semantics through the existing provider capability document.
- Reserve a transport adapter boundary so future WebSocket or SSE implementations can reuse the same synchronization state machine and error categories without changing the OpenAPI contract or storage model.
- Add documentation for runtime startup, reconnect, acknowledgment, and full-resync handling.

## Capabilities

### New Capabilities

- `runtime-capability-and-event-sync-contract`: Defines the transport-independent capability, snapshot, cursor, ordered event replay, synchronization completion, retention, and recovery contract for external runtimes.

### Modified Capabilities

- `agent-runtime-provider-adapter`: Extends provider capability discovery, runtime binding, replay metadata, bounded errors, and conformance requirements to cover resumable synchronization.
- `runtime-memory-provider-contract`: Extends offline provider contract coverage with synchronization snapshots, event replay, cursor recovery, and transport-neutral deterministic replay cases.

## Impact

- Affected public surface: provider capability schema and new OpenAPI synchronization routes or operation shapes under the existing provider boundary.
- Affected runtime code: provider binding/session state, event sequencing and replay services, bounded error mapping, and conformance execution.
- Affected persistence: PostgreSQL-backed synchronization cursor/run state and durable event retention or projection references; PostgreSQL remains the only system of record.
- Affected tests and docs: OpenAPI contract tests, scope and isolation tests, replay/reconnect tests, retention-gap tests, provider conformance evidence, and self-hosting guidance.
- No new external dependency is required for the first transport. WebSocket/SSE support remains a future adapter over the same transport-neutral protocol.

## Non-goals

- No WebSocket or SSE server implementation in this change.
- No second event store, message broker, or client-owned canonical memory store.
- No direct canonical-memory writes, lifecycle bypass, or provider-side agent/model execution.
- No unbounded event history, arbitrary namespace subscription, or cross-scope synchronization.
- No change to default retrieval visibility, ranking, context assembly, or memory lifecycle semantics.
- No replacement of existing provider capability, runtime binding, or conformance contracts; this change extends them with synchronization behavior.
