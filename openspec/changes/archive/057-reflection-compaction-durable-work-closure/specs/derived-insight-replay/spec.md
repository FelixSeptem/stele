## ADDED Requirements

### Requirement: Replay uses the unified derived work queue

Derived insight replay and backfill SHALL enqueue bounded, exact-scope work items
with stable replay identity, source watermark, actor/reason attribution, and
idempotent retry behavior.

#### Scenario: Operator applies a replay plan

- **WHEN** an authorized operator submits a bounded replay apply
- **THEN** the service records a durable work identity and returns before broad replay execution

#### Scenario: Replay worker restarts

- **WHEN** replay execution stops after partial progress
- **THEN** the next worker resumes from the durable checkpoint and does not duplicate insight lifecycle transitions

### Requirement: Replay loss is not reported as applied

A replay task dropped in `memory_buffer` mode, rejected during flush, or exhausted
without completion MUST remain non-applied and visible as a bounded disposition.

#### Scenario: Replay work is dropped

- **WHEN** an unflushed replay task is lost
- **THEN** the replay report records a dropped/non-applied category and active insights remain unchanged
