## Purpose

Provide MCP-compatible agents with a small, optional adapter over Stele's
authoritative OpenAPI contracts while preserving exact scope authorization,
lifecycle-safe context, governed mutations, citations, and PostgreSQL ownership.

## ADDED Requirements

### Requirement: MCP adapter is optional and OpenAPI-backed

The service SHALL expose the MCP surface only when explicitly enabled, and every
MCP operation MUST delegate to an existing authoritative OpenAPI/service contract
for authentication, scope, retrieval, context, lifecycle, or governed mutation.
The MCP surface MUST NOT become a second persistence or authorization boundary.

#### Scenario: MCP is disabled by default

- **WHEN** API mode starts without the MCP adapter explicitly enabled
- **THEN** the ordinary OpenAPI service remains available and no MCP endpoint is
  advertised or accepting requests

#### Scenario: Enabled MCP delegates to an existing contract

- **WHEN** an authenticated client invokes an enabled MCP tool
- **THEN** the adapter resolves the same principal and scope context and invokes
  the corresponding OpenAPI/service behavior without a parallel storage path

#### Scenario: Adapter is disabled after deployment

- **WHEN** an operator disables the MCP adapter
- **THEN** MCP requests fail closed while OpenAPI routes, PostgreSQL records, and
  existing runtime/provider operations remain unchanged

### Requirement: MCP scope is server-resolved and grant-bounded

Every scoped MCP operation SHALL use the authenticated principal's exact
`tenant`/`project`/`namespace` grant. A caller-provided explicit scope MAY select
one of the principal's grants; otherwise the adapter MAY use a server-owned
runtime/session active scope. Active scope MUST never widen a grant, and an
explicit authorized scope MUST take precedence over the active default.

#### Scenario: Explicit authorized scope is selected

- **WHEN** a caller supplies a tenant, project, and namespace that match an
  active exact grant
- **THEN** the tool uses that normalized scope for the operation

#### Scenario: Explicit scope is outside grants

- **WHEN** a caller supplies a valid-looking scope outside the principal's exact
  grants
- **THEN** the adapter rejects the operation before reading or writing scoped
  records and does not reveal foreign-resource existence

#### Scenario: Active scope is used as a default

- **WHEN** a caller omits explicit scope and a server-owned active runtime/session
  binding exists
- **THEN** the adapter uses that binding only after validating its grant and
  exact scope, without accepting client-side widening

#### Scenario: Active scope is unavailable

- **WHEN** a caller omits scope and no authorized active binding exists
- **THEN** the adapter returns a bounded missing-scope error and does not guess a
  project, tenant, or namespace

### Requirement: MCP tools have separate bounded contracts

The adapter SHALL expose distinct, schema-validated tools for current identity
and access (`who_am_i`), query-relevant memory search, assembled context/profile,
governed memory write/forget, and lifecycle-safe memory browsing. A tool MUST NOT
silently combine search results with profile/context sections or expose an
unbounded internal service operation.

#### Scenario: Agent requests query-relevant memory

- **WHEN** the caller invokes memory search with a bounded query and authorized
  scope
- **THEN** the response contains only governed ranked hits and citation-safe
  metadata for that scope

#### Scenario: Agent requests assembled context

- **WHEN** the caller invokes context/profile assembly with a bounded budget and
  authorized scope
- **THEN** the adapter returns the existing structured context sections and
  projection-backed profile material without changing section names or budget
  semantics

#### Scenario: Agent requests identity and grants

- **WHEN** the caller invokes `who_am_i`
- **THEN** the response reports bounded principal role, access mode, authorized
  scope summary only for an exact scope verified in this request, and
  active-scope state without claiming to enumerate all principal grants,
  credentials, or hidden records

#### Scenario: Agent browses memories

- **WHEN** the caller invokes lifecycle-safe memory browsing for one authorized
  scope
- **THEN** pagination and filtering use the existing public memory resource
  contract and omit suppressed, forgotten, expired, deleted, or out-of-scope
  content by default

### Requirement: Search and context responses preserve safety contracts

MCP search and context tools MUST preserve existing temporal selectors,
lifecycle visibility, exact scope filtering, context budgets, citations, and
projection freshness gates. Ordinary MCP responses MUST omit raw ranking scores,
candidate pools, calibration weights, feedback history, hidden identifiers,
queries, scopes, provider payloads, and diagnostic trajectories.

#### Scenario: Historical search is explicit

- **WHEN** a caller requests `as_of` or `valid_during` through the search tool
- **THEN** the adapter passes the explicit temporal constraint to the existing
  authorized temporal search contract and does not broaden current retrieval

#### Scenario: Projection is stale or foreign

- **WHEN** a context request references a missing, stale, divergent, or foreign
  projection
- **THEN** the adapter fails closed or uses the existing bounded baseline
  fallback without returning the invalid projection or widening scope

#### Scenario: Ordinary caller asks for diagnostics

- **WHEN** a non-admin MCP caller requests calibration, ranking, feedback, or
  retrieval trajectory internals
- **THEN** the adapter preserves the ordinary response contract and omits those
  internals

#### Scenario: Context budget is exceeded

