## ADDED Requirements

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
