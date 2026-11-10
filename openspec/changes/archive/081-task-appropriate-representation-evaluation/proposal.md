## Why

EV1/EV2 now provide bounded, externally judged longitudinal observations, but
Stele still lacks a controlled way to determine which already-supported
representation is appropriate for a task family.  The current progressive and
parent-first experiments compare generic strategies, but do not establish a
replayable comparison among exact source excerpts, topic projections, and
hierarchical evidence while preserving search as a companion channel.

The roadmap places this work after EV2 evaluation evidence.  It is therefore
time to add only the experimental evidence needed to evaluate representation
selection; it is not time to replace the stable retrieval strategy or activate
a new context policy.

## What Changes

- Add a versioned, repository-owned task-family representation evaluation
  fixture and deterministic offline/owned-stack comparison runner.
- Compare `flat-fusion-v1` with exact-excerpt plus search, topic-projection
  plus search, and bounded hierarchical/parent-first plus search candidates,
  under identical exact-scope source data and declared resource budgets.
- Cover exact terms/numbers, cross-document evidence, profile preference, and
  reusable procedural-lesson task families, with explicit expected evidence,
  exclusions, source watermarks, and representation eligibility.
- Emit only redacted aggregate evidence for critical-fact retention, source and
  citation coverage, broken-reference/stale outcomes, size/budget use,
  maintenance cost, task-utility observations, and deterministic replay.
- Add versioned hard gates so foreign, lifecycle-hidden, stale, broken,
  nondeterministic, or over-budget representation evidence cannot be offset by
  quality gains and cannot authorize activation.
- Keep all candidates offline, diagnostics-only, or shadow-only.  Keep
  ordinary retrieval, context assembly, Provider/MCP surfaces, canonical
  memory, and the `flat-fusion-v1` default unchanged.

## Non-goals

- Do not create a separate authoritative wiki, graph, vector, or document
  store; PostgreSQL remains the sole system of record and pgvector remains the
  semantic index.
- Do not introduce an automatic representation selector, public route, model
  invocation, prompt construction, agent execution, or rollout activation.
- Do not make derived projections canonical or relax tenant/project/namespace,
  temporal, lifecycle, provenance, citation, byte/item, or retention rules.
- Do not claim that an observed condition difference proves a causal task
  benefit.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `progressive-context-hierarchical-retrieval`: require bounded,
  task-family-aware representation comparisons that retain search and stay
  shadow-only.
- `retrieval-evaluation-baseline`: require repository-owned representation
  fixtures and deterministic replay metrics for the declared task families.
- `retrieval-release-gate-and-progressive-context-evaluation`: require
  task-family representation evidence, compatibility, hard safety gates, and
  rollback evidence before any future activation is eligible.

## Impact

- Affected code: existing retrieval experiment/evaluation packages, benchmark
  fixture loading and report serialization, PostgreSQL experiment evidence
  repository where durable owned-stack evidence is required, and focused
  evaluator tests.
- Affected documentation: progressive-context experiment and retrieval release
  gate guides, architecture/components/self-hosting material, and the Stele
  memory skill.
- APIs and dependencies: no public API or dependency addition is planned;
  operator-only evaluation contracts remain internal/offline and PostgreSQL
  backed.
- Related commands/skills: `openspec validate`, `go test`, the existing owned
  PostgreSQL + pgvector evaluation verifier pattern, and
  `skills/stele-memory/SKILL.md`.
