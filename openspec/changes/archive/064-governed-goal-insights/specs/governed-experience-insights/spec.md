## MODIFIED Requirements

### Requirement: Derived insights are governed records

The service SHALL persist derived experience insights as governed records with
explicit scope, insight type, lifecycle state, confidence, derivation metadata,
evidence citations, and audit history. A goal record MUST additionally retain
its bounded goal state, validity metadata, review state, policy decision, and
source watermark.

#### Scenario: Derived insight is stored with governance metadata

- **WHEN** a derived insight, including a goal, is created or updated
- **THEN** the service stores its tenant, project, namespace, type, lifecycle state, confidence, derivation source, evidence references, policy/review metadata, and observed or derived timestamps

#### Scenario: Derived insight does not overwrite canonical memory

- **WHEN** the service derives a new insight from canonical memory, job history, recovery history, embedding failure state, or goal evidence
- **THEN** the service records the insight separately without mutating canonical memories, memory versions, vector revisions, or existing provenance in place

### Requirement: Reserved insight vocabulary is non-active

The service SHALL reserve `hypothesis`, `goal`, `contradiction`, and
`causal_link` vocabulary and SHALL keep each type disabled unless a separate
versioned, exact-scope activation policy defines its evidence, provenance,
lifecycle, replay, feedback, review, and rollback rules. A provider or replay
runner MUST NOT autonomously activate a reserved type outside that policy.

#### Scenario: Unsupported insight type is requested for derivation

- **WHEN** the derivation job encounters a request to infer a reserved type without a compatible activation policy
- **THEN** the service skips or quarantines that derivation path and records no active insight of that type

#### Scenario: Goal candidate is policy-gated

- **WHEN** a goal candidate satisfies the enabled type policy's exact scope, evidence, provenance, state, review, confidence, and idempotency checks
- **THEN** the service retains the candidate or creates a governed derived record according to the policy without making it visible in ordinary context by default

#### Scenario: Policy-gated insight is admitted

- **WHEN** a reserved candidate satisfies the enabled type policy, exact scope, evidence, provenance, confidence, and idempotency checks
- **THEN** the service creates the insight through the governed derived-insight lifecycle and preserves the admission decision and audit history

#### Scenario: Policy is stale or rolled back

- **WHEN** a reserved insight policy is stale, expired, disabled, or rolled back
- **THEN** the service does not activate new candidates and preserves prior insight versions and evidence history

#### Scenario: Future insight type is represented in schema

- **WHEN** a future change adds support for another insight type
- **THEN** the existing derived insight substrate can preserve scope, lifecycle, confidence, evidence, policy, review, and audit semantics for that type

### Requirement: Reasoning-derived candidates preserve derivation provenance

The service SHALL preserve provider-neutral derivation metadata for a
reasoning-derived candidate or activated insight, including operation mode,
provider and schema identity, normalized input digest, source watermark,
uncertainty bounds, type-specific state or validity metadata when applicable,
and the policy and review decision that consumed it.

#### Scenario: Reasoning goal candidate is retained for review

- **WHEN** an offline or shadow run produces a structurally valid goal candidate
- **THEN** the retained candidate includes provenance, uncertainty, state, validity, and review metadata sufficient to reproduce and audit the derivation

#### Scenario: Reasoning candidate is retained for review

- **WHEN** an offline or shadow run produces a structurally valid candidate
- **THEN** the retained candidate includes provenance and uncertainty metadata sufficient to reproduce and audit the derivation

#### Scenario: Candidate provenance is incomplete

- **WHEN** a reasoning candidate lacks provider identity, source watermark, normalized input identity, or required goal metadata
- **THEN** the service does not expose it as an active insight and records an incomplete-provenance disposition
