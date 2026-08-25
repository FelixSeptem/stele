## Context

Stele has accumulated the service-side surfaces needed for an agent memory product loop: scoped event ingestion, governance, retrieval, context assembly, memory sessions, turn outcomes, verification, usefulness feedback, task evaluation, ranking rollout, quality repair, scope proof, and assurance/conformance readiness. The remaining service-owned gap is orchestration of evidence capture, not execution of the external agent.

Today, an external integration must read documentation and call multiple APIs in the correct order. If it misses a turn outcome, writes feedback without a useful subject, omits task evidence, or records only opaque tokens where internal evidence is required, conformance can detect the gap later but Stele cannot guide the integration while the run is in progress. This change adds a durable integration evidence workflow contract that tracks a single external turn, task, or job through expected evidence steps and exposes bounded next actions.

The design follows the repository constraints in `AGENTS.md`: PostgreSQL remains the only system of record, public APIs remain OpenAPI-first and self-host friendly, every API/query/job is tenant/project/namespace scoped, and Stele remains the memory service only.

## Goals / Non-Goals

**Goals:**

- Persist scoped workflow templates that define expected evidence steps for external-agent integrations.
- Persist workflow runs, step records, evidence links, gap diagnostics, next actions, and append-only transitions.
- Let external integrations record progress and retrieve next actions without giving Stele responsibility for executing the integration.
- Normalize references to existing Stele evidence such as sessions, turns, context, outcomes, verification, feedback, task evaluations, proof runs, repair plans, ranking rollouts, conformance runs, readiness reports, and incidents.
- Detect missing, stale, out-of-order, duplicated, hidden, opaque-only, contradictory, invalid, and out-of-scope evidence with bounded diagnostic categories.
- Feed workflow health into conformance, readiness, incidents, alert candidates, and recovery verification.
- Add scheduler/worker paths for stale workflow detection, gap materialization, and retention cleanup.
- Emit low-cardinality metrics and bounded lifecycle logs.
- Document a golden integration path for self-hosted operators and external-agent authors.

**Non-Goals:**

- No SDK, UI, hosted onboarding, chat surface, or end-user product workflow.
- No external-agent execution, model invocation, prompt building, tool orchestration, or final-answer generation.
- No correctness judgment of the external agent's answer.
- No vendor-specific workflow, alerting, ticketing, or incident-management integrations.
- No new system of record beyond PostgreSQL.
- No mutation of canonical memory, feedback, task evaluation, ranking, repair, conformance, incident, or assurance records as a side effect of workflow diagnostics.

## Decisions

### Decision 1: Workflows are contracts over evidence, not agent runtimes

An integration workflow template declares required or optional evidence steps and allowed evidence kinds. A workflow run records which evidence was supplied for one external turn, task, or job. Stele can say "record a turn outcome next" or "task evaluation evidence is stale"; it does not run the turn, invoke a model, or decide the answer's quality.

Alternatives considered:

- Add a first-party agent runtime: rejected because the repository owns the memory service only.
- Keep this only in docs: rejected because docs cannot prevent or diagnose incomplete live integrations.

### Decision 2: Workflow steps use bounded enums and normalized evidence links

The first implementation uses explicit workflow step kinds such as `session_started`, `context_requested`, `turn_outcome_recorded`, `session_verification_recorded`, `usefulness_feedback_recorded`, `task_evaluation_recorded`, `quality_checked`, `repair_reviewed`, `ranking_rollout_checked`, `conformance_checked`, `readiness_checked`, and `recovery_verified`.

Evidence links use bounded evidence kinds mapped to existing Stele surfaces. Internal links store scoped record references; opaque links store caller tokens for audit but cannot satisfy steps that require internal evidence.

Alternatives considered:

- Free-form step names and evidence kinds: rejected because they make validation, diagnostics, metrics, and conformance unsafe.
- Copy evidence payloads into workflow records: rejected because canonical records already preserve provenance and lifecycle; copying increases leakage and drift risk.

### Decision 3: Public workflow APIs are scoped and minimal

Public callers can start a workflow run, record step evidence, read their own run status, and retrieve next actions. Template management, stale run inspection, evidence supersession, cleanup review, and cross-run diagnostics stay under admin routes.

Alternatives considered:

