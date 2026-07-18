## 1. Workflow Data Model And Validation

- [ ] 1.1 Add PostgreSQL migrations for workflow templates, template steps, workflow runs, step records, evidence links, gap diagnostics, next actions, transition history, and workflow retention metadata.
- [ ] 1.2 Add Go domain types for workflow template status, run status, step kind, step status, evidence kind, evidence link status, diagnostic category, next-action category, integration kind, completion policy, readiness impact, and retention class, including subject-missing and insufficient-evidence diagnostic categories.
- [ ] 1.3 Implement validation for scope, bounded enums, required/optional/repeatable step policy, freshness windows, completion windows, evidence minimums, idempotency keys, actor/reason attribution, metadata bounds, opaque evidence tokens, and internal evidence link shape.
- [ ] 1.4 Add repository methods for creating, updating, disabling, reading, and listing templates; starting, reading, listing, and transitioning runs; recording steps; linking evidence; recording diagnostics; recording next actions; superseding evidence links; and finding retention-eligible workflow history with tenant/project/namespace filters.
- [ ] 1.5 Add repository tests for scope isolation, idempotent run creation, append-only transitions, step ordering state preservation, evidence-link scope validation, opaque evidence preservation, diagnostic reads, next-action reads, superseded link history, retention eligibility, and high-cardinality evidence preservation.

## 2. Workflow Service And Diagnostics

- [ ] 2.1 Implement workflow template management service methods for create, update, disable, read, and list with bounded step/evidence validation.
- [ ] 2.2 Implement workflow run creation and idempotent resume from active templates without executing external agents or mutating source records.
- [ ] 2.3 Implement step recording with append-only step records, normalized evidence links, duplicate detection, predecessor checks, and bounded out-of-order diagnostics.
- [ ] 2.4 Implement evidence-link validation against existing Stele evidence surfaces for memory sessions, turns, context evidence, outcome events, verification, usefulness feedback, task evaluations, scope proofs, repair plans, ranking rollouts, conformance runs, readiness reports, incidents, and recovery verification.
- [ ] 2.5 Implement gap diagnostic materialization for missing, stale, out-of-order, duplicate, hidden, opaque-only, contradictory, invalid, subject-missing, insufficient-evidence, and out-of-scope evidence with configured scan bounds.
- [ ] 2.6 Implement next-action generation that maps incomplete workflow steps to bounded public or admin route categories without exposing prompts, model output, hidden memory content, or out-of-scope identifiers.
- [ ] 2.7 Add service tests proving templates reject unbounded kinds, runs are idempotent, step evidence is scoped, opaque-only evidence cannot satisfy internal requirements, out-of-order steps are diagnosed, hidden evidence is reported safely, next actions are actionable, and source memory/session/feedback/task/proof/assurance records are not mutated.

## 3. HTTP And OpenAPI Surfaces

- [ ] 3.1 Add public scoped HTTP endpoints for workflow run creation, workflow run read, step evidence recording, and next-action read.
- [ ] 3.2 Add admin HTTP endpoints for template create/update/disable/list/read, workflow run list/read, step/evidence/diagnostic/next-action inspection, and evidence link supersession.
- [ ] 3.3 Add OpenAPI paths, schemas, bounded enum definitions, auth requirements, and error responses for workflow templates, runs, steps, evidence links, diagnostics, next actions, and supersession.
- [ ] 3.4 Add HTTP tests for public auth, admin auth, missing scope rejection, out-of-scope denial, invalid step/evidence rejection, idempotency, next-action shape, admin-only template management, and redaction of hidden or sensitive workflow fields.

## 4. Conformance, Assurance, And Existing Evidence Integration

- [ ] 4.1 Extend conformance inspection to include workflow templates, workflow runs, step completion, evidence link validity, and workflow gap diagnostics when profiles require workflow evidence.
- [ ] 4.2 Extend readiness report aggregation to include recent workflow completion health, stale workflow counters, blocking gap diagnostics, and bounded recommended actions.
- [ ] 4.3 Extend health evaluation and incident generation to treat severe or repeated workflow gaps as integration-readiness findings with bounded component, severity, reason category, and runbook hints.
- [ ] 4.4 Extend alert candidate generation and recovery verification to reference workflow-related incidents, conformance failures, workflow runs, gap diagnostics, and remediation evidence without resolving incidents automatically.
- [ ] 4.5 Add integration tests proving workflow gaps degrade conformance/readiness, workflow recovery can be linked durably, alert candidates dedupe for workflow incidents, and out-of-scope workflow evidence never leaks.

## 5. Scheduler, Worker, Retention, And Runtime Configuration

- [ ] 5.1 Add runtime configuration for workflow diagnostic cadence, stale run windows, completion windows, diagnostic scan bounds, next-action refresh bounds, workflow retention windows, and disabled-by-default safety behavior where applicable.
- [ ] 5.2 Add scheduler dispatch for stale workflow detection, gap materialization, next-action refresh, and workflow retention cleanup using existing maintenance scope discovery.
- [ ] 5.3 Add worker execution paths for workflow diagnostic materialization, next-action refresh, and cleanup with durable lease, retry, idempotency, and bounded failure summaries.
- [ ] 5.4 Add scheduler/worker tests for cadence idempotency, duplicate dispatch handling, stale run detection, diagnostic retry behavior, cleanup idempotency, lease-safe execution, and scope isolation.

## 6. Observability, Documentation, And Verification

- [ ] 6.1 Add low-cardinality metrics for workflow template lifecycle, run lifecycle, step recording, evidence link recording, gap diagnostics, next-action generation, cleanup, conformance impact, and readiness impact.
- [ ] 6.2 Add structured lifecycle logs for workflow transitions without tenant, project, namespace, template id, run id, evidence id, actor, reason text, query text, prompt text, model output, webhook URL, or recipient fields.
- [ ] 6.3 Update self-hosting docs with the golden integration workflow path and state that SDK/UI, external agent execution, model invocation, prompt orchestration, tool orchestration, and final-answer generation remain outside Stele's service boundary.
- [ ] 6.4 Add tests proving workflow metrics/logs are low-cardinality, sensitive fields are not exported, docs reference workflow routes and service boundaries, and configuration validation rejects unsafe or unbounded workflow settings.
- [ ] 6.5 Run targeted tests for workflow domain validation, repositories, services, HTTP handlers, conformance/readiness integration, worker/scheduler dispatch, telemetry, OpenAPI, and self-hosting docs.
- [ ] 6.6 Run `go test ./... -count=1`.
- [ ] 6.7 Run `openspec validate integration-evidence-workflow-contract --strict`.
