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

### Requirement: Disclosure conformance checks serialized mode and alternate-route behavior

Provider contract conformance SHALL validate canonical disclosure inputs and actual serialized public responses against the mode-specific OpenAPI schema. It MUST distinguish authorized nonempty/empty references, authorized bounded content, missing/incompatible result shapes, unsupported labels/modes, profile denial, and dependency failure. It MUST test reference-mode text absence and independently authorized citations across optional sections, alternate native/Provider/MCP/sync routes, write/replay echoes, grant changes using existing bindings, and foreign/hidden/high/restricted sources. Outcomes MUST remain bounded/redacted and separate from ranking quality metrics.

#### Scenario: Text exists in every optional source section

- **WHEN** reference-only fixtures contain marked content, title, summary, lesson, goal, metadata, and diagnostic text
- **THEN** the serialized response passes the closed reference schema and exposes none of the prohibited fields/values

#### Scenario: Consumer assumes reference response contains content

- **WHEN** a content consumer receives reference items or a response lacks required mode/disclosure/category metadata
- **THEN** conformance records an incompatible/failed-contract outcome rather than a successful empty content projection

#### Scenario: Restricted credential tries alternate reads or replay echoes

- **WHEN** fixtures request retrieve/sync/native/MCP content or replay a content-bearing write using a constrained credential
- **THEN** conformance verifies equivalent permitted disclosure or a bounded refusal/safe acknowledgment and records any content or identifier leak as a safety failure

### Requirement: Public disclosure and trust evidence proves durable governance and restart

PC2 live integration evidence SHALL use a pinned service revision, fresh owned PostgreSQL/pgvector, official bootstrap and admin grant provisioning, public labeled ingestion, bounded governance completion observation, and exact/prefix Provider context in the same scope. It MUST verify stored source labels and non-root path at candidate/canonical/version boundaries, replay/conflict behavior, content/reference modes, grant downgrade/revocation, and lifecycle/foreign-source refusal before and after API/worker restart. Required unsupported, skipped, stale, or degraded evidence MUST prevent a passing conclusion. Experimental derivation tests MUST be identified separately when only focused domain/repository integration evidence is available, without claiming public live coverage.

#### Scenario: Fresh profiled runtime passes after restart

- **WHEN** public ingestion completes governance and authorized content/reference reads, replay, grant changes, and safety checks pass before and after restart
- **THEN** evidence records a passing disclosure result with bounded service/image/schema/policy/source-metadata provenance and no stored credentials or raw content

#### Scenario: HTTP succeeds but durable labels are missing

- **WHEN** ingestion/context succeeds but governance completion or required candidate/canonical/version label evidence is missing
- **THEN** the verifier records a nonpassing durable integration result and does not claim PC2 completion

#### Scenario: Mixed-source derivation loses restrictive labels

- **WHEN** a summary, insight, chunk, or projection combines differently labeled sources
- **THEN** focused integration evidence verifies the restrictive trust/classification join and records a contract failure if derivation upgrades trust or weakens classification

### Requirement: Disclosure rollout and maintenance documentation are part of acceptance

PC2 acceptance MUST include matching OpenAPI, README, Provider and admin/bootstrap/self-host integration, source-lineage/governance, best-practice, MCP, and agent-skill documentation for changed semantics. Documentation MUST describe capability negotiation, explicit provisioning, reference-only defaults, mode-specific response consumption, classification/trust defaults, unsupported endpoint denials, external business permission ownership, and safe rollback. Historical PC1 success MUST NOT be treated as PC2 readiness or completion of PC3–PC6.

#### Scenario: Migration consumer reads capabilities

- **WHEN** a consumer prepares to use the revised disclosure contract
- **THEN** maintained examples require `context-disclosure-v1` discovery and valid grants, explain reference/content response validation, and forbid silent retry or fallback to broader disclosure

#### Scenario: Operator rolls back to a binary without enforcement

- **WHEN** an operator follows the documented rollback path
- **THEN** affected constrained credentials are denied or disabled across all public content surfaces before reverting, and durable labels/grants/history are retained

