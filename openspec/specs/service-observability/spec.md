# service-observability Specification

## Purpose
Define the baseline observability requirements for API, worker, and scheduler runtimes, including structured logs and operational signals.
## Requirements
### Requirement: Structured runtime logs
The service SHALL emit structured logs across `api`, `worker`, and `scheduler` modes so operators can correlate runtime events.

#### Scenario: Operator inspects a failing worker action
- **WHEN** a worker job fails or is retried
- **THEN** the emitted logs include enough stable identifiers and runtime context to correlate the failure with the affected job and mode

### Requirement: Operational metrics and tracing hooks
The service MUST expose baseline operational signals for API, governance, retrieval, and lifecycle workloads.

#### Scenario: Operator monitors service behavior over time
- **WHEN** the service handles ingest, governance, retrieval, or forgetting work
- **THEN** the runtime exposes metrics or tracing hook points for latency, throughput, error rate, and backlog-oriented inspection

#### Scenario: Operator monitors embedding lifecycle backlogs
- **WHEN** semantic projection work accumulates because embeddings are missing, stale, failed, or provider-mismatched
- **THEN** the runtime exposes backlog and execution telemetry for embedding rebuild eligibility, attempted generation, promotion outcomes, and provider or model drift processing

### Requirement: Actionable readiness and backlog diagnostics
The service MUST surface operational diagnostics that help explain degraded or unhealthy states.

#### Scenario: Backlog grows while dependencies remain reachable
- **WHEN** workers fall behind or maintenance work accumulates
- **THEN** operators can distinguish backlog pressure from simple dependency failure through the emitted diagnostics

### Requirement: Concrete liveness and readiness endpoints
The service SHALL expose concrete runtime liveness and readiness endpoints for self-hosted deployments.

#### Scenario: Runtime liveness is requested
- **WHEN** a caller requests the liveness endpoint
- **THEN** the service returns a result that confirms the process can respond without checking external provider dependencies

#### Scenario: Runtime readiness is requested
- **WHEN** a caller requests the readiness endpoint
- **THEN** the service evaluates readiness according to the active runtime mode and enabled execution dependencies

### Requirement: Embedding admission and rebuild metrics are exported
The service MUST expose concrete metrics for embedding cutover admission and embedding rebuild execution.

#### Scenario: Cutover admission is evaluated
- **WHEN** a cutover preflight or activation admission evaluation completes
- **THEN** the metrics surface records the decision and any blocker or warning codes without high-cardinality identifiers

#### Scenario: Embedding execution health is observed
- **WHEN** embedding rebuild, provider probe, or cutover wave dispatch activity is observed
- **THEN** the metrics surface records backlog, result, and dispatch signals needed to diagnose rollout progress and execution failure

### Requirement: Insight feedback metrics are exported
The service MUST expose operational metrics for derived insight quality feedback using low-cardinality labels.

#### Scenario: Feedback is recorded
- **WHEN** an insight feedback record is created or superseded
- **THEN** the metrics surface records the operation result by feedback type, insight type, and outcome without embedding tenant, project, namespace, insight id, actor, or reason text as metric labels

#### Scenario: Feedback-driven lifecycle decision occurs
- **WHEN** a background job suppresses, reviews, preserves, or prioritizes an insight based on feedback
- **THEN** the metrics surface records the decision category and result without high-cardinality identifiers

### Requirement: Insight feedback diagnostics are operator-visible
The service SHALL expose diagnostics that help operators understand derived insight quality trends.

#### Scenario: Operator inspects insight quality health
- **WHEN** an operator requests operational diagnostics for derived insights
- **THEN** the service can report feedback coverage, noisy insight rate, needs-review count, and feedback-driven suppression counts for an authorized scope

#### Scenario: Diagnostics include hidden insights
- **WHEN** suppressed or hidden insights contribute to quality diagnostics
- **THEN** the service includes aggregate counts without exposing hidden insight content through public metrics or non-admin diagnostics

### Requirement: Replay execution metrics are exported
The service MUST expose low-cardinality metrics for derived insight replay planning and execution.

#### Scenario: Replay dry-run completes
- **WHEN** a replay dry-run finishes
- **THEN** the metrics surface records the result, mode, insight type category, and decision categories without tenant, project, namespace, replay id, insight id, actor, or reason text labels

#### Scenario: Replay apply completes or fails
- **WHEN** replay apply work creates, updates, suppresses, preserves, skips, or fails insight decisions
- **THEN** the metrics surface records counters by outcome and reason category using low-cardinality labels

### Requirement: Smoke loop diagnostics are operator-visible
The service SHALL expose diagnostics that help operators determine which stage of the self-hosting smoke loop failed.

