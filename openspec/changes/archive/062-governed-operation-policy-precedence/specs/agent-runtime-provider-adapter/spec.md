## MODIFIED Requirements

### Requirement: Runtime scope is resolved and cannot be widened by callers

The provider SHALL resolve runtime scope from authenticated principal grants and server-owned agent/session context. Every scoped operation MUST carry or reference the resolved tenant, project, namespace, agent, session, and provider-instance identity, and the service MUST apply the governed-operation precedence contract by rejecting missing, mismatched, caller-invented, or widened scope before lifecycle, policy, idempotency, provider, or memory access.

#### Scenario: Runtime initializes within an exact grant

- **WHEN** an authenticated principal initializes a provider runtime for an authorized exact scope and agent identity
- **THEN** the service returns a stable runtime scope descriptor and session binding that subsequent operations can reference

#### Scenario: Caller widens namespace or tenant

- **WHEN** a subsequent provider operation supplies scope values outside the server-resolved grant or runtime binding
- **THEN** the service rejects the operation at the scope gate before accessing lifecycle, policy, idempotency, or scoped records and does not disclose whether foreign records exist

#### Scenario: Session and agent identity remain distinct

- **WHEN** multiple sessions use the same agent identity or one session is recreated
- **THEN** the service preserves separate session/conversation attribution while retaining the same authorized canonical scope rules

### Requirement: Provider composes governed memory operations without bypasses

The provider SHALL expose bounded operations for raw event ingest, governed memory intents, retrieval, context assembly, forgetting/lifecycle requests, and scoped status/report inspection by delegating to existing service contracts. Each operation MUST reuse the governed-operation precedence decision and MUST NOT permit direct canonical-memory writes, bypass admission or governance, execute an external agent, or invoke a model.

#### Scenario: Agent submits a memory-relevant event

- **WHEN** a runtime submits a valid event through the provider
- **THEN** the service applies the shared scope, lifecycle, grant, policy, replay, handoff, and mutation boundaries before ordinary event-to-candidate-to-active governance behavior

#### Scenario: Agent requests context

- **WHEN** a runtime requests retrieval or assembled context for its resolved session
- **THEN** the provider applies lifecycle-safe defaults, projection freshness gates, bounded budgets, and exact-scope filtering after the same precedence decision

#### Scenario: Caller attempts canonical mutation

- **WHEN** a provider operation attempts to overwrite canonical memory or set lifecycle state directly
- **THEN** the service rejects the operation at the governance boundary and requires the governed intent or admin lifecycle contract
