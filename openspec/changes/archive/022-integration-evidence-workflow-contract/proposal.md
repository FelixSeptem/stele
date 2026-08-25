## Why

Stele now has service-side evidence surfaces for memory sessions, outcomes, verification, usefulness feedback, task evaluation, ranking rollout, proof, assurance, and conformance, but an external agent integration still has to hand-wire the sequence and can easily leave gaps. A scoped integration evidence workflow contract gives operators and integration authors a durable, service-owned way to know where one external turn or job is in the evidence chain, what is missing, and which Stele API surface should be called next without moving SDK, UI, model execution, or agent runtime logic into this repository.

## What Changes

- Add scoped integration workflow templates that declare bounded expected steps, allowed evidence kinds, freshness windows, completion policy, and runbook hints for an external-agent evidence chain.
- Add durable workflow runs for one external agent turn, job, or integration attempt, including step state, normalized evidence links, gap diagnostics, next actions, and append-only transition history.
- Add public, scoped APIs that let an external integration start a workflow run, record step evidence, read its own run status, and retrieve bounded next actions.
- Add admin-only APIs to create/update/disable templates, list/read workflow runs, inspect gap diagnostics, supersede bad evidence links, and review stale or incomplete workflow records.
- Feed workflow run health into conformance runs, readiness reports, health evaluations, incidents, and alert candidates using bounded categories.
- Add scheduler/worker paths for stale workflow detection, gap materialization, cleanup of high-volume workflow history, and retry-safe background reporting.
- Add low-cardinality metrics and bounded structured logs for workflow lifecycle, step recording, gap diagnostics, next-action generation, cleanup, and conformance/readiness impact.
- Update self-hosting docs with a golden external-agent integration path that shows how to use workflow contracts to close the product evidence loop.

## Non-goals

- Do not add SDKs, UI, hosted onboarding, chat interfaces, or end-user application logic.
- Do not execute external agents, invoke models, build prompts, orchestrate tools, generate final answers, or infer whether an external answer is correct.
- Do not mutate canonical memory, lifecycle state, task evaluations, feedback, ranking policies, repair plans, conformance runs, incidents, or assurance records as a side effect of workflow checks.
- Do not introduce Redis, external queues, object storage, or any system of record other than PostgreSQL.
- Do not add vendor-specific alert, ticketing, incident-management, or workflow SaaS integrations.
- Do not put tenant, project, namespace, workflow run id, template id, evidence id, session id, task id, memory id, actor, reason text, query text, prompt text, model output, webhook URL, or recipient into metric labels or non-admin logs.

## Capabilities

### New Capabilities

- `integration-evidence-workflow-contract`: Durable scoped workflow templates, runs, steps, evidence links, gap diagnostics, next actions, and cleanup for external-agent integration evidence chains.

### Modified Capabilities

- `admin-inspection-surface`: Add admin-only inspection and controls for integration workflow templates, runs, steps, evidence links, gap diagnostics, and stale workflow remediation.
- `scope-proof-and-session-loop`: Allow memory session and scope proof evidence to be attached to integration workflow runs and reported as workflow steps without changing proof/session execution semantics.
- `memory-usefulness-feedback`: Allow usefulness feedback evidence to be linked to workflow steps and diagnosed when feedback is missing, stale, out-of-scope, or subjectless.
- `task-success-evaluation`: Allow task evaluation evidence to be linked to workflow steps and diagnosed when task evidence is missing, opaque-only, stale, or out-of-scope.
- `self-hosted-assurance-and-conformance`: Include workflow run completion and gap diagnostics in conformance, readiness, incidents, alert candidates, and recovery verification.
- `service-observability`: Add low-cardinality metrics and bounded logs for workflow lifecycle, step evidence recording, gap diagnostics, next actions, and readiness/conformance impact.
- `worker-orchestration-and-maintenance-jobs`: Add scheduler/worker responsibilities for stale workflow detection, gap materialization, and workflow history cleanup.
- `self-hosting-bootstrap`: Document the golden integration workflow path after the existing session, task evaluation, conformance, and readiness flows.

## Impact

- Affected APIs: new scoped public routes for workflow run start/step record/status/next actions and new admin routes for template management, workflow inspection, diagnostics, evidence-link correction, and cleanup review.
- Affected storage: additive PostgreSQL tables for workflow templates, template steps, workflow runs, workflow step records, evidence links, gap diagnostics, next actions, transition history, and retention metadata.
- Affected services: memory session, feedback, task evaluation, scope proof, conformance, assurance/readiness, scheduler, worker, telemetry, OpenAPI, and self-hosting docs.
- Affected tests: domain validation, repository isolation, append-only workflow transitions, evidence-link scope safety, HTTP auth and scope denial, conformance/readiness integration, worker/scheduler idempotency, metrics/log low-cardinality safety, OpenAPI coverage, and docs consistency.
- Artifact references: use `openspec validate integration-evidence-workflow-contract --strict` before implementation and `openspec instructions apply --change integration-evidence-workflow-contract --json` to work tasks.
