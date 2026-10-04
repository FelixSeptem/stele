## Context

The existing provider boundary already exposes bounded capabilities, initializes an exact runtime binding, accepts operation metadata, and composes governed event, intent, retrieval, context, lifecycle, and status operations. See `openspec/specs/agent-runtime-provider-adapter/spec.md` and `openspec/specs/runtime-memory-provider-contract/spec.md`.

The missing contract is synchronization after initialization: a runtime needs a consistent starting view, durable event ordering, an acknowledged resume point, and an explicit recovery path when history has expired. PostgreSQL remains the only system of record, and all synchronized content remains subject to existing scope, lifecycle, provenance, and redaction rules.

## Goals / Non-Goals

**Goals:**

- Define one transport-neutral synchronization state machine.
- Implement the first transport as bounded OpenAPI pull requests.
- Make initial snapshot, continuation, acknowledgment, completion, replay, and retention-gap behavior deterministic.
- Reuse runtime binding and provider error categories rather than creating a second authorization path.
- Leave a clean adapter boundary for future WebSocket and SSE transports.
- Produce offline and real-stack conformance evidence without executing an external agent or mutating canonical records.

**Non-Goals:**

- Implement WebSocket or SSE in this change.
- Introduce a broker, external event store, or client-owned canonical state.
- Change retrieval ranking, context assembly, memory lifecycle, or governed intent semantics.
- Provide arbitrary subscriptions, cross-scope streams, or unbounded history.

## Decisions

### 1. Use a transport-neutral cursor protocol with OpenAPI pull first

The synchronization contract is expressed as a request/response state machine: initialize or resume with a cursor, return a bounded snapshot or event batch, and finish with `sync_complete` plus the next cursor. The first public transport is an OpenAPI operation under the existing provider boundary.

WebSocket and SSE are deliberately not implemented now. Their future adapters can frame the same response units as messages or streamed records without changing event identity, ordering, acknowledgment, or recovery semantics. This avoids committing the service to connection-specific behavior before the pull contract is proven.

### 2. Bind synchronization to the existing runtime binding

The server resolves tenant, project, namespace, agent, session, and provider-instance identity once during runtime initialization. Synchronization requests carry an opaque synchronization cursor and contract version, but never caller-owned scope values that could widen the binding. Cursor ownership is checked before any event lookup.

### 3. Use a durable snapshot watermark and monotonic event sequence

An initial sync captures a snapshot watermark, reads a bounded lifecycle-safe snapshot, and then returns events strictly after that watermark. Each event has a monotonic sequence within the exact synchronization scope, a stable replay identity, a source watermark, and a bounded redacted payload or reference. The next request's cursor is the acknowledgment boundary; no separate side-effecting acknowledgment endpoint is required.

The implementation may persist synchronization run/cursor state in PostgreSQL or encode immutable cursor material that can be validated against PostgreSQL watermarks, but the observable contract must preserve ownership, expiry, and deterministic replay. Cursor state must never become a second source of record.

### 4. Make retention failure explicit and fail closed

The provider advertises a bounded replay window. If a cursor is malformed, expired, revoked, incompatible, or older than the retained window, the service returns `resync_required` and no apparently complete partial stream. The runtime must start a fresh snapshot. This is safer than silently skipping events and is compatible with future streaming transports.

### 5. Keep event envelopes redacted and governance-aware

Event envelopes contain only bounded event kind, sequence, replay identity, source watermark, schema version, and authorized payload references or redacted transition data. Hidden, forgotten, deleted, stale, or out-of-scope content is omitted or represented by a bounded unavailable transition. Synchronization is read/replay only; all writes continue through existing event, intent, and admin lifecycle contracts.

### 6. Extend capability and conformance contracts together

Capability discovery advertises sync versions, event kinds, limits, retention, and currently enabled transports. Provider conformance must verify the same semantics offline and against the real stack, including replay determinism, restart recovery, retention gaps, scope isolation, redaction, and future-transport equivalence at the protocol level. A synchronization failure blocks readiness evidence for the provider profile.

## Risks / Trade-offs

- **[Risk]** A snapshot plus event stream can be expensive for large scopes. → **Mitigation:** enforce bounded snapshot/event limits, continuation cursors, configured retention, and explicit backpressure categories.
- **[Risk]** Cursor ownership or scope binding could leak foreign state. → **Mitigation:** validate binding and contract version before cursor lookup; return indistinguishable bounded scope/compatibility errors.
- **[Risk]** Event retention can expire during a long-lived client session. → **Mitigation:** advertise the retention window, return `resync_required` without partial completion, and test restart and retention-gap recovery.
- **[Risk]** Future WebSocket/SSE adapters could drift from OpenAPI behavior. → **Mitigation:** keep state-machine semantics transport-neutral and require conformance fixtures to compare equivalent cursor, ordering, completion, and recovery outcomes.
- **[Risk]** Synchronized payloads could expose lifecycle-hidden content. → **Mitigation:** reuse existing lifecycle filters and redaction helpers, cap payload references, and include hidden-state omission scenarios in contract tests.

## Migration Plan

1. Add the capability fields, provider synchronization schemas, and transport-neutral error categories with synchronization disabled unless the provider runtime supports the contract.
2. Add PostgreSQL-backed synchronization watermark/cursor and bounded event projection support without rewriting existing canonical records.
3. Add the OpenAPI pull operation and runtime binding checks, then enable it for compatible schema versions.
4. Add offline and real-stack conformance fixtures and document startup, resume, and full-resync handling.
5. If a deployment must roll back, advertise synchronization as unavailable and continue serving existing provider operations; retained source records remain unchanged.
6. Implement WebSocket or SSE later as separate adapters only after they pass the same transport-neutral conformance profile.

## Open Questions

- The exact PostgreSQL cursor storage shape can be selected during implementation as long as cursor ownership, retention, deterministic replay, and single-source-of-record behavior remain unchanged.
- The first implementation can choose one bounded snapshot representation from existing provider-safe projections; the public contract must retain the snapshot watermark and continuation semantics.
