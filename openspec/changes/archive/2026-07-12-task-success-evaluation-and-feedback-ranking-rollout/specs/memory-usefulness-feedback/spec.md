## ADDED Requirements

### Requirement: Task evaluations can contribute bounded usefulness signals
The service SHALL allow task-success evaluations to contribute bounded aggregate usefulness signals without mutating feedback records or canonical memory.

#### Scenario: Task success references useful memory
- **WHEN** a task evaluation records a successful task with linked memory, citation, session, turn, verification, feedback, or expected-recall evidence
- **THEN** usefulness summaries can include bounded task-success counts and last task evaluation time for authorized reports and rollout diagnostics

#### Scenario: Task failure references memory contribution issue
- **WHEN** a task evaluation records `failed` or `partial` with memory contribution categories such as missing, noisy, stale, or irrelevant
- **THEN** usefulness summaries can include bounded task-failure contribution counts without creating or superseding feedback records automatically

#### Scenario: Task evidence is hidden or out of scope
- **WHEN** task evaluation evidence references hidden, suppressed, forgotten, deleted, or out-of-scope memory
- **THEN** usefulness summaries expose only aggregate lifecycle-safe diagnostics and stable reason codes where authorized

### Requirement: Usefulness feedback can link to task evaluations
The service SHALL allow usefulness feedback records to reference task evaluations as source evidence while preserving feedback idempotency and supersession behavior.

#### Scenario: Feedback references task evaluation
- **WHEN** a caller records usefulness feedback after a task evaluation
- **THEN** the feedback can link to the scoped task evaluation id and source surface without treating the task verdict as direct memory mutation authority

#### Scenario: Task-linked feedback is superseded
- **WHEN** task-linked feedback is superseded by an authorized correction
- **THEN** active usefulness and rollout summaries exclude the superseded feedback while preserving the task link for admin audit history
