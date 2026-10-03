## MODIFIED Requirements

### Requirement: Reserved insight activation requires an explicit versioned policy

The service SHALL represent reserved insight activation through a versioned
policy bound to an exact tenant, project, and namespace scope. A policy MUST
identify its owner, enabled insight types, provider and schema compatibility,
evidence requirements, expiry or freshness bound, review requirement, and
rollback state. For `goal`, the policy MUST define allowed states, validity
interval handling, and whether any future active admission is permitted. No
reserved type SHALL be active when a compatible policy is absent, disabled,
expired, or stale.

#### Scenario: No activation policy exists

- **WHEN** a validated reserved candidate proposes a type without a compatible policy
- **THEN** the service rejects or quarantines the candidate and records a bounded unsupported-policy disposition without creating an active insight

#### Scenario: Goal policy omits type-specific bounds

- **WHEN** a `goal` policy does not define allowed states, evidence freshness, review, or rollback behavior
- **THEN** the service treats the policy as incompatible and does not admit a goal candidate

#### Scenario: Policy scope does not match

- **WHEN** an activation policy is presented for a different tenant, project, or namespace than the candidate
- **THEN** the service fails closed without disclosing foreign-scope data or applying the candidate

#### Scenario: Policy expires or is disabled

- **WHEN** a policy is expired, stale, disabled, or marked rolled back before admission
- **THEN** the service leaves existing governed history intact and does not activate new candidates under that policy

### Requirement: Candidate admission is evidence-backed and type-specific

The service SHALL admit a reserved insight only from a structurally valid,
scope-eligible candidate or governed intent whose evidence citations are a
subset of authorized source evidence and satisfy the enabled type's policy.
Admission MUST apply the governed-operation precedence contract in order,
validating scope and lifecycle visibility before principal grant, explicit
policy enablement/version, idempotency identity, provenance, provider
compatibility, confidence or uncertainty bounds, type-specific state and
review requirements, and the final derived-insight governance handoff.

#### Scenario: Reviewed goal candidate is eligible

- **WHEN** a `goal` candidate has exact scope, visible evidence, compatible versions, bounded uncertainty, an allowed state, an authorized reviewer, and an enabled policy
- **THEN** the service admits it through ordinary derived-insight governance and records the policy and review decision without provider-directed activation

#### Scenario: Reviewed hypothesis candidate is eligible

- **WHEN** a `hypothesis` candidate has exact scope, visible eligible evidence, compatible versions, bounded uncertainty, an authorized principal, and an enabled policy
- **THEN** the service admits it through ordinary derived-insight governance and records the policy decision and source provenance

#### Scenario: Candidate cites foreign or hidden evidence

- **WHEN** a candidate cites evidence outside the resolved scope or excluded by lifecycle visibility
- **THEN** the service rejects or quarantines the candidate before policy, idempotency, provider, or activation work without revealing the foreign or hidden record

#### Scenario: Disabled type is proposed

- **WHEN** a candidate proposes `goal`, `hypothesis`, `contradiction`, or `causal_link` while that type has no independently enabled compatible policy
- **THEN** the service records a type-disabled disposition and does not create an active insight or invoke the activation handoff

### Requirement: Activation replay and shadow are deterministic and non-authoritative

The service SHALL support bounded offline replay and shadow evaluation of
activation decisions using normalized inputs, exact scope proof, source
watermarks, policy/provider versions, type-specific metadata, and redacted
categorized outcomes. Replay and shadow MUST apply precedence gates in order,
MUST NOT activate new insights, alter default retrieval or context behavior, or
emit a readiness claim when dependencies are stale or incompatible.

#### Scenario: Replay repeats a goal decision

- **WHEN** the same goal candidate, evidence watermark, scope proof, policy version, provider contract, state, and idempotency identity are replayed
- **THEN** the service produces the same bounded disposition without remote invocation or canonical or derived mutation

#### Scenario: Replay repeats an activation decision

- **WHEN** the same candidate, evidence watermark, scope proof, policy version, provider contract, and idempotency identity are replayed
- **THEN** the service produces the same bounded disposition without remote invocation or canonical or derived mutation

#### Scenario: Shadow goal would be activated

- **WHEN** shadow evaluation finds a goal that would pass current precedence and type policy after review requirements are satisfied
- **THEN** the service records a non-authoritative would-activate result while ordinary behavior and active insight state remain unchanged

#### Scenario: Shadow candidate would be activated

- **WHEN** shadow evaluation finds a candidate that satisfies the current activation gates
- **THEN** the service records a non-authoritative would-activate result while ordinary behavior and active insight state remain unchanged

#### Scenario: Activation dependency is stale

- **WHEN** policy, source evidence, provider compatibility, or watermark metadata is missing or expired
- **THEN** the run is marked stale or incomplete at the applicable gate and no activation or readiness claim is emitted
