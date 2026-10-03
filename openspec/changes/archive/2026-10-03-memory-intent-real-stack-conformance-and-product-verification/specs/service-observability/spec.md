## ADDED Requirements

### Requirement: Intent product conformance telemetry is bounded
The service and its product-verification tooling MUST expose low-cardinality metrics and bounded lifecycle logs for intent conformance phases, prerequisite outcomes, replay/conflict checks, queue recovery, rollback, cleanup, and redaction checks without scope values, payloads, claims, identifiers, credentials, or raw errors.

#### Scenario: Conformance phase completes
- **WHEN** an intent conformance phase passes, skips, degrades, times out, or fails
- **THEN** telemetry records bounded operation, phase, result, prerequisite category, recovery category, and duration buckets only

#### Scenario: Sensitive conformance data is supplied
- **WHEN** the runner or instrumentation receives a DSN, tenant/project/namespace value, intent ID, memory ID, evidence reference, payload, or raw database/provider error
- **THEN** the exported telemetry and ordinary logs reject, redact, or bucket the value before emission

### Requirement: Intent product conformance diagnostics are operator-visible
Authorized operators MUST be able to inspect aggregate intent conformance status, dominant failure categories, recovery outcomes, rollback outcomes, and evidence freshness without receiving hidden content or foreign-scope identifiers.

#### Scenario: Operator inspects intent conformance health
- **WHEN** an authorized operator requests the latest intent product-verification summary
- **THEN** the response reports bounded phase and gate categories sufficient to decide whether evidence is consumable for release review

#### Scenario: Conformance includes hidden or foreign evidence
- **WHEN** a hard gate fails because of hidden lifecycle data, foreign scope, stale work, or an unsafe retry
- **THEN** diagnostics expose only aggregate counts and stable reason categories while preserving the underlying records for authorized audit
