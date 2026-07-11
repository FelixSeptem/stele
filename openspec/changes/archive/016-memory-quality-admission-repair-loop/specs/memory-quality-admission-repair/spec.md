## ADDED Requirements

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
