## ADDED Requirements

### Requirement: Integration workflow metrics are exported
The service MUST expose low-cardinality metrics for workflow template lifecycle, workflow run lifecycle, step recording, evidence link recording, gap diagnostics, next-action generation, cleanup jobs, and conformance/readiness impact.

#### Scenario: Workflow run changes state
- **WHEN** a workflow run is created, advances, completes, expires, blocks, or is abandoned
- **THEN** metrics record operation, result, run status, template status, integration kind, and completion category without tenant, project, namespace, template id, run id, evidence id, actor, reason, query text, prompt text, or model output labels

#### Scenario: Workflow gap is recorded
- **WHEN** a workflow diagnostic is created, resolved, superseded, or retained
- **THEN** metrics record step kind, evidence kind, gap category, readiness impact, and result without high-cardinality identifiers

#### Scenario: Workflow cleanup completes
- **WHEN** workflow history cleanup completes
- **THEN** metrics record record category, result, and bounded deletion category without scope, record id, or evidence identifiers

### Requirement: Integration workflow lifecycle logs are bounded
The service SHALL emit structured lifecycle logs for workflow templates, runs, steps, evidence links, diagnostics, next actions, and cleanup using bounded fields only.

#### Scenario: Workflow transition is logged
- **WHEN** a workflow template, run, step, evidence link, diagnostic, next action, or cleanup job changes state
- **THEN** logs include bounded operation, result, step kind, evidence kind, run status, diagnostic category, and next-action category without tenant, project, namespace, ids, actor, reason text, query text, prompt text, model output, webhook URL, or recipient

#### Scenario: Workflow references hidden evidence
- **WHEN** hidden, suppressed, forgotten, deleted, or out-of-scope evidence contributes to workflow diagnostics
- **THEN** logs and public metrics expose only aggregate counts and stable categories
