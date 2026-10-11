## ADDED Requirements

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