### Requirement: Budget conformance validates actual serialized responses

Provider contract conformance SHALL validate canonical allocation requests and actual public HTTP bodies against authoritative OpenAPI and `context-budget-v1`. It MUST cover omitted, zero, minimum, oversized, invalid and overflowing limits; both modes; multibyte/escaped content; paths/source metadata; required and repeated citations; accounting self-size; and optional-section gates. Missing/malformed accounting, impossible usage, over-limit bodies, incompatible versions or false empty results MUST produce failed-contract outcomes. Safety cases MUST include foreign scope, hidden lifecycle, restrictive classification, current grant changes and dependency failures. Contract reports MUST remain bounded/redacted and separate from ranking metrics; deterministic ample-budget ordering/protected recall and tight-budget admission regressions MUST be verified separately.

#### Scenario: Body is apparently small but accounting is wrong

- **WHEN** a response fits transport space but reports characters as UTF-8 bytes, omits accounting, or reports a response size different from the captured body
- **THEN** conformance records a failed contract rather than accepting an approximate result

#### Scenario: Allocation is infeasible

- **WHEN** a fixture requests a limit smaller than the mandatory empty response
- **THEN** conformance verifies the bounded non-success with no partial projection instead of treating zero decoded items as a pass

#### Scenario: Hidden candidates would leak diagnostics

- **WHEN** fixtures include foreign, suppressed, forgotten, deleted, high/restricted or source-ineligible records
- **THEN** conformance verifies no protected content, identity, private counter or evidence-existence reason in final items or accounting

### Requirement: Budget integration proves fresh durable initial and restart behavior

PC3 integration evidence SHALL use a pinned service revision/image and fresh owned PostgreSQL/pgvector fixture with official bootstrap, disclosure grants, public governed ingestion, bounded governance completion checks, and exact-scope non-root exact/prefix context reads before and after API/worker restart. Both modes MUST satisfy actual body/content/item/citation bounds, required evidence, empty/omitted/error distinctions and lifecycle/scope safety. Missing, stale, skipped or degraded dependencies MUST prevent a passing result. Unsupported public sections MUST be tested as unsupported; focused internal section tests MUST be labeled separately. Artifacts MUST contain bounded service/image/OpenAPI/disclosure/budget-version provenance and outcomes without credentials, raw payloads, content, scope values or memory identities.

#### Scenario: Fresh fixture passes and survives restart

- **WHEN** governance completes and authorized content/reference allocations and safety checks pass before and after API/worker restart
- **THEN** evidence records a passing budget integration result with actual measured bounded usage and declared contract provenance

#### Scenario: Only write receipt or transient lookup succeeds

- **WHEN** HTTP accepts an event but durable governance or required restart evidence is incomplete
- **THEN** the evidence remains nonpassing and does not claim PC3 readiness

### Requirement: Budget maintenance documentation is an implementation gate

PC3 acceptance MUST include matching OpenAPI, README, Provider integration, configuration/self-host/architecture/component/best-practice, affected MCP, and agent-skill documentation in the same change. Documents MUST state units, independent defaults/caps, omitted versus zero limits, exact complete-body and repeated-evidence accounting, bounded reservation, validation/errors, current authorization and unsupported-section behavior, explicit extension discovery, strict response consumption, migration and rollback. Danny's business/tool policy and remaining prompt allocation MUST stay external. Historical PC1/PC2 evidence MUST NOT count as PC3 readiness; PC3 completion MUST NOT imply PC4–PC6 completion.

#### Scenario: Consumer allocates memory space

- **WHEN** a consumer follows maintained integration examples
- **THEN** it reserves its other prompt components, discovers the budget extension, sends explicit unit limits, validates usage/mode/result shape, and performs no local ranking/packing or byte-to-token conversion

#### Scenario: Service lacks the required extension

- **WHEN** a PC3 consumer discovers an older service or follows documented rollback
- **THEN** the integration fails compatibility or disables the affected access before rollback, retaining PC2 authority enforcement and durable data rather than silently stripping limits
