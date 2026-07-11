## ADDED Requirements

### Requirement: Insight feedback metrics are exported
The service MUST expose operational metrics for derived insight quality feedback using low-cardinality labels.

#### Scenario: Feedback is recorded
- **WHEN** an insight feedback record is created or superseded
- **THEN** the metrics surface records the operation result by feedback type, insight type, and outcome without embedding tenant, project, namespace, insight id, actor, or reason text as metric labels

#### Scenario: Feedback-driven lifecycle decision occurs
- **WHEN** a background job suppresses, reviews, preserves, or prioritizes an insight based on feedback
- **THEN** the metrics surface records the decision category and result without high-cardinality identifiers

### Requirement: Insight feedback diagnostics are operator-visible
The service SHALL expose diagnostics that help operators understand derived insight quality trends.

#### Scenario: Operator inspects insight quality health
- **WHEN** an operator requests operational diagnostics for derived insights
- **THEN** the service can report feedback coverage, noisy insight rate, needs-review count, and feedback-driven suppression counts for an authorized scope

#### Scenario: Diagnostics include hidden insights
- **WHEN** suppressed or hidden insights contribute to quality diagnostics
- **THEN** the service includes aggregate counts without exposing hidden insight content through public metrics or non-admin diagnostics
