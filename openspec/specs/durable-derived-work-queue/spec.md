# durable-derived-work-queue Specification

## Purpose
Provide a scoped, idempotent execution queue for derived reflection, compaction,
projection, and maintenance work while keeping PostgreSQL the durable source of
truth and making lossy buffering explicit.

## Requirements

### Requirement: Derived work has one exact-scope identity

The service SHALL assign each derived work item a stable identity from work kind,
normalized tenant/project/namespace scope, input/source watermark, and bounded
idempotency parameters.

#### Scenario: Duplicate derived trigger arrives

- **WHEN** the same scoped work kind and watermark is submitted again
- **THEN** the service returns the existing work identity and does not create a second durable execution

#### Scenario: Foreign scope trigger arrives

- **WHEN** a trigger references a scope outside the authorized exact scope
- **THEN** the service rejects it before creating or revealing a work item

### Requirement: Durable queue claims are lease-safe

The PostgreSQL queue SHALL persist queued, claimed, running, retry, terminal,
lease, and bounded failure state, and a worker that loses its lease MUST be
unable to checkpoint, complete, or publish derived output.

#### Scenario: Worker lease expires

- **WHEN** a worker stops renewing an active derived work lease
- **THEN** another worker can reclaim the item from its latest committed checkpoint

#### Scenario: Stale worker writes after reclaim

- **WHEN** a stale worker attempts a checkpoint or completion after ownership changed
- **THEN** the service rejects the mutation with a bounded lease-conflict result

### Requirement: Memory buffering is explicitly lossy

The `memory_buffer` mode SHALL bound capacity and flush work to PostgreSQL in
batches; unflushed work MAY be lost on process loss, overflow, eviction, or
flush failure and MUST be reported as non-durable operational loss.

#### Scenario: Buffer accepts work

- **WHEN** a derived task is placed in the memory buffer
- **THEN** it is reported as buffered rather than durable until PostgreSQL flush succeeds

#### Scenario: Buffer drops work

- **WHEN** capacity, eviction, or process loss removes an unflushed task
- **THEN** the service increments a bounded loss category and does not report durable completion

### Requirement: Queue mode changes are startup-governed

The service SHALL select exactly one queue mode at process startup and MUST NOT
silently transfer in-flight durable work into the lossy buffer.

#### Scenario: Process starts in durable mode

- **WHEN** `postgres_durable` is configured
- **THEN** new work is persisted before worker execution and existing durable work remains claimable

#### Scenario: Process starts in memory mode

- **WHEN** `memory_buffer` is configured
- **THEN** the process exposes non-durable mode and bounded buffer status to authorized inspection

### Requirement: Derived queue retention is bounded

The service SHALL retain terminal summaries and required audit transitions while
pruning only expired queue detail and loss/flush detail according to bounded
retention policy.

#### Scenario: Queue detail expires

- **WHEN** derived work detail exceeds its configured retention window
- **THEN** cleanup removes only eligible detail and preserves terminal outcome and evidence references
