## ADDED Requirements

### Requirement: Scheduler runs have stable append-only history

Every scheduled maintenance dispatch SHALL create or resolve a stable run
identity from job class, exact scope, cadence/idempotency window, and bounded
execution parameters. The service MUST append attempt and terminal disposition
records for accepted, duplicate, skipped, retrying, exhausted, cancelled,
reclaimed, and completed outcomes without replacing prior history.

#### Scenario: Duplicate dispatch is recorded once
- **WHEN** two scheduler ticks submit the same job identity for the same exact
  scope and cadence window
- **THEN** one run acquires execution ownership and the other records a bounded
  duplicate disposition without running the maintenance body

#### Scenario: Retry creates append-only attempt history
- **WHEN** a leased run fails retryably and is eligible for another attempt
- **THEN** the next attempt references the same stable run identity, records its
  attempt number and retry category, and preserves the earlier failure record

#### Scenario: Terminal run is replayed
- **WHEN** the same stable identity is submitted after a terminal completion,
  exhaustion, cancellation, or bounded skip
- **THEN** the scheduler records a duplicate/terminal disposition and does not
  execute durable maintenance work again

### Requirement: Lease recovery exposes checkpointed terminal state

Scheduler and worker execution SHALL persist lease owner/expiry, attempt,
checkpoint or source watermark, retry eligibility, recovery disposition, and
terminal state as one bounded run record. Stale-owner reclamation MUST be
lease-safe and MUST resume from the last durable checkpoint without rewriting
canonical source records.

#### Scenario: Worker restarts after checkpoint
- **WHEN** a worker stops after recording a checkpoint and its lease expires
- **THEN** a later worker reclaims the run, records the recovery transition,
  resumes from that checkpoint, and preserves the prior attempt history

#### Scenario: Lease renewal conflicts
- **WHEN** a worker renews or completes a run owned by another worker or already
  terminal run
- **THEN** the operation returns a bounded lease-conflict/terminal category and
  performs no further durable mutation

### Requirement: Retry exhaustion and cancellation remain inspectable

The scheduler SHALL mark retry-exhausted, operator-cancelled, and cleanup-
blocked runs as terminal dispositions that are not silently re-enqueued.
Explicit recovery MAY create a new attempt only through the ordinary lease-safe
claim path and MUST retain the original terminal history.

#### Scenario: Retry budget is exhausted
- **WHEN** a run reaches its configured retry limit
- **THEN** the run is terminally marked exhausted or manual-review-required,
  remains inspectable, and is excluded from automatic claims

#### Scenario: Operator cancels a pending run
- **WHEN** an authorized operator cancels a run that is not actively leased
- **THEN** the run records cancellation and is not claimed by later scheduler
  ticks unless an explicit governed recovery reopens it
