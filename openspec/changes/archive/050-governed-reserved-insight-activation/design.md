## Context

The proposal builds on the archived reasoning-provider boundary and
OpenAI-compatible adapter. Those changes deliberately stop at validated,
non-authoritative candidates; the existing governed-insight contract also
reserves `hypothesis`, `goal`, `contradiction`, and `causal_link`. See
`proposal.md` for motivation and the delta specs for the externally visible
behavior.

The implementation must continue to use PostgreSQL as the only system of
record, preserve exact tenant/project/namespace isolation, keep derived
insights separate from canonical memory, and remain disabled by default.

## Goals / Non-Goals

**Goals:**

- Introduce one reusable admission path for reserved insight candidates.
- Make policy, evidence, provenance, lifecycle, replay, and rollback explicit
  and auditable.
- Limit the first active type to reviewed `hypothesis` candidates while
  leaving higher-risk types policy-disabled.
- Keep offline replay and shadow evaluation deterministic and non-authoritative.
- Expose bounded operator diagnostics without leaking prompts, raw provider
  payloads, credentials, hidden IDs, or foreign scope values.

**Non-Goals:**

- Autonomous activation of every reserved type.
- Direct model-to-canonical-memory writes or changes to ordinary retrieval
  ranking/context behavior.
- A new provider SDK, graph store, UI, or independent profile/insight store.

## Decisions

### 1. Add a policy-mediated candidate admission stage

The flow is:

```text
provider adapter
      │ validated candidate
      ▼
scope + evidence + provenance validation
      ▼
activation policy resolution (exact scope, type, versions, freshness)
      ├── disabled/stale/incompatible → reject or quarantine
      ▼
type-specific admission checks
      ├── replay/shadow → record would-disposition only
      └── governed apply → append derived insight version + audit
```

The provider boundary remains incapable of setting an active lifecycle state.
The activation stage owns the decision and calls the existing derived-insight
governance path, so lifecycle transitions, feedback, and audit semantics remain
consistent with `failure_pattern` and `lesson`.

Alternative considered: let the provider return an already-active insight.
Rejected because it would make provider behavior an authority, bypass review
and policy checks, and make rollback/replay non-deterministic.

### 2. Store activation policy and decisions as append-only, scope-bound records

Policies and admission decisions are identified by exact scope, policy version,
provider contract version, source watermark, candidate fingerprint, and
idempotency identity. Updates create a new policy/decision version rather than
mutating the prior decision in place. A rollback marks the policy unavailable
for new admission and leaves historical decisions inspectable.

The design reuses existing policy, provenance, lifecycle, and audit concepts
where possible. Any new persistence records remain rebuildable from PostgreSQL
and contain bounded metadata only; prompt text, chain-of-thought, raw model
payloads, credentials, and hidden identifiers are never stored.

Alternative considered: keep policy in process configuration only. Rejected
because restart, replay, multi-scope operation, and audit would lose the exact
policy used for a decision.

### 3. Sequence type enablement conservatively

The activation policy supports all four reserved vocabulary values, but the
initial shipped policy schema and conformance profile enable only reviewed
`hypothesis` activation. `goal` remains disabled because it could alter
context priorities; `contradiction` requires explicit alignment with temporal
fact and evidence rules; `causal_link` requires stronger causal evidence and
evaluation. Each later type must add its own evidence and quality gate rather
than inheriting hypothesis thresholds implicitly.

Alternative considered: enable all four behind one global feature flag.
Rejected because the types have materially different safety and evidence
requirements and would make rollback too coarse.

### 4. Make replay and shadow decision-only

Replay normalizes candidate, evidence watermark, scope proof, policy version,
and provider contract before evaluating admission. It emits categorized
`reject`, `quarantine`, `would_activate`, or `incompatible` results. Shadow
execution can run the same evaluator against live candidates, but neither mode
creates active insights or changes default retrieval/context. A stale policy,
source watermark, or provider compatibility record produces `stale` or
`incomplete`, never an activation claim.

Alternative considered: allow replay to apply accepted decisions immediately.
Rejected because replay is used for evidence and recovery; applying from a
stale or differently configured policy would make historical runs destructive.

### 5. Keep diagnostics aggregate and authorization-gated

Operator inspection returns policy/type/version identifiers, counts, stable
reason categories, freshness, and bounded evidence references. It is exposed
only through existing authorized diagnostic/admin surfaces. Responses never
include raw candidate text, prompts, hidden record IDs, foreign scope values,
raw scores, or provider payloads.

## Risks / Trade-offs

- **[Risk]** A permissive hypothesis policy could surface low-quality derived
  insights. **Mitigation:** default-disabled policy, reviewed first rollout,
  evidence subset checks, confidence/uncertainty bounds, shadow evidence, and
  explicit suppression/rollback.
- **[Risk]** Policy version drift can make replay disagree with historical
  decisions. **Mitigation:** persist policy/provider/source-watermark versions
  with every decision and classify stale or incompatible replay explicitly.
- **[Risk]** New activation records could become a second memory authority.
  **Mitigation:** keep them derived and evidence-linked; canonical memory is
  never rewritten and ordinary retrieval/context remains unchanged by default.
- **[Risk]** Cross-scope evidence could leak through diagnostics. **Mitigation:**
  resolve exact scope before evaluation, revalidate every citation, and expose
  only bounded aggregates on authorized surfaces.
- **[Risk]** A retry could duplicate an insight or lifecycle transition.
  **Mitigation:** candidate fingerprint plus idempotency identity and append-only
  decision records make retries return the original disposition.

## Migration Plan

1. Ship policy and decision contracts with all reserved types disabled; existing
   reasoning, replay, retrieval, and context behavior remains unchanged.
2. Enable offline replay/conformance fixtures for `hypothesis` only and require
   exact scope, evidence, provenance, and freshness evidence.
3. Enable shadow evaluation and operator inspection without active mutation.
4. Activate reviewed `hypothesis` policy for explicitly selected scopes only;
   preserve a stop switch and rollback path.
5. Keep `goal`, `contradiction`, and `causal_link` disabled until separate
   type-specific evidence is added. Any future enablement must not reuse this
   proposal's acceptance claim automatically.

Rollback disables the affected policy and stops new admissions. Existing
derived insights transition through normal governed suppression/expiry paths;
no database downgrade or canonical-memory rewrite is required.

## Open Questions

None that change the contract or selected architecture. Per-type thresholds
and operator UI wording can be tuned during implementation as long as they
remain within the versioned policy and bounded-diagnostics requirements.
