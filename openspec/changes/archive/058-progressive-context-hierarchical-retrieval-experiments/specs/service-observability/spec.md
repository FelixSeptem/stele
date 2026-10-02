## ADDED Requirements

### Requirement: Progressive experiment telemetry is low-cardinality

The service MUST expose bounded metrics and structured logs for progressive
level evaluation, parent-first planning, shadow outcomes, fallback, freshness,
budget, rollback, and release eligibility. Labels MUST use fixed categories or
buckets and MUST exclude tenant, project, namespace, query text, source text,
memory identifiers, report identifiers, credentials, DSNs, and raw scores.

#### Scenario: Shadow evaluation emits telemetry
- **WHEN** a progressive level or parent-first plan completes, degrades, or
  fails
- **THEN** telemetry records bounded level, strategy, mode, result, freshness,
  fallback, and latency categories without high-cardinality identifiers

#### Scenario: Sensitive telemetry input is supplied
- **WHEN** instrumentation receives a raw scope, query, identifier, DSN, or
  reason string
- **THEN** it rejects, redacts, or buckets the value before emission

### Requirement: Experimental diagnostics are authorized and bounded

Operator diagnostics MAY summarize shadow comparison, freshness, budget,
expansion, and rollback outcomes only for an authorized scope. Diagnostics MUST
not disclose hidden content, foreign identifiers, raw provider data, or ordinary
public retrieval internals.

#### Scenario: Operator inspects experiment health
- **WHEN** an authorized operator requests progressive retrieval diagnostics
- **THEN** the service returns aggregate counts and stable reason categories
  sufficient to understand eligibility and fallback
