# Evidence-Verified Memory Maintenance Roadmap

## Status and purpose

Research captured on 2026-11-08; delivery status reconciled on 2026-11-11.
This document records the research, implementation sequence and remaining
evidence requirements. The status table distinguishes archived service work
from candidate follow-ups; an archived evaluator does not authorize rollout
or establish general task benefit.

The reference is Anthropic's [The components of agent memory](https://www.anthropic.com/research/components-of-agent-memory),
published 2026-11-06. This roadmap extends the
[Stele v1 reference-informed backlog](2026-05-28-stele-v1-roadmap.md#reference-informed-expansion-backlog).
It prioritizes evidence verification and correction over adding another memory
store or rebuilding existing reflection, projection, and evaluation foundations.

The central question is whether Stele can correct a memory that previously
misled an agent, preserve its evidence and history, and demonstrate fewer
subsequent errors under a controlled evaluation.

## Research conclusions and limits

- In-session capture and background maintenance serve complementary purposes.
  Background work can compare notes with source records, identify drift, and
  reorganize derived representations while keeping ingestion inexpensive.
- Useful learning includes testing an explanation and retaining the basis for
  a rule. Recording a failure, attaching citations, or assigning confidence
  does not by itself validate a proposed remedy.
- Transcripts establish what was recorded, not necessarily what is true.
  Model speculation, tool observations, explicit user corrections, and external
  evaluation results need distinguishable evidence attribution.
- Representations should fit the task. In the article's internal-document
  experiment, search alone scored 53%, search plus a growing wiki rose from
  54% to 61%, and wiki alone peaked at 48%. Cross-document tasks benefited,
  while some single-document tasks showed no clear gain; one wiki grew larger
  than its sources. These are task-specific results, not a Stele performance
  forecast or a universal case for wiki construction.
- Always-loaded memory needs a small budget and a way to demote stale or
  unhelpful entries. Reading frequency is a maintenance signal, not proof of
  factual correctness.
- Capturing only failures can miss successful strategies and stable constraints.
  The article's model comparisons motivate experiments, but do not isolate
  memory design from differences in model capability.
- Concurrent edits to shared summaries can lose facts. Source version checks,
  bounded ownership of derivation work, and append-only publication are useful
  protections; summary compression must not erase exact terms or numeric facts.

## Existing foundations to reuse

| Area | Repository basis | Boundary for follow-up |
| --- | --- | --- |
| Reflection and governance | [Reflection/intents guide](../governed-memory-intents-reflection.md), [run contract](../../internal/memory/reflection_run.go), [worker executor boundary](../../internal/jobs/reflection_worker.go) | Run bookkeeping, leases, replay, and review are foundations; their existence does not demonstrate semantic correction or future-task benefit. |
| Experience and reasoning | [Failure-pattern evaluator](../../internal/insights/evaluator.go), [insight domain](../../internal/memory/derived_insight.go), [reasoning boundary](../../openspec/specs/reasoning-provider-boundary/spec.md) | Repeated failures support a failure pattern; they do not establish that a remedy works. Optional reasoning produces governed candidates, without direct canonical authority. |
| Corrections and versions | [Provider correction handoff](../provider-evidence-projections.md#correction-to-version-handoff) | Reuse negotiated correction attribution, expected-version checks, intents, and successor versions where applicable; do not assume every maintenance candidate satisfies this Provider-specific contract. |
| Context hierarchy | [Context projections](../context-projections.md), [progressive experiments](../retrieval-progressive-context-experiments.md) | Projection consumption is opt-in and disabled by default. L0/L1/L2 and parent-first experiments remain offline/shadow; ordinary retrieval retains `flat-fusion-v1`. |
| Context placement | [Projection policy](../../internal/memory/context_projection_policy.go), [Provider contract](../agent-runtime-memory-provider.md) | Reuse profile confidence/size gates and existing independent item, content-byte, body-byte, and citation limits. Client model-token budgeting remains external. |
| Feedback and evaluation | [Quality findings and repair](../../internal/memory/quality_admission_repair.go), [longitudinal evaluation](../local-agent-memory-benchmark.md#longitudinal-task-series-evaluation) | Reuse feedback and externally judged task outcomes. Existing condition differences are observational, not causal proof. |

## Candidate sequence

The identifiers below belong to this research roadmap; they are not OpenSpec
change numbers or new global priorities. Reconcile the current implementation
and archives before opening each proposal to avoid duplicating delivered work.

| Track | Priority and dependency | Proposed outcome | Current status |
| --- | --- | --- | --- |
| EV1: Verification evidence | First; EV5 fixtures begin here | Distinguish claim verification from provenance, confidence, and lifecycle | Delivered bounded slice in [080](../../openspec/changes/archive/080-verified-memory-correction-loop/proposal.md); administrator maintenance only |
| EV2: Targeted correction loop | After EV1, evaluated with EV5 | Governed, bounded revalidation and versioned correction | Delivered bounded slice in [080](../../openspec/changes/archive/080-verified-memory-correction-loop/proposal.md); unresolved evidence and governed successor versions remain explicit |
| EV3: Task-appropriate representations | After EV2 has evaluation evidence | Compare exact excerpts, topic projections, and hierarchical retrieval | Delivered evaluator in [081](../../openspec/changes/archive/081-task-appropriate-representation-evaluation/proposal.md), with [verification](../../openspec/changes/archive/081-task-appropriate-representation-evaluation/verification.md); candidates remain shadow-only |
| EV4: Small resident context | After EV1; prioritize after EV2/EV3 evidence | Evidence-based promotion and demotion within existing context budgets | Delivered bounded offline/shadow evaluator in [082](../../openspec/changes/archive/082-evidence-gated-resident-context-evaluation/proposal.md), with [verification](../../openspec/changes/archive/082-evidence-gated-resident-context-evaluation/verification.md) and focused independent follow-up review; existing defaults remain authoritative; no task-benefit claim |
| EV5: Longitudinal benefit evaluation | Starts with EV1 and accompanies every track | Measure future utility, repeated errors, harm, and cost | Delivered locked offline evaluator in [083](../../openspec/changes/archive/083-locked-longitudinal-memory-evaluation/proposal.md); externally judged task benefit remains unmeasured |

### EV1: Verification evidence for procedural memories and lessons

Start with a narrow domain of reusable procedures and lessons. Assess reuse of
existing evidence, review, and insight contracts before introducing fields.
Keep verification separate from the five memory classes and the existing
event/candidate/active/suppressed/forgotten/deleted lifecycle.

Candidate evidence includes the claim and applicability conditions, supporting
and contrary observations, verification method and actor, source versions,
verification time, and revalidation conditions. Possible dispositions include
untested, supported, refuted, and needs-revalidation; exact names and schemas
must be decided in the proposal rather than treated as an existing API.

Acceptance evidence:

- A repeated failure can support a failure pattern without verifying a remedy.
- A proposed remedy becomes supported only with an attributable check and its
  applicability conditions; inconclusive evidence remains explicit.
- Model-generated text and repeated self-citations cannot silently become
  independent corroboration. Source trust and claim verification remain distinct.
- Source changes invalidate applicable verification evidence without erasing
  history or automatically changing lifecycle or authorization.
- Exact scope, source eligibility, and lifecycle restrictions hold for both
  supporting and contrary evidence.

### EV2: Targeted background revalidation and correction

Use existing reflection, jobs, quality findings, and governed mutation paths.
Candidate triggers include explicit correction feedback, source-version
changes, repeated related failures, and stale-memory feedback. Reading frequency
and expected volatility may prioritize work under bounded per-scope budgets;
neither may establish truth or unconditionally promote a memory.

The intended sequence is eligible evidence selection, a bounded verification
attempt, correction candidate or insufficient-evidence result, ordinary review
and governance, successor version publication, dependent projection invalidation
or rebuild, and subsequent-task evaluation. Check source versions again before
publication to prevent concurrent work from overwriting newer evidence.

Acceptance evidence:

- An intentionally incorrect memory is corrected through a governed successor
  version, retaining the original record and the reason for correction.
- Stale, hidden, forgotten, or foreign sources cannot become visible through
  a correction, derived summary, or background revalidation operation.
- Interrupted work resumes or retries within limits without duplicate effects;
  concurrent corrections either reconcile under explicit policy or conflict.
- Insufficient evidence and provider failure preserve an auditable unresolved
  result; no canonical rewrite or fabricated verification occurs.
- Fresh PostgreSQL + pgvector evidence covers API/worker restart, version
  conflicts, dependent projections, and the existing correction contract where
  used. Disabling maintenance stops new work while retaining durable history.

Verification may use externally supplied observations or the existing optional
reasoning boundary. Stele does not assume ownership of an agent's task loop,
business success criteria, or tool execution.

### EV3: Representations selected for the task

The implementation supplies a source-locked four-family replay with actual flat
baseline, repeated candidate and baseline rollback conditions. It measures exact
fact values, versions, citations, valid time, combined search/representation
budgets and separate external observations; hashes bind actual evidence as well
as aggregates. The owned PostgreSQL + pgvector harness checks materialized
projections, production retrieval, source-version drift, restart persistence and
scoped expiry cleanup. This is a fixed synthetic service-conformance corpus;
broader task utility and maintenance benefit still require external observations.

Extend the existing progressive-context experiments with task families: exact
terms and numbers use source excerpts; cross-document questions use topic
projections and bounded relations; stable preferences use profile projections;
reusable procedures use evidence-backed lessons. Temporary task state remains
application-owned, with bounded context or durable references where supported.

Keep search available alongside derived representations. Store indexes,
summaries, and navigation edges as versioned PostgreSQL projections with source
references, watermarks, and rebuild identities. No separate authoritative wiki
or graph store is introduced.

Acceptance evidence:

- Compare flat retrieval, hierarchical retrieval, and representations plus
  search on both single-document and cross-document tasks.
- Measure critical-fact retention, citation/source coverage, broken links,
  representation size, maintenance cost, and task utility. Compression ratio
  alone cannot justify losing exact terms or numeric facts.
- Changed sources, concurrent derivations, and broken references produce
  explicit stale/conflict/repair outcomes without lost canonical history.
- Ordinary behavior changes only after the existing retrieval release gates
  and separately authorized activation pass; retain a tested baseline rollback.

### EV4: Promotion and demotion of resident context

The archived bounded evaluator evaluates existing profile materialization against
a candidate with exact source-version/content support and later held-out
utility. Default limits are independently 8 items, 4096 UTF-8 content bytes,
1024 whitespace tokens and 16 citations in each condition. Popularity has no
influence. Omission/demotion retains source history and on-demand access;
disablement returns the baseline plan. Reports bind actual source/output
hashes and keep external outcomes separate; missing outcomes stay inconclusive.
The owned PostgreSQL/pgvector verifier targets synthetic service conformance,
source drift, persistence and restart. There is no live placement capability,
public contract change or migration; broader utility and rollout remain future
evidence requirements. The owned-stack run is synthetic conformance only and
external outcomes remain inconclusive. A focused independent follow-up review
confirmed the three evidence-boundary fixes after the initial delivery. See [archive 082](../../openspec/changes/archive/082-evidence-gated-resident-context-evaluation/verification.md)
and [context projections](../context-projections.md#evidence-gated-resident-context-evaluation).

Evaluate stability, verification, applicability, and demonstrated utility as
promotion signals within existing projection policies. Keep the resident layer
small; retain detailed material on demand and use concise navigation references
where useful. Corrections, expiration, or declining usefulness can motivate
demotion without deleting canonical memory.

Acceptance evidence:

- A frequently accessed but unverified rule cannot gain authority through
  popularity alone. Resident data never becomes an instruction or tool grant.
- Promotion and demotion preserve source lineage, disclosure, citations, and
  independent Provider byte/item/citation budgets.
- Compare task utility and stale-memory harm against the existing placement
  policy under equal budgets; document a reversible policy rollout.
- Do not copy a vendor's line-count limit or assume server bytes equal model
  tokens. Any new public placement capability requires explicit contract work.

### EV5: Longitudinal utility and harm evaluation

Extend the existing task-series evaluator and retain externally supplied
verdicts. Establish fixtures before EV1/EV2 implementation. Compare explicitly
declared conditions: no memory, authorized source-record retrieval, organized
memory plus retrieval, verified/corrected memory plus retrieval, and a small
resident index. The source-record condition belongs to an authorized evaluation
harness; it must not add raw-transcript access to public Provider credentials.

Include successful strategies, explicit corrections, stable constraints,
repeated mistakes, changing facts, and concurrent updates. Avoid a policy that
permits only failure memories or indiscriminately promotes transient state.

Acceptance evidence:

- Lock task order, source provenance, model/runtime versions, evaluation
  criteria, seeds where applicable, and comparable budgets. Use held-out later
  stages so future answers do not leak into earlier verification inputs.
- Report task utility, repeated-error rate, returned stale/noisy evidence harm,
  provenance coverage, and service maintenance cost. Record externally reported
  token/tool costs separately; Stele does not infer model usage or task success.
- Missing verdicts and incomplete evidence stay inconclusive. Observational
  condition differences remain labeled as such; causal claims need an explicitly
  controlled experiment and adequate repeated measurements.
- Isolation, lifecycle, disclosure, latency, and rollback failures remain
  independent release blockers regardless of utility gains. Generated answers
  or LLM judges do not replace deterministic service safety checks.

The delivered v3 evaluator locks five conditions, stage-visible source
  windows, independent observations and separate hard gates behind an opt-in
  internal schema. It replays against owned PostgreSQL scopes and keeps
  synthetic outcomes inconclusive. It is service-conformance evidence, not
  measured future-task benefit, and it authorizes no retrieval activation.

## Proposal and maintenance gates

The recommended first proposal is a bounded EV1 + EV2 slice with EV5 fixtures:
one procedural lesson, an observed error, attributable verification, a governed
correction, and a later task that can reveal whether the same error recurs.
Set quantitative utility, latency, and cost criteria before running the
experiment; the article's results do not supply Stele acceptance thresholds.

Each proposal must identify the existing contracts it reuses, the concrete gap,
an owner, policy/schema versions, resource bounds, evidence requirements, and
stop/rollback behavior. Follow the current branch and OpenSpec lifecycle in the
main roadmap. An archived infrastructure change is not proof of semantic
verification quality or authorization to activate experimental retrieval.

Preserve PostgreSQL as the sole system of record, exact tenant/project/namespace
scope in every API/query/job, append-only canonical versions, provenance and
audit, lifecycle-safe default retrieval, and service-only ownership. Adopt
mature Go libraries where useful after checking pkg.go.dev for any new core
infrastructure dependency.

Any later externally visible semantic change must update affected OpenAPI,
README, integration guides, agent skills, and maintenance documentation in the
same change, covering shapes, validation/errors, authorization, defaults,
compatibility, and migration. Include affected ingestion, consolidation,
retrieval, forgetting, and isolation tests, plus owned-stack evidence where
required. This research document itself introduces no runtime or API change.
