## MODIFIED Requirements

### Requirement: Reserved insight vocabulary is non-active

The service SHALL reserve `hypothesis`, `goal`, `contradiction`, and
`causal_link` vocabulary and SHALL keep each type disabled unless a separate
versioned, exact-scope activation policy defines its evidence, provenance,
lifecycle, replay, feedback, and rollback rules. A provider or replay runner
MUST NOT autonomously activate a reserved type outside that policy.

#### Scenario: Unsupported insight type is requested for derivation

- **WHEN** the derivation job encounters a request to infer a reserved type without a compatible activation policy
- **THEN** the service skips or quarantines that derivation path and records no active insight of that type

#### Scenario: Policy-gated insight is admitted

- **WHEN** a reserved candidate satisfies the enabled type policy, exact scope, evidence, provenance, confidence, and idempotency checks
- **THEN** the service creates the insight through the governed derived-insight lifecycle and preserves the admission decision and audit history

#### Scenario: Policy is stale or rolled back

- **WHEN** a reserved insight policy is stale, expired, disabled, or rolled back
- **THEN** the service does not activate new candidates and preserves prior insight versions and evidence history

#### Scenario: Future insight type is represented in schema

- **WHEN** a future change adds support for another insight type
- **THEN** the existing derived insight substrate can preserve scope, lifecycle, confidence, evidence, policy, and audit semantics for that type
