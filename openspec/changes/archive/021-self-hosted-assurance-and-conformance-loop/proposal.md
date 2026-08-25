## Why

Stele now has service-side loops for ingestion, governance, retrieval, context assembly, scope proof, memory sessions, usefulness feedback, task evaluation, repair, and governed ranking rollout, but a self-hosted operator still lacks one durable answer to whether a scope is product-ready. The next product-completeness gap is to prove both runtime health and external-agent integration conformance without moving SDK, UI, model execution, or agent runtime logic into this repository.

## What Changes

- Add a self-hosted assurance and conformance capability that evaluates scope readiness from runtime signals, backlog state, proof/session outcomes, repair status, ranking rollout state, and integration evidence completeness.
- Add durable operational health evaluations, incident records, alert candidates, runbook hints, recovery verification reports, and scope readiness reports, including bounded capacity/load proof and backup/restore proof as service-owned operational evidence.
- Add service-side integration conformance profiles and conformance runs that verify whether an external agent integration produced the expected Stele evidence chain: session, context, outcome, verification, usefulness feedback, task evaluation, and optional ranking rollout evidence.
- Add bounded missing-evidence diagnostics for incomplete integrations, such as session-without-outcome, task-evaluation-without-evidence, feedback-without-subject, rollout-without-dry-run, or repair-without-verification.
- Add retention and cleanup behavior for high-volume assurance and conformance history while preserving append-only incident audit transitions.
- Add admin inspection and control surfaces for assurance evaluations, incidents, alert candidates, conformance profiles, conformance runs, readiness reports, and recovery verification.
- Add optional self-host-friendly alert delivery adapters with bounded payloads, initially limited to `disabled`, `stdout`, and generic `webhook`, with explicit outbound webhook safety validation.
- Extend low-cardinality metrics, structured logs, and self-hosting docs for the assurance and conformance loop.
- Keep Stele strictly as the memory service: no SDK, UI, external-agent execution, model invocation, prompt orchestration, final-answer generation, or hosted incident-management product behavior.

## Non-goals

- Do not add SDKs, UI, hosted-product onboarding, chat interfaces, or end-user application logic.
- Do not execute external agents, invoke models, build prompts, generate final answers, or infer whether an external agent's answer is correct.
- Do not bind directly to Slack, PagerDuty, email providers, or other vendor-specific incident platforms beyond a generic webhook payload.
- Do not introduce Redis, queues, object storage, or any system of record other than PostgreSQL.
- Do not put tenant, project, namespace, incident id, conformance run id, proof id, session id, task id, memory id, actor, reason, query text, webhook URL, or alert recipient into metric labels.
- Do not mutate canonical memory, lifecycle state, task evaluations, feedback, ranking policies, or repair plans as a direct side effect of assurance or conformance evaluation.
- Do not replace existing readiness endpoints; this adds deeper product-readiness reports alongside existing liveness/readiness surfaces.

## Capabilities

### New Capabilities

- `self-hosted-assurance-and-conformance`: Durable health evaluations, incidents, alert candidates, integration conformance runs, scope readiness reports, runbook hints, and recovery verification for self-hosted deployments.

### Modified Capabilities

- `admin-inspection-surface`: Add admin-only inspection/control for assurance evaluations, incidents, alert candidates, conformance profiles, conformance runs, readiness reports, and recovery verification.
- `scope-proof-and-session-loop`: Allow proof/session evidence to participate in conformance and readiness reports, and allow recovery verification to rerun or reference proof/session checks.
- `self-hosting-bootstrap`: Document the operational assurance and integration conformance loop as the production-readiness path after smoke checks.
- `service-runtime-foundation`: Validate assurance cadence, conformance cadence, retention windows, backup/restore proof settings, capacity/load proof settings, and safe alert delivery configuration at startup.
- `service-observability`: Add low-cardinality metrics and bounded structured logs for health evaluation, incident lifecycle, alert candidate generation/delivery, conformance runs, readiness reports, and recovery verification.
- `worker-orchestration-and-maintenance-jobs`: Add scheduler/worker responsibilities for periodic assurance evaluation, alert candidate delivery attempts, stale incident checks, and optional recovery verification dispatch.

## Impact

- Affected APIs: new admin routes for operational assurance, incidents, alert candidates, conformance profiles/runs, readiness reports, and recovery verification.
- Affected storage: PostgreSQL tables for health evaluations, incidents, incident transitions, alert candidates, alert delivery attempts, conformance profiles, conformance runs, missing-evidence diagnostics, operational proof records, readiness reports, recovery verification reports, and retention metadata.
- Affected services: admin HTTP handlers, runtime configuration, worker/scheduler jobs, telemetry, readiness diagnostics, scope proof/session reporting, task/feedback/ranking/repair inspection, and self-hosting docs.
- Affected tests: domain validation, repository isolation, HTTP auth and scope denial, worker/scheduler dispatch, metrics/log label safety, OpenAPI contracts, self-hosting docs, and full OpenSpec validation.
- Artifact references: use `openspec validate self-hosted-assurance-and-conformance-loop --strict` before implementation and `openspec instructions apply --change self-hosted-assurance-and-conformance-loop --json` to work tasks.
