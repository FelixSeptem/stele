# runtime-memory-provider-contract Specification

## Purpose
Validate offline replay of supported agent memory-provider operation contracts.

## Requirements

### Requirement: BFCL memory operations replay offline
The system SHALL support offline replay of the BFCL-v4 `memory_kv`, `memory_rec_sum`, and `memory_vector` operation subsets or equivalent checksum-locked contract fixtures without requiring a remote model, search service, or judge.

#### Scenario: Replay a valid memory operation
- **WHEN** a contract fixture contains a valid memory read/write/search/update operation
- **THEN** the runner validates the operation name, arguments, expected scope, and result shape and records an operation-level outcome

#### Scenario: Handle malformed or irrelevant operations
- **WHEN** an operation has malformed arguments or is irrelevant to the supplied context
- **THEN** the runner records a contract failure or correct refusal without converting the case into a successful memory result

### Requirement: Provider contract preserves scope and lifecycle controls
The contract runner SHALL pass project, tenant, namespace, session, and lifecycle expectations through every memory operation and SHALL detect cross-scope or hidden-memory access.

#### Scenario: Reject a cross-tenant memory call
- **WHEN** a replayed operation requests a memory outside the run tenant
- **THEN** the operation fails with a scope-safety outcome and no foreign memory is returned

#### Scenario: Exclude forgotten memory by default
- **WHEN** a replayed search targets a forgotten, suppressed, or deleted record without an explicit debug allowance
- **THEN** the provider returns no hidden record and the report records zero must-not-return violations

### Requirement: Contract metrics remain separate from retrieval ranking
The system SHALL report operation accuracy, malformed-call rate, refusal correctness, scope-safety failures, and lifecycle-safety failures under a provider-contract family identity and SHALL NOT merge them into Recall@k, MRR, or nDCG.

#### Scenario: Produce a contract report
- **WHEN** all selected BFCL memory cases finish
- **THEN** the report contains family identity, subset counts, operation metrics, safety outcomes, and artifact provenance independent of retrieval reports

### Requirement: Provider conformance covers governed memory intent operations
The provider conformance suite MUST include the supported intent operations and scoped status/history reads as contract cases, while preserving the existing event, retrieval, context, and lifecycle operation families.

#### Scenario: Provider replays a governed intent
- **WHEN** a conformance fixture submits a valid intent and repeats the same request after a runtime restart
- **THEN** the provider returns the original intent identity and bounded replay outcome without creating a duplicate request or transition

#### Scenario: Provider rejects an unsafe intent operation
- **WHEN** a fixture uses a foreign scope, conflicting idempotency payload, unsupported type, missing attribution, or hidden lifecycle target
- **THEN** the provider returns a bounded contract failure and does not expose foreign content or mutate canonical memory directly

### Requirement: Provider conformance proves intent processing dependencies
An intent conformance result MUST remain ineligible for readiness when the durable queue, worker recovery, migration compatibility, or required policy dependency is missing, stale, or degraded.

#### Scenario: Intent dependency is degraded
- **WHEN** the provider can accept an intent request but the durable worker or compatible policy cannot process it
- **THEN** the conformance report records a dependency or readiness failure and does not claim provider readiness

#### Scenario: Intent processing recovers after restart
- **WHEN** the provider conformance runner restarts the affected runtime after durable intent handoff
- **THEN** it observes recovery through the same intent identity and records the replay, lease, and lifecycle result separately from retrieval accuracy metrics

### Requirement: Offline provider contract replay covers synchronization

The provider contract runner SHALL support offline fixtures for capability synchronization metadata, initial snapshot, ordered event batches, cursor continuation, `sync_complete`, duplicate retry, and `resync_required` recovery without requiring a remote agent, model, or streaming transport.

#### Scenario: Replay a valid synchronization fixture

- **WHEN** a fixture contains a supported exact scope, snapshot watermark, ordered events, and acknowledged cursor
- **THEN** the runner validates event identities, ordering, completion, scope, and bounded response shape and records an operation-level result

#### Scenario: Replay a retention-gap fixture

- **WHEN** a fixture presents an expired cursor
- **THEN** the runner records the required resynchronization outcome and does not treat skipped events as a successful synchronized state

### Requirement: Synchronization safety remains separate from retrieval quality

Synchronization operation accuracy, replay determinism, cursor recovery, retention-gap handling, scope safety, lifecycle safety, and redaction outcomes SHALL be reported under the provider-contract family and MUST NOT be merged into retrieval Recall@k, MRR, nDCG, or ranking rollout evidence.

