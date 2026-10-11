# agent-runtime-provider-adapter Specification

## Purpose
Provide external agent runtimes with a discoverable, scope-safe memory-provider contract that composes Stele's existing governed APIs, preserves citations and provenance, and remains replay-safe without executing the agent or creating a second source of record.

## Requirements

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

### Requirement: Provider exposes resumable synchronization

The provider adapter SHALL expose a bounded synchronization operation that uses the authenticated runtime binding, supports initial snapshot and cursor-based continuation, and returns machine-readable completion, retry, compatibility, scope, and resynchronization outcomes.

#### Scenario: Provider starts a synchronization

- **WHEN** a bound runtime requests synchronization with a supported schema version
- **THEN** the adapter returns the scoped snapshot or next event batch with the current cursor and completion state

#### Scenario: Provider reports a retention gap

- **WHEN** the runtime cursor is older than the provider's retained replay window
- **THEN** the adapter returns the provider error category for resynchronization and does not claim that the runtime is synchronized

### Requirement: Provider synchronization preserves runtime identity

Synchronization requests SHALL retain the provider binding's tenant, project, namespace, agent, session, and provider-instance identity across initial sync, continuation, and retry. The adapter MUST reject cursor reuse with another binding or scope.

#### Scenario: Cursor is reused by another session

- **WHEN** a runtime presents a cursor issued to a different session or provider instance
- **THEN** the adapter returns a bounded scope or compatibility error and does not reveal the cursor owner's state

### Requirement: Provider conformance verifies synchronization recovery

The provider conformance profile SHALL include initial snapshot, ordered replay, cursor acknowledgment, duplicate retry, restart recovery, retention-gap resynchronization, scope rejection, schema incompatibility, redaction, and transport-neutral equivalence cases. A failed or incomplete synchronization case MUST make the conformance result ineligible for provider readiness.

#### Scenario: Conformance passes synchronization recovery

- **WHEN** all required synchronization fixtures complete with deterministic replay and exact-scope evidence
- **THEN** the conformance report records synchronization compatibility and recovery evidence as separate bounded categories

#### Scenario: Conformance detects nondeterministic replay

- **WHEN** the same binding and cursor produce different event identity, ordering, or completion results across replay
- **THEN** the conformance run records a synchronization failure and does not claim provider readiness

### Requirement: Provider context accepts an explicit canonical JSON input

`POST /v1/provider/context` SHALL publish and accept a closed request envelope containing required `metadata` and `input` objects. Its input MUST use canonical snake-case properties: required nonblank `query` and positive integer `budget`; optional `path`, `path_prefix`, `include_relations`, `include_experience_insights`, `include_goal_context`, `include_diagnostics`, `include_feedback_diagnostics`, and `feedback_aware_ranking`; the documented rejection-only `feedback_ranking_policy` string; and optional `projection_intent`, `disclosure_profile`, and `response_mode`. Optional booleans MUST default to false. Projection intent MUST be one of `conversation`, `task_context`, `compilation`, or `handoff` and default to `conversation`; it MUST NOT grant business or data authority. The profile MUST be an exact supported versioned registry identifier of at most 64 ASCII token bytes and default to the current grant's configured profile. Response mode MUST be `reference_only` or `bounded_content`, defaulting to `reference_only`. Profile/mode authorization MUST use current exact-scope disclosure entitlements. Path selectors MUST retain existing normalization, mutual-exclusion, and exact-scope rules. A nonempty `feedback_ranking_policy` MUST be rejected. Undocumented fields, case aliases, duplicate keys, null values, missing required objects, malformed types, invalid enums, and trailing JSON MUST return a bounded validation error rather than silently selecting defaults.

#### Scenario: Documented options are applied

- **WHEN** an authorized bound runtime sends a supported-schema request with `path_prefix` and the documented boolean/disclosure options
- **THEN** the operation applies those options under current disclosure authorization and existing context assembly semantics without discarding them because of internal field spelling

#### Scenario: Minimal canonical request uses defaults

- **WHEN** the runtime has a valid disclosure grant and sends metadata plus an input containing only a nonblank `query` and positive `budget`
- **THEN** the operation uses false boolean defaults, conversation intent, the current grant's default profile, reference-only mode, and binding-owned scope/session attribution

#### Scenario: Invalid path or budget is submitted

- **WHEN** a request supplies conflicting path selectors, an invalid path, a missing query, or a nonpositive or noninteger budget
- **THEN** the service returns the documented bounded validation error and does not return a successful context result

#### Scenario: Unsupported policy is submitted

- **WHEN** a request includes `role`, bare `reference_only`, a digest/continuation field, or a nonempty `feedback_ranking_policy`
- **THEN** the service rejects it explicitly without interpreting it as authority or applying an alternate retrieval policy

#### Scenario: An undocumented case alias is submitted

- **WHEN** a runtime sends `PathPrefix`, `IncludeRelations`, `ResponseMode`, or another noncanonical spelling at the public context boundary
- **THEN** the service returns a bounded validation error and does not accept the alias as the documented option

#### Scenario: Malformed envelope is submitted

- **WHEN** the request omits/nulls required objects, supplies malformed scalar types, adds unknown fields, repeats a key, or contains trailing JSON
- **THEN** the service returns a bounded validation error without echoing submitted content or internal decoding details

