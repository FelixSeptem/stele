## ADDED Requirements

### Requirement: Scope proof runs are durable and scoped
The service SHALL allow authorized administrators to create durable proof runs for one tenant, project, and namespace.

#### Scenario: Administrator creates a scope proof run
- **WHEN** an authorized administrator creates a proof run for a resolved scope with actor and reason attribution
- **THEN** the service persists a proof run with scope, status, requested checks, fixture mode, actor, reason, and creation time

#### Scenario: Proof run targets unauthorized scope
- **WHEN** an administrator attempts to create or inspect a proof run outside their authorized scope
- **THEN** the service rejects the request without exposing proof run content

#### Scenario: Proof run uses explicit fixture metadata
- **WHEN** a proof run writes smoke or fixture data
- **THEN** each created event or evidence link is marked with proof fixture metadata and remains bound to the requested scope

### Requirement: Scope proof runs execute bounded proof steps
The service MUST execute scope proof runs through a bounded sequence of service-side proof steps.

#### Scenario: Proof run validates the service loop
- **WHEN** a proof run executes successfully
- **THEN** it records step outcomes for scope resolution, event ingestion, governance processing, retrieval recall, context assembly, optional replay, quality evaluation, and final verdict

#### Scenario: Proof step fails
- **WHEN** a proof step cannot complete within configured limits
- **THEN** the proof run records a bounded failure reason, step status, diagnostic evidence, and final verdict without continuing unsafe mutations

#### Scenario: Proof run observes degraded dependencies
- **WHEN** a proof step completes but reports degraded admission, worker, semantic projection, or context quality state
- **THEN** the proof run can finish with `passed_degraded` and include the degraded finding codes in the report

### Requirement: Memory sessions model external agent memory use
The service SHALL provide a scoped memory session contract for external agents to use Stele memory without Stele executing the agent.

#### Scenario: Client creates a memory session
- **WHEN** a scoped caller creates a memory session with optional actor, reason, and metadata
- **THEN** the service persists a session run with scope, status, creation time, and attribution

#### Scenario: Client requests session context
- **WHEN** a caller starts a session turn with an input query or interaction summary
- **THEN** the service assembles scoped context and records the context evidence used for that turn

#### Scenario: Client records external turn outcome
- **WHEN** the external agent has produced its response and memory-relevant outcome
- **THEN** the caller can record bounded turn outcome events through the session contract without Stele generating the response

#### Scenario: Session verifies post-turn memory
- **WHEN** governed processing for the turn outcome becomes visible or reaches a bounded wait limit
- **THEN** the service can verify retrieval or context recall for expected session evidence and record a session verdict

### Requirement: Proof and session reports are inspectable
The service SHALL expose scoped reports for proof and memory session runs.

#### Scenario: Administrator reads a proof report
- **WHEN** an authorized administrator requests a proof report within scope
- **THEN** the service returns run status, verdict, step summaries, failure categories, evidence links, and recommended next admin actions

#### Scenario: Caller reads a session report
- **WHEN** an authorized caller requests a session report within scope
- **THEN** the service returns session status, turn summaries, context evidence, outcome event ids, verification status, and bounded failure reasons

#### Scenario: Report contains high-cardinality evidence
- **WHEN** a report references specific events, memories, evaluations, replay runs, repair plans, sessions, or proof runs
- **THEN** the identifiers are stored only in scoped durable evidence and are not exported as metric labels

### Requirement: Proof and session reruns preserve history
The service MUST preserve prior proof and session history when rerunning checks.

#### Scenario: Proof run is rerun
- **WHEN** an administrator reruns a prior proof run
- **THEN** the service creates a new proof run linked to the prior run template instead of overwriting the previous report

#### Scenario: Session verification is rerun
- **WHEN** a caller reruns session verification for a completed session
- **THEN** the service creates fresh verification evidence while preserving the original session turns and prior verdict
