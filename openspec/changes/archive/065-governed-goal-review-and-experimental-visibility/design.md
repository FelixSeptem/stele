## Context

See `proposal.md` for motivation and scope. The implementation extends the existing PostgreSQL-backed governed goal, reserved-insight activation, context assembly, admin inspection, replay, and redacted observability paths. Existing goal records are already append-only and excluded from ordinary retrieval; this change adds a review projection and a separately gated experimental projection.

## Goals / Non-Goals

**Goals:**

- Reuse existing exact-scope authorization, scope-proof, policy-precedence, replay, lifecycle, and redaction primitives.
- Keep review and experimental visibility decisions auditable without copying goal content into a second store.
- Make `goal_context` independently requestable, versioned, fail-closed, and removable through policy rollback.
- Preserve ordinary retrieval and context response compatibility when the experiment is disabled or unavailable.

**Non-Goals:**

- No provider-side activation, task execution, canonical-memory mutation, or automatic planner.
- No new graph database, rollout platform, or generic feature flag service.
- No exposure of raw goal data through public telemetry or diagnostics.

## Decisions

### Reuse governed policy and precedence evaluation

Add a goal visibility policy type to the existing versioned policy and reserved-insight precedence path. Evaluate scope, principal grant, lifecycle, replay, evidence, freshness, review, and rollback in the existing order, then persist a bounded visibility decision. This avoids a second authorization implementation and ensures provider output cannot bypass governance.

**Alternative considered:** a dedicated goal visibility service. Rejected because it would duplicate policy state and create inconsistent rollback semantics.

### Store append-only decisions, project redacted review data

Persist policy decisions and transitions as append-only derived records with replay identity, actor/policy attribution, and bounded reason categories. Build the admin response from a redacted projection that contains only exact-scope aggregate state and stable references. Do not store a parallel copy of goal text for review.

**Alternative considered:** expose the canonical goal row directly to admins. Rejected because it risks content, prompt, provider, and hidden-identifier disclosure.

### Add `goal_context` as an independent context section

Extend the context request/response envelope with an explicit experimental section selector and a bounded section payload. The assembler evaluates this selector after ordinary sections are planned and keeps its candidate list, budget, ranking, counters, and fallback state separate. If the selector is absent or any gate fails, the section is omitted with no observable change to ordinary output.

**Alternative considered:** merge goals into the normal candidate pool. Rejected because it would change ranking and make disablement impossible to reason about.

### Use exact-scope policy keys and fixed telemetry categories

Policy lookup and decision persistence use the resolved tenant/project/namespace scope and policy version, while telemetry hashes or buckets only fixed categories and never emits scope values or identifiers. Diagnostics require the same scope proof and principal grant as the operation being inspected.

**Alternative considered:** global policy and global dashboards. Rejected because global scope would weaken isolation and encourage high-cardinality labels.

### Fail closed and preserve prior evidence on rollback

Any missing or stale gate omits `goal_context`; disablement and rollback prevent new inclusion but do not rewrite goal history or source evidence. Replay returns the existing bounded disposition idempotently.

**Alternative considered:** serve the last successful experimental result during outages. Rejected because stale goal visibility could violate freshness and policy rollback requirements.

## Risks / Trade-offs

- [Risk] A new section selector may be ignored by older clients. → Keep the selector optional and preserve the existing response envelope when absent.
- [Risk] Review projections can accidentally reveal existence through counts or errors. → Apply the existing redaction helper, exact-scope authorization, and no-record indistinguishability tests.
- [Risk] Policy and projection versions can drift. → Store both versions in the append-only decision and reject mismatches during evaluation.
- [Risk] Experimental evaluation adds query work. → Bound candidate counts, use existing scope indexes, and avoid evaluating the section unless explicitly requested.
- [Risk] Rollback may leave operators unsure which requests were affected. → Record bounded inclusion, omission, disablement, and rollback categories with policy version and freshness buckets.

## Migration Plan

1. Add the visibility policy and decision schema fields using additive PostgreSQL migrations.
2. Implement policy evaluation, review projection, and context section behind the default-disabled policy path.
3. Add unit, integration, isolation, replay, rollback, and product-verification coverage.
4. Deploy with no enabled policies; verify ordinary retrieval and context snapshots are unchanged.
5. Enable one exact-scope policy only after review and redacted diagnostics checks pass.
6. Roll back by disabling the policy version; retain all append-only decisions and source evidence.

## Open Questions

None. The remaining choices are implementation details that do not change the contracts or task breakdown.