#### Scenario: Smoke loop detects degraded service
- **WHEN** a smoke check reports failure or degradation
- **THEN** operators can inspect readiness, job backlog, replay status, retrieval/context diagnostics, or metrics to identify the failed stage without direct PostgreSQL access

### Requirement: Quality evaluation and repair metrics are exported
The service MUST expose low-cardinality metrics for quality evaluation, admission pressure, repair execution, and post-repair verification.

#### Scenario: Quality evaluation completes
- **WHEN** an evaluation run completes, fails, or requires manual review
- **THEN** the metrics surface records status, check category, finding category, severity, and component labels without tenant, project, namespace, memory id, event id, repair plan id, actor, or reason text labels

#### Scenario: Admission pressure is evaluated
- **WHEN** ingestion or repair admission pressure is evaluated
- **THEN** the metrics surface records decision, component, and stable finding codes without high-cardinality scope or record identifiers

#### Scenario: Repair action executes
- **WHEN** a repair action completes, fails, retries, is skipped, or requires manual review
- **THEN** the metrics surface records action category, result, and reason category without high-cardinality target identifiers

#### Scenario: Verification completes
- **WHEN** post-repair verification completes
- **THEN** the metrics surface records verification status and residual finding categories without exposing detailed evidence through metric labels

### Requirement: Quality repair diagnostics are operator-visible
The service SHALL expose diagnostics that help operators understand memory quality, ingestion pressure, repair progress, and verification outcomes.

#### Scenario: Operator inspects quality loop health
- **WHEN** an operator requests diagnostics for an authorized scope
- **THEN** the service can report recent evaluation status, dominant finding categories, admission pressure state, repair backlog, repair failures, and verification outcomes

#### Scenario: Diagnostics include hidden lifecycle evidence
- **WHEN** hidden lifecycle states contribute to quality or repair diagnostics
- **THEN** the service includes aggregate counts and stable finding codes without exposing hidden memory content through public metrics or non-admin diagnostics

### Requirement: Proof and session metrics are exported
The service MUST expose low-cardinality metrics for scope proof and memory session execution.

#### Scenario: Proof step completes
- **WHEN** a proof step completes, fails, retries, or is skipped
- **THEN** metrics record step, status, verdict, component, and failure category without tenant, project, namespace, proof id, session id, event id, memory id, actor, or reason labels

#### Scenario: Session verification completes
- **WHEN** session verification completes, degrades, fails, or requires manual review
- **THEN** metrics record verification status and failure category without high-cardinality identifiers

### Requirement: Proof and session diagnostics are operator-visible
The service SHALL expose diagnostics that explain proof and memory-session loop health.

#### Scenario: Operator inspects loop health
- **WHEN** an operator requests diagnostics for an authorized scope
- **THEN** the service can summarize recent proof verdicts, session verdicts, dominant failure categories, pending verification work, and recommended next admin surfaces

#### Scenario: Diagnostics reference hidden evidence
- **WHEN** hidden lifecycle state contributes to a proof or session failure
- **THEN** diagnostics expose aggregate counts and stable reason codes without exposing hidden memory content through public metrics

### Requirement: Usefulness feedback metrics are exported
The service MUST expose low-cardinality metrics for feedback ingestion, feedback summaries, and feedback-derived quality outcomes.

#### Scenario: Feedback is recorded
- **WHEN** usefulness feedback is created, deduplicated, rejected, or superseded
- **THEN** metrics record result, feedback type, subject kind, source surface, and component without tenant, project, namespace, memory id, event id, insight id, session id, turn id, actor, or reason labels

#### Scenario: Feedback summary is updated
- **WHEN** usefulness summaries are aggregated or rebuilt
- **THEN** metrics record summary status, subject kind, effective quality category, and dominant feedback category without high-cardinality identifiers

### Requirement: Feedback diagnostics are operator-visible
The service SHALL expose diagnostics that summarize usefulness feedback health for an authorized scope.

#### Scenario: Operator inspects feedback health
- **WHEN** an operator requests feedback or loop health diagnostics
- **THEN** the service can summarize feedback volume, dominant negative categories, missing expected recall counts, needs-review counts, unsafe feedback counts, and linked quality or repair surfaces

#### Scenario: Diagnostics reference hidden evidence
- **WHEN** hidden lifecycle state contributes to feedback-derived diagnostics
- **THEN** diagnostics expose aggregate counts and stable reason codes without exposing hidden memory content through public metrics or non-admin diagnostics

### Requirement: Task evaluation metrics are exported
The service MUST expose low-cardinality metrics for task evaluation creation, deduplication, rejection, summary aggregation, and task-derived quality outcomes.

