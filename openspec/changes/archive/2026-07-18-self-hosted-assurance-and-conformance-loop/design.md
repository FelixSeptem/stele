## Context

Stele has reached a broad service-side product loop: scoped ingestion, governance, memory lifecycle, semantic and lexical retrieval, context assembly, derived insights, quality repair, scope proof, memory sessions, usefulness feedback, task-success evaluation, and governed ranking rollout. A self-hosted operator can exercise each loop, but the service still lacks a durable product-readiness layer that answers whether a scope is usable in production and whether an external agent integration is producing the expected service evidence.

This design keeps Stele inside the repository boundary defined in `AGENTS.md`. Stele remains a Go service backed by PostgreSQL. External systems remain responsible for SDKs, UI, model calls, prompts, final answers, and agent runtime orchestration. The new layer evaluates durable Stele records and runtime signals, stores bounded reports, and recommends existing admin surfaces for remediation.

## Goals / Non-Goals

**Goals:**

- Persist scoped operational health evaluations that summarize runtime, backlog, dependency, proof/session, repair, ranking, feedback, task, conformance, capacity/load, and backup/restore health.
- Persist incident records, incident transitions, alert candidates, alert delivery attempts, runbook hints, readiness reports, and recovery verification reports.
- Persist integration conformance profiles that declare which evidence chain an external agent integration is expected to produce.
- Run conformance checks against existing service evidence without executing the external agent.
- Run bounded service-owned capacity/load and backup/restore proof checks without becoming a benchmark suite or disaster-recovery product.
- Apply configurable retention and cleanup to high-volume assurance/conformance history while preserving append-only incident audit transitions.
- Expose admin-only inspection and control surfaces for assurance and conformance records.
- Emit low-cardinality metrics and bounded logs for health evaluation, incident lifecycle, alert candidate generation/delivery, conformance, readiness, and recovery verification.
- Document the production-readiness path for self-hosted operators after smoke checks.

**Non-Goals:**

- No SDK, UI, hosted incident product, chat application, or end-user product workflow.
- No model invocation, prompt orchestration, external-agent execution, or final-answer generation.
- No vendor-specific Slack, PagerDuty, email, or incident-management SDK integration.
- No new system of record beyond PostgreSQL.
- No canonical memory, lifecycle, ranking policy, feedback, task evaluation, or repair mutation as a side effect of health or conformance evaluation.
- No high-cardinality metric labels.

## Decisions

### Decision 1: Scope readiness is a durable report, not an expanded `/ready`

The existing liveness and readiness endpoints remain process/dependency-oriented. The new scope readiness report is an admin-inspectable product-readiness artifact that can include runtime health, integration conformance, active incidents, alert candidates, recent proof/session outcomes, repair state, ranking rollout state, and recommended actions.

Alternatives considered:

- Expand `/ready` to include all product checks: rejected because orchestration readiness should remain fast, low-risk, and not expose privileged details.
- Only document manual checks: rejected because operators need durable, comparable readiness history.

### Decision 2: Incidents and alert candidates are separate records

An incident represents a durable degraded/unhealthy condition. An alert candidate represents a bounded notification opportunity derived from an incident, conformance failure, or critical evaluation finding. Delivery attempts are recorded separately and remain optional.

Alternatives considered:

- Alert directly from metrics: rejected because self-hosted users need auditable state and retryable delivery history.
- Make every alert an incident: rejected because delivery throttling and incident lifecycle have different state machines.

### Decision 3: Conformance verifies evidence completeness, not agent correctness

Conformance profiles declare expected Stele evidence such as memory sessions, turns, context evidence, outcome events, verifications, usefulness feedback, task evaluations, and optional rollout evidence. A conformance run checks durable records for missing, stale, contradictory, or out-of-scope evidence.

Alternatives considered:

- Execute the external agent in a test harness: rejected because this repository does not own agent runtime or model calls.
- Require a specific SDK workflow: rejected because this repository remains API-first and self-host friendly.

### Decision 4: Alert delivery adapters stay generic and bounded

Initial delivery modes are `disabled`, `stdout`, and generic `webhook`. Payloads contain stable categories, severity, status, recommended surfaces, and bounded counts. They exclude scope identifiers, record ids, actor, reason, query text, webhook URL, and raw evidence content from metric labels and non-admin diagnostics.

Webhook configuration is startup-validated before use. The service accepts only `http` or `https` URLs, defaults to requiring `https`, requires an explicit local/self-host override for insecure local endpoints, uses bounded timeouts and payload sizes, rejects unsafe headers, redacts configured secrets, and prevents delivery to unsafe network targets such as link-local metadata addresses. Delivery is performed by durable worker-owned attempts rather than synchronously inside scheduler dispatch.

Alternatives considered:

