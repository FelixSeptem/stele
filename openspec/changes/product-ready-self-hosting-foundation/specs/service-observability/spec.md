## ADDED Requirements

### Requirement: Migration and runtime lifecycle telemetry is bounded and actionable
The service SHALL emit low-cardinality metrics and bounded structured logs for
migration evaluation/execution, startup, readiness transitions, signal receipt,
drain completion, forced drain timeout, and dependency cleanup outcomes.

#### Scenario: Runtime migrates and becomes ready
- **WHEN** a runtime applies or validates migrations and starts successfully
- **THEN** observability records bounded mode, operation, result, migration
  status, and duration categories without database DSNs, credentials, scope
  values, principal identifiers, migration SQL, or raw error payloads

#### Scenario: Runtime drains after termination
- **WHEN** a runtime begins or completes graceful shutdown after a supported
  signal
- **THEN** observability records bounded lifecycle and result categories that
  allow operators to distinguish normal drain, timeout, cleanup failure, and
  startup failure

### Requirement: Product verification and recovery signals are inspectable
The service and repository verification commands SHALL provide bounded,
actionable evidence for product verification, backup creation, restore,
restore verification, and the handoff of successful restore proof to assurance.

#### Scenario: Product verification fails
- **WHEN** a product-verification phase fails
- **THEN** its result identifies the bounded phase and failing subsystem, retains
  or points to safe diagnostic artifacts, and excludes generated credentials,
  unredacted connection strings, and scope identifiers from ordinary logs

#### Scenario: Operator records restore proof
- **WHEN** an operator submits a successful restore-verification result to the
  assurance surface
- **THEN** telemetry records a bounded proof outcome and freshness category
  without treating the backup artifact path, checksum, target database, or scope
  as a metric label

