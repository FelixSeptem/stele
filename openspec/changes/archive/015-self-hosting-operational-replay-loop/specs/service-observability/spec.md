## ADDED Requirements

### Requirement: Replay execution metrics are exported
The service MUST expose low-cardinality metrics for derived insight replay planning and execution.

#### Scenario: Replay dry-run completes
- **WHEN** a replay dry-run finishes
- **THEN** the metrics surface records the result, mode, insight type category, and decision categories without tenant, project, namespace, replay id, insight id, actor, or reason text labels

#### Scenario: Replay apply completes or fails
- **WHEN** replay apply work creates, updates, suppresses, preserves, skips, or fails insight decisions
- **THEN** the metrics surface records counters by outcome and reason category using low-cardinality labels

### Requirement: Smoke loop diagnostics are operator-visible
The service SHALL expose diagnostics that help operators determine which stage of the self-hosting smoke loop failed.

#### Scenario: Smoke loop detects degraded service
- **WHEN** a smoke check reports failure or degradation
- **THEN** operators can inspect readiness, job backlog, replay status, retrieval/context diagnostics, or metrics to identify the failed stage without direct PostgreSQL access
