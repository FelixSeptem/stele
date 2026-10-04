## ADDED Requirements

### Requirement: Offline provider contract replay covers synchronization

The provider contract runner SHALL support offline fixtures for capability synchronization metadata, initial snapshot, ordered event batches, cursor continuation, `sync_complete`, duplicate retry, and `resync_required` recovery without requiring a remote agent, model, or streaming transport.

#### Scenario: Replay a valid synchronization fixture

- **WHEN** a fixture contains a supported exact scope, snapshot watermark, ordered events, and acknowledged cursor
- **THEN** the runner validates event identities, ordering, completion, scope, and bounded response shape and records an operation-level result

#### Scenario: Replay a retention-gap fixture

- **WHEN** a fixture presents an expired cursor
- **THEN** the runner records the required resynchronization outcome and does not treat skipped events as a successful synchronized state

### Requirement: Synchronization safety remains separate from retrieval quality

Synchronization operation accuracy, replay determinism, cursor recovery, retention-gap handling, scope safety, lifecycle safety, and redaction outcomes SHALL be reported under the provider-contract family and MUST NOT be merged into retrieval Recall@k, MRR, nDCG, or ranking rollout evidence.

#### Scenario: Produce a synchronization contract report

- **WHEN** synchronization fixtures finish
- **THEN** the report contains contract family identity, snapshot and replay counts, recovery outcomes, safety failures, and artifact provenance independent of retrieval metrics
