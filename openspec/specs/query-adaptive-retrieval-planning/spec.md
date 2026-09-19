# query-adaptive-retrieval-planning Specification

## Purpose
Define deterministic, bounded retrieval planning that adapts existing retrieval channels and budgets to a query family without weakening scope, lifecycle, release, or fallback guarantees.

## Requirements

### Requirement: Query families are versioned and deterministic
The service SHALL classify an accepted query into exactly one bounded query family using a named planner version, an approved policy version, caller constraints, and validated query-analysis categories. Supported families SHALL include exact lookup, semantic, temporal, entity-relation, multi-hop, procedural, and general fallback. Equivalent inputs under the same versions MUST produce the same family and disposition.

#### Scenario: Equivalent planner input is replayed
- **WHEN** the same accepted-query identity, validated analysis categories, caller constraints, and planner policy are evaluated repeatedly
- **THEN** the service produces the same query family, plan identity, ordered plan parameters, and bounded disposition

#### Scenario: Query family is ambiguous
- **WHEN** validated inputs do not deterministically satisfy one specialized query-family rule
- **THEN** the service selects the general fallback family without inventing a hint, widening a constraint, or invoking an online model

#### Scenario: Multiple family rules match
- **WHEN** more than one specialized family rule matches the same bounded input
- **THEN** the service resolves the family through a versioned deterministic precedence rule and records only the selected family in authorized diagnostics

### Requirement: Retrieval plans are complete, bounded, and replayable
The service SHALL produce a complete retrieval plan containing the planner and policy identities, query family, enabled existing recall channels, per-channel and aggregate candidate budgets, fusion identity and parameters, memory-class quotas, reranker eligibility, context-section priorities, pass limit, latency budget, and fallback identity. Missing, unknown, incompatible, negative, or over-limit parameters MUST be rejected before plan execution.

#### Scenario: Valid plan is constructed
- **WHEN** an approved planner policy resolves every required field within the service hard limits
- **THEN** the service accepts one immutable plan whose complete identity and bounded parameters can be replayed and compared

#### Scenario: Plan exceeds a service hard limit
- **WHEN** a policy requests a channel, candidate count, weight, quota, pass count, latency, or context allowance beyond the service hard limit
- **THEN** the service rejects the plan and executes the approved baseline fallback rather than silently increasing the limit

### Requirement: Candidate budgets adapt within one hard envelope
The service SHALL derive per-channel and total candidate budgets only from versioned query-family policy, bounded query-complexity categories, post-filter-attrition categories, and declared reranker headroom. The total work across all passes MUST remain within one request-level candidate, latency, and context envelope and MUST NOT exceed service hard caps.

#### Scenario: Filtering removes most first-pass candidates
- **WHEN** first-pass exact-scope and lifecycle filtering produces an approved high-attrition category
- **THEN** the service may allocate only the declared remaining candidate budget to an eligible follow-up pass and does not weaken the filters

#### Scenario: Complex query receives additional headroom
- **WHEN** an approved query family and complexity category justify more candidates than the baseline allocation
- **THEN** the service may redistribute unused candidate headroom among declared channels without exceeding the request envelope

### Requirement: Plan execution preserves exact scope and lifecycle safety
Every planned operation MUST execute within the already resolved tenant, project, namespace, and authorized optional session or user constraints. A plan MUST only narrow existing constraints and MUST NOT discover, infer, or broaden scope.

#### Scenario: Planned signal implies broader evidence
- **WHEN** a plan or derived signal would require evidence outside the resolved scope or caller constraints
- **THEN** the service rejects that effect before repository access and discloses no foreign or hidden evidence

### Requirement: At most one evidence-triggered follow-up pass is allowed
The service MAY execute one follow-up retrieval pass only when the first pass produces an approved bounded insufficient-evidence category and the immutable plan declares a compatible follow-up action. The follow-up MUST retain the original query, exact scope, lifecycle rules, and remaining resource envelope; it MUST NOT trigger a third pass or recursively re-plan.

#### Scenario: Required evidence groups are incomplete
- **WHEN** first-pass evaluation finds an approved incomplete-evidence category and sufficient declared budget remains
- **THEN** the service executes at most one planned follow-up and fuses validated evidence through the existing deterministic path

#### Scenario: Follow-up remains incomplete
- **WHEN** the bounded follow-up finishes without satisfying evidence completeness
- **THEN** the service returns eligible evidence within budget and records an incomplete terminal disposition without further retrieval or answer generation

### Requirement: Planner rollout is exact-scope and reversible
Adaptive planning SHALL remain on the approved baseline unless an exact-scope rollout selects diagnostics-only, shadow, or active-for-scope for compatible dependency identities. Diagnostic and shadow execution MUST NOT change ordinary results.

#### Scenario: Shadow rollout applies
- **WHEN** a compatible exact-scope planner rollout is in shadow
- **THEN** the service may compare a bounded planned result with baseline while returning only baseline to the ordinary caller

#### Scenario: Active rollout is disabled
- **WHEN** an operator disables or rolls back an active planner policy
- **THEN** subsequent requests return to the approved baseline without rewriting canonical memory

### Requirement: Planner diagnostics are bounded and redacted
Planner diagnostics SHALL be available only to authorized evaluation or administrative paths and MUST NOT expose query text, derived signal text, scope values, identifiers, hidden candidates, provider payloads, raw scores, credentials, or internal reasoning.

#### Scenario: Ordinary retrieval uses an adaptive plan
- **WHEN** an ordinary caller receives search or assembled-context results
- **THEN** the response preserves its existing public shape and omits the internal plan and planner diagnostics
