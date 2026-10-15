## Purpose

Define a durable, exact-scope reference contract that lets Provider callers
write governed task/run/handoff/compilation evidence and resolve the resulting
identifier, value and path after asynchronous processing and restart.

## ADDED Requirements

### Requirement: Durable reference capability is explicitly negotiated

The Provider SHALL advertise a versioned durable-reference capability with its
supported write, status and exact-path read operations. A consumer requiring
this contract MUST fail closed when the capability or a required version is
absent and MUST NOT infer support from the base Provider schema alone.

#### Scenario: Client discovers durable references
- **WHEN** an authorized runtime requests Provider capabilities
- **THEN** the response identifies the durable-reference contract version,
  supported event kinds, status categories, read modes and bounded limits

#### Scenario: Older service lacks the capability
- **WHEN** a strict consumer requires the durable-reference version but the
  service does not advertise it
- **THEN** the consumer records a compatibility failure without rewriting the
  request or falling back to ranked search

### Requirement: Reference writes use a documented governed path

The service SHALL accept only documented reference-capable event or intent
types, validate exact scope, bounded value and normalized memory path, and route
the write through the ordinary event-to-candidate-to-canonical governance
pipeline. HTTP acceptance SHALL NOT imply that a searchable reference exists.

#### Scenario: Supported reference event is accepted
- **WHEN** an authorized caller submits a supported reference event with a
  bounded value, non-root path and idempotency key
- **THEN** the service returns a stable event identity and an explicit accepted
  or pending governance status without bypassing normal governance

#### Scenario: Unsupported arbitrary event type is submitted
- **WHEN** a caller submits an event type that is not documented as
  reference-capable
- **THEN** the service returns a bounded unsupported-type outcome or archives it
  without claiming a searchable canonical reference

### Requirement: Governance completion is observable and bounded

The service SHALL distinguish accepted, pending, failed, completed-governance,
and visible-canonical states through a scoped status contract. A completed
status SHALL require durable candidate/canonical/provenance persistence rather
than transient worker or HTTP success.

#### Scenario: Governance completes
- **WHEN** a supported reference event is processed successfully
- **THEN** status returns completed governance with bounded event, memory,
  version and provenance identities and the durable path/value reference

#### Scenario: Governance fails or remains pending
- **WHEN** processing is pending or fails with a retryable or terminal reason
- **THEN** status returns the corresponding bounded category and does not expose
  a canonical reference that was not durably created

### Requirement: Exact-path lookup resolves current authorized references

The service SHALL provide an exact-scope, exact-path reference lookup that
returns the recorded identifier, value, normalized path, source event identity,
memory identity and selected canonical version when currently authorized.
Deterministic reference lookup MUST remain distinct from ranked top-k search.

#### Scenario: Exact reference round trip succeeds
- **WHEN** an authorized caller looks up a completed reference using the exact
  scope, path and reference selector
- **THEN** the response returns the recorded value and bounded lineage identities
  for the current visible canonical version

#### Scenario: Ranked search cannot prove deterministic lookup
- **WHEN** a caller uses a top-k search request where an exact reference lookup
  is required
- **THEN** the contract does not treat a ranked hit or empty result as proof of
  reference identity or governance completion

### Requirement: Reference identity and path survive promotion and replay

The service SHALL preserve normalized non-root `memory_path`, UUID-compatible
lineage identifiers, provenance and reference identity through candidate
creation, canonical promotion, deduplication, consolidation and identical
idempotent replay. A conflicting reuse of an idempotency key SHALL fail closed.

#### Scenario: Canonical promotion preserves path and lineage
- **WHEN** a reference-capable event is promoted from candidate to canonical
  memory
- **THEN** exact-path lookup uses the original non-root path and returns valid
  event, memory, version and provenance identifiers

#### Scenario: Replay and conflict are handled safely
- **WHEN** the same scoped caller repeats an equivalent write or reuses its key
  with a changed value, path, type or scope
- **THEN** the equivalent write returns the original identity while the changed
  request returns a bounded conflict without a second reference

### Requirement: Scope, grant and lifecycle gates apply to every reference read

Every status and lookup SHALL revalidate exact tenant, project, namespace,
principal grant, disclosure constraints and current lifecycle. References MUST
NOT resurrect suppressed, forgotten or deleted content or disclose foreign
scope state.

#### Scenario: Foreign scope is requested
- **WHEN** a caller requests a reference outside its authorized scope
- **THEN** the service returns a bounded scope outcome without revealing whether
  the reference exists

#### Scenario: Reference is removed from ordinary visibility
- **WHEN** a previously completed reference is suppressed, forgotten or deleted
- **THEN** ordinary status/read returns an explicit unavailable or lifecycle
  outcome while privileged history retains only its authorized audit lineage

### Requirement: Restart recovery is part of durable reference readiness

The durable-reference contract SHALL be recoverable across API and worker
restart. Public readiness evidence MUST use a fresh PostgreSQL + pgvector
fixture, official bootstrap and ordinary public APIs, and MUST show governance
completion and exact-path lookup after restart.

#### Scenario: API and worker restart after acceptance
- **WHEN** the service restarts after a reference write but before or during
  asynchronous governance
- **THEN** the same event/status identity resumes or reports a bounded failure,
  and a completed reference remains readable after restart

#### Scenario: Evidence is incomplete
- **WHEN** a verifier observes only HTTP acceptance, an in-memory lookup, stale
  database rows or missing restart/bootstrap dependencies
- **THEN** the readiness result remains nonpassing and does not claim durable
  reference support
