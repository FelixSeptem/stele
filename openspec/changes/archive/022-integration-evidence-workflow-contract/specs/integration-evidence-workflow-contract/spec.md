## ADDED Requirements

### Requirement: Workflow templates define bounded evidence contracts
The service SHALL allow authorized administrators to define scoped integration workflow templates using bounded step kinds, evidence kinds, completion policy, freshness windows, and runbook hint categories.

#### Scenario: Administrator creates workflow template
- **WHEN** an authorized administrator creates a workflow template for a tenant, project, and namespace
- **THEN** the service persists the template with bounded required, optional, and repeatable steps, allowed evidence kinds, actor, reason, status, and creation time

#### Scenario: Template uses unsupported step or evidence kind
- **WHEN** a workflow template references an unsupported free-form step kind or evidence kind
- **THEN** the service rejects the template before it can create unbounded workflow behavior

#### Scenario: Template targets unauthorized scope
- **WHEN** an administrator attempts to create, update, disable, read, or list workflow templates outside an authorized scope
- **THEN** the service rejects the request without exposing template existence or evidence expectations from that scope

### Requirement: Workflow runs track one external integration attempt
The service SHALL persist scoped workflow runs for one external agent turn, task, job, or integration attempt without executing the external integration.

#### Scenario: External integration starts workflow run
- **WHEN** an authorized scoped caller starts a workflow run from an active template
- **THEN** the service records run status, template reference, bounded integration kind, idempotency key, actor, reason, and initial next actions for the same tenant, project, and namespace

#### Scenario: Workflow run is idempotently started
- **WHEN** the same scoped caller repeats workflow run creation with the same idempotency key
- **THEN** the service returns or resumes the original run instead of creating duplicate active runs

#### Scenario: Workflow run cannot execute agent logic
- **WHEN** a workflow run is created, advanced, inspected, or diagnosed
- **THEN** Stele does not invoke models, build prompts, execute tools, run the external agent, or generate final answers

### Requirement: Workflow steps record append-only progress
The service SHALL record workflow step transitions using bounded step kinds, statuses, result categories, and timestamps.

#### Scenario: Integration records step progress
- **WHEN** an authorized scoped caller records a workflow step for a run
- **THEN** the service appends a step record with bounded status, result category, observed time, actor, reason, and optional evidence links

#### Scenario: Step is out of order
- **WHEN** a caller records a step before required predecessor steps have enough valid evidence
- **THEN** the service records or returns a bounded `out_of_order` diagnostic according to the template completion policy

#### Scenario: Step record targets unauthorized scope
- **WHEN** a caller records a step for a workflow run outside the caller's authorized scope
- **THEN** the service rejects the request without exposing run existence or prior step state

### Requirement: Evidence links normalize references to existing Stele records
The service SHALL attach workflow evidence through normalized links to existing scoped Stele records or bounded opaque caller tokens.

#### Scenario: Internal evidence link is recorded
- **WHEN** a workflow step references a memory session, session turn, context evidence, outcome event, verification, usefulness feedback, task evaluation, proof run, repair plan, ranking rollout, conformance run, readiness report, incident, or recovery verification in the same scope
- **THEN** the service stores a normalized evidence link with bounded evidence kind and source surface without copying the source record payload

#### Scenario: Evidence link is out of scope
- **WHEN** a workflow step references evidence outside the workflow run scope
- **THEN** the service rejects the link or records a bounded `out_of_scope` diagnostic without exposing the target record

#### Scenario: Opaque evidence cannot satisfy internal requirement
- **WHEN** a template step requires service-verifiable internal evidence and the workflow step supplies only opaque caller evidence
- **THEN** the service preserves the opaque token for audit and records an `opaque_only` gap diagnostic for the required evidence

### Requirement: Workflow gap diagnostics are bounded and actionable
The service SHALL detect missing, stale, out-of-order, duplicated, hidden, opaque-only, contradictory, invalid, subject-missing, insufficient-evidence, and out-of-scope workflow evidence using stable diagnostic categories.

#### Scenario: Required step is missing
- **WHEN** a workflow run lacks a required step or required evidence after the configured freshness or completion window
- **THEN** the service records a gap diagnostic with step kind, evidence kind, gap category, readiness impact, and recommended next-action category

#### Scenario: Linked evidence becomes hidden
- **WHEN** linked evidence is suppressed, forgotten, deleted, hidden, or otherwise not visible for ordinary retrieval
- **THEN** the workflow report exposes only lifecycle-safe aggregate diagnostics and stable categories outside authorized admin detail

#### Scenario: Diagnostic scan reaches configured bounds
- **WHEN** workflow gap materialization reaches configured evidence, diagnostic, or time bounds
- **THEN** the service records bounded degraded or continuation-required status instead of scanning beyond limits

### Requirement: Workflow next actions guide external integrations
The service SHALL provide bounded next actions that identify missing workflow steps and the Stele API or admin surface that can provide the next evidence.

#### Scenario: Integration asks for next action
- **WHEN** an authorized scoped caller reads next actions for an incomplete workflow run
- **THEN** the service returns bounded action categories, step kinds, evidence kinds, route categories, and completion hints without raw prompts, model output, hidden memory content, or out-of-scope identifiers

#### Scenario: Workflow run is complete
- **WHEN** all required template steps are satisfied with valid evidence
- **THEN** the next-action report records completion with no required action and a bounded summary of satisfied evidence categories

#### Scenario: Workflow cannot progress
- **WHEN** a workflow run has invalid, stale, contradictory, subject-missing, insufficient, or out-of-scope evidence that blocks completion
- **THEN** the next-action report recommends existing public or admin surfaces for recording, replacing, or reviewing evidence without mutating source records inline

### Requirement: Workflow history retention is bounded
The service SHALL apply configurable retention and cleanup to high-volume workflow runs, step records, evidence links, gap diagnostics, and next-action history while preserving template definitions and required audit transitions.

#### Scenario: Workflow history exceeds retention
- **WHEN** completed, abandoned, or expired workflow history exceeds configured retention windows
- **THEN** the service can clean up eligible high-volume records without deleting active templates or required audit transitions

#### Scenario: Cleanup is retried
- **WHEN** workflow cleanup is retried after partial execution or restart
- **THEN** cleanup remains idempotent and preserves tenant, project, and namespace isolation
