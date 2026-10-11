## MODIFIED Requirements

### Requirement: Content projection remains data with explicit source metadata

`bounded_content` MUST require separate profile authorization, reuse existing context eligibility/selection and governed section gates, and disclose typed effective source metadata for every text-bearing object. Unprovable optional-section lineage MUST cause omission. Neither source attestation, profile, projection intent, nor derived guidance SHALL grant external execution, tool, write, task-state, or compiler authority. The final disclosed projection MUST obey `context-budget-v1` item/content/citation/body allocations through deterministic whole-item admission owned by context assembly, without text truncation or removing required evidence. The complete serialized response, including accounting, MUST also satisfy the configured Provider response ceiling; an oversized or unprovable finalized response MUST fail with a bounded non-success outcome rather than partial output or an alternate local ranking/packing mechanism. Caller `budget` retains item-count semantics; safe accounting MUST not expose hidden-source identities/counts or privileged diagnostics.

#### Scenario: Content-capable runtime receives selected evidence

- **WHEN** an authorized runtime requests bounded content and existing selection gates pass
- **THEN** it receives the categorized content and authorized citations with conservative trust/classification labels, without an instruction-authority claim

#### Scenario: Optional text has unknown lineage

- **WHEN** a requested insight or goal text section lacks safely established source metadata or eligibility
- **THEN** the section is omitted rather than assigned a guessed low classification or attested trust

#### Scenario: Serialization exceeds configured ceiling

- **WHEN** the final authorized projection exceeds the configured Provider response limit
- **THEN** the operation returns a bounded non-success response and emits no partial projection

#### Scenario: A content item exceeds caller allocation

- **WHEN** an authorized whole item and its required evidence cannot fit the effective allocation
- **THEN** existing assembly omits that item and returns only fit authorized evidence with safe accounting, without truncating text or changing the requested mode
