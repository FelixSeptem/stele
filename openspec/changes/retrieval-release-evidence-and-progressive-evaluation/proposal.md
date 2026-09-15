## Why

The archived P5 release-gate capability defines the evidence contract, but the
roadmap still requires an explicitly owned PostgreSQL 18 + pgvector run before
retrieval changes can be considered release-ready. Progressive context levels
and parent-first retrieval also need to be exercised against the same evidence
without changing default production ranking. This change closes that evidence
loop with one reproducible, fail-closed evaluation run.

## What Changes

- Add a release-evidence run that executes only with an explicitly configured,
  project-owned evaluation DSN and provider profile; never fall back to the
  service DSN or ambient production credentials.
- Compare `canonical-v1` / `baseline-v1` and compatible candidate identities,
  recording bounded quality, latency, resource, freshness, isolation, lifecycle,
  and rollback outcomes.
- Execute three progressive context levels (short retrieval projection,
  medium session/context overview, canonical or chunk evidence) against one
  exact scope, preserving source watermarks, citation coverage, budgets, and
  deterministic rebuild identities.
- Execute parent-first retrieval only in offline or shadow mode with bounded
  exact-scope child/adjacent expansion and a reversible comparison to flat
  fusion.
- Produce one redacted report and machine-readable verdict in which safety,
  lifecycle, freshness, scope, and rollback failures override aggregate quality
  gains.
- Make missing DSN, unavailable PostgreSQL/pgvector, incompatible fixtures, or
  stale evidence return stable skipped/degraded/non-pass outcomes; CI may
  continue, but no such run can claim release readiness.
- Extend operator documentation and CI/release entrypoints while retaining
  existing trajectory, integrity, retention, and canonical-source boundaries.

## Capabilities

### New Capabilities

- `retrieval-release-evidence-run`: Owned real-provider release evidence,
  progressive context comparison, parent-first shadow evaluation, bounded
  verdicts, and operator-facing release readiness.

### Modified Capabilities

- `retrieval-release-gate-and-progressive-context-evaluation`: Clarify that a
  release gate may only become eligible after the explicitly owned real-stack
  evidence run and that progressive/parent-first results remain shadow-only.

## Impact

- Affected code includes retrieval evaluation orchestration, context projection
  comparison, benchmark/replay entrypoints, release reports, CI scripts, and
  operator documentation.
- PostgreSQL remains the only system of record; evaluation artifacts remain
  derived, bounded, redacted, and rebuildable.
- No default retrieval or public search/context response shape changes.
- No new model, SDK, MCP, UI, SSE/WebSocket replay, or agent-execution
  dependency is introduced.
- Related workflow commands: `openspec validate --strict`,
  `openspec instructions apply`, and the repository's release verification
  scripts.

## Non-goals

- No automatic promotion of an experimental strategy into default retrieval.
- No fallback from an evaluation DSN to `STELE_POSTGRES_DSN` or another service
  database.
- No mandatory remote provider, LLM judge, RAGAS score, benchmark download, or
  network dependency for ordinary contributor tests.
- No completion of the broader P6 maintenance/observability roadmap item.
- No canonical-memory rewrite, second persistence system, or raw trajectory,
  query, credential, prompt, scope, or provider-payload export.
