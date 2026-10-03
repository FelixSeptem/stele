## MODIFIED Requirements

### Requirement: Reasoning lifecycle telemetry is bounded

The service SHALL emit low-cardinality metrics and structured logs for
reasoning derivation, replay, shadow evaluation, provider fallback, evidence
eligibility, activation handoff, review, goal-state transitions, and rollback
using fixed operation, mode, type, result, freshness, review, and reason
categories. Telemetry MUST exclude scope values, prompts, chain-of-thought,
source content, identifiers, raw scores, provider payloads, credentials, and
reason text.

#### Scenario: Goal reasoning run completes

- **WHEN** an offline, shadow, replay, review, or policy evaluation operation for a goal completes, degrades, or fails
- **THEN** telemetry records bounded operation, mode, type, result, goal-state, review, eligibility, freshness, and duration categories without sensitive fields

#### Scenario: Reasoning run completes

- **WHEN** an offline, shadow, or replay reasoning operation completes, degrades, or fails
- **THEN** telemetry records bounded operation, mode, type, result, eligibility, freshness, and duration categories without sensitive fields

#### Scenario: Sensitive reasoning field is supplied

- **WHEN** instrumentation receives a prompt, scope, candidate identifier, provider payload, goal text, or raw error
- **THEN** it rejects, redacts, or buckets the value before emission

### Requirement: Reasoning diagnostics are authorized and redacted

Operator diagnostics MAY summarize reasoning candidate counts, disposition
categories, provider compatibility, freshness, fallback, activation-policy
health, goal-state counts, and review outcomes only for an authorized scope and
MUST omit candidate content and hidden or foreign identifiers.

#### Scenario: Operator inspects goal reasoning health

- **WHEN** an authorized operator requests reasoning diagnostics for one exact scope
- **THEN** the response contains bounded aggregate goal-state, review, replay, policy, freshness, and disposition categories sufficient to assess eligibility and rollback readiness

#### Scenario: Operator inspects reasoning health

- **WHEN** an authorized operator requests reasoning diagnostics for one scope
- **THEN** the response contains bounded aggregate counters and stable categories sufficient to assess eligibility and rollback readiness

#### Scenario: Diagnostic includes hidden evidence

- **WHEN** hidden, suppressed, forgotten, deleted, or foreign evidence affected a goal reasoning decision
- **THEN** diagnostics expose only aggregate counts and stable reason categories
