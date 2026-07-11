## ADDED Requirements

### Requirement: Quality evaluation and repair metrics are exported
The service MUST expose low-cardinality metrics for quality evaluation, admission pressure, repair execution, and post-repair verification.

#### Scenario: Quality evaluation completes
- **WHEN** an evaluation run completes, fails, or requires manual review
- **THEN** the metrics surface records status, check category, finding category, severity, and component labels without tenant, project, namespace, memory id, event id, repair plan id, actor, or reason text labels

#### Scenario: Admission pressure is evaluated
- **WHEN** ingestion or repair admission pressure is evaluated
- **THEN** the metrics surface records decision, component, and stable finding codes without high-cardinality scope or record identifiers

#### Scenario: Repair action executes
- **WHEN** a repair action completes, fails, retries, is skipped, or requires manual review
- **THEN** the metrics surface records action category, result, and reason category without high-cardinality target identifiers

#### Scenario: Verification completes
- **WHEN** post-repair verification completes
- **THEN** the metrics surface records verification status and residual finding categories without exposing detailed evidence through metric labels

### Requirement: Quality repair diagnostics are operator-visible
The service SHALL expose diagnostics that help operators understand memory quality, ingestion pressure, repair progress, and verification outcomes.

#### Scenario: Operator inspects quality loop health
- **WHEN** an operator requests diagnostics for an authorized scope
- **THEN** the service can report recent evaluation status, dominant finding categories, admission pressure state, repair backlog, repair failures, and verification outcomes

#### Scenario: Diagnostics include hidden lifecycle evidence
- **WHEN** hidden lifecycle states contribute to quality or repair diagnostics
- **THEN** the service includes aggregate counts and stable finding codes without exposing hidden memory content through public metrics or non-admin diagnostics
