# Retrieval Release Gate and Progressive Context Evaluation

## Decision summary

Stele will add a composition layer around the existing retrieval replay,
comparison, projection, benchmark, rollout, and observability contracts. The
layer will require an explicitly owned PostgreSQL + pgvector evaluation DSN for
real-provider evidence, keep progressive-context and parent-first strategies
offline or shadow-only, and make isolation, lifecycle visibility, information
integrity, freshness, latency, and rollback hard release gates.

## Scope

- Real-provider canonical-v1/baseline-v1 evidence with logical provider identity
  and strict redaction.
- Three progressive context levels with watermarks, freshness, budgets,
  citations, and deterministic rebuild evidence.
- Reversible parent-first retrieval comparison with exact-scope expansion.
- Bounded redacted retrieval trajectories and deterministic retention cleanup.
- Memory-organization integrity reporting separate from action success.
- Versioned release policy, checklist, rebuild/re-index runbook, and rollback
  runbook.

## Explicit exclusions

This design does not introduce a second canonical store, change default public
retrieval behavior, require remote model services or LLM judging, or implement
the unrelated portions of the P6 maintenance follow-up.

The complete OpenSpec proposal, behavior contract, implementation tasks, and
acceptance scenarios are maintained in:

`openspec/changes/retrieval-release-gate-and-progressive-context-evaluation/`
