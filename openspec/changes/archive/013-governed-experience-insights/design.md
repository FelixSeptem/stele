## Context

Stele already has durable raw events, governed canonical memory, provenance, lifecycle state, summaries, relation-enhanced retrieval, worker/scheduler execution, job history, embedding rebuild failure state, and admin inspection. These records preserve evidence, but the service does not yet derive reusable experience insights such as "this kind of workflow repeatedly fails" or "avoid this operator pattern unless the provider is healthy."

The Stash reference demonstrates value in agent-facing concepts such as failures, goals, hypotheses, causal links, and lessons. Stele should adopt the useful vocabulary cautiously: first by adding a governed derived insight substrate and a concrete `failure_pattern` insight type that can be derived from existing evidence. More speculative inference such as hypotheses, causal links, contradictions, and goals should wait for a reasoning-provider boundary and evaluation tests.

## Goals / Non-Goals

**Goals:**

- Persist derived insight records with explicit scope, type, lifecycle, confidence, evidence citations, derivation metadata, provenance, and audit history.
- Implement `failure_pattern` as the first active insight type using repeated evidence from existing Stele records.
- Allow evidence-backed `lesson` output when it is tied to a failure pattern.
- Run insight derivation asynchronously through worker or scheduler execution.
- Expose admin inspection for derived insight state, evidence, and lifecycle decisions.
- Let context assembly optionally include `known_failures` and `experience_lessons` when requested and budget allows.

**Non-Goals:**

- MCP tools or any MCP-first public contract.
- Autonomous `hypothesis`, `goal`, `contradiction`, or `causal_link` inference.
- A reasoning-provider abstraction.
- Rewriting canonical memory, versions, vector revisions, or provenance in place.
- A global agent-self namespace that bypasses existing scope isolation.
- SDK, UI, hosted-product, or end-user product logic.

## Decisions

### Decision: Store insights as a separate derived record type

Derived insights should live outside canonical memory while retaining canonical-style governance properties: scope, lifecycle, provenance, evidence, confidence, and versions or transition history. This avoids overloading `canonical_memories` with non-canonical conclusions while still making insights inspectable and retrievable.

Alternatives considered:

- Store insights as `procedural` canonical memories: simpler, but it blurs source facts with derived conclusions and risks accidental retrieval without evidence context.
- Store insights only in summaries: cheaper, but summaries are not a lifecycle or audit substrate for repeated operational lessons.
- Store insights only in job execution metadata: too operational and not usable by context assembly or admin inspection as memory-like knowledge.

### Decision: First active type is `failure_pattern`

`failure_pattern` is concrete because Stele already has evidence sources: governance raw event failures, job execution records, embedding rebuild failures, recovery history, procedural memories, summaries, and canonical event-derived memory. A pattern should require repeated evidence in the same authorized scope and should not be produced from one isolated incident unless explicitly allowed by an admin debug path.

Alternatives considered:

- Start with `hypothesis`: valuable but requires reasoning semantics and support/contradiction evaluation that Stele does not yet have.
- Start with `goal`: agent-facing, but ambiguous without a clear actor/objective ownership model.
- Implement all Stash-like insight types together: too broad and likely to weaken Stele's audit-first model.

### Decision: Lessons are evidence-backed projections, not free-form wisdom

A `lesson` should be attached to a failure pattern and cite the same evidence or a curated subset. It can be used in context assembly as "avoid repeating this" guidance, but it should not exist without a source failure pattern.

Alternatives considered:

- Let the derivation job emit arbitrary lessons: faster to demo, but difficult to audit and likely to create hallucinated advice.
- Make lessons manual-only: safer, but loses the value of repeated operational evidence already present in Stele.

### Decision: Derivation is asynchronous and idempotent

Insight derivation should run through the existing worker/scheduler model, never the ingest, mutation, retrieval, or context assembly foreground path. The job should be scope-aware and idempotent by using stable fingerprints derived from insight type, scope, normalized pattern key, and evidence windows.

Alternatives considered:

- Inline derivation during ingest: increases write latency and couples ingestion to inference complexity.
- Derive on every context assembly request: expensive, hard to audit, and makes responses unstable.
- Require an external queue: unnecessary because Stele already has durable worker/scheduler patterns.

### Decision: Context assembly includes insights only when requested

The default context assembly shape should remain stable. Callers can opt into `known_failures` and `experience_lessons`, and the assembler should apply the same scope, lifecycle, citation, and budget rules as other sections.

Alternatives considered:

- Always include insights: risks overloading prompts and leaking operational guidance into contexts that do not need it.
- Expose insights only through admin APIs: safe, but misses the core value of helping agents avoid repeated mistakes.

## Risks / Trade-offs

- **Risk:** Derived insights could feel authoritative even when evidence is weak. -> **Mitigation:** Require confidence, evidence citations, lifecycle state, and admin inspection; keep default context inclusion opt-in.
- **Risk:** Pattern detection could produce duplicate or noisy insights. -> **Mitigation:** Use stable fingerprints, bounded evidence windows, minimum evidence thresholds, and idempotent upserts.
- **Risk:** Lessons could become ungrounded advice. -> **Mitigation:** Lessons must attach to a failure pattern and cite evidence; free-form wisdom generation is out of scope.
- **Risk:** Context assembly could exceed budget. -> **Mitigation:** Add insight sections after higher-priority profile/recent/relevant summary sections unless the caller explicitly prioritizes insights.
- **Risk:** Future hypothesis or causal inference could be constrained by the first schema. -> **Mitigation:** Keep type-specific payloads extensible, but only activate `failure_pattern` and evidence-backed `lesson` now.

## Migration Plan

1. Add derived insight domain types and PostgreSQL schema without changing existing retrieval defaults.
2. Add repository reads/writes for insight records, evidence links, lifecycle transitions, and audit history.
3. Add a scope-aware derivation job that identifies repeated failure evidence and creates or updates failure patterns idempotently.
4. Add admin inspection endpoints for listing and reading derived insights with evidence context.
5. Add optional context assembly sections for `known_failures` and `experience_lessons`.
6. Document self-hosting and operator workflows.

Rollback is straightforward because derived insights are additive and do not mutate canonical memory. If needed, disable the derivation job and hide insight sections while preserving stored records for audit.

## Open Questions

- Should first-slice failure evidence require at least two independent source records, or should the threshold be configurable per scope?
- Should lessons be generated deterministically from templates first, with LLM-assisted wording deferred until a reasoning-provider boundary exists?
- Should derived insights share the existing memory lifecycle enum exactly, or use a narrower lifecycle such as `candidate`, `active`, `suppressed`, and `deleted`?
