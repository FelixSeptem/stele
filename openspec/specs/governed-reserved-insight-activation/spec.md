# governed-reserved-insight-activation Specification

## Purpose
Define a separately governed, scope-bound path for admitting validated
reasoning candidates as derived insights while preserving append-only history,
evidence provenance, lifecycle controls, deterministic replay, and rollback.

## Requirements

### Requirement: Reserved insight activation requires an explicit versioned policy

The service SHALL represent reserved insight activation through a versioned
policy bound to an exact tenant, project, and namespace scope. A policy MUST
identify its owner, enabled insight types, provider and schema compatibility,
evidence requirements, expiry or freshness bound, and rollback state. No
reserved type SHALL be active when a compatible policy is absent, disabled,
expired, or stale.

#### Scenario: No activation policy exists

- **WHEN** a validated candidate proposes a reserved insight type without a compatible policy
- **THEN** the service rejects or quarantines the candidate and records a bounded unsupported-policy disposition without creating an active insight

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
Admission MUST validate provenance, derivation policy/version, provider
compatibility, confidence or uncertainty bounds, lifecycle input, and
idempotency identity before creating an insight version.

#### Scenario: Reviewed hypothesis candidate is eligible

- **WHEN** a `hypothesis` candidate has exact scope, complete eligible evidence, compatible versions, bounded uncertainty, and an enabled policy
- **THEN** the service admits it through ordinary derived-insight governance and records the policy decision and source provenance

#### Scenario: Candidate cites foreign or hidden evidence

- **WHEN** a candidate cites evidence outside the resolved scope or excluded by lifecycle visibility
- **THEN** the service rejects or quarantines the candidate without revealing the foreign or hidden record

#### Scenario: Disabled type is proposed

- **WHEN** a candidate proposes `goal`, `contradiction`, or `causal_link` while that type has no independently enabled policy
- **THEN** the service records a type-disabled disposition and does not create an active insight

### Requirement: Activation preserves append-only lifecycle and canonical-memory boundaries

An accepted reserved insight SHALL be stored as a derived, versioned, scoped
record with evidence, provenance, policy/provider metadata, and lifecycle
history. Activation MUST NOT overwrite canonical memory, erase prior insight
versions, delete source evidence, or let provider output set an active
lifecycle state without the admission decision.

#### Scenario: Candidate is activated

- **WHEN** admission accepts a candidate under a compatible policy
- **THEN** the service creates an append-only derived insight version, records an activation audit event, and links the exact evidence and policy versions

#### Scenario: Duplicate activation is retried

- **WHEN** the same candidate and policy decision are retried with the same idempotency identity
- **THEN** the service returns or links the original disposition without creating duplicate insight versions or audit transitions

#### Scenario: Provider requests direct activation or canonical mutation

- **WHEN** provider output asks to activate an insight, rewrite canonical memory, delete evidence, or bypass admission
- **THEN** the service rejects the request and leaves canonical and derived records unchanged

### Requirement: Activation replay and shadow are deterministic and non-authoritative

The service SHALL support bounded offline replay and shadow evaluation of
activation decisions using normalized inputs, exact scope proof, source
watermarks, policy/provider versions, and redacted categorized outcomes.
Replay and shadow MUST NOT activate new insights, alter default retrieval or
context behavior, or emit a readiness claim when dependencies are stale or
incompatible.

#### Scenario: Replay repeats an activation decision

- **WHEN** the same candidate, evidence watermark, scope proof, policy version, and provider contract are replayed
- **THEN** the service produces the same bounded disposition without remote invocation or canonical mutation

#### Scenario: Shadow candidate would be activated

- **WHEN** shadow evaluation finds a candidate that would pass the current policy
- **THEN** the service records a non-authoritative would-activate result while ordinary behavior and active insight state remain unchanged

#### Scenario: Activation dependency is stale

- **WHEN** policy, source evidence, provider compatibility, or watermark metadata is missing or expired
- **THEN** the run is marked stale or incomplete and no activation or readiness claim is emitted

### Requirement: Activation supports bounded rollback and safe diagnostics

The service SHALL provide an authorized, scope-bound stop and rollback control
for each activation policy. Rollback MUST stop new admissions, preserve prior
audit and evidence history, and use normal derived-insight lifecycle transitions
for any previously activated records. Diagnostics MUST expose only bounded
counts, dispositions, versions, and redacted references.

#### Scenario: Operator rolls back a policy

- **WHEN** an authorized operator rolls back a policy for one exact scope
- **THEN** new admissions under that policy stop, the rollback is audited, and prior records remain inspectable with their original provenance

#### Scenario: Operator inspects activation evidence

- **WHEN** an authorized operator requests an activation report
- **THEN** the response includes bounded policy/candidate counters and reason categories without prompts, chain-of-thought, credentials, raw provider payloads, hidden IDs, or foreign scope values