- Add Slack/PagerDuty/email adapters: rejected for this proposal because vendor-specific behavior is product integration outside the service core.
- Skip delivery entirely: rejected because self-hosted operators need a closed alert loop, even if generic.

### Decision 5: Recovery verification references existing proof/session/conformance surfaces

Recovery verification records whether an operator action or elapsed recovery window restored health. It can reference a rerun proof, session verification, conformance run, repair verification, ranking rollout rollback, capacity/load proof, backup/restore proof, or health evaluation, but it does not execute unsafe mutations inline.

Alternatives considered:

- Automatically run all recovery checks after every incident: rejected because some checks are expensive and scope-specific.
- Treat incident resolution as manual only: rejected because durable verification is needed for operational confidence.

### Decision 6: Capacity/load and backup/restore are bounded assurance proofs

Capacity/load proof records whether the scope is operating within configured service-owned thresholds such as backlog depth, scheduler/worker latency, repository query bounds, and recent evaluation processing limits. It does not run an unbounded load test or simulate arbitrary user traffic.

Backup/restore proof records that a configured operator-owned backup/restore check has recent successful evidence, such as a supplied restore verification marker, check timestamp, checksum/status reference, or bounded admin-provided proof payload. Stele stores and evaluates the evidence; it does not own external backup tooling, object storage, or cross-region disaster-recovery orchestration.

Alternatives considered:

- Build a full benchmark runner: rejected because this repository owns the memory service, not deployment-scale load generation.
- Own backup scheduling and restore execution: rejected because deployment environments differ and PostgreSQL remains the system of record; Stele should evaluate proof evidence, not become backup infrastructure.

### Decision 7: Profiles are explicit and cadences are dedicated

Conformance profiles are explicitly created, updated, or disabled by authorized administrators. The service may expose inactive built-in templates later, but the first implementation does not auto-create active profiles from observed traffic because that could bless incomplete integrations as expected behavior.

The scheduler uses dedicated assurance and conformance cadences with defaults that fall back to the existing maintenance interval. This gives operators clear tuning knobs without changing the runtime mode model.

### Decision 8: Assurance history cleanup is explicit

Health evaluations, component summaries, alert delivery attempts, conformance runs, missing-evidence diagnostics, readiness reports, recovery verification reports, and worker execution details can be high-volume. They receive configurable retention windows and cleanup jobs. Incident records and incident transitions preserve append-only audit history unless a future explicit admin data-retention proposal changes that behavior.

## Risks / Trade-offs

- Assurance scope can become too broad -> keep the first implementation focused on durable evaluation, incident, alert candidate, conformance, readiness, and recovery records; defer vendor integrations and adaptive policies.
- Conformance could leak integration details -> keep reports scoped and admin-only; public routes receive only ordinary session/task/feedback reports.
- Alert noise could overwhelm operators -> require bounded severity, stable reason codes, deduplication windows, delivery throttling, and explicit disabled/default-safe adapter configuration.
- Health evaluation can be stale -> record observed windows and freshness status, and expose stale/unknown as first-class readiness states.
- Recovery verification could mutate product state accidentally -> restrict it to existing safe checks and references; remediation actions stay in their existing governed admin surfaces.
- Webhook delivery could become an SSRF or secret-leak vector -> validate outbound target configuration, bound payloads/timeouts, block unsafe destinations, and redact configured secrets from storage, logs, metrics, and admin views.
- Assurance history could grow without bound -> require retention settings and cleanup jobs for high-volume records while preserving incident audit transitions.

## Migration Plan

1. Add additive PostgreSQL tables for assurance evaluations, incidents, incident transitions, alert candidates, alert delivery attempts, conformance profiles, conformance runs, conformance diagnostics, operational proof records, readiness reports, recovery verification, and retention metadata.
2. Add domain types and validation for status, severity, component, reason code, expected evidence, adapter kind, delivery result, proof target, retention class, and runbook hint categories.
3. Add repositories with strict tenant/project/namespace filters and append-only history where appropriate.
4. Add service methods that aggregate existing Stele evidence, capacity/load proof evidence, and backup/restore proof evidence into evaluations and reports without mutating source records.
5. Add admin HTTP/OpenAPI surfaces and low-cardinality telemetry.
6. Add scheduler/worker dispatch for periodic evaluation, alert candidate delivery, stale incident detection, assurance/conformance cleanup, and optional recovery verification.
7. Update self-hosting docs with the production-readiness loop.

Rollback strategy:

- Database migration is additive.
- Disabling scheduler jobs or alert delivery stops new automated records but preserves existing incident, alert, conformance, and readiness history.
- Existing memory, feedback, task, repair, ranking, proof, and session records remain untouched.

## Open Questions

- None. The first implementation uses dedicated cadence settings with maintenance-interval fallbacks, durable worker-owned webhook delivery attempts, and explicit admin-created conformance profiles.