- **WHEN** assembled context or citation material cannot fit the caller's
  bounded budget
- **THEN** the adapter returns the existing bounded omission behavior and does
  not increase the budget or fetch broader evidence

### Requirement: Mutable MCP operations use governed intents and lifecycle APIs

MCP write, update, remember, and forget operations SHALL delegate to governed
memory intents or existing privileged lifecycle contracts. No MCP operation MAY
write canonical memory directly, overwrite a version in place, bypass admission,
or bypass lifecycle and audit controls.

#### Scenario: Agent remembers content

- **WHEN** an authorized caller submits a memory-relevant remember request
- **THEN** the adapter records a scoped governed intent with idempotency and
  provenance metadata and returns its bounded outcome

#### Scenario: Agent requests an update

- **WHEN** an authorized caller requests a change to existing memory
- **THEN** the adapter requires the existing version/concurrency and governance
  controls and preserves prior canonical history

#### Scenario: Agent requests a lifecycle action

- **WHEN** a caller requests suppress, expire, or delete behavior
- **THEN** the adapter allows it only through the existing privileged lifecycle
  authorization and records actor, reason, operation, and timestamp audit data

#### Scenario: Caller attempts direct canonical mutation

- **WHEN** an MCP request attempts to set canonical content or lifecycle state
  outside a governed operation
- **THEN** the adapter rejects it without mutating canonical or projection data

### Requirement: Semantic bulk forgetting is preview-bound and auditable

Any MCP operation that selects memories semantically for forgetting SHALL support
a dry-run preview and SHALL require the apply step to use the caller-reviewed,
exact bounded memory-ID set returned by that preview. The apply operation MUST
remain exact-scope, idempotent, lifecycle-safe, and auditable. Preview manifests
and batch idempotency outcomes MUST survive process restart and be shared across
API replicas using PostgreSQL operational metadata, without storing memory
content.

#### Scenario: Forget preview is requested

- **WHEN** a caller requests semantic forgetting with a query, scope, threshold,
  and maximum count
- **THEN** the adapter returns bounded candidates and a preview identity without
  mutating memory or revealing out-of-scope candidates, and persists only the
  exact scope, principal, IDs, and expiry required to validate a later apply

#### Scenario: Reviewed IDs are applied

- **WHEN** a caller submits the reviewed preview IDs for application within the
  same authorized scope
- **THEN** the adapter forgets only those validated IDs, records one bounded
  audit batch, stores the batch outcome for durable retry, and returns stable
  per-item lifecycle outcomes

#### Scenario: Apply set drifts or is unbounded

- **WHEN** preview IDs are unknown, foreign, stale beyond the documented policy,
  duplicated, or exceed the configured bound
- **THEN** the adapter rejects or safely narrows the request without rerunning an
  unconstrained semantic deletion

### Requirement: MCP operations are replay-safe and errors are bounded

Mutating MCP operations SHALL accept bounded request or idempotency metadata where
the delegated contract supports it. Equivalent retries MUST return the original
durable outcome; conflicting reuse MUST return a machine-readable conflict. All
MCP errors MUST omit credentials, raw SQL, stack traces, hidden-resource
existence, provider payloads, and unbounded diagnostic details.

#### Scenario: Equivalent mutation is retried

- **WHEN** a caller retries a governed MCP mutation with the same scope,
  idempotency key, operation kind, and normalized payload
- **THEN** the adapter returns the original durable outcome without duplicate
  intent, lifecycle action, event, or audit side effect

#### Scenario: Idempotency key conflicts

- **WHEN** a caller reuses an idempotency key with a different scope, operation,
  or normalized payload
- **THEN** the adapter returns a bounded conflict and does not reveal or mutate
  the earlier operation's data

#### Scenario: Scope or lifecycle denial is returned

- **WHEN** authentication, scope, lifecycle, validation, or dependency checks
  reject an MCP request
- **THEN** the response uses a stable machine-readable category without
  disclosing foreign-resource existence or internal details

### Requirement: MCP conformance and disablement are independently verifiable

The service SHALL provide bounded conformance coverage for the enabled MCP
adapter, including capability discovery, exact-scope isolation, read-only grant
behavior, explicit-versus-active scope precedence, lifecycle filtering,
idempotent retry, preview-bound forgetting, citation safety, budget handling,
redaction, and adapter disablement. Conformance MUST not require an external
agent or mutate canonical fixture data outside governed test setup/cleanup.

#### Scenario: Conformance passes for an exact scope

- **WHEN** an authorized test runs the MCP conformance suite against one exact
  scope and compatible OpenAPI/provider contract
- **THEN** it records bounded pass/fail categories and cited evidence without
  exposing raw test content or credentials

#### Scenario: Conformance attempts a foreign scope

- **WHEN** a conformance case requests an ungranted scope or hidden memory
- **THEN** it records an isolation/lifecycle failure category, returns no foreign
  content, and leaves canonical records unchanged

#### Scenario: Adapter disablement is tested

- **WHEN** the operator disables the MCP adapter and reruns the conformance
  probe
- **THEN** MCP availability fails closed while ordinary OpenAPI/provider
  behavior and PostgreSQL state remain unchanged
