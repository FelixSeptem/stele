# memory-quality-admission-repair Specification

## Purpose
TBD - created by archiving change memory-quality-admission-repair-loop. Update Purpose after archive.
## Requirements
### Requirement: Scoped memory quality evaluations
The service SHALL allow authorized administrators to create durable memory quality evaluation runs scoped by tenant, project, and namespace.

#### Scenario: Administrator creates a scoped evaluation
- **WHEN** an authorized administrator requests a quality evaluation for a resolved tenant, project, and namespace
- **THEN** the service persists an evaluation run with scope, requested checks, actor attribution, status, and creation time

#### Scenario: Evaluation targets an unauthorized scope
- **WHEN** an administrator requests a quality evaluation outside the authorized tenant, project, or namespace
- **THEN** the service rejects the request without creating an evaluation run

#### Scenario: Evaluation includes lifecycle-safe retrieval checks
- **WHEN** an evaluation probes retrieval or context assembly behavior
- **THEN** the evaluation verifies that suppressed, forgotten, expired, and deleted memory are excluded from default non-admin recall results

#### Scenario: Evaluation observes degraded semantic projection
- **WHEN** memory eligible for semantic recall lacks an active embedding because projection is missing, stale, rebuilding, failed, or provider-mismatched
- **THEN** the evaluation records a bounded finding without failing lexical retrieval or rewriting canonical memory

### Requirement: Evaluation findings use bounded taxonomy
The service MUST classify quality evaluation findings with stable categories, severities, component labels, and suggested repair action categories.

#### Scenario: Evaluation records a retrieval quality finding
- **WHEN** retrieval or context assembly output misses an expected memory, returns a lifecycle-hidden memory, or depends on unavailable semantic projection
- **THEN** the finding uses stable codes suitable for API responses, logs, reports, and metrics

#### Scenario: Evaluation records detailed evidence
- **WHEN** a finding references specific memory, event, embedding, or job records
- **THEN** the service stores detailed evidence in scoped durable records while excluding those high-cardinality identifiers from metric labels

### Requirement: Admission pressure participates in quality evaluation
The service SHALL include ingestion and repair admission pressure checks as part of quality evaluation runs when requested.

#### Scenario: Evaluation probes ingestion pressure
- **WHEN** the configured evaluation includes ingestion pressure checks
- **THEN** the service records the current admission decision, pressure finding codes, observed backlog signals, and dependency health summary for the requested scope

#### Scenario: Evaluation probes repair pressure
- **WHEN** the configured evaluation includes repair feasibility checks
- **THEN** the service records whether repair work can be accepted, queued, degraded, or rejected under current worker and dependency pressure

### Requirement: Repair plans are generated from evaluation findings
The service SHALL generate durable repair plans from scoped evaluation findings using only bounded and explicitly supported action categories.

#### Scenario: Repair plan is created from failed evaluation
- **WHEN** an authorized administrator requests a repair plan for an evaluation run with actionable findings
- **THEN** the service persists a repair plan linked to the evaluation run with proposed actions, action categories, target scope, limits, actor attribution, and dry-run status

#### Scenario: Finding has no safe automatic repair
- **WHEN** an evaluation finding cannot be mapped to a supported safe action
- **THEN** the repair plan records a `manual_review` action instead of attempting an unsupported mutation

#### Scenario: Repair action would rewrite canonical memory
- **WHEN** a proposed repair would require rewriting canonical memory content, provenance, or version history in place
- **THEN** the service rejects that action from the repair plan

### Requirement: Repair execution is durable and lease-safe
The service MUST execute approved repair plans through durable worker or scheduler execution with lease, retry, idempotency, and audit safeguards.

#### Scenario: Worker claims repair action
- **WHEN** an approved repair action is pending and eligible
- **THEN** a worker can claim it with durable ownership and execute it without requiring the admin request to remain open

