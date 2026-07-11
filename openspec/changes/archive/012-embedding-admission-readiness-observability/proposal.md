## Why

Embedding provider cutovers are now durable and scheduler-driven, but operators still lack a first-class way to know whether a draft plan can safely be activated before rollout begins. Runtime observability also remains hook-based, so self-hosted deployments cannot reliably distinguish cutover admission problems from rebuild execution or provider readiness issues.

This change introduces a small admission and readiness framework with embedding cutover and rebuild as the first consumers. It keeps provider migration decisions explicit while exposing health and metrics signals that are useful during rollout.

## What Changes

- Add a reusable admission/readiness model for producing structured decisions, blockers, warnings, and diagnostic findings.
- Add an embedding cutover preflight endpoint for draft plans that computes an immediate activation report without persisting the report.
- Make cutover activation rerun the same admission evaluator and reject plans with hard blockers before membership registration or wave scheduling.
- Add runtime health endpoints for `livez` and `readyz`, with readiness behavior that respects `api`, `worker`, and `scheduler` mode responsibilities.
- Add a Prometheus-style metrics endpoint focused on embedding cutover admission, cutover plan state, rebuild backlog, provider readiness probes, and scheduler dispatch behavior.
- Keep first-version framework integration limited to embedding cutover and embedding rebuild execution.

## Capabilities

### New Capabilities

- `admission-readiness-diagnostics`: Defines the shared admission, readiness, diagnostic finding, health, and metrics contract used by operator-facing runtime checks.

### Modified Capabilities

- `embedding-provider-cutover-governance`: Adds explicit preflight evaluation for draft cutover plans and hard activation blockers for invalid or empty rollout attempts.
- `admin-inspection-surface`: Adds admin-only cutover preflight inspection and standard runtime health and metrics surfaces.
- `service-observability`: Requires concrete health and metrics exposure for the embedding admission and rebuild execution path rather than only internal observer hooks.

## Impact

- API: Adds `POST /v1/admin/embedding/cutovers/{cutover_plan_id}:preflight`, `GET /livez`, `GET /readyz`, and `GET /metrics`.
- Internal packages: Adds shared admission/readiness diagnostic types and embedding-specific evaluators or adapters.
- Runtime: `api` readiness checks service and PostgreSQL availability; `worker` and `scheduler` readiness also consider embedding provider reachability when embedding rebuild or cutover execution is enabled.
- Storage: No persistence is required for preflight reports in this change.
- Dependencies: May add a mature Prometheus HTTP exporter library if it fits the existing Go service shape.
- Tests: Adds coverage for cutover preflight decisions, activation rejection, health mode behavior, metrics output, and OpenAPI documentation.

## Non-goals

- Persisting preflight report snapshots or building a preflight history ledger.
- Applying the admission framework to governance, summary compaction, retention, or unrelated maintenance jobs.
- Making transient provider network failures a hard cutover activation blocker.
- Rewriting existing vector revision history, cutover item history, or recovery ledger semantics.

## Artifact References

- Proposal workflow: `.codex/skills/openspec-propose/SKILL.md`
- Implementation workflow after approval: `.codex/skills/openspec-apply-change/SKILL.md`
- Archive workflow after implementation: `.codex/skills/openspec-archive-change/SKILL.md`
