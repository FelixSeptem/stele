## Why

EV1–EV4 provide bounded correction, representation and resident evaluators.
The existing longitudinal runner seeds a whole condition before any stage, so
it cannot establish held-out evidence visibility. It also lacks experiment
identity locks and independent model/tool observations required by EV5.

## What Changes

- Extend the existing task-series contract with opt-in v3 and five explicit
  conditions: no memory, authorized source snapshots, organized snapshots,
  verified/corrected snapshots, and a small resident snapshot.
- Lock model/runtime/criteria identities, seeds, equal independent budgets,
  source provenance, ordered stages and evidence availability/withdrawal.
- Replay each stage against only its eligible snapshot in an owned PostgreSQL
  scope, enforce disclosure/lifecycle/scope, and verify a baseline replay.
- Separately report external utility, repeated error, maintenance, tool calls
  and model tokens; measure service retrieval latency independently. Missing
  evidence stays inconclusive. Every difference remains observational.
- Correct v2 coverage denominators to use non-superseded stages, including
  inconclusive utility stages, so independently supplied observation coverage
  cannot exceed one. Existing v1/v2 defaults and fixture locks remain valid.

## Non-goals

No agent/model/tool execution, answer generation, LLM judging, public raw-record
access, runtime retrieval activation, new store, schema migration or dependency.
Externally prepared condition snapshots are not proof that EV1–EV4 produced
them. Synthetic conformance is not longitudinal task benefit or rollout authority.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `longitudinal-agent-task-memory-evaluation`: locked v3 staged replay,
  independent cost coverage, leakage prevention and separate hard gates.

## Impact

`internal/benchmark` task-series types, validation, aggregation and PostgreSQL
runner; CLI compatibility tests; locked synthetic fixture; owned Docker verifier;
README, benchmark/self-hosting guides, agent skill and roadmaps. No OpenAPI
shape or authorization change. Use `openspec-apply-change`, Go tests and
`scripts/openspec-archive-seq.ps1` for implementation and delivery.
