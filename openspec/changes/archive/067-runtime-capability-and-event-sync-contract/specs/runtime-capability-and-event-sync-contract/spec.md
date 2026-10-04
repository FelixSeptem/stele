## Purpose

Define a bounded, exact-scope synchronization contract that lets an external runtime obtain a consistent snapshot, replay durable state events in order, resume after interruption, and recover deterministically when retained history is insufficient. The contract is transport-independent so OpenAPI pull, WebSocket, and SSE adapters can share the same semantics.

## ADDED Requirements

### Requirement: Synchronization capabilities are discoverable

The provider capability document SHALL advertise whether synchronization is supported, the synchronization contract and schema versions, supported transport-neutral modes, event kinds, maximum batch size, snapshot and cursor limits, and the retained replay window. It MUST NOT expose secrets, scope values, raw event payloads, database details, or operational internals.

#### Scenario: Runtime discovers pull synchronization

- **WHEN** an authorized runtime reads provider capabilities
- **THEN** the response identifies the supported synchronization contract, OpenAPI pull mode, event kinds, limits, and the conditions that require a full resynchronization

#### Scenario: Unsupported future transport is not advertised

- **WHEN** WebSocket or SSE synchronization has not been enabled
- **THEN** the capability document omits those transports or marks them unavailable without changing the OpenAPI synchronization semantics

### Requirement: Synchronization is bound to an exact runtime scope

Every synchronization request SHALL resolve tenant, project, namespace, agent, session, and provider-instance identity from the authenticated runtime binding. The service MUST reject missing, mismatched, widened, or caller-invented scope before reading synchronization state or events.

#### Scenario: Runtime starts an in-scope synchronization

- **WHEN** an authorized runtime requests an initial synchronization for its bound session
- **THEN** the service creates or returns a synchronization identity bound to the exact runtime scope and does not include events from another scope

#### Scenario: Runtime attempts to widen scope

- **WHEN** a synchronization request supplies a namespace, tenant, project, agent, or session outside the resolved binding
- **THEN** the service returns a bounded scope error without revealing whether foreign synchronization state exists

### Requirement: Initial synchronization returns a consistent snapshot

The initial synchronization response SHALL identify one snapshot watermark and return a bounded, lifecycle-safe snapshot followed by events ordered after that watermark. The response MUST include a terminal `sync_complete` marker or an equivalent completion status before the runtime treats the snapshot as usable.

#### Scenario: Initial snapshot completes

- **WHEN** a runtime starts synchronization with no prior cursor
- **THEN** the service returns a bounded snapshot identity, source watermark, ordered event batches, and `sync_complete` with the next resumable cursor

#### Scenario: Snapshot exceeds configured limits

- **WHEN** the snapshot cannot fit within the advertised response or event limits
- **THEN** the service returns a bounded continuation cursor and the runtime can request the remaining snapshot without changing scope or ordering

### Requirement: Events are ordered, replayable, and acknowledged by cursor

Each synchronization event SHALL carry a monotonic sequence within its synchronization scope, a stable replay identity, schema version, event kind, source watermark, and a bounded redacted payload or reference. A runtime SHALL acknowledge progress by presenting the last accepted cursor on the next request, and equivalent retries MUST return equivalent events without replaying side effects.

#### Scenario: Runtime resumes after interruption

- **WHEN** a runtime reconnects with the last cursor it durably acknowledged
- **THEN** the service returns the next ordered event or completion marker and does not skip, reorder, or duplicate durable events

#### Scenario: Runtime repeats a cursor request

- **WHEN** a runtime repeats a synchronization request with the same binding, cursor, and contract version
- **THEN** the service returns a deterministic bounded result and does not create a second event, mutation, or synchronization side effect

### Requirement: Retention gaps require a full resynchronization

The service SHALL detect cursors that are expired, malformed, incompatible, revoked, or older than the retained replay window. It MUST return a machine-readable `resync_required` outcome with a bounded reason and the current synchronization contract version instead of silently skipping events.

#### Scenario: Cursor is outside the retention window

- **WHEN** a runtime presents a cursor older than the retained event history
- **THEN** the service returns `resync_required`, identifies that a full snapshot is required, and returns no partial event stream that could appear complete

#### Scenario: Cursor uses an unsupported contract version

- **WHEN** a runtime presents a cursor encoded for an unsupported synchronization schema
- **THEN** the service returns a compatibility error with supported versions and leaves the existing synchronization state unchanged

### Requirement: Synchronization cannot bypass governed memory behavior

Synchronization SHALL expose only durable, already-authorized state transitions and evidence references. It MUST preserve lifecycle filtering, exact-scope isolation, redaction, provenance, and append-only semantics, and MUST NOT accept an operation that directly creates, overwrites, suppresses, forgets, or deletes canonical memory.

#### Scenario: Hidden state changes after a snapshot

- **WHEN** an event source becomes suppressed, forgotten, deleted, stale, or out of scope before a runtime receives a later batch
- **THEN** the service omits the hidden payload or returns a bounded redacted transition and does not leak the prior content

#### Scenario: Runtime submits a mutation through synchronization

- **WHEN** a synchronization request contains a direct canonical-memory mutation or lifecycle command
- **THEN** the service rejects it as an unsupported operation and requires the existing governed intent or admin lifecycle contract

### Requirement: Future transports reuse the synchronization contract

The synchronization state machine SHALL separate transport framing from snapshot, cursor, event ordering, acknowledgment, completion, retention, and error semantics. Any future WebSocket or SSE adapter MUST use the same scope binding, event envelopes, cursor rules, redaction, and `resync_required` behavior as the OpenAPI pull transport.

#### Scenario: Future adapter advertises a supported transport

- **WHEN** a WebSocket or SSE adapter is enabled for a compatible runtime
- **THEN** capability discovery adds only the transport availability while preserving the same synchronization contract version, event identities, cursor semantics, and recovery outcomes

#### Scenario: Transport disconnects during replay

- **WHEN** a future streaming transport disconnects after the runtime acknowledges a cursor
- **THEN** reconnect through that transport or OpenAPI pull resumes from the acknowledged cursor with equivalent ordering and retention behavior
