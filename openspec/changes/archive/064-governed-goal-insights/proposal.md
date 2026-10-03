## Why

Stele now has a provider-neutral reasoning envelope, reserved-insight activation,
bi-temporal evidence, governed operation precedence, and real-stack intent
verification. The `goal` insight type is still only reserved vocabulary, so a
runtime cannot produce an auditable, replayable goal candidate without either
falling back to an unstructured insight or inventing a bypass around the
existing governance path. This change adds the smallest useful goal-specific
contract while keeping it shadow/review-only and non-authoritative by default.

## What Changes

- Add a `governed-goal-insights` capability for evidence-backed, exact-scope
  goal candidates that reuse the existing reasoning envelope and reserved
  insight lifecycle.
- Define bounded goal metadata: title/summary, state, optional validity
  interval, evidence references, uncertainty, source watermark, provider and
  policy versions, and deterministic replay identity.
- Restrict goal derivation to offline or shadow execution and require visible,
  in-scope evidence before a candidate can be retained for review.
- Require an independently versioned goal policy and explicit review handoff;
  provider output cannot activate a goal, mutate canonical memory, delete
  evidence, or change another insight type.
- Keep ordinary retrieval and context assembly unchanged: goal candidates and
  derived goal records remain excluded by default and are visible only through
  bounded authorized diagnostics or explicit experimental surfaces.
- Add append-only goal review and lifecycle dispositions for `proposed`,
  `active`, `completed`, `abandoned`, and `stale`, while the default proposal
  path emits reviewable or `would_activate` outcomes rather than active
  context-visible records.
- Add deterministic replay, policy-disablement, rollback, redaction, exact
  scope, and PostgreSQL + pgvector conformance evidence for the goal path.
- Reconcile the authoritative roadmap so archived changes 060–063 are no
  longer presented as pending and this proposal is the current bounded
  candidate direction.

## Capabilities

### New Capabilities

- `governed-goal-insights`: Define the bounded goal candidate, metadata,
  evidence, review, replay, lifecycle, and non-authoritative activation
  contract.

### Modified Capabilities

- `governed-autonomous-reasoning-insights`: Extend the reasoning candidate
  envelope and validation rules with goal-specific bounded metadata and state
  semantics.
- `governed-reserved-insight-activation`: Add an independently versioned goal
  policy, review handoff, goal state validation, and rollback behavior.
- `governed-experience-insights`: Keep goal records derived, append-only,
  provenance-backed, and excluded from ordinary retrieval/context by default.
- `service-observability`: Add bounded goal operation, review, replay, policy,
  and disposition categories without exposing content, scope values, or IDs.

## Impact

- Affected Go packages: `internal/reasoning`, `internal/insights`,
  `internal/memory`, `internal/telemetry`, and the PostgreSQL repository and
  migration surfaces used by derived insights.
- Affected contracts: reasoning candidate validation, reserved insight policy
  configuration, replay diagnostics, admin review evidence, and redacted
  conformance reporting. Default retrieval and context response shapes remain
  unchanged.
- Affected documentation: the authoritative roadmap and its candidate status
  references.
- No new storage system, provider dependency, SDK, UI, or end-user product
  logic is introduced. PostgreSQL remains the system of record and all derived
  goal state must be rebuildable from durable evidence.
- Related workflow references: use `openspec status --change
  "governed-goal-insights" --json`, `openspec validate --all --strict`, and
  the repository's `openspec-archive-seq.ps1` script when the change is ready
  for completion.

## Non-goals

- Do not enable goal activation or make goals visible in default retrieval or
  ordinary context assembly.
- Do not implement autonomous goal planning, task execution, scheduling, or
  agent final-answer generation.
- Do not overwrite canonical memory, source evidence, or prior derived insight
  versions in place.
- Do not introduce a global agent goal namespace, a graph database, a second
  source of record, or provider-specific authorization semantics.
- Do not add a new adapter, SDK, UI, or user-facing product workflow.
