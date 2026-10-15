## ADDED Requirements

### Requirement: Provider conformance proves durable reference readiness

The Provider conformance suite SHALL include supported reference writes,
governance status, exact-path reads, identical/conflicting replay, non-root path
and UUID lineage checks, lifecycle removal, foreign-scope safety and API/worker
restart recovery. Results SHALL remain separate from ranking metrics.

#### Scenario: Fresh public reference round trip passes
- **WHEN** a fresh PostgreSQL + pgvector fixture uses official bootstrap and the
  public Provider API to write, govern and read a task/run or compilation
  reference before and after restart
- **THEN** the report records a passing durable-reference outcome with bounded
  provenance and no raw credential, scope or payload disclosure

#### Scenario: HTTP acceptance lacks durable completion
- **WHEN** an event is accepted but governance is pending, failed, or missing
  after the bounded observation window
- **THEN** conformance records a nonpassing dependency/contract outcome

### Requirement: Reference evidence records upstream provenance

Durable-reference evidence SHALL pin service/image, OpenAPI, Provider schema,
migration and fixture provenance and SHALL redact credentials, authorization
headers, raw values, scope values and unbounded payloads.

#### Scenario: Evidence artifact is inspected
- **WHEN** a maintainer reviews the durable-reference report
- **THEN** it contains bounded outcome categories, counts, hashes where needed
  and provenance sufficient to reproduce the fixture without secret material
