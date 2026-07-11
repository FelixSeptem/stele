## Context

Stele already has durable embedding provider cutover plans, append-only vector revision history, scheduler-driven rollout waves, and embedding recovery audit. Activation currently validates provider support at a basic level, but operators do not have a dedicated preflight report before activating a draft plan. Observability is also limited to internal telemetry hooks, so self-hosted operators cannot scrape concrete health or metrics signals during rollout.

This design introduces a thin admission/readiness layer that can be reused later but is integrated only with embedding cutover and embedding rebuild in this change.

## Goals / Non-Goals

**Goals:**

- Provide an immediate preflight report for draft embedding cutover plans.
- Reuse the same admission evaluator during activation so hard blockers cannot be bypassed.
- Keep provider reachability semantics clear: activation uses static runtime target resolution; runtime readiness can include live provider checks for worker and scheduler modes when embedding execution is enabled.
- Expose concrete `livez`, `readyz`, and Prometheus-style metrics for embedding admission and rebuild operation.
- Keep the first framework integration limited to embedding cutover and embedding rebuild.

**Non-Goals:**

- Persisting preflight reports or creating a preflight history ledger.
- Applying admission/readiness to governance workers, summary compaction, retention sweeps, or unrelated jobs.
- Treating transient provider network failures as hard activation blockers.
- Changing vector revision lineage, cutover item history, recovery audit, or rollback semantics.

## Decisions

### Decision: Add a small shared admission/readiness model

The shared model should define decisions, blockers, warnings, readiness checks, diagnostic findings, and metric-friendly labels. Embedding-specific code supplies evaluators and probes, while handlers and runtime wiring depend on the shared shape.

Alternatives considered:

- Cutover-local evaluator only: smaller now, but duplicates finding and health semantics once rebuild readiness or later maintenance jobs need similar diagnostics.
- Full operator control plane: more complete, but too broad for this change because it would introduce persisted evaluation state and multi-job rollout governance.

### Decision: Make preflight immediate and non-persistent

`POST /v1/admin/embedding/cutovers/{cutover_plan_id}:preflight` computes a report from the current draft plan, runtime provider registry, scope state, and eligible memory population. The report is returned to the caller and not written back to the plan.

This avoids stale "last preflight passed" state. Activation reruns the evaluator and treats that fresh result as the source of truth.

### Decision: Activation uses static admission blockers

Activation is denied when the evaluator finds deterministic blockers:

- target provider, model, or dimensions cannot be resolved by the configured runtime provider registry
- class route support cannot satisfy the plan target
- the same scope already has an active or paused cutover plan
- the plan has zero eligible memories

Provider network reachability is not an activation blocker. It is reported through warnings, readiness, and metrics so an operator can see runtime execution risk without binding plan activation to transient external availability.

### Decision: Readiness is mode-aware

`livez` reports process liveness. `readyz` reports runtime readiness:

- `api` mode checks Stele runtime and PostgreSQL availability.
- `worker` and `scheduler` modes check Stele runtime and PostgreSQL availability, and include provider reachability only when embedding rebuild or cutover execution is enabled.

This keeps public API readiness stable while allowing execution modes to surface provider dependency failures when they matter.

### Decision: Metrics are concrete but scoped

The metrics endpoint should expose Prometheus-style counters and gauges for embedding admission and rebuild only:

- preflight allow and deny counts
- blocker and warning counts by code
- active and paused cutover plan counts
- cutover item counts by status
- rebuild backlog counts by status and target
- provider probe success and failure counts
- scheduler cutover wave dispatch counts and lag

The implementation can use a mature Go Prometheus library if it fits the service shape; otherwise it can start with a minimal text exposition adapter over the existing telemetry observer.

## Risks / Trade-offs

- **Risk:** A plan can pass preflight and fail activation later because runtime configuration or scope state changed. -> **Mitigation:** Activation always reruns admission and returns the fresh block report.
- **Risk:** Not persisting preflight reports reduces auditability of rejected activation attempts. -> **Mitigation:** Activation rejection responses are deterministic and metrics count blocker codes; durable audit can be added later if operator demand is clear.
- **Risk:** Provider readiness probes can add latency or noisy failures. -> **Mitigation:** Probe only when execution mode and embedding execution are enabled, bound probe timeouts, and keep activation static.
- **Risk:** Metrics labels can become high-cardinality. -> **Mitigation:** Use stable low-cardinality labels such as component, operation, decision, blocker code, warning code, status, provider, and model; do not label by memory id or plan id.

## Migration Plan

1. Add shared admission/readiness diagnostic types without changing existing handlers.
2. Add embedding admission evaluator and repository queries for eligible counts, class breakdowns, and conflicting plan detection.
3. Add preflight handler and OpenAPI contract.
4. Update activation path to rerun the evaluator and reject blocker reports before membership registration.
5. Add health and metrics handlers, then wire runtime readiness dependencies by mode.
6. Document self-hosting checks and operator workflow.

Rollback is straightforward because the change adds endpoints and stricter activation checks without schema persistence for preflight reports. If needed, the activation path can be returned to the prior validator while keeping read-only preflight available.

## Open Questions

- Whether provider probe implementation should be part of the provider interface or a separate readiness adapter.
- Whether `readyz` should return only status text or include a compact JSON diagnostic body for failed checks.
