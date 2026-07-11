## ADDED Requirements

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
