## ADDED Requirements

### Requirement: Task success evaluations are durable and scoped
The service SHALL allow authorized scoped callers to record durable task-success evaluations for external agent tasks without Stele executing the task or judging the answer.

#### Scenario: Caller records task evaluation
- **WHEN** a caller records a task evaluation with scope, task objective, success criteria, external verdict, evidence links, actor, reason, metadata, and optional idempotency key
- **THEN** the service persists the evaluation with tenant, project, namespace, bounded verdict, attribution, creation time, and scoped evidence references

#### Scenario: Task evaluation targets unauthorized scope
- **WHEN** a caller records or reads a task evaluation outside the authorized tenant, project, or namespace
- **THEN** the service rejects the request without exposing task, session, memory, feedback, or event content

#### Scenario: Caller repeats task evaluation request
- **WHEN** a caller records a task evaluation with the same resolved scope and idempotency key
- **THEN** the service returns the existing evaluation rather than creating duplicate task evidence

### Requirement: Task verdict taxonomy is bounded
The service MUST classify task outcomes using stable bounded verdict and failure categories suitable for API responses, diagnostics, ranking signals, quality findings, and metrics.

#### Scenario: Task verdict is valid
- **WHEN** a task evaluation uses a supported verdict such as `succeeded`, `failed`, `partial`, or `inconclusive`
- **THEN** the service accepts the verdict and stores it as a bounded category

#### Scenario: Task verdict is unsupported
- **WHEN** a task evaluation uses a free-form or unsupported verdict
- **THEN** the service rejects the request rather than creating an unbounded category

#### Scenario: Failure category is valid
- **WHEN** a task evaluation includes failure categories such as `memory_missing`, `memory_noisy`, `memory_stale`, `memory_irrelevant`, `agent_runtime`, `external_tool`, or `unknown`
- **THEN** the service stores those categories as bounded diagnostic signals

### Requirement: Task evidence references service records
The service SHALL allow task evaluations to reference scoped memory sessions, turns, raw events, outcome events, verification attempts, expected recall targets, usefulness feedback, context citations, derived insights, quality findings, repair plans, and memory subjects as evidence.

#### Scenario: Evaluation references session evidence
- **WHEN** a task evaluation references a session, turn, verification, outcome event, expected recall target, feedback record, context citation, derived insight, quality finding, repair plan, raw event, or memory subject in the same scope
- **THEN** the service links the evaluation to that evidence without copying hidden content into public task report fields

#### Scenario: Evaluation references out-of-scope evidence
- **WHEN** a task evaluation references evidence outside the resolved scope
- **THEN** the service rejects or omits the reference without revealing whether the target evidence exists

#### Scenario: Evaluation references opaque external evidence
- **WHEN** a task evaluation includes a caller-provided opaque evidence token
- **THEN** the service stores the token as opaque task evidence and does not treat it as a Stele event, memory, citation, insight, session, turn, verification, feedback, quality, or repair identifier

### Requirement: Task evaluation correction preserves audit history
The service SHALL support append-only correction or supersession of task evaluations without deleting the original task evaluation record.

#### Scenario: Caller supersedes task evaluation
- **WHEN** an authorized caller or administrator supersedes a task evaluation with actor and reason attribution
- **THEN** the service records the supersession as a new audit event and excludes the superseded task evaluation from active summaries, quality findings, and rollout signals

#### Scenario: Supersession targets unauthorized task evaluation
- **WHEN** a caller attempts to supersede a task evaluation outside the authorized scope
- **THEN** the service rejects the request without exposing task evaluation existence, verdict, evidence, or linked memory content

### Requirement: Task reports expose bounded memory contribution diagnostics
The service SHALL expose scoped task reports that summarize task verdict evidence, memory contribution signals, feedback summaries, session verification, quality findings, and recommended next actions.

#### Scenario: Caller reads authorized task report
- **WHEN** an authorized caller reads a task evaluation report within scope
- **THEN** the response includes bounded verdict, success criteria summary, linked session/turn/verification ids, feedback summary ids, memory contribution categories, quality finding codes, repair plan ids, and next actions

#### Scenario: Report references hidden memory
- **WHEN** hidden, suppressed, forgotten, deleted, or out-of-scope memory contributes to task diagnostics
- **THEN** the task report exposes only lifecycle-safe aggregate diagnostics and stable reason codes without hidden memory content

#### Scenario: Administrator inspects detailed task evidence
- **WHEN** an authorized administrator reads task evaluation detail within scope
- **THEN** the service can include scoped evidence links and audit metadata while preserving isolation and lifecycle visibility rules

### Requirement: Task summaries are rebuildable from durable evidence
The service SHALL derive task-success summaries from durable task evaluation records while preserving evaluation records as the source of truth.

#### Scenario: Summary is computed for a memory subject
- **WHEN** task evaluations reference a memory, raw event, citation, derived insight, expected recall target, session, turn, verification, quality finding, or repair plan
- **THEN** the service can expose aggregate success, failure, partial, inconclusive, memory-contribution, and last-evaluation counts for that subject

#### Scenario: Summary is rebuilt
- **WHEN** task summary aggregation is rerun or repaired
- **THEN** the service recomputes summaries from durable task evaluations and linked evidence rather than relying on non-auditable counters

#### Scenario: Corrected task evaluation exists
- **WHEN** a task evaluation has been superseded or corrected by an authorized record
- **THEN** default summaries exclude the superseded evaluation while admin inspection can still expose historical task evidence

### Requirement: Task evaluations remain service-side contracts
The service MUST NOT execute external agents, invoke models, build prompts, generate final answers, or infer task success while recording task evaluations.

#### Scenario: Caller records task outcome
- **WHEN** a caller submits objective, criteria, verdict, and evidence after an external task run
- **THEN** Stele validates and persists memory-related task evidence without running the external task or judging the answer

#### Scenario: Caller omits external verdict
- **WHEN** a task evaluation request lacks a supported external verdict
- **THEN** the service rejects the request or records an explicit `inconclusive` verdict only when the request declares that bounded verdict
