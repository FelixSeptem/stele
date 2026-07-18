# scope-proof-and-session-loop Specification

## Purpose
TBD - created by archiving change scope-proof-and-agent-session-memory-loop. Update Purpose after archive.
## Requirements
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

### Requirement: Memory session turns are idempotent
The service SHALL support idempotent memory session turn and outcome writes for external agent integrations.

#### Scenario: Caller repeats a turn creation request
- **WHEN** a caller creates a session turn with the same scope, session id, and idempotency key
- **THEN** the service returns the existing turn result rather than creating duplicate turn evidence

#### Scenario: Caller repeats an outcome request
- **WHEN** a caller records a turn outcome with the same scope, session id, turn id, and idempotency key
- **THEN** the service preserves idempotent behavior and avoids duplicate outcome events, expected recall records, or feedback attribution

### Requirement: Session outcomes can ingest bounded event payloads
The service SHALL allow memory session outcomes to include bounded memory-relevant event payloads that are written through the existing event ingestion contract.

#### Scenario: Caller records outcome payloads
- **WHEN** an external agent records a turn outcome with bounded event payloads
- **THEN** the service ingests those payloads through the normal event ingestion path and attaches session id, turn id, actor, reason, and outcome attribution metadata

#### Scenario: Outcome payload is invalid
- **WHEN** an outcome payload violates event validation, admission, size, or scope rules
- **THEN** the service rejects or reports that payload without bypassing event ingestion governance

### Requirement: Session reports include verification and feedback history
The service SHALL preserve session verification history and feedback summaries in scoped session reports.

#### Scenario: Session verification is requested multiple times
- **WHEN** a caller reruns verification for a session or turn
- **THEN** the service creates fresh verification evidence while preserving prior verification attempts and verdicts

#### Scenario: Session feedback exists
- **WHEN** feedback has been recorded for session context, citations, outcomes, expected recall, or turns
- **THEN** the session report includes bounded feedback summaries and next actions without exposing hidden or out-of-scope memory content

### Requirement: Memory sessions remain service-side contracts
The service MUST NOT execute the external agent while strengthening session contracts.

#### Scenario: Caller uses session APIs
- **WHEN** a caller creates turns, records outcomes, requests verification, or records feedback
- **THEN** Stele provides memory context, ingestion, verification, feedback evidence, and reports without invoking models, building prompts, or generating final answers

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

### Requirement: Proof and session evidence support conformance checks
The service SHALL allow assurance and conformance runs to reference proof and memory-session evidence without changing proof or session execution semantics.

#### Scenario: Conformance run checks session evidence chain
- **WHEN** a conformance profile requires session, context, outcome, and verification evidence
- **THEN** the conformance run can inspect scoped memory session records and report bounded missing-evidence diagnostics

#### Scenario: Conformance run references proof status
- **WHEN** a conformance profile requires recent scope proof evidence
- **THEN** the conformance run can include proof verdict, degraded components, and failure categories in its diagnostic summary

#### Scenario: Proof or session evidence is out of scope
- **WHEN** a conformance run encounters proof or session references outside the requested scope
- **THEN** the service excludes them from conformance evidence and does not expose their existence

### Requirement: Recovery verification can rerun proof and session checks
The service SHALL allow recovery verification to reference or dispatch existing proof and session verification checks while preserving history.

#### Scenario: Recovery verification reruns proof
- **WHEN** an incident recommends validating scope usability after remediation
- **THEN** recovery verification can reference a new proof run linked to the incident without overwriting prior proof history

#### Scenario: Recovery verification reruns session verification
- **WHEN** a conformance or health failure involved session recall evidence
- **THEN** recovery verification can request a new session verification attempt and link the result to the recovery report

#### Scenario: Recovery verification does not execute agent
- **WHEN** recovery verification checks memory session health
- **THEN** Stele verifies service-side memory evidence without invoking the external agent or generating a final answer