#### Scenario: Produce a synchronization contract report

- **WHEN** synchronization fixtures finish
- **THEN** the report contains contract family identity, snapshot and replay counts, recovery outcomes, safety failures, and artifact provenance independent of retrieval metrics

### Requirement: Provider context conformance validates public JSON and result shape

The Provider contract suite SHALL validate canonical context request examples and actual serialized public-handler responses against the authoritative OpenAPI contract. It MUST distinguish a valid nonempty categorized result, a genuine empty result, incompatible/malformed results, unsupported input/schema, and scope/lifecycle denial. Missing required result sections, null/scalar sections, invalid item/citation types, or a `references/content` substitute MUST produce a failed-contract outcome rather than an empty success. Findings and artifact provenance MUST remain bounded and redacted, and context contract outcomes MUST remain separate from retrieval ranking metrics.

#### Scenario: Public route accepts documented options

- **WHEN** the suite submits canonical snake-case path, relation, insight, goal, diagnostic, and feedback-ranking options through the authenticated Provider HTTP route
- **THEN** it verifies their intended existing behavior and the serialized response schema rather than constructing a passing request from internal Go field names

#### Scenario: Valid nonempty result is consumed

- **WHEN** the suite validates a typed result containing authorized selected memory items and citations
- **THEN** it records a context-contract pass with bounded attribution and does not locally rerank or repack memory

#### Scenario: Genuine empty result is consumed

- **WHEN** the response has an empty array for every required categorized section and citation collection
- **THEN** the suite records a valid empty context outcome distinct from an incompatible or malformed response

#### Scenario: Required result shape is missing

- **WHEN** a response lacks `result` or a required section, uses null/scalar sections, has invalid item/citation types, or substitutes `references/content`
- **THEN** the suite records a failed-contract outcome and never treats zero decoded items as proof of a successful empty projection

#### Scenario: Unsupported field or version is refused

- **WHEN** a fixture submits an unsupported policy field, undocumented case alias, invalid request value, or unsupported Provider schema
- **THEN** the suite checks the documented bounded validation/compatibility outcome and records failure if the service silently accepts or rewrites the request

#### Scenario: Foreign or hidden evidence is requested

- **WHEN** a fixture uses a foreign binding/session/scope or targets suppressed, forgotten, deleted, or out-of-scope evidence
- **THEN** it verifies denial or lifecycle-safe omission with no leaked content/identifiers and records any violation as a contract safety failure

#### Scenario: Raw ranking data leaks in an optional section

- **WHEN** serialized context includes raw scores, internal plans, feedback history, hidden identifiers, or unbounded payloads in any section
- **THEN** the suite records a redaction contract failure regardless of whether the primary memory sections passed

### Requirement: Public context integration evidence is fresh and exact scoped

PC1 public integration evidence SHALL use a pinned repaired service revision and a fresh owned PostgreSQL/pgvector fixture, official principal/runtime bootstrap, and ordinary governed public ingestion. The verifier MUST establish existing governance completion, check a non-root exact path and explicit prefix through Provider context, validate nonempty and empty responses, and repeat eligible reads after API/worker restart. Missing, stale, skipped, or degraded dependencies/evidence MUST prevent a passing integration conclusion. Evidence MUST identify the service/OpenAPI/Provider schema provenance using bounded redacted fields and MUST NOT retain credentials, authorization headers, raw payloads, or unbounded content. This contract MUST NOT be interpreted as implementing PC3 budget redesign or PC5's expanded durable reference API.

#### Scenario: Fresh context fixture passes

- **WHEN** public fixture ingestion completes existing governance and canonical evidence is read through documented Provider context JSON in the same exact scope before and after restart
- **THEN** the verifier records a passing context integration result with non-root path selection, typed responses, citations, and bounded version provenance

#### Scenario: Write acceptance lacks governance completion

- **WHEN** public ingestion returns success but required governed evidence is missing or incomplete within the verifier's bounded observation window
- **THEN** the verifier records incomplete/degraded integration evidence and does not claim readiness from write acceptance or a transient retrieval result

#### Scenario: Fixture dependencies or shape are missing

- **WHEN** official runtime bootstrap, fresh PostgreSQL/pgvector, required restart evidence, or valid typed response evidence is unavailable
- **THEN** the verifier preserves a nonpassing integration outcome rather than substituting an in-memory or direct-database fixture
