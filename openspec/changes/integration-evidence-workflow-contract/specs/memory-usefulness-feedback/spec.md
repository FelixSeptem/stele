## ADDED Requirements

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
