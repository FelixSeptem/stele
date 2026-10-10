## ADDED Requirements

### Requirement: Provider context accepts an explicit canonical JSON input

`POST /v1/provider/context` SHALL publish and accept a closed request envelope containing required `metadata` and `input` objects. Its input MUST use canonical snake-case properties: required nonblank `query` and positive integer `budget`; optional `path`, `path_prefix`, `include_relations`, `include_experience_insights`, `include_goal_context`, `include_diagnostics`, `include_feedback_diagnostics`, and `feedback_aware_ranking`; and the documented rejection-only `feedback_ranking_policy` string. Optional booleans MUST default to false. Path selectors MUST retain the existing normalization, mutual-exclusion, and exact-scope rules. A nonempty `feedback_ranking_policy` MUST be rejected. Undocumented fields, case aliases, missing/null required objects, malformed types, and trailing JSON MUST return a bounded validation error rather than silently selecting defaults.

#### Scenario: Documented options are applied

- **WHEN** an authorized bound runtime sends a supported-schema request with `path_prefix` and the documented boolean options
- **THEN** the operation applies those options to the existing context assembly semantics and does not discard them because of internal field spelling

#### Scenario: Minimal canonical request uses defaults

- **WHEN** the runtime sends valid metadata and an input containing only a nonblank `query` and positive `budget`
- **THEN** the operation uses the documented false defaults and the binding's scope/session attribution

#### Scenario: Invalid path or budget is submitted

- **WHEN** a request supplies conflicting path selectors, an invalid path, a missing query, or a nonpositive or noninteger budget
- **THEN** the service returns the documented bounded validation error and does not return a successful context result

#### Scenario: Unsupported policy is submitted

- **WHEN** a request includes `role`, `reference_only`, a digest/continuation field, or a nonempty `feedback_ranking_policy`
- **THEN** the service rejects the request explicitly without interpreting it as an authorization grant or applying an alternate retrieval policy

#### Scenario: An undocumented case alias is submitted

- **WHEN** a runtime sends `PathPrefix`, `IncludeRelations`, or another noncanonical spelling at the public context request boundary
- **THEN** the service returns a bounded validation error and does not accept the alias as the documented option

#### Scenario: Malformed envelope is submitted

- **WHEN** the request omits or nulls `metadata` or `input`, supplies a scalar where an object or boolean is required, adds an unknown envelope field, or contains trailing JSON
- **THEN** the service returns a bounded validation error without echoing submitted content or internal decoding details

### Requirement: Provider context attribution remains server owned

Provider context MUST derive tenant, project, namespace, agent, session, and provider-instance attribution from authenticated grants and the validated runtime binding. Context input MUST NOT expose caller-owned scope, session, or user overrides. Supported path and diagnostic selectors MUST NOT widen access, confer admin privileges, or bypass governed-operation precedence, lifecycle, freshness, or optional-section authorization.

#### Scenario: Caller submits an attribution override

- **WHEN** a context input includes `scope`, `session_id`, or `user_id`
- **THEN** the service rejects the unsupported input and does not use it to select another scope or session

#### Scenario: Binding or scope headers are foreign

- **WHEN** a runtime supplies another principal's binding or mismatched exact-scope/session headers
- **THEN** the existing authentication/scope gate rejects the operation before scoped data access and reveals no foreign record existence

#### Scenario: Public runtime requests privileged diagnostics

- **WHEN** an ordinary public runtime enables diagnostic flags without privileged diagnostic authorization
- **THEN** it receives no privileged feedback history, ranking plan, raw score, candidate-pool, or hidden-identifier detail

#### Scenario: Goal opt-in lacks policy authorization

- **WHEN** a runtime enables `include_goal_context` but the existing exact-scope goal visibility gates do not pass
- **THEN** `goal_context` is absent and ordinary categorized context remains governed by its existing contract

### Requirement: Provider context returns a typed categorized result

The service SHALL publish a dedicated Provider context response schema with required `metadata`, `result`, and outer Provider `citations`. `result` MUST contain array-valued `profile`, `recent_session`, `recent_episodes`, `relevant_summaries`, `related_entities`, and memory `citations`, including empty arrays when no eligible evidence exists. Memory items MUST expose explicitly documented memory identity, authorized scope, path, class, lifecycle, content, timestamp/temporal metadata, and evidence citations without raw ranking scores. The existing optional governed insight, goal, and authorized bounded diagnostic sections MUST be explicitly typed and retain their existing visibility semantics. Outer Provider citations MUST use the existing bounded citation contract; their relationship to inner memory citations and configured truncation MUST be documented. No synthetic `references`, flat `content`, context digest, or trusted-content guarantee SHALL be implied by this result.

#### Scenario: Nonempty context is returned

- **WHEN** the existing assembler selects authorized active evidence for a valid context request
- **THEN** the response conforms to the documented categorized schema, preserves selected order/identity/path/content, and includes the applicable evidence citations

#### Scenario: No eligible evidence is returned

- **WHEN** a valid request finds no lifecycle-visible matching context
- **THEN** the service returns HTTP 200 with every required result section and both required citation collections represented as empty arrays

#### Scenario: Internal scores are present

- **WHEN** selected internal context hits contain lexical, semantic, relation, or overall scores
- **THEN** the Provider context JSON omits those scores and other internal ranking fields from every result section

#### Scenario: Optional sections are returned

- **WHEN** an explicitly requested insight or goal section passes its existing visibility gates, or a diagnostic section is authorized
- **THEN** that section conforms to its documented public schema without raw feedback, hidden evidence, internal query/plan text, or unbounded provider payloads

#### Scenario: Outer citation summary reaches its limit

- **WHEN** selected context contains more evidence than the configured outer Provider citation limit permits
- **THEN** the outer summary respects that existing limit, the documented inner/outer distinction remains valid, and no additional source access is authorized by the summary

### Requirement: Provider context compatibility is explicit

The repaired context contract SHALL retain supported `provider-v1` canonical request semantics, configured schema negotiation, operation identity, and existing capability/build/OpenAPI-digest discovery. The service MUST reject unsupported schema versions before assembler dispatch with the bounded compatibility outcome. Documentation MUST identify the migration from undocumented Go-name/case fields and accidentally serialized scores, and MUST distinguish categorized context from a consumer expecting `references/content`. PC1 conformance MUST NOT imply new disclosure/trust, complete serialized-response budgeting, digest, continuity, or reference-durability capabilities.

#### Scenario: Supported client uses canonical fields

- **WHEN** a client sends the documented minimal or optional-field request using a configured supported Provider schema
- **THEN** the repaired service accepts the shape and returns the typed categorized result identified by its published OpenAPI document

#### Scenario: Schema is unsupported

- **WHEN** the request metadata names an unsupported Provider schema version
- **THEN** the operation returns the existing bounded compatibility error and does not call the context assembler

#### Scenario: Consumer discovers the repaired revision

- **WHEN** a client reads capability/version metadata and the runtime OpenAPI document
- **THEN** it can identify the supported schema, running build, updated document digest, canonical input, typed output, and documented migration limitations without mistaking the document digest for a context digest
