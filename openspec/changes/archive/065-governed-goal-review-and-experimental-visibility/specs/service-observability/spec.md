## ADDED Requirements

### Requirement: Goal review and experimental visibility telemetry is bounded

The service SHALL emit low-cardinality metrics and bounded structured logs for goal review, visibility-policy evaluation, experimental inclusion or omission, freshness and evidence gates, fallback, disablement, and rollback. Telemetry MUST use fixed categories or buckets and MUST exclude tenant, project, namespace, goal text, prompts, provider payloads, identifiers, raw errors, and source content.

#### Scenario: Review and visibility evaluation completes

- **WHEN** an offline, shadow, review, or experimental visibility operation completes, degrades, or fails
- **THEN** telemetry records bounded operation, mode, result, review, policy, freshness, inclusion, and rollback categories

#### Scenario: Sensitive telemetry input is supplied

- **WHEN** instrumentation receives a scope value, goal content, provider payload, identifier, or raw error
- **THEN** it rejects, redacts, or buckets the value before emission

### Requirement: Experimental diagnostics remain authorized and redacted

Authorized diagnostics MAY summarize aggregate goal review and `goal_context` eligibility for one exact scope. They MUST omit hidden content, foreign identifiers, raw provider data, and ordinary public retrieval internals.

#### Scenario: Operator inspects experimental health

- **WHEN** an authorized operator requests visibility health for one exact scope
- **THEN** the service returns aggregate review, policy, freshness, inclusion, fallback, and rollback categories sufficient for operational assessment

#### Scenario: Diagnostics include hidden evidence

- **WHEN** hidden, stale, deleted, suppressed, or foreign evidence affected eligibility
- **THEN** diagnostics expose only aggregate counts and stable reason categories
