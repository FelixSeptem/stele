# memory-usefulness-feedback Specification

## Purpose
TBD - created by archiving change memory-usefulness-feedback-and-agent-session-contract. Update Purpose after archive.
## Requirements
### Requirement: Usefulness feedback is durable and scoped
The service SHALL allow authorized scoped callers to record durable usefulness feedback for memory use without mutating canonical memory directly.

#### Scenario: Caller records feedback for recalled memory
- **WHEN** a caller records feedback for a memory hit, raw event, context citation, derived insight, session, session turn, verification, or expected-recall miss within an authorized scope
- **THEN** the service persists the feedback with tenant, project, namespace, subject references, feedback type, actor or source attribution, reason, idempotency key when provided, and creation time

#### Scenario: Feedback targets unauthorized scope
- **WHEN** a caller records feedback outside the authorized tenant, project, or namespace
- **THEN** the service rejects the request without exposing target feedback, memory, insight, session, or event content

#### Scenario: Feedback attempts direct memory mutation
- **WHEN** feedback marks a memory as noisy, stale, irrelevant, missing, unsafe, or needs review
- **THEN** the service records feedback evidence without rewriting canonical memory content, memory versions, vector revisions, derived insight evidence, or lifecycle state

### Requirement: Feedback taxonomy is bounded
The service MUST classify usefulness feedback using stable bounded categories suitable for API responses, diagnostics, quality findings, and metrics.

#### Scenario: Feedback type is valid
- **WHEN** feedback is recorded with a supported type such as `useful`, `irrelevant`, `noisy`, `stale`, `missing_expected`, `unsafe_or_hidden`, or `needs_review`
- **THEN** the service accepts the feedback and records the bounded category

#### Scenario: Feedback type is unsupported
- **WHEN** feedback is recorded with a free-form or unsupported type
- **THEN** the service rejects the request rather than creating an unbounded category

### Requirement: Usefulness summaries are derived from feedback evidence
The service SHALL derive usefulness summaries from durable feedback records while preserving the feedback log as the source of truth.

#### Scenario: Summary is computed for a feedback subject
- **WHEN** feedback exists for a memory, raw event, citation, derived insight, session, session turn, verification, or expected-recall target
- **THEN** the service can expose aggregate counts, dominant feedback categories, effective quality state, and last feedback time for that subject

#### Scenario: Summary is rebuilt
- **WHEN** feedback aggregation is rerun or repaired
- **THEN** the service recomputes summaries from durable feedback records rather than relying on non-auditable counters

#### Scenario: Superseded feedback exists
- **WHEN** a feedback record has been superseded by an authorized correction
- **THEN** default usefulness summaries exclude the superseded record while admin inspection can still expose the original and superseding feedback history

### Requirement: Feedback correction preserves audit history
The service SHALL support append-only correction or supersession of feedback without deleting the original feedback record.

#### Scenario: Caller supersedes prior feedback
- **WHEN** an authorized caller or administrator supersedes a feedback record with actor and reason attribution
- **THEN** the service records the supersession as a new audit event and excludes the superseded feedback from active summaries

#### Scenario: Supersession targets unauthorized feedback
- **WHEN** a caller attempts to supersede feedback outside the authorized scope
- **THEN** the service rejects the request without exposing feedback existence or subject content

### Requirement: Expected recall feedback uses typed targets
The service SHALL represent expected recall feedback with bounded target kinds rather than unstructured subject semantics.

#### Scenario: Expected recall references known evidence
- **WHEN** feedback marks expected recall as missing for a known event, memory, citation, insight, session, session turn, or verification target
- **THEN** the service records the target kind and identifier as scoped feedback evidence

#### Scenario: Expected recall is opaque
- **WHEN** feedback marks expected recall as missing for a caller-provided opaque token
- **THEN** the service records the token as an opaque expected-recall target and does not treat it as an event, memory, citation, insight, session, turn, or verification identifier

### Requirement: Feedback reports preserve detailed evidence without metric leakage
The service SHALL store high-cardinality feedback evidence only in scoped durable records and authorized reports.

#### Scenario: Feedback references concrete evidence
- **WHEN** feedback references memory ids, event ids, citation ids, insight ids, session ids, turn ids, verification ids, actor, or reason text
- **THEN** those identifiers are preserved in scoped feedback evidence and are not exported as metric labels

#### Scenario: Public diagnostics include feedback effects
- **WHEN** feedback contributes to a public or scoped diagnostic result
- **THEN** diagnostics expose bounded reason codes and aggregate counts without exposing hidden memory content or out-of-scope evidence

### Requirement: Feedback summary visibility is bounded
The service MUST keep cross-subject feedback summaries under admin inspection while allowing scoped callers to see summaries for their own session reports.

#### Scenario: Caller reads own session report
- **WHEN** a scoped caller reads a session report for an authorized session that contains feedback
- **THEN** the service can include bounded feedback summaries for that session, turn, context, outcome, and expected-recall evidence

#### Scenario: Caller requests cross-subject summary
- **WHEN** a non-admin caller requests aggregate feedback summaries outside an authorized session report
- **THEN** the service rejects the request or omits the summary rather than exposing cross-subject usefulness data

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

### Requirement: Usefulness feedback can attach to workflow runs
The service SHALL allow workflow steps to reference usefulness feedback as scoped evidence for external-agent integration completeness.

#### Scenario: Workflow step links feedback
- **WHEN** a workflow step records usefulness feedback evidence in the same tenant, project, and namespace
- **THEN** the service links the feedback to the workflow step without mutating or superseding the feedback record

#### Scenario: Feedback evidence lacks subject
- **WHEN** a workflow template requires subject-linked feedback and the supplied feedback has no valid subject link
- **THEN** the workflow diagnostic records a bounded `missing_subject` or `invalid_evidence` category and recommends the feedback recording surface

#### Scenario: Feedback evidence is out of scope
- **WHEN** a workflow step references usefulness feedback outside the workflow run scope
- **THEN** the service rejects the link or records a bounded out-of-scope diagnostic without exposing the feedback record

