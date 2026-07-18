## ADDED Requirements

### Requirement: Assurance metrics are exported
The service MUST expose low-cardinality metrics for health evaluations, incident lifecycle, alert candidate generation, alert delivery attempts, capacity/load proof, backup/restore proof, readiness reports, cleanup jobs, and recovery verification.

#### Scenario: Health evaluation completes
- **WHEN** a health evaluation completes, degrades, fails, or reports unknown status
- **THEN** metrics record operation, result, status, component, severity, operational proof category, and reason category without tenant, project, namespace, evaluation id, incident id, actor, or reason labels

#### Scenario: Incident lifecycle changes
- **WHEN** an incident is opened, acknowledged, suppressed, resolved, reopened, or verified
- **THEN** metrics record lifecycle operation, result, status, component, severity, and reason category without high-cardinality identifiers

#### Scenario: Alert delivery attempt completes
- **WHEN** an alert delivery attempt succeeds, fails, retries, is skipped, or is disabled
- **THEN** metrics record adapter kind, result, severity, component, and failure category without webhook URL, recipient, scope, incident id, or alert id labels

#### Scenario: Assurance cleanup completes
- **WHEN** an assurance or conformance history cleanup job completes
- **THEN** metrics record record category, result, and bounded deletion category without tenant, project, namespace, record id, or evidence identifiers

### Requirement: Conformance metrics are exported
The service MUST expose low-cardinality metrics for conformance profiles, conformance runs, missing-evidence diagnostics, and readiness summaries.

#### Scenario: Conformance run completes
- **WHEN** a conformance run passes, degrades, fails, or reports unknown status
- **THEN** metrics record result, profile status, evidence category, missing-evidence category, and readiness impact without scope or record identifiers

#### Scenario: Readiness report is generated
- **WHEN** scope readiness is generated or read
- **THEN** metrics record readiness status, runtime category, conformance category, incident category, and recommended-action category without high-cardinality labels

### Requirement: Assurance and conformance lifecycle logs are bounded
The service SHALL emit structured logs for assurance and conformance lifecycle transitions using bounded fields.

#### Scenario: Assurance transition is logged
- **WHEN** a health evaluation, incident transition, alert candidate, alert delivery attempt, or recovery verification changes state
- **THEN** logs include bounded operation, result, component, severity, status, and reason category without tenant, project, namespace, ids, actor, reason text, query text, webhook URL, or recipient

#### Scenario: Conformance transition is logged
- **WHEN** a conformance profile, conformance run, missing-evidence diagnostic, or readiness report changes state
- **THEN** logs include bounded operation, result, evidence category, readiness status, and missing-evidence category without high-cardinality fields

### Requirement: Assurance diagnostics are operator-visible
The service SHALL expose diagnostics that help operators understand operational assurance and integration conformance health for an authorized scope.

#### Scenario: Operator inspects assurance health
- **WHEN** an operator requests assurance diagnostics for an authorized scope
- **THEN** the service can summarize recent evaluation status, capacity/load status, backup/restore status, open incidents, alert candidate state, delivery failures, cleanup status, recovery verification status, and recommended next admin surfaces

#### Scenario: Operator inspects conformance health
- **WHEN** an operator requests conformance diagnostics for an authorized scope
- **THEN** the service can summarize profile coverage, latest run status, dominant missing-evidence categories, stale evidence windows, and readiness impact

#### Scenario: Diagnostics reference hidden evidence
- **WHEN** hidden lifecycle state contributes to assurance or conformance diagnostics
- **THEN** diagnostics expose aggregate counts and stable reason codes without exposing hidden memory content through public metrics or non-admin diagnostics