#### Scenario: Repair action targets active leased work
- **WHEN** a repair action would retry or requeue work that is already owned by an active worker lease
- **THEN** the service rejects or defers the action rather than seizing ownership

#### Scenario: Repair execution fails
- **WHEN** repair execution fails before completion
- **THEN** the service records attempt count, failure summary, next eligibility, and audit metadata so the action can be retried or inspected

#### Scenario: Repair action is retried
- **WHEN** the same repair action is retried after partial progress
- **THEN** the service preserves idempotent behavior and avoids duplicate lifecycle transitions, embedding promotions, replay effects, or governance side effects

### Requirement: Repair actions reuse existing operational paths
The service SHALL implement repair actions as transitions back into existing governed execution paths rather than ad hoc data mutations.

#### Scenario: Repair retries embedding work
- **WHEN** a repair action targets failed, stale, missing, or provider-mismatched semantic projection
- **THEN** the action requeues or retries eligible embedding rebuild work through the existing durable rebuild path

#### Scenario: Repair requeues governance work
- **WHEN** a repair action targets exhausted or quarantined governance work
- **THEN** the action returns eligible work to the ordinary governance worker claim path

#### Scenario: Repair schedules derived insight replay
- **WHEN** a repair action targets stale or missing derived insight output
- **THEN** the action creates bounded replay work through the existing replay execution path

#### Scenario: Repair marks manual review
- **WHEN** a repair finding requires operator judgment
- **THEN** the action records manual review state and audit attribution without changing default retrieval behavior

### Requirement: Post-repair verification compares before and after quality
The service SHALL support verification runs that compare repair outcomes against the baseline evaluation findings.

#### Scenario: Verification runs after repair completion
- **WHEN** all executable actions in a repair plan complete or reach terminal status
- **THEN** the service can create a verification evaluation linked to the repair plan and baseline evaluation run

#### Scenario: Verification shows improvement
- **WHEN** the verification run no longer reports the targeted actionable findings
- **THEN** the service records the repair plan verification status as `passed` with before and after summary counts

#### Scenario: Verification still reports residual issues
- **WHEN** the verification run still reports targeted findings or dependency pressure remains degraded
- **THEN** the service records residual finding codes and marks the repair plan as `failed` or `manual_review` without rewriting canonical memory

### Requirement: Quality and repair reports are inspectable
The service SHALL expose scoped admin reads for evaluation reports, repair plans, repair actions, and verification outcomes.

#### Scenario: Administrator inspects evaluation report
- **WHEN** an authorized administrator requests an evaluation report within scope
- **THEN** the service returns run status, requested checks, summary counts, bounded finding codes, and links to detailed scoped evidence

#### Scenario: Administrator inspects repair plan
- **WHEN** an authorized administrator requests a repair plan within scope
- **THEN** the service returns proposed actions, execution status, verification status, audit attribution, and residual finding summaries

#### Scenario: Administrator requests out-of-scope report
- **WHEN** an administrator requests an evaluation or repair report outside the authorized tenant, project, or namespace
- **THEN** the service rejects the request without exposing report content

### Requirement: Proof and session failures can reference quality evaluation
The service SHALL allow proof and memory-session reports to create or reference scoped quality evaluations when memory quality affects proof or session verdicts.

#### Scenario: Proof detects retrieval quality failure
- **WHEN** a proof run misses expected fixture recall or observes lifecycle-hidden output
- **THEN** the proof report links to a scoped quality evaluation with bounded finding codes

#### Scenario: Session verification detects degraded recall
- **WHEN** session verification cannot recall expected turn outcome evidence
- **THEN** the session report can link to a quality evaluation or finding summary for the same scope

### Requirement: Proof and session repair recommendations remain approval-gated
The service MUST NOT automatically approve repair actions from proof or memory-session failures.

#### Scenario: Proof produces actionable findings
- **WHEN** proof-linked quality findings can be mapped to embedding retry, governance requeue, derived insight replay, or manual review
- **THEN** the report may recommend a repair plan but leaves approval to an authorized admin action

