## Purpose

Define a separately governed review and experimental visibility boundary for goal insights so authorized operators can assess exact-scope evidence while an opt-in `goal_context` section remains isolated from ordinary retrieval and context behavior.

## ADDED Requirements

### Requirement: Goal review is exact-scope and redacted

The service SHALL expose an authorized review/admin contract for one exact tenant, project, and namespace scope. The contract MUST require a principal grant and scope proof, and MUST return only bounded goal state, review state, policy and replay categories, freshness and evidence eligibility categories, rollback state, and redacted evidence references. It MUST omit raw goal content, prompts, provider payloads, hidden identifiers, and foreign-scope data.

#### Scenario: Authorized reviewer inspects a goal

- **WHEN** a reviewer with a valid grant requests a goal review for an exact scope
- **THEN** the service returns bounded review data for that scope and no raw goal content or foreign-scope identifier

#### Scenario: Review scope or grant is invalid

- **WHEN** the request lacks a valid principal grant, scope proof, or exact scope match
- **THEN** the service fails closed with a bounded authorization disposition and does not reveal whether a goal exists

### Requirement: Experimental goal visibility requires an independent policy

The service SHALL represent `goal_context` visibility through an independently versioned, exact-scope policy. A policy MUST identify its owner, enabled version, principal grant requirements, evidence and freshness gates, review state requirements, precedence compatibility, expiry, and rollback state. Visibility MUST be disabled by default and MUST fail closed when any required gate is absent, stale, expired, denied, or rolled back.

#### Scenario: Policy enables an eligible goal

- **WHEN** an authorized request supplies the exact scope, matching policy version, valid grant, current evidence, and required review state
- **THEN** the service may emit a bounded `goal_context` section for that request and records the policy decision

#### Scenario: A visibility gate fails

- **WHEN** policy, scope, grant, review, evidence, freshness, precedence, or replay validation fails
- **THEN** the service omits `goal_context`, records a bounded disposition, and leaves ordinary context behavior unchanged

### Requirement: Goal context is isolated from ordinary context

The `goal_context` section SHALL remain an independently authorized optional section. Its presence or absence MUST NOT alter ordinary retrieval candidates, ranking, ordinary context sections, token budgets for those sections, public counts, lifecycle state, or canonical memory. Provider output and replay results MUST NOT directly request or activate the section.

#### Scenario: Ordinary context is requested while visibility is enabled

- **WHEN** a caller uses the ordinary retrieval or context contract without the experimental section request
- **THEN** the response excludes goals and is behaviorally identical to a response without any goal records

#### Scenario: Experimental section is requested

- **WHEN** an authorized caller explicitly opts into `goal_context` for a policy-enabled exact scope
- **THEN** only the independent section is evaluated and ordinary sections, ordering, counts, and fallback semantics remain unchanged

### Requirement: Experimental visibility is append-only and reversible

The service SHALL append policy decisions, review decisions, inclusion outcomes, disablement, and rollback transitions with actor or policy attribution and replay identity. Disabling or rolling back visibility MUST stop new inclusions while preserving prior review history, derived goal history, source evidence, and bounded audit records.

#### Scenario: Operator rolls back visibility

- **WHEN** an authorized operator disables or rolls back the visibility policy for one exact scope
- **THEN** future `goal_context` requests fail closed and prior decisions remain inspectable with their original provenance

#### Scenario: Identical visibility evaluation is replayed

- **WHEN** the same goal identity, evidence watermark, scope proof, policy version, and principal grant are replayed
- **THEN** the service returns the same bounded disposition without duplicate inclusion or destructive mutation

### Requirement: Experimental goal diagnostics are bounded

The service SHALL expose low-cardinality telemetry and authorized diagnostics for review, policy evaluation, inclusion, omission, freshness, fallback, disablement, and rollback. Fields and labels MUST exclude goal content, prompts, provider payloads, scope values, hidden identifiers, foreign identifiers, raw errors, and evidence content.

#### Scenario: Visibility evaluation emits diagnostics

- **WHEN** a visibility evaluation completes, degrades, or fails
- **THEN** telemetry records bounded operation, result, policy, review, freshness, inclusion, and rollback categories only

#### Scenario: Hidden evidence affects eligibility

- **WHEN** suppressed, forgotten, deleted, stale, or foreign evidence affects a decision
- **THEN** diagnostics expose only aggregate counts and stable reason categories
