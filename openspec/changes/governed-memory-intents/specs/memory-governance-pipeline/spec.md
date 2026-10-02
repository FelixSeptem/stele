## ADDED Requirements

### Requirement: Accepted intents enter asynchronous governance
The governance pipeline MUST be able to consume accepted memory intents as durable work items and MUST apply the same scope, lifecycle, provenance, candidate, retry, and audit controls used for raw-event governance.

#### Scenario: Worker claims an accepted intent
- **WHEN** an accepted intent is queued and its lease is available
- **THEN** a worker claims it within the owning scope and emits candidate or lifecycle work through the existing governance path

#### Scenario: Intent processing exceeds retry budget
- **WHEN** an intent repeatedly fails a bounded dependency or processing step
- **THEN** the worker records a failed or suppressed outcome and stops retrying after the configured budget