#### Scenario: Session produces actionable findings
- **WHEN** session verification produces actionable quality findings
- **THEN** the service preserves the existing repair planning approval boundary and does not run repairs inline with the session request

### Requirement: Feedback can create bounded quality findings
The service SHALL convert repeated active negative usefulness feedback, missing expected recall, and hidden-memory safety feedback into scoped quality findings or finding summaries.

#### Scenario: Repeated negative feedback is observed
- **WHEN** active feedback repeatedly marks recalled memory, citations, derived insights, or session context as noisy, stale, irrelevant, or needs review
- **THEN** the service can create a quality finding with bounded category, severity, component, subject kind, and scoped evidence links

#### Scenario: Negative feedback is superseded
- **WHEN** negative feedback has been superseded by an authorized correction
- **THEN** the service excludes that superseded feedback from new quality finding generation unless an administrator explicitly inspects historical evidence

#### Scenario: Expected recall is missing
- **WHEN** session feedback or verification records expected evidence that retrieval or context assembly did not recall
- **THEN** the service can create a retrieval or context quality finding without exposing hidden or out-of-scope memory content

#### Scenario: Hidden memory safety feedback is recorded
- **WHEN** feedback indicates suppressed, forgotten, expired, deleted, or out-of-scope memory was visible through a non-admin path
- **THEN** the service records a high-severity quality finding using stable lifecycle safety codes

### Requirement: Feedback-derived repairs remain approval-gated
The service MUST keep feedback-derived repair recommendations under existing admin approval boundaries.

#### Scenario: Feedback finding is actionable
- **WHEN** a feedback-derived finding maps to embedding retry, governance inspection, derived insight replay, suppression review, or manual review
- **THEN** the service may recommend a repair plan while leaving approval and execution to authorized admin actions

#### Scenario: Public feedback requests repair
- **WHEN** a public scoped caller records feedback that implies a repair
- **THEN** the service records feedback and quality evidence without approving or executing repair inline

### Requirement: Task failures can create bounded quality findings
The service SHALL convert repeated task-level memory contribution failures into scoped quality findings or finding summaries using bounded codes.

#### Scenario: Repeated task failures reference missing memory
- **WHEN** active task evaluations repeatedly record failed or partial outcomes with `memory_missing` contribution evidence
- **THEN** the service can create a retrieval or context quality finding with bounded task-success finding code and scoped evidence links

#### Scenario: Repeated task failures reference noisy or stale memory
- **WHEN** active task evaluations repeatedly record failed or partial outcomes with noisy, stale, or irrelevant memory contribution evidence
- **THEN** the service can create a quality finding with bounded category, severity, component, subject kind, and scoped evidence links

#### Scenario: Task evaluation is superseded
- **WHEN** a task evaluation has been superseded or corrected by an authorized record
- **THEN** the service excludes that superseded task evaluation from new quality finding generation unless an administrator explicitly inspects historical evidence

#### Scenario: Task evidence includes hidden memory
- **WHEN** task evidence indicates hidden, suppressed, forgotten, deleted, or out-of-scope memory affected a task outcome
- **THEN** the service records a lifecycle-safe quality finding using stable codes without exposing hidden content through public reports

### Requirement: Task-derived repair recommendations remain approval-gated
The service MUST keep task-derived repair recommendations under existing admin approval boundaries.

#### Scenario: Task-derived finding is actionable
- **WHEN** a task-derived quality finding maps to embedding retry, governance inspection, derived insight replay, suppression review, ranking rollout rollback, or manual review
- **THEN** the service may recommend a repair plan while leaving approval and execution to authorized admin actions

#### Scenario: Task evaluation implies repair
- **WHEN** a public scoped caller records a failed task evaluation that implies a repair
- **THEN** the service records task and quality evidence without approving, executing, suppressing, reranking, or replaying inline

