## MODIFIED Requirements

### Requirement: Provider context accepts an explicit canonical JSON input

`POST /v1/provider/context` SHALL publish and accept a closed request envelope containing required `metadata` and `input` objects. Its input MUST use canonical snake-case properties: required nonblank `query` and positive integer item-placement `budget`; optional closed `budget_limits` containing `max_content_bytes`, `max_response_bytes`, and `max_citations`; optional `path`, `path_prefix`, `include_relations`, `include_experience_insights`, `include_goal_context`, `include_diagnostics`, `include_feedback_diagnostics`, and `feedback_aware_ranking`; the documented rejection-only `feedback_ranking_policy` string; and optional `projection_intent`, `disclosure_profile`, and `response_mode`. Budget values MUST obey the explicit integer bounds and omitted/zero rules of `context-budget-v1`; token fields remain unsupported. Optional booleans MUST default to false. Projection intent MUST be one of `conversation`, `task_context`, `compilation`, or `handoff` and default to `conversation`; it MUST NOT grant business or data authority. The profile MUST be an exact supported versioned registry identifier of at most 64 ASCII token bytes and default to the current grant's configured profile. Response mode MUST be `reference_only` or `bounded_content`, defaulting to `reference_only`. Profile/mode authorization MUST use current exact-scope disclosure entitlements. Path selectors MUST retain existing normalization, mutual-exclusion, and exact-scope rules. A nonempty `feedback_ranking_policy` MUST be rejected. Undocumented fields, case aliases, duplicate keys, null values, missing required objects, malformed types, invalid enums, and trailing JSON MUST return a bounded validation error rather than silently selecting defaults.

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

#### Scenario: Explicit limits use the documented units

- **WHEN** a runtime supplies canonical item/content/response/citation limits
- **THEN** the operation validates their independent units and applies caller/server intersections without changing disclosure mode or treating bytes as tokens

### Requirement: Provider context returns a typed categorized result

The service SHALL publish a dedicated mode-discriminated Provider context response schema with required `metadata`, `result`, and outer Provider `citations`. `result` MUST contain `response_mode`, effective versioned `disclosure` metadata, required closed `budget_accounting`, and array-valued `profile`, `recent_session`, `recent_episodes`, `relevant_summaries`, `related_entities`, and memory `citations`, including empty arrays when no eligible evidence exists. In `reference_only` mode categorized items MUST use the closed reference/citation shape without text-bearing memory or insight objects, and optional text/diagnostic/goal sections MUST NOT disclose text. In `bounded_content` mode authorized memory items MUST expose explicitly documented identity, scope, path, class, lifecycle, content, timestamp/temporal metadata, typed source metadata, and evidence citations without raw ranking scores. Optional governed insight, goal, and independently authorized diagnostic sections MUST be explicitly typed, retain their existing visibility gates, and respect source/disclosure eligibility. Outer and inner citations MUST derive only from finally returned authorized items and independently authorized sources. The inner/outer distinction and unique citation-identity budget MUST be documented; repeated serialized citations consume response bytes, and an item MUST be omitted if its mandatory citations cannot fit. All categorized output and metadata MUST obey `context-budget-v1`. No flat `references/content`, context digest, or instruction-authority guarantee SHALL be implied by this result.

#### Scenario: Nonempty context is returned

- **WHEN** the existing assembler selects authorized active evidence and content mode is explicitly requested and authorized
- **THEN** the response conforms to the categorized content schema, preserves selected order/identity/path/content, and includes typed effective source metadata and authorized evidence citations

#### Scenario: No eligible evidence is returned

- **WHEN** a valid authorized request finds no disclosable matching context
- **THEN** HTTP 200 contains effective mode/disclosure metadata, exact safe budget accounting with an empty outcome, and every required category and citation collection as empty arrays

#### Scenario: Internal scores are present

- **WHEN** selected internal context hits contain lexical, semantic, relation, or overall scores
- **THEN** Provider context omits those scores and other internal ranking fields from every mode and section

#### Scenario: Optional sections are returned

- **WHEN** an explicitly requested insight/goal section passes existing and disclosure/source gates in content mode, or a diagnostic section is independently authorized
- **THEN** it conforms to its typed public schema without raw feedback, hidden evidence, internal query/plan text, or unbounded payloads

#### Scenario: Outer citation summary reaches its limit

- **WHEN** a further otherwise eligible item would exceed the effective unique citation limit
- **THEN** the whole item is omitted, finally returned items retain their required evidence, and inner/outer citations and accounting respect their documented limits without implying broader source access

#### Scenario: Reference mode is requested

- **WHEN** an authorized runtime requests reference-only mode or omits the mode
- **THEN** returned categorized items contain only the closed structured reference allowlist, and content, titles, summaries, lessons, goal text, and raw/privileged diagnostics are absent throughout the response; only the closed safe budget accounting is present

### Requirement: Provider context compatibility is explicit

The context contract SHALL retain supported `provider-v1` base metadata, configured schema negotiation, operation identity, and existing capability/build/OpenAPI-digest discovery, while explicitly advertising `context-disclosure-v1`, supported profiles/modes, default reference-only behavior, and `source-metadata-v1`, plus `context-budget-v1`, fixed public packing version, explicit independent resolved ceilings, and unsupported token budgets. Public capability metadata MUST NOT reveal individual principal entitlements or denied evidence. Unsupported base schema versions MUST be rejected before assembler dispatch with the bounded compatibility outcome. Documentation MUST identify PC3 independent item ceilings, required accounting, final-response admission and strict consumer migration, alongside the PC2 breaking changes to provisioning, defaults, mode-specific items, alternate-route denials, and source metadata, as well as the PC1 migration from Go-name/case fields, accidentally serialized scores, and flat `references/content` assumptions. Consumers MUST NOT infer disclosure or budget support from the base version alone, mistake a reference shape for empty content, or retry with broader disclosure as a compatibility workaround. PC1/PC2 conformance MUST NOT imply PC3 readiness, and PC3 conformance MUST NOT imply PC4 digest/continuity or PC5/PC6 reference/evidence workflow completion.

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

#### Scenario: Service lacks budget extension

- **WHEN** a consumer requiring final-output allocation cannot discover `context-budget-v1`
- **THEN** it reports incompatibility rather than removing limits, estimating tokens, retrying through another route, or doing local context packing
