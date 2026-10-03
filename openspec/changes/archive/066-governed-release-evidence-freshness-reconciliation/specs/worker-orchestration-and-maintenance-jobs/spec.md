## ADDED Requirements

### Requirement: Evidence reconciliation runs through durable maintenance orchestration

The scheduler and worker SHALL dispatch reconciliation as a scope-bound maintenance job using existing lease, retry, checkpoint, cancellation, and recovery semantics. The job MUST be bounded by evidence count, time, and retry budgets and MUST be safe to replay.

#### Scenario: Scheduler dispatches reconciliation

- **WHEN** the configured cadence reaches an eligible exact scope
- **THEN** the scheduler creates or reuses one durable reconciliation run and the worker claims it through the normal lease path

#### Scenario: Reconciliation exceeds its bounds

- **WHEN** a reconciliation run reaches its batch, time, or retry limit
- **THEN** it records a bounded terminal or resumable disposition and does not silently mark unprocessed evidence eligible

### Requirement: Manual reconciliation reuses the same durable execution path

An authorized manual trigger SHALL enqueue or reuse the same scope-bound job class used by scheduled reconciliation. It MUST NOT execute unbounded synchronous database work or seize an active worker lease.

#### Scenario: Operator triggers reconciliation

- **WHEN** an authorized operator requests a bounded reconciliation for an exact scope
- **THEN** the service returns a stable run identity and lets the worker process it using normal retry and recovery rules

#### Scenario: Operator targets an active run

- **WHEN** an operator requests a duplicate reconciliation while an equivalent run is leased
- **THEN** the service returns the existing run or a deduplicated outcome without taking ownership from the active worker

