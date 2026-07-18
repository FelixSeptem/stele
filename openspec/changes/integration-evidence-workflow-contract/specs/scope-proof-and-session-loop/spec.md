## ADDED Requirements

### Requirement: Session and proof evidence can attach to workflow runs
The service SHALL allow workflow runs to reference memory session and scope proof evidence without changing session or proof execution semantics.

#### Scenario: Workflow step references session context
- **WHEN** a workflow step records memory session, turn, context, outcome, or verification evidence for the same scope
- **THEN** the workflow run links that evidence as step progress without overwriting session, turn, outcome, or verification records

#### Scenario: Workflow step references scope proof
- **WHEN** a workflow step records scope proof run or proof report evidence for the same scope
- **THEN** the workflow run links the proof evidence and can use proof status as a bounded diagnostic signal

#### Scenario: Session or proof evidence is out of scope
- **WHEN** a workflow step references session or proof evidence outside the workflow run scope
- **THEN** the service excludes that evidence from workflow completion and does not expose target existence

### Requirement: Workflow next actions can recommend session and proof surfaces
The service SHALL recommend existing session and proof surfaces when workflow evidence is missing or stale.

#### Scenario: Workflow lacks session outcome
- **WHEN** a workflow run has context evidence but no required turn outcome evidence
- **THEN** next actions include a bounded recommendation to record the turn outcome through the memory session outcome route

#### Scenario: Workflow lacks proof verification
- **WHEN** a workflow run requires recent scope proof evidence and no valid proof exists
- **THEN** next actions include a bounded recommendation to create or inspect a scope proof through the existing admin proof routes
