## Purpose

Provide external agent runtimes with a discoverable, scope-safe memory-provider contract that composes Stele's existing governed APIs, preserves citations and provenance, and remains replay-safe without executing the agent or creating a second source of record.

## ADDED Requirements

### Requirement: Provider capabilities and compatibility are discoverable

The service SHALL expose a bounded provider capability document through the public OpenAPI contract. It MUST identify provider and schema versions, supported operations, canonical scope model, and configured maximums for event, intent, retrieval, context, citation, and metadata sizes without exposing secrets or operational internals.

#### Scenario: Compatible runtime discovers provider

- **WHEN** an external runtime requests the provider capability document from a healthy API instance
- **THEN** the response contains bounded version/build metadata, supported operation names, limit values, and the required exact scope dimensions

#### Scenario: Unknown build metadata is unavailable

- **WHEN** the service has no injected release identifier
- **THEN** the capability response uses documented bounded unknown/default values while retaining a valid contract and schema version

#### Scenario: Capability response is not a secret channel

- **WHEN** any caller reads provider capabilities
- **THEN** the response excludes credentials, DSNs, scope values, principal records, migration SQL, provider payloads, backlog details, and raw error data

### Requirement: Runtime scope is resolved and cannot be widened by callers

The provider SHALL resolve runtime scope from authenticated principal grants and server-owned agent/session context. Every scoped operation MUST carry or reference the resolved tenant, project, namespace, agent, session, and provider-instance identity, and the service MUST reject missing, mismatched, caller-invented, or widened scope before reading or writing memory.

#### Scenario: Runtime initializes within an exact grant

- **WHEN** an authenticated principal initializes a provider runtime for an authorized exact scope and agent identity
- **THEN** the service returns a stable runtime scope descriptor and session binding that subsequent operations can reference

#### Scenario: Caller widens namespace or tenant

- **WHEN** a subsequent provider operation supplies scope values outside the server-resolved grant or runtime binding
- **THEN** the service rejects the operation before accessing scoped records and does not disclose whether foreign records exist

#### Scenario: Session and agent identity remain distinct

- **WHEN** multiple sessions use the same agent identity or one session is recreated
- **THEN** the service preserves separate session/conversation attribution while retaining the same authorized canonical scope rules

### Requirement: Provider operations use bounded replay metadata and idempotency

The provider contract SHALL accept and return bounded `request_id`, `operation_id`, `idempotency_key`, `event_seq`, and `schema_version` metadata where applicable. Retries with equivalent metadata and payload MUST be idempotent, while conflicting reuse MUST return a bounded conflict or retryable outcome without duplicate events, intents, lifecycle actions, or evidence.

#### Scenario: Ingest retry returns original result

- **WHEN** a runtime retries an ingest operation with the same resolved scope, idempotency key, and normalized payload
- **THEN** the provider returns the original durable result and metadata without creating another raw event or provenance chain

#### Scenario: Operation key is reused with different payload

- **WHEN** a runtime reuses an idempotency key for a different normalized payload or operation kind
- **THEN** the provider returns a bounded conflict and does not reveal or mutate the prior operation's data

#### Scenario: Event sequence resumes safely

- **WHEN** a client submits an operation with a duplicate or older event sequence inside the same session
- **THEN** the service applies documented duplicate/stale handling and preserves monotonic durable attribution without replaying side effects

### Requirement: Provider composes governed memory operations without bypasses

The provider SHALL expose bounded operations for raw event ingest, governed memory intents, retrieval, context assembly, forgetting/lifecycle requests, and scoped status/report inspection by delegating to existing service contracts. It MUST NOT permit direct canonical-memory writes, bypass admission or governance, execute an external agent, or invoke a model.

#### Scenario: Agent submits a memory-relevant event

- **WHEN** a runtime submits a valid event through the provider
- **THEN** the service applies ordinary authentication, exact scope, idempotency, admission, provenance, and event-to-candidate-to-active governance behavior

#### Scenario: Agent requests context

- **WHEN** a runtime requests retrieval or assembled context for its resolved session
- **THEN** the provider applies lifecycle-safe defaults, projection freshness gates, bounded budgets, and exact-scope filtering

#### Scenario: Caller attempts canonical mutation

- **WHEN** a provider operation attempts to overwrite canonical memory or set lifecycle state directly
- **THEN** the service rejects the operation and requires the governed intent or admin lifecycle contract

### Requirement: Provider responses include safe citations and provenance

Provider retrieval, context, intent, and lifecycle responses SHALL include stable, bounded citations or provenance references sufficient for an authorized runtime to attribute returned content and outcomes. Responses MUST omit hidden or out-of-scope content, internal ranking plans, raw scores, query text, provider payloads, credentials, and unbounded identifiers outside authorized inspection evidence.

#### Scenario: Context item has durable evidence

- **WHEN** a visible memory or projection item is returned to an authorized runtime
- **THEN** the response includes a bounded citation identifying its source kind and stable reference plus applicable version/watermark metadata

#### Scenario: Source is hidden after derivation

- **WHEN** a cited source becomes suppressed, forgotten, deleted, stale, or out of scope
- **THEN** ordinary provider reads omit the item or return a bounded unavailable citation state while privileged history remains governed by existing admin paths

#### Scenario: Citation budget is exceeded

- **WHEN** the caller's citation or context budget cannot fit another evidence reference
- **THEN** the provider fails closed with a bounded omission reason and does not broaden the query or budget

### Requirement: Provider conformance is durable, scoped, and diagnostic

The service SHALL provide a provider conformance profile and run contract that exercises the public OpenAPI provider operations against one exact scope. Conformance MUST report capability compatibility, scope enforcement, idempotent replay, lifecycle safety, citation completeness, restart/fallback behavior, and evidence freshness as bounded diagnostic categories without executing the external agent or mutating canonical source records.

#### Scenario: Conformance run passes

- **WHEN** an authorized administrator runs a supported provider profile with all required fixture operations and durable evidence in scope
- **THEN** the service records a passing run with bounded counters, verdict categories, cited evidence references, and contract/schema provenance

#### Scenario: Conformance finds a scope violation

- **WHEN** a fixture or integration attempt requests an ungranted scope or hidden memory
- **THEN** the run records a scope/lifecycle safety failure, returns no foreign or hidden content, and leaves canonical records unchanged

#### Scenario: Conformance dependency is degraded

- **WHEN** provider compatibility, projection freshness, worker processing, or PostgreSQL evidence is missing or stale
- **THEN** the run records incomplete/degraded status with bounded next actions and does not claim provider readiness

#### Scenario: Conformance is rerun

- **WHEN** an administrator reruns a provider conformance profile
- **THEN** the service creates a new linked diagnostic run and preserves all prior run history and evidence

### Requirement: Provider failures are bounded and compatible

The provider SHALL return stable error categories for authentication, scope mismatch, capability/version incompatibility, validation, idempotency conflict, lifecycle denial, stale projection, dependency degradation, and retryable interruption. Error responses MUST be bounded, machine-readable, and free of secrets, hidden-record existence, raw SQL, provider payloads, and internal stack traces.

#### Scenario: Client uses unsupported schema version

- **WHEN** a runtime submits a request with an unsupported provider schema version
- **THEN** the service rejects it with a compatibility category and supported-version metadata without processing the operation

#### Scenario: Durable operation is interrupted

- **WHEN** an operation is interrupted after durable claim but before completion
- **THEN** a retry receives the original result after bounded recovery or a documented retryable category, with no duplicate side effect

