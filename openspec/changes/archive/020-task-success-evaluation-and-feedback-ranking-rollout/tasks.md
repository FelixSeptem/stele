## 1. Task Evaluation Data Model

- [x] 1.1 Add migrations for task evaluations, task evidence links, task evaluation supersession/correction history, task summaries, ranking rollout policies, dry-run reports, impact entries, active policy state, and rollback audit history.
- [x] 1.2 Add Go domain types for task verdicts, contribution categories, evidence target kinds, task correction state, task report fields, ranking rollout policy status, rollout surfaces, signal sources, threshold status, activation gates, impact reason codes, and rollback records.
- [x] 1.3 Add repository methods for creating, deduplicating, reading, listing, superseding, and summarizing task evaluations with tenant/project/namespace filters.
- [x] 1.4 Add repository methods for creating, reading, dry-running, activating, disabling, rolling back, and listing ranking rollout policies with scoped audit history.
- [x] 1.5 Add repository tests for task evaluation idempotency, scope isolation, opaque evidence tokens, supersession exclusion from active summaries/ranking signals, high-cardinality evidence preservation, ranking policy activation gates, rollback, and active-policy lookup.

## 2. Task Evaluation Service And API

- [x] 2.1 Implement task evaluation validation for scope, verdict taxonomy, contribution categories, success criteria bounds, evidence reference shape across sessions/events/citations/insights/feedback/quality/repair, actor/reason attribution, metadata bounds, and idempotency keys.
- [x] 2.2 Implement service methods to create, list, read, supersede, and summarize task evaluations without executing agents or inferring verdicts.
- [x] 2.3 Implement task evaluation reports that link session, turn, raw event, outcome event, verification, expected recall, feedback, context citation, derived insight, memory, quality finding, and repair plan evidence without exposing hidden or out-of-scope content.
- [x] 2.4 Add public or service-side HTTP endpoints for scoped task evaluation creation and task report reads, plus admin endpoints for list/detail/supersession/summary inspection through the existing admin boundary.
- [x] 2.5 Add API tests for task creation, duplicate idempotency behavior, invalid verdict rejection, invalid contribution category rejection, opaque evidence handling, supersession, report visibility, admin inspection, and out-of-scope denial.

## 3. Session, Feedback, And Quality Integration

- [x] 3.1 Extend memory session reports with bounded task evaluation ids, verdict categories, memory contribution categories, linked quality finding codes, and next actions.
- [x] 3.2 Extend usefulness feedback creation and summaries to allow scoped task evaluation references and task-success aggregate fields without mutating feedback records automatically.
- [x] 3.3 Implement task-summary aggregation from active task evaluations with rebuildable source-of-truth semantics and deterministic exclusion of superseded task evaluations.
- [x] 3.4 Map repeated task-level missing, noisy, stale, irrelevant, hidden, or inconclusive memory contribution failures into bounded quality finding codes or summaries.
- [x] 3.5 Generate approval-gated repair recommendations from task-derived findings for embedding retry, governance inspection, derived insight replay, suppression review, ranking rollout rollback, or manual review.
- [x] 3.6 Add tests proving task evaluations can drive session reports, usefulness summaries, quality findings, and repair recommendations without auto-approving repairs, executing repairs inline, mutating canonical memory, or leaking hidden evidence.

## 4. Ranking Rollout Governance

- [x] 4.1 Implement ranking rollout policy validation for scope, surfaces, signal sources, thresholds, evidence minimums, dry-run requirements, activation blockers, actor/reason attribution, status transitions, and unsupported unbounded signals.
- [x] 4.2 Implement dry-run comparison for search that records baseline rank, adjusted rank, changed lifecycle-visible subjects, bounded reason codes, signal categories, and evidence counts without changing default results.
- [x] 4.3 Implement dry-run comparison for context assembly that records candidate priority, included/omitted status, budget impact, bounded reason codes, signal categories, and evidence counts without changing default context output.
- [x] 4.4 Implement activation gates, active-policy lookup, disable, rollback, and impact report reads with durable audit history and no canonical memory mutation.
- [x] 4.5 Wire active ranking policies into search as bounded ranking hints while preserving lifecycle visibility, scope isolation, and baseline behavior when no policy is active.
- [x] 4.6 Wire active ranking policies into context assembly as bounded ranking hints while preserving lifecycle visibility, citation safety, section constraints, and context budget.
- [x] 4.7 Add tests proving default search/context ranking remains stable without active policy, dry-run is non-mutating, activation requires successful dry-run and no blockers, active policies affect only authorized scopes/surfaces, rollback restores baseline behavior, thresholds prevent single-signal default ranking shifts, and hidden memory is not exposed.

## 5. Observability, OpenAPI, And Docs

- [x] 5.1 Add low-cardinality metrics for task evaluation creation, deduplication, rejection, supersession, summary aggregation, task-derived quality findings, ranking dry-run, activation, policy evaluation, impact, disable, and rollback.
- [x] 5.2 Add structured logs for task evaluation and ranking rollout lifecycle transitions without task ids, session ids, memory ids, actor, reason, query text, tenant, project, or namespace in metric labels.
- [x] 5.3 Update OpenAPI documentation and OpenAPI tests for task evaluation endpoints, admin task inspection, task reports, task summaries, ranking rollout policy endpoints, dry-run/activation/impact/rollback reports, extended session reports, extended feedback summaries, and ranking diagnostics.
- [x] 5.4 Update self-hosting docs to show the external-agent task-success loop and governed feedback-ranking rollout from session evidence through task evaluation, dry-run, activation, impact inspection, rollback, and report verification.
- [x] 5.5 Document remaining product gaps after this proposal: SDK/UI collection surfaces, external agent runtime integration, operational assurance, alert delivery adapters, and advanced scoring calibration.

## 6. Verification

- [x] 6.1 Run targeted tests for task evaluation domain services, repositories, HTTP handlers, session report integration, feedback summary integration, quality bridge, retrieval ranking, context ranking, telemetry, docs, and OpenAPI.
- [x] 6.2 Run `go test ./... -count=1`.
- [x] 6.3 Run `openspec validate task-success-evaluation-and-feedback-ranking-rollout --strict`.
