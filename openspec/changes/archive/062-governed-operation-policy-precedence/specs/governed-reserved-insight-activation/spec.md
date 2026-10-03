## MODIFIED Requirements

### Requirement: Candidate admission is evidence-backed and type-specific

The service SHALL admit a reserved insight only from a structurally valid, scope-eligible candidate or governed intent whose evidence citations are a subset of authorized source evidence and satisfy the enabled type's policy. Admission MUST apply the governed-operation precedence contract in order, validating scope and lifecycle visibility before principal grant, explicit policy enablement/version, idempotency identity, provenance, provider compatibility, confidence or uncertainty bounds, and the final derived-insight governance handoff.

#### Scenario: Reviewed hypothesis candidate is eligible

- **WHEN** a `hypothesis` candidate has exact scope, visible eligible evidence, compatible versions, bounded uncertainty, an authorized principal, and an enabled policy
- **THEN** the service admits it through ordinary derived-insight governance and records the policy decision and source provenance

#### Scenario: Candidate cites foreign or hidden evidence

- **WHEN** a candidate cites evidence outside the resolved scope or excluded by lifecycle visibility
- **THEN** the service rejects or quarantines the candidate before policy, idempotency, provider, or activation work without revealing the foreign or hidden record

#### Scenario: Disabled type is proposed

- **WHEN** a candidate proposes `goal`, `contradiction`, or `causal_link` while that type has no independently enabled compatible policy
- **THEN** the service records a type-disabled disposition and does not create an active insight or invoke the activation handoff

### Requirement: Activation replay and shadow are deterministic and non-authoritative

The service SHALL support bounded offline replay and shadow evaluation of activation decisions using normalized inputs, exact scope proof, source watermarks, policy/provider versions, and redacted categorized outcomes. Replay and shadow MUST apply precedence gates in order, MUST NOT activate new insights, alter default retrieval or context behavior, or emit a readiness claim when dependencies are stale or incompatible.

#### Scenario: Replay repeats an activation decision

- **WHEN** the same candidate, evidence watermark, scope proof, policy version, provider contract, and idempotency identity are replayed
- **THEN** the service produces the same bounded disposition without remote invocation or canonical or derived mutation

#### Scenario: Shadow candidate would be activated

- **WHEN** shadow evaluation finds a candidate that would pass the current precedence and type policy
- **THEN** the service records a non-authoritative would-activate result while ordinary behavior and active insight state remain unchanged

#### Scenario: Activation dependency is stale

- **WHEN** policy, source evidence, provider compatibility, or watermark metadata is missing or expired
- **THEN** the run is marked stale or incomplete at the applicable gate and no activation or readiness claim is emitted
