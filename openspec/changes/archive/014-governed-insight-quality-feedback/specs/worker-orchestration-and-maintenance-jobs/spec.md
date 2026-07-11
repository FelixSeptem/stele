## ADDED Requirements

### Requirement: Insight derivation consumes feedback idempotently
The service SHALL allow background insight derivation and maintenance jobs to consume quality feedback summaries in an idempotent, retry-safe manner.

#### Scenario: Derivation consumes same feedback twice
- **WHEN** a derivation job is retried for the same scope, evidence window, and feedback state
- **THEN** the service applies the same effective insight update or lifecycle decision without creating duplicate feedback-driven transitions

#### Scenario: Feedback changes between derivation runs
- **WHEN** new feedback is recorded after a prior derivation run
- **THEN** a later derivation or maintenance run can evaluate the updated quality summary and record any resulting insight change with fresh audit attribution

### Requirement: Feedback-driven suppression is scheduler-safe
The service MUST perform feedback-driven suppression or review marking through durable background execution semantics rather than foreground feedback write shortcuts.

#### Scenario: Operator records negative feedback
- **WHEN** an operator records negative feedback for an active insight
- **THEN** the feedback write path does not synchronously execute broad insight derivation or maintenance work

#### Scenario: Scheduler processes feedback-driven lifecycle work
- **WHEN** scheduled insight maintenance evaluates feedback that meets a suppression or review policy
- **THEN** it records the lifecycle or review decision through the same durable, auditable job path used for derived insight maintenance
