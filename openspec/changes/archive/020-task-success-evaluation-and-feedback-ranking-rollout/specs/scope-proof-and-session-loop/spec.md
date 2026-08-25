## ADDED Requirements

### Requirement: Memory sessions can link task-success evaluations
The service SHALL allow scoped task-success evaluations to link to memory sessions, turns, outcomes, and verification attempts without changing session execution semantics.

#### Scenario: Task evaluation references session run
- **WHEN** a task evaluation references an authorized memory session, turn, outcome event, or verification attempt
- **THEN** the session report can expose bounded task evaluation ids, verdict categories, memory contribution categories, and next actions for the same scope

#### Scenario: Session report includes task failure signal
- **WHEN** a linked task evaluation records `failed` or `partial` with memory contribution categories
- **THEN** the session report includes bounded task-success diagnostics without exposing hidden or out-of-scope evidence

#### Scenario: Session has no task evaluation
- **WHEN** a caller reads a session report that has no linked task-success evaluation
- **THEN** the service preserves the existing report shape except for empty or omitted task evaluation fields

### Requirement: Session verification can support task-success evidence
The service SHALL allow session verification history to be used as evidence in task-success reports and ranking rollout diagnostics.

#### Scenario: Task evaluation uses verification evidence
- **WHEN** a task evaluation references session verification attempts in the same scope
- **THEN** the task report can summarize latest verification verdict and verification history links as bounded evidence

#### Scenario: Verification evidence is out of scope
- **WHEN** a task evaluation references verification evidence outside the authorized scope
- **THEN** the service rejects or omits the reference without exposing verification existence
