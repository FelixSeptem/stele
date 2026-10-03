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

### Requirement: Reasoning candidate handoff is an explicit admission boundary

The service SHALL accept reasoning-derived candidates only through an explicit
handoff that carries exact scope proof, evidence digest, replay identity,
provider/schema compatibility, uncertainty bounds, and the target policy
version. Provider output alone MUST NOT constitute an admission decision.

#### Scenario: Eligible reasoning candidate is handed off

- **WHEN** a candidate satisfies the target policy's scope, evidence, freshness, provenance, uncertainty, and idempotency gates
- **THEN** the activation path evaluates it under the existing type-specific policy and records the admission disposition

#### Scenario: Candidate omits handoff proof

- **WHEN** a candidate lacks exact scope proof, evidence digest, replay identity, or compatible policy version
- **THEN** the service rejects or quarantines it without creating an active insight

### Requirement: Reasoning activation remains independently rollbackable

The service SHALL record reasoning-derived admissions with provider and policy
versions so an operator can disable or roll back that policy without changing
other insight types or rewriting prior evidence history.

#### Scenario: Reasoning policy is rolled back

- **WHEN** an operator rolls back the policy used by reasoning-derived admissions
- **THEN** new admissions stop for that policy and prior records remain auditable under their original provenance

### Requirement: Contradiction activation requires type-specific temporal gates

An enabled contradiction policy SHALL require two exact-scope evidence sides,
a mutually exclusive contradiction key, a valid-time overlap or explicit
review override, bounded uncertainty, compatible provenance, and an
idempotency identity. The policy MUST define whether operator review is
required before activation.

#### Scenario: Overlapping contradiction is eligible

- **WHEN** both fact versions are visible, mutually exclusive, temporally overlapping, policy-compatible, and reviewed as required
- **THEN** activation admits one append-only contradiction insight through the ordinary reserved-insight lifecycle

#### Scenario: Temporal coexistence is submitted for activation

- **WHEN** a candidate has disjoint valid-time intervals and no explicit review override
- **THEN** activation records temporal-coexistence and does not create an active contradiction insight

#### Scenario: Contradiction policy requires review

- **WHEN** an otherwise eligible candidate has not reached the policy's required review state
- **THEN** activation records a review-required disposition and creates no active insight

### Requirement: Reserved goal activation precedes experimental visibility

The reserved-insight activation boundary MUST evaluate goal visibility only after scope, lifecycle, principal grant, policy, replay, evidence, freshness, and review precedence checks succeed. A provider, replay operation, or activation result MUST NOT bypass the visibility policy or directly emit `goal_context`.

#### Scenario: Precedence checks pass

- **WHEN** a governed goal has a compatible activation policy and all visibility gates pass
- **THEN** the service records an eligible handoff for the separately authorized experimental section

#### Scenario: Provider attempts direct visibility

- **WHEN** provider output requests goal activation or context inclusion without a valid visibility-policy evaluation
- **THEN** the service rejects or quarantines the request and records a bounded precedence failure
