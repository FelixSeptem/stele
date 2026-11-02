## Why

Stele already records append-only Provider projection corrections and supports
asynchronous governed `update` intents, but an intent's generic evidence list
does not prove that it names an existing, current correction revision owned by
the bound runtime. A client can therefore describe a correction without the
service being able to attest its relationship to the affected scoped memory
lineage.

The MemOS-informed roadmap item requires a narrow, end-to-end conformance
contract: an external correction must be attributable to governed source
evidence, enter an authorized mutation path, append a successor version, and
leave superseded signals out of active summaries. This change joins existing
contracts without making Provider evidence a direct canonical write authority.

## What Changes

- Add a versioned Provider correction-attribution capability for an explicit
  correction reference on an existing `update` intent. It is opt-in and does
  not alter ordinary native or Provider intent compatibility.
- Resolve the reference server-side against an exact-scope, runtime-binding
  owned, corrected Provider projection revision. Validate the current durable
  reference lineage against the update target and expected version before any
  idempotency claim or durable handoff.
- Persist the verified correction-to-intent association as bounded immutable
  lineage. The worker continues to create candidates and append canonical
  successor versions through the existing governed path; neither projection
  admission nor intent admission claims activation inline.
- Make replay, stale correction, binding mismatch, foreign/missing reference,
  hidden evidence, target-version conflict, restart, and superseded-feedback
  outcomes deterministic and non-disclosing.
- Publish additive OpenAPI/capability shapes, operator guidance, Provider
  integration guidance, agent-skill rules, focused unit/contract tests, and a
  fresh PostgreSQL + pgvector initial/restart conformance fixture.

## Non-goals

- No direct mutation, lifecycle activation, task completion, lease release,
  generation commit, artifact publication, ranking activation, or insight
  activation from a Provider projection.
- No caller-controlled scope, binding, grant, role, target lineage, source
  revision, or disclosure override; no cross-binding or cross-scope correction
  reuse.
- No new scheduler abstraction or queue framework beyond Riverqueue. Riverqueue
  is limited to governed memory-intent execution; other derived-work kinds stay
  on the existing queue. No new memory class, canonical store, SDK, UI, agent
  execution path, or local fallback ledger.
- No resurrection of a superseded projection, feedback signal, hidden source,
  or stale canonical version merely to satisfy correction attribution.

## Capabilities

### New Capabilities

- `provider-correction-governed-version-contract`: exact-scope, binding-owned,
  replay-safe attribution from a corrected Provider projection to an
  asynchronously governed canonical successor version.

### Modified Capabilities

- `provider-turn-outcome-feedback-projection-contract`: corrected projection
  revisions become a bounded, server-resolved correction source without
  granting direct mutation authority.
- `governed-memory-intents`: correction-attributed updates validate their
  source lineage before replay/handoff and preserve that immutable association
  in intent history.

## Impact

- API/OpenAPI: additive Provider capability and a dedicated closed correction
  operation, with bounded errors and backward compatibility for callers that
  do not negotiate the capability.
- Storage/runtime: add Riverqueue v0.44.1 with its matching pgx v5 driver (the
  newest inspected release retaining the repository's Go 1.25 baseline), an
  additive PostgreSQL correction-attribution ledger, and an exact-scope
  resolver. Persisting an accepted memory intent and inserting its River job
  MUST share one PostgreSQL transaction. The memory-intent River worker MUST
  commit governance effects, intent transition, legacy work status/checkpoint
  where applicable, and `JobCompleteTx` together; a rollback leaves the
  accepted intent intact and lets River retry. `memory_intents` remains
  authoritative and River job arguments contain only the intent identifier.
  Other derived-work kinds continue to use the existing queue.
- Verification: unit, API contract, repository, authorization/isolation,
  replay/stale, lifecycle, active-summary exclusion, PostgreSQL/pgvector,
  River enqueue/complete rollback, legacy queued-intent handoff, API/worker
  restart, and redacted evidence coverage.
- Documentation: update OpenAPI, provider projection maintenance, runtime
  integration, architecture, components, self-hosting, best practices, and
  `skills/stele-memory/SKILL.md` in the same change.

## References

- [Provider projection contract](../../specs/provider-turn-outcome-feedback-projection-contract/spec.md)
- [Governed memory intents](../../specs/governed-memory-intents/spec.md)
- [Canonical lifecycle](../../specs/canonical-memory-lifecycle/spec.md)
- [Provider projection maintenance](../../../docs/provider-evidence-projections.md)
- `openspec validate --all --strict`
- `scripts/stele-provider-context-verify.ps1`
