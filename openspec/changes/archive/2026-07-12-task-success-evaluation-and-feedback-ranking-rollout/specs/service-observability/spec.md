## ADDED Requirements

### Requirement: Task evaluation metrics are exported
The service MUST expose low-cardinality metrics for task evaluation creation, deduplication, rejection, summary aggregation, and task-derived quality outcomes.

#### Scenario: Task evaluation is recorded
- **WHEN** a task evaluation is created, deduplicated, rejected, or superseded
- **THEN** metrics record operation result, verdict category, source surface, and bounded failure category without tenant, project, namespace, task id, session id, memory id, actor, or reason labels

#### Scenario: Task summary is aggregated
- **WHEN** task-success summaries are aggregated or rebuilt
- **THEN** metrics record summary status, subject kind, verdict category, and dominant contribution category without high-cardinality identifiers

### Requirement: Ranking rollout metrics are exported
The service MUST expose low-cardinality metrics for ranking rollout dry-run, activation, impact, rollback, and policy evaluation.

#### Scenario: Rollout dry-run completes
- **WHEN** a ranking rollout dry-run completes
- **THEN** metrics record result, surface, policy mode, evidence threshold status, and bounded impact category without high-cardinality identifiers

#### Scenario: Active policy evaluates request
- **WHEN** search or context assembly evaluates an active ranking rollout policy
- **THEN** metrics record surface, policy status, applied decision, and bounded reason category without tenant, project, namespace, policy id, query text, memory id, actor, or reason labels

#### Scenario: Policy is rolled back
- **WHEN** a ranking rollout policy is paused, disabled, or rolled back
- **THEN** metrics record operation result and policy terminal category without high-cardinality labels

### Requirement: Task and ranking diagnostics are operator-visible
The service SHALL expose diagnostics that help operators understand task-success trends and ranking rollout health for an authorized scope.

#### Scenario: Operator inspects task-success health
- **WHEN** an operator requests task evaluation diagnostics for an authorized scope
- **THEN** the service can summarize recent verdict distribution, dominant memory contribution categories, linked feedback volume, linked quality findings, and recommended next admin surfaces

#### Scenario: Operator inspects ranking rollout health
- **WHEN** an operator requests ranking rollout diagnostics for an authorized scope
- **THEN** the service can summarize active policies, dry-run results, impact counters, rollback history, insufficient-evidence decisions, and bounded reason codes

#### Scenario: Diagnostics reference hidden evidence
- **WHEN** hidden lifecycle state contributes to task or rollout diagnostics
- **THEN** diagnostics expose aggregate counts and stable reason codes without exposing hidden memory content through public metrics or non-admin diagnostics

### Requirement: Task and rollout lifecycle logs are bounded
The service SHALL emit structured logs for task evaluation and ranking rollout lifecycle transitions without high-cardinality fields.

#### Scenario: Task lifecycle transition is logged
- **WHEN** a task evaluation is created, deduplicated, rejected, superseded, summarized, or linked to quality findings
- **THEN** logs include bounded operation, result, verdict, source, and contribution category fields without task id, session id, memory id, actor, or reason text

#### Scenario: Ranking rollout transition is logged
- **WHEN** a rollout policy is created, dry-run, activated, disabled, rolled back, or evaluated for a request
- **THEN** logs include bounded operation, result, surface, policy status, and decision fields without high-cardinality identifiers
