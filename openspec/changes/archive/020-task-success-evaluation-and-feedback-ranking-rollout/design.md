## Context

Stele already supports scoped memory sessions, session verification, usefulness feedback, quality findings, repair plans, and per-request feedback-aware ranking hints. The remaining memory-quality product gap is that external task success is not represented as durable service evidence, and feedback-aware ranking cannot be rolled out as a governed default for a scope.

This design keeps Stele as the memory service only. External agents remain responsible for prompts, model calls, final answers, and task-success judgment. Stele validates and stores caller-provided task verdicts, links them to memory evidence, derives bounded diagnostics, and applies ranking policy only through auditable rollout records.

## Goals / Non-Goals

**Goals:**

- Persist task-success evaluations with scope, task objective, success criteria, external verdict, memory-session links, outcome evidence, expected recall targets, feedback links, actor, reason, idempotency, and audit timestamps.
- Expose task evaluation reports that show memory contribution evidence, bounded failure categories, related feedback, session verification, quality findings, repair plans, and next actions.
- Add scoped ranking rollout governance that can dry-run, activate, inspect impact, pause/disable, and roll back feedback/task-success-aware ranking for search and context assembly.
- Keep ranking signals rebuildable from source evidence: active usefulness feedback, task evaluations, session verification, and quality findings.
- Preserve lifecycle, provenance, tenant/project/namespace isolation, and high-cardinality metric safety.

**Non-Goals:**

- No SDK, UI, hosted product, external agent runtime, prompt orchestration, model invocation, or generated final answers.
- No automatic LLM judge or autonomous task-verdict inference.
- No canonical memory rewrites, vector history rewrites, or direct lifecycle mutations from task evaluations or ranking policy.
- No use of superseded feedback, hidden memory content, forgotten memory, deleted memory, or out-of-scope evidence as active ranking input.
- No external alert delivery or product analytics pipeline.

## Decisions

### Decision 1: Task verdicts are caller-provided evidence, not Stele-generated judgments

Task evaluations will accept bounded verdicts such as `succeeded`, `failed`, `partial`, and `inconclusive`, plus caller-provided success criteria and evidence links. Stele will validate shape, scope, references, idempotency, and metadata bounds, but it will not decide whether the task succeeded.

Alternatives considered:

- Stele computes success with an evaluator model: rejected because it adds model orchestration and prompt behavior outside the repository boundary.
- Only reuse session verification as task success: rejected because memory recall can pass while the task fails for non-memory reasons, and vice versa.

### Decision 2: Ranking rollout is a durable policy with dry-run and rollback, not a config toggle

Scope-level feedback-aware ranking changes will be represented as durable rollout policies with explicit status, actor, reason, signal configuration, thresholds, dry-run reports, activation history, and rollback history. Default ranking remains unchanged until a policy is active for the resolved scope.

Alternatives considered:

- Add a config flag for default feedback-aware ranking: rejected because it is not scoped, not auditable, and hard to roll back safely.
- Keep only per-request opt-in forever: rejected because it prevents product-quality learning loops from improving ordinary retrieval.

### Decision 3: Signals are bounded and source-of-truth rebuildable

Ranking input will be derived from active usefulness feedback, task evaluations, session verification, and quality findings. Materialized summaries may be stored for performance, but they must be rebuildable from durable evidence and exclude superseded or unauthorized evidence by default.

Alternatives considered:

- Store a mutable usefulness score directly on memory rows: rejected because it obscures provenance and makes audit/rebuild difficult.
- Use every feedback item immediately: rejected because single-item noise can destabilize ranking.

### Decision 4: Search and context assembly share policy evaluation but keep response semantics separate

The rollout policy evaluator should be shared by retrieval and context assembly. Search can expose hit-level impact diagnostics; context assembly must additionally preserve budget, section, and citation safety.

Alternatives considered:

- Implement separate policy paths for search and context: rejected because behavior would drift and be harder to audit.
- Force context assembly to mirror search ordering exactly: rejected because context has section and budget constraints that are not present in search.

### Decision 5: Quality bridge remains approval-gated

Repeated task failures with memory contribution evidence can create bounded quality findings or summaries, but repair actions remain approval-gated through the existing quality/repair plan path.

Alternatives considered:

- Automatically suppress or demote memory after task failure: rejected because task failure can have non-memory causes and must not mutate lifecycle inline.
- Keep task failures out of quality evaluations: rejected because operators need task-level evidence to diagnose memory contribution failures.

## Risks / Trade-offs

- Task verdicts may be noisy or biased toward external agent behavior -> require bounded verdict taxonomy, explicit evidence links, metadata limits, and minimum evidence thresholds before ranking impact.
- Ranking rollout could degrade recall if signal weights are too aggressive -> require dry-run impact reports, default-off rollout, scoped activation, rollback, and tests proving lifecycle visibility remains primary.
- Task evaluations can include high-cardinality evidence -> store it only in scoped durable records and expose metrics with bounded labels only.
- Context assembly ranking is harder than search ranking because of budget and section constraints -> treat ranking policy as a hint layer, not as permission to bypass context budget or citation requirements.
- This proposal combines two related but sizable capabilities -> keep implementation phased so task evaluation can be completed and tested before rollout activation behavior.

## Migration Plan

1. Add PostgreSQL tables for task evaluations, task evidence links, task summaries, ranking rollout policies, rollout dry-run reports, impact entries, and rollback audit history.
2. Add domain types, repository methods, and rebuild helpers with scope isolation and idempotency.
3. Add task evaluation service and HTTP/OpenAPI contracts.
4. Link task evaluations into session reports, usefulness summaries, and quality findings.
5. Add ranking policy dry-run and diagnostics without changing default search/context behavior.
6. Add activation/rollback paths and wire active policies into retrieval/context ranking only after dry-run and scope checks.
7. Add metrics/logs/docs and run targeted plus full verification.

Rollback strategy:

- Database migration should be additive.
- Policy rollback disables active ranking policy for a scope without modifying memories, feedback records, task evaluations, or session history.
- If rollout behavior is defective, operators can disable the policy and default ranking returns to baseline.

## Scope Decisions

- Initial ranking weights are conservative fixed constants or policy-defined bounded enum profiles in this proposal; adaptive calibration is deferred.
- Percentage rollout and traffic splitting are out of scope; rollout modes are `diagnostics_only`, `dry_run`, and active-for-scope.
- Task evaluation summaries are rebuildable from durable evidence and can be computed on read or materialized by explicit rebuild helpers; background aggregation is optional only if needed for implementation performance.
- Active policy precedence is explicit: per-request diagnostics can request comparison details, but ordinary request ranking follows the active scoped policy when present; rollback or disable removes that default effect.
- Policy activation requires a successful dry-run for the same scope and surfaces, evidence threshold satisfaction, actor/reason attribution, and no active blocker finding that would make rollout unsafe.
