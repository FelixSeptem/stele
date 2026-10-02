## ADDED Requirements

### Requirement: Derived queue telemetry is low-cardinality

The service SHALL emit bounded metrics and structured logs for enqueue, duplicate,
claim, renew, reclaim, checkpoint, flush, drop, retry, exhaustion, completion,
and freshness outcomes using fixed labels and buckets.

#### Scenario: Buffer drops work

- **WHEN** memory-buffer work is dropped before durable flush
- **THEN** telemetry records a bounded drop/result category without task IDs, scope values, or raw reasons

#### Scenario: Reflection checkpoint advances

- **WHEN** a lease-owned reflection checkpoint is committed
- **THEN** telemetry records a bounded checkpoint/result category and duration bucket without transcript content

### Requirement: Sensitive queue fields are excluded

Queue telemetry MUST reject, redact, or bucket raw payloads, query text, scope
values, identifiers, credentials, provider responses, and worker IDs.

#### Scenario: Instrumentation receives sensitive input

- **WHEN** queue instrumentation receives raw task content or a scope value
- **THEN** the emitted event contains only an allowed fixed category or bucket
