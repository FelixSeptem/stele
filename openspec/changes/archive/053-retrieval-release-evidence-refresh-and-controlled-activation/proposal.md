## Why

Stele already has release-gate and retrieval-evidence contracts, but the
repository's remaining rollout decision still depends on stale or missing
owned PostgreSQL + pgvector evidence. The next step is to make a fresh,
operator-reproducible evidence run the only path to an explicitly scoped
activation, while keeping the stable baseline active whenever evidence is
skipped, stale, incompatible, unsafe, or not fully reversible.

## What Changes

- Add a fresh owned-real-stack release-evidence workflow for the current
  retrieval baseline and the existing progressive-context, parent-first,
  adaptive-planning, and temporal evaluation profiles.
- Require an explicit disposable PostgreSQL + pgvector DSN, ownership marker,
  exact scope, compatible fixture/policy identities, bounded timeout, and
  deterministic fixture cleanup; never reuse the runtime service DSN.
- Extend evidence reports with a stable run identity, source-watermark and
  freshness verdicts, protected-recall and temporal/multi-hop coverage,
  candidate/context/latency budgets, trajectory and memory-integrity summaries,
  and rollback/replay outcomes using only redacted low-cardinality fields.
- Add a controlled activation decision that can authorize only an exact scoped
  rollout after every hard safety, compatibility, freshness, replay, budget,
  and rollback gate passes; diagnostics and shadow results remain ineligible.
- Make activation fail closed on missing or stale evidence, scope or lifecycle
  leakage, semantic-hit absence, nondeterministic replay, budget overflow,
  incompatible identities, or failed rollback, and return to the approved flat
  baseline on disablement or rollback.
- Add focused tests and an operator-facing command for missing-DSN skip,
  real-stack pass/fail, stale/incompatible evidence, exact-scope activation,
  disablement, rollback, deterministic replay, and redaction.
- Calibrate the roadmap and self-hosting status checks so change 052 is marked
  archived and this proposal is the current P8.8 target.

### Non-goals

- No change to ordinary search, context, ingestion, lifecycle, or OpenAPI
  response behavior unless an explicitly authorized exact-scope rollout is
  active.
- No automatic global rollout, default enablement, or provider-specific
  reasoning dependency.
- No new canonical memory class, database, graph store, SDK, UI, hosted
  service, or second authorization boundary.
- No raw query, scope value, memory/event identifier, source content, prompt,
  credential, DSN, score, or provider payload in persisted or emitted evidence.
- No destructive schema downgrade or in-place rewrite of canonical memory;
  rollback disables the experimental policy and preserves append-only history.

## Capabilities

### New Capabilities

None. This change strengthens and operationalizes existing release-evidence and
rollout contracts.

### Modified Capabilities

- `retrieval-release-gate-and-progressive-context-evaluation`: require fresh,
  owned, exact-scope evidence and an explicit, reversible activation decision
  before any experimental retrieval strategy can become active for a scope.
- `retrieval-release-evidence-run`: add the refresh workflow, stable skip/fail
  categories, activation eligibility, redacted run identity, and deterministic
  replay/rollback evidence needed for operator-controlled rollout.

## Impact

- Tests and runtime helpers under `internal/retrieval`, `internal/memory`,
  `internal/storage/postgres`, and related admin/evidence boundaries.
- Operator tooling under `scripts/` and retrieval/self-hosting guidance under
  `docs/`.
- Existing rollout policy and PostgreSQL-derived evidence records may gain
  versioned compatibility and activation metadata; PostgreSQL remains the only
  system of record.
- No migration is required unless the existing evidence/rollout tables need
  forward-only columns for the new run identity or activation verdict.
- Related workflow commands: `openspec validate --all`, the focused retrieval
  and rollout test suites, the owned real-stack evaluation command, and the
  standard OpenSpec apply/archive workflow.
