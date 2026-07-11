## ADDED Requirements

### Requirement: Usefulness feedback metrics are exported
The service MUST expose low-cardinality metrics for feedback ingestion, feedback summaries, and feedback-derived quality outcomes.

#### Scenario: Feedback is recorded
- **WHEN** usefulness feedback is created, deduplicated, rejected, or superseded
- **THEN** metrics record result, feedback type, subject kind, source surface, and component without tenant, project, namespace, memory id, event id, insight id, session id, turn id, actor, or reason labels

#### Scenario: Feedback summary is updated
- **WHEN** usefulness summaries are aggregated or rebuilt
- **THEN** metrics record summary status, subject kind, effective quality category, and dominant feedback category without high-cardinality identifiers

### Requirement: Feedback diagnostics are operator-visible
The service SHALL expose diagnostics that summarize usefulness feedback health for an authorized scope.

#### Scenario: Operator inspects feedback health
- **WHEN** an operator requests feedback or loop health diagnostics
- **THEN** the service can summarize feedback volume, dominant negative categories, missing expected recall counts, needs-review counts, unsafe feedback counts, and linked quality or repair surfaces

#### Scenario: Diagnostics reference hidden evidence
- **WHEN** hidden lifecycle state contributes to feedback-derived diagnostics
- **THEN** diagnostics expose aggregate counts and stable reason codes without exposing hidden memory content through public metrics or non-admin diagnostics
