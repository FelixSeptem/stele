## 1. Assurance And Conformance Data Model

- [x] 1.1 Add PostgreSQL migrations for health evaluations, health component summaries, incidents, incident transitions, alert candidates, alert delivery attempts, conformance profiles, conformance runs, missing-evidence diagnostics, capacity/load proof records, backup/restore proof records, readiness reports, recovery verification reports, and assurance/conformance retention metadata.
- [x] 1.2 Add Go domain types for readiness status, health status, severity, health component, incident status, incident action, alert adapter kind, alert delivery result, conformance profile status, expected evidence kind, missing-evidence category, conformance result, operational proof target, recovery verification target, retention class, and runbook hint categories.
- [x] 1.3 Implement validation for scope, bounded enums, freshness windows, evidence minimums, delivery adapter configuration, webhook URL/network/header safety, alert payload bounds, capacity/load proof bounds, backup/restore proof bounds, actor/reason attribution, metadata bounds, and idempotency keys.
- [x] 1.4 Add repository methods for creating, reading, listing, acknowledging, suppressing, resolving, and linking health evaluations, incidents, alert candidates, conformance profiles/runs, readiness reports, and recovery verification records with tenant/project/namespace filters.
- [x] 1.5 Add repository tests for scope isolation, append-only incident transitions, alert delivery attempt history, conformance profile validation, missing-evidence diagnostics, capacity/load proof reads, backup/restore proof reads, readiness report reads, recovery verification links, retention eligibility, and high-cardinality evidence preservation.

## 2. Assurance Evaluation And Incident Loop

- [x] 2.1 Implement health evaluation aggregation from runtime readiness, backlog state, embedding health, proof/session verdicts, usefulness feedback, task evaluation summaries, repair status, ranking rollout state, conformance status, capacity/load proof, and backup/restore proof without mutating source records.
- [x] 2.2 Implement incident creation and deduplication from degraded or unhealthy evaluation findings using bounded severity, component, reason category, observed window, runbook hints, and linked evaluation references.
- [x] 2.3 Implement incident lifecycle actions for acknowledge, suppress, resolve, reopen, and verify with actor/reason attribution and durable transition history.
- [x] 2.4 Implement alert candidate generation from incidents and critical evaluation findings with deduplication windows, delivery policy, bounded payload shape, and recommended admin surfaces.
- [x] 2.5 Implement alert delivery attempts for `disabled`, `stdout`, and generic `webhook` adapters with retries, redaction, failure categories, HTTPS-by-default target validation, unsafe destination blocking, bounded timeout/payload behavior, and no vendor-specific alert platform dependency.
- [x] 2.6 Add tests proving health evaluation is scoped, incidents preserve source evidence, capacity/load and backup/restore failures degrade readiness, alert candidates dedupe, delivery attempts are retry-safe, unsafe webhooks are rejected, webhook secrets are redacted, and source memory/task/feedback/ranking/repair records are not mutated.

## 3. Integration Conformance And Readiness Reports

- [x] 3.1 Implement conformance profile creation, update, disable, read, and list for expected evidence chains covering session, context, outcome, verification, usefulness feedback, task evaluation, proof, repair, and optional ranking rollout evidence.
- [x] 3.2 Implement conformance runs that inspect durable Stele evidence for missing, stale, contradictory, opaque-only, hidden, or out-of-scope evidence without executing external agents.
- [x] 3.3 Implement bounded missing-evidence diagnostics such as `session_without_outcome`, `turn_without_context`, `verification_missing`, `feedback_without_subject`, `task_evaluation_missing_evidence`, `repair_without_verification`, and `rollout_without_dry_run`.
- [x] 3.4 Implement scope readiness reports that combine latest health evaluation, conformance run, proof/session outcomes, repair health, ranking rollout health, capacity/load status, backup/restore status, incident counters, alert counters, and recommended actions.
- [x] 3.5 Implement recovery verification that links incidents, conformance failures, repair verification, ranking rollback, proof rerun, session verification, capacity/load proof, or backup/restore proof to a durable recovery result.
- [x] 3.6 Add tests proving conformance profiles reject unbounded evidence kinds, conformance runs detect missing integration evidence, readiness is degraded by conformance failure, capacity/load failure, or backup/restore proof failure, readiness is unknown without recent evidence, and recovery verification preserves history.

## 4. Admin API, OpenAPI, And Worker/Scheduler Integration

- [x] 4.1 Add admin HTTP endpoints for health evaluation create/list/detail, incident list/detail/actions, alert candidate list/detail/delivery attempts, conformance profile CRUD, conformance run create/list/detail, readiness report read, and recovery verification create/read.
- [x] 4.2 Add OpenAPI schemas and tests for assurance, incident, alert, conformance, readiness, and recovery verification endpoints with scoped auth and admin-only boundaries.
- [x] 4.3 Wire assurance/conformance services into API runtime dependencies without changing public read/write routes.
- [x] 4.4 Add scheduler dispatch for periodic health evaluations, incident refresh, alert candidate generation, conformance runs, stale integration checks, capacity/load proof checks, backup/restore proof freshness checks, assurance/conformance history cleanup, and optional recovery verification.
- [x] 4.5 Add worker execution paths for alert delivery and bounded conformance, cleanup, or recovery jobs using durable lease, retry, and idempotency patterns.
- [x] 4.6 Add API and worker tests for admin authorization, out-of-scope denial, scheduler cadence idempotency, worker retry behavior, delivery deduplication, cleanup idempotency, and lease-safe execution.

## 5. Observability, Documentation, And Runtime Configuration

- [x] 5.1 Add runtime configuration for assurance cadence, conformance cadence, incident freshness windows, assurance/conformance retention windows, capacity/load proof thresholds, backup/restore proof freshness, alert delivery mode, webhook URL/headers, retry limits, delivery timeout, payload size limit, HTTPS/local override behavior, and disabled-by-default safety behavior.
- [x] 5.2 Add low-cardinality metrics for health evaluation, incident lifecycle, alert candidate generation, alert delivery attempts, conformance runs, missing-evidence diagnostics, capacity/load proof, backup/restore proof, readiness reports, cleanup jobs, and recovery verification.
- [x] 5.3 Add structured logs for assurance and conformance lifecycle transitions without tenant, project, namespace, record ids, actor, reason text, query text, webhook URL, or recipient fields.
- [x] 5.4 Update self-hosting docs with the production-readiness workflow: run health evaluation, define conformance profile, run conformance, inspect capacity/load and backup/restore proof status, inspect readiness, review incidents/alerts, remediate through runbook hints, and verify recovery.
- [x] 5.5 Update docs to state remaining product gaps after this proposal: SDK/UI collection surfaces, external agent runtime implementation, vendor-specific alert integrations, hosted incident management, and adaptive scoring calibration.
- [x] 5.6 Add tests proving metrics/logs are low-cardinality, webhook secrets are not exported, docs reference the new routes and boundaries, and configuration validation rejects unsafe or incomplete alert settings.

## 6. Verification

- [x] 6.1 Run targeted tests for assurance domain validation, repositories, services, HTTP handlers, worker/scheduler dispatch, telemetry, OpenAPI, and self-hosting docs.
- [x] 6.2 Run `go test ./... -count=1`.
- [x] 6.3 Run `openspec validate self-hosted-assurance-and-conformance-loop --strict`.
