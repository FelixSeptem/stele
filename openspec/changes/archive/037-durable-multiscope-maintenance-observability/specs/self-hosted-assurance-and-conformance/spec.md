## ADDED Requirements

### Requirement: Assurance includes durable maintenance closure

The service SHALL include maintenance scope coverage, lease recovery,
projection freshness/rebuild, retention safety, and telemetry redaction in
authorized conformance and readiness evidence. The evidence SHALL remain
diagnostic, bounded, and scope-safe.

#### Scenario: Readiness includes maintenance evidence

- **WHEN** an authorized conformance run evaluates a durable scope
- **THEN** its readiness evidence includes bounded maintenance coverage, freshness, recovery, retention, and observability outcomes

#### Scenario: Maintenance evidence is incomplete

- **WHEN** a required scope, recovery result, freshness result, or redaction check is missing
- **THEN** the conformance result is incomplete or failed and cannot be reported as operationally ready
