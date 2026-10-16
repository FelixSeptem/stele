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

### Requirement: Digest and continuity conformance verifies fresh public projections

Provider conformance SHALL validate actual serialized public context responses and discovery against authoritative OpenAPI and supported digest/continuity versions. It MUST independently verify canonical digest using authenticated binding scope and normalized original input, mandatory closed fields, reference/expiry formats, input/output union correspondence, matching citation summaries and exact whole-body budget accounting. Fixtures MUST cover both modes, empty/nonempty results, Unicode/escapes, duplicate placements, order/version/path/content/provenance/policy changes, equivalent effective limits and volatile metadata exclusion. Missing, malformed, forged or incompatible digest/continuity metadata MUST produce failed-contract rather than an empty/unchanged success. Receipt authority, changed-request, foreign/unknown/expired/retention-gap, unsupported-contract, dependency and capacity cases MUST be tested. Reports MUST remain bounded/redacted and distinct from ranking metrics; validation MUST NOT rerank or locally pack results.

#### Scenario: Service returns a plausible but incorrect digest

- **WHEN** a response has the documented shape but its canonical semantic projection does not hash to its supplied value
- **THEN** conformance records failed-contract regardless of its item count or HTTP status

#### Scenario: Compare response contradicts the request

- **WHEN** a compare request receives not-requested/issued metadata, malformed expiry, a missing reference or an unsupported digest version
- **THEN** the consumer rejects it rather than treating zero decoded context as unchanged success

#### Scenario: Receipt overhead is omitted from accounting

- **WHEN** a success body adds digest/continuity fields but its response usage excludes their serialized bytes
- **THEN** conformance records an exact-body accounting failure

### Requirement: Continuity live evidence proves durable restart and current authorization

PC4 integration evidence SHALL use a pinned service revision/image, fresh owned PostgreSQL/pgvector, official exact-scope bootstrap and disclosure grants, public labeled ingestion and database-proven governance completion. It MUST prove supported digest determinism, issue/compare in both modes, non-root exact/prefix and empty projections, retained metadata-only receipt durability and same-binding comparison after API/worker restart. Changed source versions/lifecycle, grant downgrade/revocation, expiry/unavailable references and bounded admission/error behavior MUST be verified through the public contract; SQL checks MUST supplement rather than replace public writes/reads. Focused transaction/concurrency/capacity/upgrade evidence MUST be labeled separately. Required missing/skipped/stale/degraded evidence MUST block PC4 readiness; historical PC1–PC3 artifacts MUST NOT substitute for a fresh PC4 run. Artifacts MUST contain only bounded outcome and service/image/OpenAPI/contract provenance, never credentials, raw content/payloads, scope/memory identities or raw references/digests.

#### Scenario: Fresh receipt survives API and worker restart

- **WHEN** public ingestion completes governance, a reference is issued and same-binding comparison passes after the owned stack restarts
- **THEN** evidence records durable fresh-compare behavior and compatible digest equality without claiming exact snapshot replay

#### Scenario: Source changes after the original projection

- **WHEN** public governed source changes complete and comparison runs with the original read/reference
- **THEN** evidence records the fresh changed result and verifies removed/ineligible evidence is absent

#### Scenario: Previous live evidence is offered as completion

- **WHEN** only PC3 or in-process fixtures exist without PC4 receipt/restart/current-grant evidence
- **THEN** readiness remains incomplete rather than inheriting an earlier context pass

### Requirement: Digest continuity documentation and migration are acceptance gates

PC4 acceptance MUST include matching OpenAPI, README, Provider integration, configuration/self-host/bootstrap, architecture/components/best-practice, affected MCP and agent-skill documentation in the same implementation. Documents MUST describe canonical identity and vectors, all discovery/input/output versions, fresh versus compare versus unsupported snapshot/watermark, reference-not-credential semantics, expiry/caps/cleanup/errors, byte-envelope overhead, strict-consumer migration, additive schema upgrade and safe rollback. Context/schema/sync digests MUST be distinguished; external tool/write/business policy and any child projection fingerprint MUST remain external and separately named. PC4 completion MUST NOT imply PC5 durable-reference or PC6 outcome workflow completion.

#### Scenario: Consumer follows maintained integration examples

- **WHEN** a runtime adopts the revised contract
- **THEN** examples require extension discovery, complete mode/accounting/digest/continuity validation and fresh current authorization without local selection or historical-body reuse

#### Scenario: Operator rolls back to a binary without PC4

- **WHEN** an operator follows maintained rollback instructions
- **THEN** PC4 consumers are disabled or receive compatibility failure, existing disclosure/budget protection and durable data are retained and unsupported continuity is not silently stripped

### Requirement: Provider conformance proves durable reference readiness

The Provider conformance suite SHALL include supported reference writes,
governance status, exact-path reads, identical/conflicting replay, non-root path
and UUID lineage checks, lifecycle removal, foreign-scope safety and API/worker
restart recovery. Results SHALL remain separate from ranking metrics.

#### Scenario: Fresh public reference round trip passes
- **WHEN** a fresh PostgreSQL + pgvector fixture uses official bootstrap and the
  public Provider API to write, govern and read a task/run or compilation
  reference before and after restart
- **THEN** the report records a passing durable-reference outcome with bounded
  provenance and no raw credential, scope or payload disclosure

#### Scenario: HTTP acceptance lacks durable completion
- **WHEN** an event is accepted but governance is pending, failed, or missing
  after the bounded observation window
- **THEN** conformance records a nonpassing dependency/contract outcome

### Requirement: Reference evidence records upstream provenance

Durable-reference evidence SHALL pin service/image, OpenAPI, Provider schema,
migration and fixture provenance and SHALL redact credentials, authorization
headers, raw values, scope values and unbounded payloads.

#### Scenario: Evidence artifact is inspected
- **WHEN** a maintainer reviews the durable-reference report
- **THEN** it contains bounded outcome categories, counts, hashes where needed
  and provenance sufficient to reproduce the fixture without secret material

### Requirement: Strict consumers negotiate and validate projection evidence

Strict runtime-memory Provider consumers SHALL negotiate
`turn-outcome-feedback-projection-v1`, validate closed serialized request and
response DTOs, distinguish accepted/pending/completed/failed/unavailable
reference attribution and correction dispositions, and reject incompatible or
missing required fields without interpreting evidence as runtime business state.

#### Scenario: Consumer observes a pending durable reference
- **WHEN** a valid projection response reports pending reference attribution
- **THEN** the consumer records it as pending evidence and does not treat it as
  a completed memory reference, task completion or ranking input

#### Scenario: Consumer receives malformed projection output
- **WHEN** required identity, revision, attribution state or business-state
  disclaimer fields are missing, null, aliased or incompatible
- **THEN** the consumer reports a failed contract outcome without local fallback
