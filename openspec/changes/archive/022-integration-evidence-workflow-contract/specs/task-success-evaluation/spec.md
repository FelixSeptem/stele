## ADDED Requirements

### Requirement: Task evaluations can attach to workflow runs
The service SHALL allow workflow steps to reference task-success evaluations as scoped evidence for external-agent integration completeness.

#### Scenario: Workflow step links task evaluation
- **WHEN** a workflow step records task evaluation evidence in the same tenant, project, and namespace
- **THEN** the service links the task evaluation to the workflow step without mutating or superseding the task evaluation record

#### Scenario: Task evaluation has insufficient evidence
- **WHEN** a workflow template requires service-verifiable task evidence and the linked task evaluation lacks required scoped evidence links
- **THEN** the workflow diagnostic records a bounded `task_evaluation_missing_evidence` or `opaque_only` category and recommends the task evaluation surface

#### Scenario: Task evaluation evidence is out of scope
- **WHEN** a workflow step references task evaluation evidence outside the workflow run scope
- **THEN** the service excludes that evidence from workflow completion and does not expose target existence
