## ADDED Requirements

### Requirement: Reconciliation telemetry is bounded and actionable

The service SHALL emit low-cardinality metrics and structured logs for reconciliation start, completion, freshness expiry, watermark mismatch, policy incompatibility, rollback-proof expiry, revocation, restoration, retry, and terminal failure. Telemetry MUST use category and outcome labels rather than scope values, evidence IDs, queries, provider payloads, or credentials.

#### Scenario: Reconciliation completes

- **WHEN** a reconciliation run records eligible, revoked, or unchanged outcomes
- **THEN** metrics and logs expose bounded outcome categories, duration and batch buckets, and a redacted run status

#### Scenario: Sensitive field reaches instrumentation

- **WHEN** instrumentation receives a raw scope value, evidence payload, query, or credential
- **THEN** the value is omitted or redacted before emission and no high-cardinality label is created

### Requirement: Eligibility changes are auditable in operational diagnostics

Operational diagnostics SHALL distinguish current eligibility from historical evidence verdicts and SHALL expose bounded counts and reason categories for reconciliation lag, failures, revocations, and restorations.

#### Scenario: Operator inspects reconciliation health

- **WHEN** an authorized operator reads release-evidence operational diagnostics
- **THEN** the response includes freshness and run-health buckets without exposing evidence content or foreign scope data