#### Scenario: Task evaluation is recorded
- **WHEN** a task evaluation is created, deduplicated, rejected, or superseded
- **THEN** metrics record operation result, verdict category, source surface, and bounded failure category without tenant, project, namespace, task id, session id, memory id, actor, or reason labels

#### Scenario: Task summary is aggregated
- **WHEN** task-success summaries are aggregated or rebuilt
- **THEN** metrics record summary status, subject kind, verdict category, and dominant contribution category without high-cardinality identifiers

### Requirement: Ranking rollout metrics are exported
The service MUST expose low-cardinality metrics for ranking rollout dry-run, activation, impact, rollback, and policy evaluation.

#### Scenario: Rollout dry-run completes
- **WHEN** a ranking rollout dry-run completes
- **THEN** metrics record result, surface, policy mode, evidence threshold status, and bounded impact category without high-cardinality identifiers

#### Scenario: Active policy evaluates request
- **WHEN** search or context assembly evaluates an active ranking rollout policy
- **THEN** metrics record surface, policy status, applied decision, and bounded reason category without tenant, project, namespace, policy id, query text, memory id, actor, or reason labels

#### Scenario: Policy is rolled back
- **WHEN** a ranking rollout policy is paused, disabled, or rolled back
- **THEN** metrics record operation result and policy terminal category without high-cardinality labels

### Requirement: Task and ranking diagnostics are operator-visible
The service SHALL expose diagnostics that help operators understand task-success trends and ranking rollout health for an authorized scope.

#### Scenario: Operator inspects task-success health
- **WHEN** an operator requests task evaluation diagnostics for an authorized scope
- **THEN** the service can summarize recent verdict distribution, dominant memory contribution categories, linked feedback volume, linked quality findings, and recommended next admin surfaces

#### Scenario: Operator inspects ranking rollout health
- **WHEN** an operator requests ranking rollout diagnostics for an authorized scope
- **THEN** the service can summarize active policies, dry-run results, impact counters, rollback history, insufficient-evidence decisions, and bounded reason codes

#### Scenario: Diagnostics reference hidden evidence
- **WHEN** hidden lifecycle state contributes to task or rollout diagnostics
- **THEN** diagnostics expose aggregate counts and stable reason codes without exposing hidden memory content through public metrics or non-admin diagnostics

### Requirement: Task and rollout lifecycle logs are bounded
The service SHALL emit structured logs for task evaluation and ranking rollout lifecycle transitions without high-cardinality fields.

#### Scenario: Task lifecycle transition is logged
- **WHEN** a task evaluation is created, deduplicated, rejected, superseded, summarized, or linked to quality findings
- **THEN** logs include bounded operation, result, verdict, source, and contribution category fields without task id, session id, memory id, actor, or reason text

#### Scenario: Ranking rollout transition is logged
- **WHEN** a rollout policy is created, dry-run, activated, disabled, rolled back, or evaluated for a request
- **THEN** logs include bounded operation, result, surface, policy status, and decision fields without high-cardinality identifiers

### Requirement: Assurance metrics are exported
The service MUST expose low-cardinality metrics for health evaluations, incident lifecycle, alert candidate generation, alert delivery attempts, capacity/load proof, backup/restore proof, readiness reports, cleanup jobs, and recovery verification.

#### Scenario: Health evaluation completes
- **WHEN** a health evaluation completes, degrades, fails, or reports unknown status
- **THEN** metrics record operation, result, status, component, severity, operational proof category, and reason category without tenant, project, namespace, evaluation id, incident id, actor, or reason labels

#### Scenario: Incident lifecycle changes
- **WHEN** an incident is opened, acknowledged, suppressed, resolved, reopened, or verified
- **THEN** metrics record lifecycle operation, result, status, component, severity, and reason category without high-cardinality identifiers

#### Scenario: Alert delivery attempt completes
- **WHEN** an alert delivery attempt succeeds, fails, retries, is skipped, or is disabled
- **THEN** metrics record adapter kind, result, severity, component, and failure category without webhook URL, recipient, scope, incident id, or alert id labels

#### Scenario: Assurance cleanup completes
- **WHEN** an assurance or conformance history cleanup job completes
- **THEN** metrics record record category, result, and bounded deletion category without tenant, project, namespace, record id, or evidence identifiers

### Requirement: Conformance metrics are exported
The service MUST expose low-cardinality metrics for conformance profiles, conformance runs, missing-evidence diagnostics, and readiness summaries.

