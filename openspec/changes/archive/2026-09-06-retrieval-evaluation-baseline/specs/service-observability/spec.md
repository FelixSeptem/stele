## ADDED Requirements

### Requirement: Redacted retrieval evaluation observability
The service SHALL emit low-cardinality, redacted observability for retrieval-evaluation
replay and release-gate decisions.

#### Scenario: Retrieval evaluation run completes
- **WHEN** a fixture replay completes
- **THEN** telemetry records bounded operation status, fixture version, ranking version,
  policy version, case count, aggregate safety outcome, and duration without recording
  tenant, project, namespace, memory IDs, source content, credentials, or DSNs

#### Scenario: Retrieval evaluation run fails
- **WHEN** a replay fails due to fixture validation, isolation, lifecycle visibility, or
  threshold regression
- **THEN** telemetry records a stable bounded failure category and does not emit raw
  query text, database errors, hidden evidence, or foreign scope details

#### Scenario: Release policy decision is made
- **WHEN** a candidate retrieval report is accepted or rejected against a baseline
- **THEN** telemetry records the policy version and bounded decision category so
  operators can audit rollout evidence without reconstructing sensitive fixture data
