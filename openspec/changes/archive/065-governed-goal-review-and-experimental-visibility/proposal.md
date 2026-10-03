## Why

The archived `governed-goal-insights` capability provides bounded, evidence-backed goal candidates and redacted diagnostics, but it does not yet define a complete operator review contract or a separately governed way to expose an eligible goal to an experimental context consumer. Without that boundary, review tooling could accidentally disclose goal content or ordinary retrieval could inherit goal state before the product has evidence that the policy, scope, freshness, and rollback controls work together.

This change establishes an exact-scope review/admin surface and an independently opt-in `goal_context` experiment so goal visibility can be evaluated safely while ordinary retrieval, context assembly, ranking, counters, and canonical memory remain unchanged.

## What Changes

- Add an exact-scope, authorized goal review/admin contract that returns bounded review state, policy/replay/freshness categories, rollback state, and redacted evidence references without raw goal content, prompts, provider payloads, hidden identifiers, or foreign-scope data.
- Add a separately versioned `goal_context` visibility policy with principal grant, scope proof, freshness, evidence, review, and precedence gates; keep it disabled by default and fail closed on any failed gate.
- Add an experimental `goal_context` section that is emitted only when the versioned policy is enabled for the exact scope and all authorization and evidence gates pass.
- Keep `goal_context` isolated from ordinary retrieval, ordinary context assembly, ranking, public counts, and default lifecycle behavior; provider output cannot activate or expose a goal directly.
- Reuse governed goal insight records, reserved-insight activation precedence, replay/idempotency, append-only lifecycle history, and redacted observability instead of introducing a second store or rollout system.
- Define rollback and disablement behavior that stops new experimental visibility while preserving prior review decisions, derived history, source evidence, and auditability.

## Capabilities

### New Capabilities

- `governed-goal-experimental-visibility`: Defines the authorized goal review contract, exact-scope versioned visibility policy, experimental `goal_context` section, evidence/freshness gates, redaction rules, and rollback semantics.

### Modified Capabilities

- `governed-goal-insights`: Extend goal visibility and diagnostics requirements with the review handoff and experimental-surface boundary while retaining default exclusion.
- `governed-reserved-insight-activation`: Add the visibility-policy precedence and principal-grant checks required before a governed goal can enter `goal_context`.
- `context-assembly`: Define `goal_context` as an independently authorized optional section that cannot affect ordinary sections, ordering, counts, or fallback behavior.
- `admin-inspection-surface`: Add the exact-scope goal review/admin contract and its redacted response boundary.
- `service-observability`: Add bounded telemetry and diagnostics for goal review, visibility-policy evaluation, experimental inclusion, freshness, fallback, disablement, and rollback.

## Impact

- Affected OpenSpec contracts: the new governed goal experimental visibility capability and the five modified capabilities listed above.
- Affected runtime areas: goal review/admin handlers, policy and precedence evaluation, context projection/assembly, redacted diagnostics, and lifecycle/observability recording.
- Affected persistence: append-only policy evaluation and visibility decision history in PostgreSQL; no second system of record, graph store, or independent rollout service.
- Affected public behavior: ordinary retrieval and context responses remain unchanged unless an authorized caller requests the separately enabled experimental section for the exact scope.
- Operational impact: operators gain bounded review and rollback evidence; experimental failures fail closed and remain observable through low-cardinality telemetry.

## Non-goals

- Do not expose raw goal titles, summaries, prompts, provider payloads, hidden identifiers, or foreign-scope data through review, diagnostics, metrics, or context.
- Do not add goals to default retrieval, default context assembly, ordinary ranking, or ordinary counters.
- Do not let providers activate goals, execute tasks, mutate canonical memory, delete source evidence, or bypass policy, scope, grant, review, freshness, or replay checks.
- Do not introduce a second storage system, graph store, generic rollout framework, or unrelated task-planning capability.
