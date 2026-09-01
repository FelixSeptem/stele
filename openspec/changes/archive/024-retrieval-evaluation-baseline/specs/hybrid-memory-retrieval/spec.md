## ADDED Requirements

### Requirement: Authorized retrieval evaluation diagnostics
The service SHALL support bounded candidate-channel and final-disposition diagnostics for
controlled retrieval evaluation without changing ordinary retrieval ranking or exposing
hidden evidence.

#### Scenario: Evaluation captures visible candidate channels
- **WHEN** an authorized evaluator runs a scoped retrieval fixture
- **THEN** it can identify whether a lifecycle-visible result was supplied by lexical,
  semantic, or enabled relation recall and record bounded channel-rank information

#### Scenario: Ordinary retrieval runs
- **WHEN** a client uses ordinary hybrid retrieval without an authorized evaluation path
- **THEN** the service preserves the existing result shape and does not expose internal
  candidate-channel diagnostics

#### Scenario: Candidate is hidden or out of scope
- **WHEN** a candidate would violate lifecycle visibility or resolved scope boundaries
- **THEN** evaluation diagnostics record only a stable aggregate exclusion or failure
  category and do not expose the candidate content or identifier