#### Scenario: Conformance run completes
- **WHEN** a conformance run passes, degrades, fails, or reports unknown status
- **THEN** metrics record result, profile status, evidence category, missing-evidence category, and readiness impact without scope or record identifiers

#### Scenario: Readiness report is generated
- **WHEN** scope readiness is generated or read
- **THEN** metrics record readiness status, runtime category, conformance category, incident category, and recommended-action category without high-cardinality labels

### Requirement: Assurance and conformance lifecycle logs are bounded
The service SHALL emit structured logs for assurance and conformance lifecycle transitions using bounded fields.

#### Scenario: Assurance transition is logged
- **WHEN** a health evaluation, incident transition, alert candidate, alert delivery attempt, or recovery verification changes state
- **THEN** logs include bounded operation, result, component, severity, status, and reason category without tenant, project, namespace, ids, actor, reason text, query text, webhook URL, or recipient

#### Scenario: Conformance transition is logged
- **WHEN** a conformance profile, conformance run, missing-evidence diagnostic, or readiness report changes state
- **THEN** logs include bounded operation, result, evidence category, readiness status, and missing-evidence category without high-cardinality fields

### Requirement: Assurance diagnostics are operator-visible
The service SHALL expose diagnostics that help operators understand operational assurance and integration conformance health for an authorized scope.

#### Scenario: Operator inspects assurance health
- **WHEN** an operator requests assurance diagnostics for an authorized scope
- **THEN** the service can summarize recent evaluation status, capacity/load status, backup/restore status, open incidents, alert candidate state, delivery failures, cleanup status, recovery verification status, and recommended next admin surfaces

#### Scenario: Operator inspects conformance health
- **WHEN** an operator requests conformance diagnostics for an authorized scope
- **THEN** the service can summarize profile coverage, latest run status, dominant missing-evidence categories, stale evidence windows, and readiness impact

#### Scenario: Diagnostics reference hidden evidence
- **WHEN** hidden lifecycle state contributes to assurance or conformance diagnostics
- **THEN** diagnostics expose aggregate counts and stable reason codes without exposing hidden memory content through public metrics or non-admin diagnostics

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

### Requirement: Migration and runtime lifecycle telemetry is bounded and actionable
The service SHALL emit low-cardinality metrics and bounded structured logs for
migration evaluation/execution, startup, readiness transitions, signal receipt,
drain completion, forced drain timeout, and dependency cleanup outcomes.

#### Scenario: Runtime migrates and becomes ready
- **WHEN** a runtime applies or validates migrations and starts successfully
- **THEN** observability records bounded mode, operation, result, migration
  status, and duration categories without database DSNs, credentials, scope
  values, principal identifiers, migration SQL, or raw error payloads

#### Scenario: Runtime drains after termination
- **WHEN** a runtime begins or completes graceful shutdown after a supported
  signal
- **THEN** observability records bounded lifecycle and result categories that
  allow operators to distinguish normal drain, timeout, cleanup failure, and
  startup failure

### Requirement: Product verification and recovery signals are inspectable
The service and repository verification commands SHALL provide bounded,
actionable evidence for product verification, backup creation, restore,
restore verification, and the handoff of successful restore proof to assurance.

#### Scenario: Product verification fails
- **WHEN** a product-verification phase fails
- **THEN** its result identifies the bounded phase and failing subsystem, retains
  or points to safe diagnostic artifacts, and excludes generated credentials,
  unredacted connection strings, and scope identifiers from ordinary logs

#### Scenario: Operator records restore proof
- **WHEN** an operator submits a successful restore-verification result to the
  assurance surface
- **THEN** telemetry records a bounded proof outcome and freshness category
  without treating the backup artifact path, checksum, target database, or scope
  as a metric label

### Requirement: Redacted retrieval evaluation observability
The service SHALL emit low-cardinality, redacted observability for retrieval-evaluation
replay and release-gate decisions.

#### Scenario: Retrieval evaluation run completes
- **WHEN** a fixture replay completes
- **THEN** telemetry records bounded operation status, fixture version, ranking version,
  policy version, case count, aggregate safety outcome, and duration without recording
  tenant, project, namespace, memory IDs, source content, credentials, or DSNs

#### Scenario: Retrieval evaluation run fails
- **WHEN** a replay fails due to fixture validation, isolation, lifecycle visibility, or
  threshold regression
- **THEN** telemetry records a stable bounded failure category and does not emit raw
  query text, database errors, hidden evidence, or foreign scope details

#### Scenario: Release policy decision is made
- **WHEN** a candidate retrieval report is accepted or rejected against a baseline
- **THEN** telemetry records the policy version and bounded decision category so
  operators can audit rollout evidence without reconstructing sensitive fixture data

