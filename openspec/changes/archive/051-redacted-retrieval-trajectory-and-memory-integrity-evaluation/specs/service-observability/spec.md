## ADDED Requirements

### Requirement: Trajectory and integrity telemetry is low-cardinality

The service MUST expose low-cardinality metrics and bounded lifecycle logs for
trajectory collection, integrity checks, replay, retention cleanup, and hard
safety failures. Labels and fields MUST exclude tenant, project, namespace,
query text, memory/event identifiers, report IDs, provider payloads,
credentials, and reason text.

#### Scenario: Evaluation and integrity checks complete

- **WHEN** trajectory collection or an integrity evaluation completes, degrades, fails, or requires review
- **THEN** metrics record status, component, verdict, stable finding category, and bounded operation outcome without high-cardinality identifiers

#### Scenario: Retention cleanup runs

- **WHEN** derived trajectory or integrity artifacts are retained, deleted, skipped, or fail cleanup
- **THEN** telemetry records the bounded cleanup outcome and expiration category without exposing source records or scope values
