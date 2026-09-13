## ADDED Requirements

### Requirement: Maintenance and freshness telemetry remains bounded

The service MUST expose low-cardinality metrics and bounded structured logs for
maintenance execution, lease/retry/recovery outcomes, projection freshness and
SLO results, and conformance closure. Labels MUST use fixed categories or
buckets and MUST NOT contain tenant, project, namespace, query, memory/event
identifiers, provider payloads, or credentials.

#### Scenario: Scope maintenance emits telemetry

- **WHEN** a scope-bound maintenance execution completes
- **THEN** metrics and logs contain only fixed job, outcome, freshness, retry, latency, and SLO categories

#### Scenario: High-cardinality label is supplied

- **WHEN** instrumentation receives a raw scope value or identifier as a label
- **THEN** it rejects or buckets the value before emission and preserves the bounded telemetry contract
