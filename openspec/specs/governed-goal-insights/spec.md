# governed-goal-insights Specification

## Purpose
Define a bounded, evidence-backed goal insight candidate that can be reviewed,
replayed, and governed without becoming an automatic task planner or default
context source.

## Requirements

### Requirement: Goal candidates are exact-scope and structurally bounded

The service SHALL accept a `goal` candidate only for one resolved tenant,
project, and namespace scope. A candidate MUST include bounded title and
summary fields, one of the states `proposed`, `active`, `completed`,
`abandoned`, or `stale`, a scope proof, a source watermark, provider/schema and
policy versions, uncertainty bounds, and a deterministic replay identity.

#### Scenario: Valid goal candidate is retained

- **WHEN** an offline or shadow derivation supplies a bounded goal candidate with exact scope and all required compatibility metadata
- **THEN** the service retains it as a reviewable candidate without activating it or changing canonical memory

#### Scenario: Goal metadata is malformed

- **WHEN** a candidate has an unsupported state, unbounded title or summary, missing scope proof, missing replay identity, or incompatible metadata
- **THEN** the service rejects or quarantines it before provider handoff or derived-insight activation

### Requirement: Goal candidates require visible evidence and bounded validity

Every retained goal candidate SHALL cite at least one lifecycle-visible,
exact-scope evidence record with provenance and a stable evidence digest. An
optional validity interval MUST be complete and ordered when supplied, and the
candidate MUST be marked stale or ineligible when its evidence watermark or
validity window is no longer compatible with the governing policy.

#### Scenario: Goal cites eligible evidence

- **WHEN** a candidate cites visible evidence within the resolved scope and source watermark
- **THEN** the service records the citation, provenance, digest, freshness result, and validity metadata with the candidate

#### Scenario: Goal cites hidden or foreign evidence

- **WHEN** any cited evidence is suppressed, forgotten, deleted, redacted, stale outside the request watermark, or outside the exact scope
- **THEN** the service rejects or quarantines the candidate without disclosing the excluded record

#### Scenario: Goal validity interval is invalid

- **WHEN** a supplied validity interval is incomplete, reversed, or incompatible with the evidence policy
- **THEN** the service records an invalid-validity disposition and does not make the candidate eligible for review handoff

### Requirement: Goal derivation and replay are non-authoritative

Goal derivation SHALL support only bounded offline or shadow execution. Replay
identity MUST cover normalized goal metadata, sorted evidence identities,
source watermark, scope proof, provider/schema identity, and policy versions.
Offline, shadow, and replay operations MUST NOT activate a goal, mutate
canonical memory, alter default retrieval or ordinary context assembly, or
delete source evidence.

#### Scenario: Identical goal inputs are replayed

- **WHEN** the same normalized goal request, evidence, watermark, provider contract, and policy versions are replayed
- **THEN** the service produces the same candidate identity and bounded disposition without remote invocation or store mutation

#### Scenario: Shadow goal would pass policy

- **WHEN** a shadow evaluation finds a goal candidate that satisfies the configured policy and review prerequisites
- **THEN** the service records a `would_activate` or review-required result while active insight state and ordinary behavior remain unchanged

### Requirement: Goal admission requires an explicit policy and review handoff

The service SHALL admit a goal candidate only through an independently
versioned, exact-scope goal policy that defines evidence, freshness, provider
compatibility, uncertainty, review, rollback, and lifecycle requirements.
Provider output or replay results MUST NOT directly set a goal to `active`.

#### Scenario: Goal policy is absent or disabled

- **WHEN** a structurally valid goal candidate has no compatible enabled policy or the policy is expired or rolled back
- **THEN** the service retains a bounded unsupported-policy disposition and creates no active goal

#### Scenario: Goal requires review

- **WHEN** a goal candidate passes evidence and precedence gates but has not reached the policy's required review state
- **THEN** the service records a review-required disposition and does not activate the goal

#### Scenario: Goal review is handed off

- **WHEN** an authorized reviewer submits a valid fixed candidate identity within the exact scope
- **THEN** the service evaluates the candidate again under the goal policy and records the review attribution without allowing direct canonical mutation

### Requirement: Goal lifecycle is append-only and independently reversible

An admitted goal SHALL be represented as a versioned derived record with
evidence, provenance, policy decision, review state, and lifecycle history.
Goal state changes MUST append a version or auditable transition. Policy
disablement or rollback MUST stop new admissions while preserving prior goal
history and source evidence.

#### Scenario: Goal state changes after review

- **WHEN** an authorized policy or review action moves a goal between allowed states
- **THEN** the service appends a lifecycle transition with actor or policy attribution and preserves prior versions

#### Scenario: Goal policy is rolled back

- **WHEN** an operator rolls back the goal policy for one exact scope
- **THEN** new goal admissions stop for that policy and prior goal records remain inspectable with their original provenance

### Requirement: Goal visibility is excluded by default and diagnostics are bounded

Ordinary retrieval and context assembly MUST exclude goal candidates and goal
derived records unless a separately versioned, exact-scope visibility policy
explicitly enables an authorized experimental surface. Authorized diagnostics
MAY return bounded counts, states, review outcomes, freshness categories, and
redacted evidence references, but MUST NOT expose goal content, prompts,
provider payloads, hidden identifiers, or foreign-scope data.

#### Scenario: Ordinary context request has goal records

- **WHEN** an ordinary caller requests retrieval or assembled context while goal candidates or derived goals exist
- **THEN** the response omits those records and does not reveal their existence through ranking, counts, or error details

#### Scenario: Authorized operator inspects goals

- **WHEN** an authorized operator requests goal diagnostics for one exact scope
- **THEN** the service returns bounded state, review, policy, replay, freshness, and disposition categories without raw goal content or foreign identifiers
