## ADDED Requirements

### Requirement: Reflection and derived maintenance use unified work items

Reflection triggers, compaction requests, context projection rebuilds, derived
insight maintenance, and bounded replay work SHALL enter the unified derived
work execution path rather than creating independent recovery queues.

#### Scenario: Session completion triggers reflection

- **WHEN** a scoped session completion produces a new input watermark
- **THEN** the service enqueues one idempotent reflection work item for that watermark

#### Scenario: Projection freshness falls behind

- **WHEN** a projection becomes stale or divergent
- **THEN** the service enqueues a scoped rebuild work item without executing rebuild inline in a public request

### Requirement: Derived workers resume from monotonic checkpoints

Workers SHALL advance transcript offsets and source-watermark checkpoints
monotonically under lease ownership and SHALL resume from the latest committed
checkpoint after restart or reclaim.

#### Scenario: Worker restarts after checkpoint

- **WHEN** a reflection worker stops after committing an offset
- **THEN** a later worker resumes from that offset and preserves earlier checkpoint history

#### Scenario: Worker attempts offset regression

- **WHEN** a worker submits an offset lower than the committed offset
- **THEN** the checkpoint is rejected and the existing progress remains unchanged

### Requirement: Compaction chains follow-up work durably

Successful compaction SHALL persist evidence coverage and enqueue any follow-up
reflection through the same idempotent derived work path.

#### Scenario: Compaction completes

- **WHEN** compaction produces a summary version and source-watermark evidence
- **THEN** the service stores the evidence and creates at most one follow-up reflection work item for the resulting watermark

### Requirement: Derived work exhaustion is inspectable

Retry-exhausted reflection, compaction, projection, replay, or insight work SHALL
remain terminal and MUST NOT be automatically re-enqueued.

#### Scenario: Retry budget is exhausted

- **WHEN** derived work reaches its configured retry limit
- **THEN** the service records an exhausted disposition and requires governed recovery to re-enter normal claim flow