- Admin-only workflows: rejected because external integrations need a low-friction way to record progress while running.
- Public template writes: rejected because templates define governance expectations and should require admin authorization.

### Decision 4: Next actions are generated from stored state

Next actions are durable, bounded recommendations derived from the template, step states, evidence links, and diagnostics. They point to existing API/admin surfaces using route categories and do not include raw prompts, model outputs, hidden memory content, or out-of-scope record existence.

Alternatives considered:

- Compute next actions only on read: rejected because operators need durable audit history and stale workflow jobs need stable outputs.
- Store arbitrary human remediation text only: rejected because downstream tooling needs bounded categories.

### Decision 5: Workflow diagnostics feed conformance and readiness

Conformance runs can inspect workflow runs for missing required steps, stale steps, opaque-only evidence, or out-of-scope links. Readiness reports can summarize recent workflow completion health and degrade or report unknown integration readiness when workflow evidence is incomplete.

Alternatives considered:

- Keep workflow health separate from assurance: rejected because the purpose is to close the product evidence loop.
- Let workflow runs automatically resolve incidents: rejected because incident transitions must remain explicit and auditable.

### Decision 6: Background jobs are bounded and idempotent

Scheduler dispatch marks eligible stale workflow runs and cleanup windows. Workers materialize gap diagnostics and next actions using durable claims, retry budgets, and idempotency keys. Cleanup applies only to high-volume workflow history and preserves template definitions and append-only transition/audit records according to configured retention.

Alternatives considered:

- Synchronous gap scanning on every step record: rejected because workflow evidence can touch many surfaces and should not make hot write paths heavy.
- Delete all completed workflow state quickly: rejected because conformance and readiness need recent integration history.

### Decision 7: Metrics and logs expose categories, not identifiers

Metrics and lifecycle logs use bounded labels/fields such as operation, result, template status, run status, step kind, evidence kind, gap category, next-action category, readiness impact, and cleanup category. They exclude scope identifiers, record ids, actor, reason text, query text, prompt text, model output, webhook URL, and recipient.

Alternatives considered:

- Add workflow ids to metrics for easier debugging: rejected because it would create high-cardinality telemetry and can leak integration details.

## Risks / Trade-offs

- Workflow contracts can become an SDK substitute -> Keep APIs generic and route-oriented; document that SDKs remain external.
- Integrations may overuse opaque evidence -> Allow opaque evidence for audit, but require internal links when a template step needs service-verifiable evidence.
- Extra workflow records can increase storage volume -> Add retention classes, cleanup jobs, and bounded list APIs from the start.
- Step ordering can be too rigid for diverse agents -> Support required, optional, and repeatable steps while keeping step kinds bounded.
- Conformance could leak out-of-scope record existence -> Validate every evidence link against the workflow run scope and report only stable categories outside authorized admin detail.
- Hot write path can become too expensive -> Record steps/evidence quickly and move stale scanning, gap materialization, and cleanup to worker/scheduler jobs.
- Operators may confuse next actions with automatic remediation -> Next actions only recommend API/admin surfaces; remediation still happens through existing governed routes.

## Migration Plan

1. Add additive PostgreSQL tables for workflow templates, template steps, workflow runs, step records, evidence links, gap diagnostics, next actions, transitions, and retention metadata.
2. Add Go domain types and validation for workflow status, template status, step kind, evidence kind, diagnostic category, next-action category, completion policy, and retention class.
3. Add repositories with strict tenant/project/namespace filters, idempotency keys, append-only transitions, and evidence-link scope checks.
4. Add workflow service methods for template management, run creation, step recording, gap diagnostics, next-action generation, and read/list operations.
5. Add public and admin HTTP/OpenAPI surfaces.
6. Wire workflow health into conformance/readiness/assurance aggregation without mutating source records.
7. Add scheduler/worker dispatch and cleanup paths.
8. Add metrics/logs and self-hosting documentation.

Rollback strategy:

- Database migration is additive.
- Disabling scheduler jobs stops stale workflow materialization and cleanup but preserves existing workflow records.
- Existing memory/session/feedback/task/ranking/proof/assurance records remain untouched.

## Open Questions

- None for the first implementation. The first version uses explicit admin-created templates, bounded step/evidence enums, public scoped run recording, admin-only template management, and asynchronous stale workflow diagnostics.