#### Scenario: Unknown or unauthorized profile is requested

- **WHEN** a syntactically valid request names an unknown or unauthorized profile or a known mode outside that profile's entitlement
- **THEN** the service returns the same bounded 403 disclosure denial without protected content or record-existence details

#### Scenario: Disclosure authority is absent

- **WHEN** an otherwise valid runtime has no configured current disclosure entitlement
- **THEN** the context operation is denied rather than defaulting to legacy content

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

The service SHALL publish a dedicated mode-discriminated Provider context response schema with required `metadata`, `result`, and outer Provider `citations`. `result` MUST contain `response_mode`, effective versioned `disclosure` metadata, and array-valued `profile`, `recent_session`, `recent_episodes`, `relevant_summaries`, `related_entities`, and memory `citations`, including empty arrays when no eligible evidence exists. In `reference_only` mode categorized items MUST use the closed reference/citation shape without text-bearing memory or insight objects, and optional text/diagnostic/goal sections MUST NOT disclose text. In `bounded_content` mode authorized memory items MUST expose explicitly documented identity, scope, path, class, lifecycle, content, timestamp/temporal metadata, typed source metadata, and evidence citations without raw ranking scores. Optional governed insight, goal, and independently authorized diagnostic sections MUST be explicitly typed, retain their existing visibility gates, and respect source/disclosure eligibility. Outer and inner citations MUST derive only from finally returned authorized items and independently authorized sources. The configured outer citation summary limit and inner/outer distinction MUST be documented. No flat `references/content`, context digest, instruction-authority guarantee, or complete caller budget-unit redesign SHALL be implied by this result.

#### Scenario: Nonempty context is returned

- **WHEN** the existing assembler selects authorized active evidence and content mode is explicitly requested and authorized
- **THEN** the response conforms to the categorized content schema, preserves selected order/identity/path/content, and includes typed effective source metadata and authorized evidence citations

#### Scenario: No eligible evidence is returned

- **WHEN** a valid authorized request finds no disclosable matching context
- **THEN** HTTP 200 contains the effective mode/disclosure metadata and every required category and citation collection as empty arrays

#### Scenario: Internal scores are present

- **WHEN** selected internal context hits contain lexical, semantic, relation, or overall scores
- **THEN** Provider context omits those scores and other internal ranking fields from every mode and section

#### Scenario: Optional sections are returned

- **WHEN** an explicitly requested insight/goal section passes existing and disclosure/source gates in content mode, or a diagnostic section is independently authorized
- **THEN** it conforms to its typed public schema without raw feedback, hidden evidence, internal query/plan text, or unbounded payloads

#### Scenario: Outer citation summary reaches its limit

- **WHEN** selected authorized context contains more evidence than the configured outer Provider citation limit permits
- **THEN** the outer summary respects the limit and no omitted candidate/source access or additional authority is implied

#### Scenario: Reference mode is requested

- **WHEN** an authorized runtime requests reference-only mode or omits the mode
- **THEN** returned categorized items contain only the closed structured reference allowlist, and content, titles, summaries, lessons, goal text, and diagnostics are absent throughout the response

### Requirement: Provider context compatibility is explicit

The context contract SHALL retain supported `provider-v1` base metadata, configured schema negotiation, operation identity, and existing capability/build/OpenAPI-digest discovery, while explicitly advertising `context-disclosure-v1`, supported profiles/modes, default reference-only behavior, and `source-metadata-v1`. Public capability metadata MUST NOT reveal individual principal entitlements or denied evidence. Unsupported base schema versions MUST be rejected before assembler dispatch with the bounded compatibility outcome. Documentation MUST identify the PC2 breaking changes to provisioning, defaults, mode-specific items, alternate-route denials, and source metadata, as well as the PC1 migration from Go-name/case fields, accidentally serialized scores, and flat `references/content` assumptions. Consumers MUST NOT infer disclosure support from the base version alone, mistake a reference shape for empty content, or retry with broader disclosure as a compatibility workaround. PC1/PC2 conformance MUST NOT imply PC3 budget redesign, PC4 digest/continuity, or PC5/PC6 reference/evidence workflow completion.

#### Scenario: Supported client uses canonical fields

- **WHEN** a client verifies disclosure capabilities and sends canonical fields under a valid current disclosure grant
- **THEN** the service accepts the documented shape and returns the mode-specific typed result identified by published OpenAPI and disclosure versions

#### Scenario: Schema is unsupported

- **WHEN** request metadata names an unsupported Provider schema version
- **THEN** the operation returns the bounded compatibility error and does not call the assembler

#### Scenario: Consumer discovers the repaired revision

- **WHEN** a client reads capability/version metadata and runtime OpenAPI
- **THEN** it can identify the running build/document digest, supported disclosure/source-metadata versions and mode schemas without treating the document digest as a context digest

#### Scenario: Old client assumes minimal request returns content

- **WHEN** a pre-PC2 consumer sends a minimal request and expects text-bearing memory items
- **THEN** maintained migration guidance requires explicit authorized content mode and strict response validation, rather than counting reference items as empty content or silently widening the credential

#### Scenario: Service lacks disclosure extension

- **WHEN** a consumer requiring PC2 cannot discover `context-disclosure-v1`
- **THEN** it treats the service as incompatible and does not remove disclosure fields or use an alternate content route to mask the missing capability
