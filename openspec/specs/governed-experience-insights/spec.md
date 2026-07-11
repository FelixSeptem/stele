# governed-experience-insights Specification

## Purpose
TBD - created by archiving change governed-experience-insights. Update Purpose after archive.
## Requirements
### Requirement: Derived insights are governed records
The service SHALL persist derived experience insights as governed records with explicit scope, insight type, lifecycle state, confidence, derivation metadata, evidence citations, and audit history.

#### Scenario: Derived insight is stored with governance metadata
- **WHEN** a derived insight is created or updated
- **THEN** the service stores its tenant, project, namespace, type, lifecycle state, confidence, derivation source, evidence references, and observed or derived timestamps

#### Scenario: Derived insight does not overwrite canonical memory
- **WHEN** the service derives a new insight from canonical memory, job history, recovery history, or embedding failure state
- **THEN** the service records the insight separately without mutating canonical memories, memory versions, vector revisions, or existing provenance in place

### Requirement: Failure patterns require repeated evidence
The service MUST create active `failure_pattern` insights only from repeated evidence within an authorized scope.

#### Scenario: Repeated failure evidence is detected
- **WHEN** the derivation job observes repeated failure evidence with a stable normalized pattern key inside one scope
- **THEN** the service creates or updates a `failure_pattern` insight with linked evidence and confidence metadata

#### Scenario: Isolated failure evidence is insufficient
- **WHEN** only one ordinary failure record matches a potential pattern
- **THEN** the service does not activate a `failure_pattern` insight from that evidence alone

### Requirement: Lessons are evidence-backed outputs
The service SHALL allow `lesson` outputs only when they are backed by a derived failure pattern and cited evidence.

#### Scenario: Lesson is attached to a failure pattern
- **WHEN** the service creates an experience lesson
- **THEN** the lesson references a source `failure_pattern` insight and evidence citations sufficient to explain the lesson

#### Scenario: Ungrounded lesson is rejected
- **WHEN** a lesson has no source failure pattern or evidence references
- **THEN** the service rejects or suppresses the lesson instead of surfacing it as active guidance

### Requirement: Insight lifecycle preserves auditability
The service MUST preserve lifecycle transitions for derived insights without deleting evidence history.

#### Scenario: Insight is suppressed
- **WHEN** an operator or derivation rule suppresses a derived insight
- **THEN** the service records the transition with actor or rule attribution and excludes the insight from default context assembly

#### Scenario: Insight evidence changes
- **WHEN** new evidence changes a derived insight's confidence or lesson text
- **THEN** the service appends or records an auditable update rather than silently replacing the prior state

### Requirement: Reserved insight vocabulary is non-active
The service SHALL reserve `hypothesis`, `goal`, `contradiction`, and `causal_link` vocabulary without performing autonomous inference for those types in this change.

#### Scenario: Unsupported insight type is requested for derivation
- **WHEN** the derivation job encounters a request to autonomously infer `hypothesis`, `goal`, `contradiction`, or `causal_link`
- **THEN** the service skips that derivation path and records no active insight of those types

#### Scenario: Future insight type is represented in schema
- **WHEN** a future change adds support for another insight type
- **THEN** the existing derived insight substrate can preserve scope, lifecycle, confidence, evidence, and audit semantics for that type

### Requirement: Derived insight governance accounts for quality feedback
The service SHALL use governed quality feedback as an input to derived insight lifecycle, confidence, and derivation decisions without rewriting insight evidence in place.

#### Scenario: Insight has strong negative feedback
- **WHEN** insight derivation evaluates an existing insight with active `noisy`, `incorrect`, or `stale` feedback
- **THEN** the service can lower the insight's effective quality, avoid reactivating it automatically, or create an auditable lifecycle transition according to policy without deleting evidence

#### Scenario: Insight has positive feedback
- **WHEN** insight derivation evaluates an existing insight with active `useful` feedback
- **THEN** the service can preserve or prioritize that insight as long as the evidence and scope remain valid

#### Scenario: Conflicting feedback exists
- **WHEN** an insight has both positive and negative active feedback signals
- **THEN** the service preserves the conflict in effective quality state rather than silently discarding either signal

### Requirement: Feedback-driven lifecycle changes remain auditable
The service MUST record any feedback-driven derived insight lifecycle transition with attribution to the policy or job that consumed the feedback.

#### Scenario: Feedback causes insight suppression
- **WHEN** a derivation or maintenance policy suppresses an insight because of effective feedback state
- **THEN** the service records a lifecycle transition that identifies the feedback-driven reason and preserves the feedback and evidence history

#### Scenario: Feedback does not justify lifecycle mutation
- **WHEN** feedback affects ranking or review status but does not meet the policy threshold for lifecycle mutation
- **THEN** the service leaves the insight lifecycle unchanged and exposes the quality state through inspection or ranking surfaces

### Requirement: Derived insight replay obeys insight governance
The service MUST apply the same derived insight governance rules during replay as during scheduled derivation, including evidence thresholds, lifecycle transitions, confidence updates, feedback policy, and audit history.

#### Scenario: Replay finds repeated failure evidence
- **WHEN** replay evaluates historical evidence that satisfies the governed `failure_pattern` threshold within one authorized scope
- **THEN** the service can create or update the corresponding derived insight through the same evidence-backed governance model as scheduled derivation

#### Scenario: Replay finds insufficient evidence
- **WHEN** replay evaluates evidence that does not satisfy the governed threshold for an active insight
- **THEN** the service records a skipped, preserved, or suppressed decision according to policy rather than creating an unsupported active insight

#### Scenario: Replay consumes feedback state
- **WHEN** replay evaluates an insight with active quality feedback
- **THEN** the replay decision accounts for effective quality state without deleting feedback records or rewriting prior evidence history

### Requirement: Replay does not activate reserved insight vocabulary
The service SHALL NOT use replay to autonomously activate reserved `hypothesis`, `goal`, `contradiction`, or `causal_link` insight types.

#### Scenario: Replay request includes unsupported type
- **WHEN** a replay request includes an insight type that is reserved but not supported for active derivation
- **THEN** the service rejects or skips that type and records the unsupported-type reason in the replay response or report

