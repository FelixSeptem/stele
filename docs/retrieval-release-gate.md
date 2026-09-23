# Retrieval release gate

The retrieval release gate is an opt-in evidence workflow. It composes the
repository-owned fixture replay, flat-fusion baseline, query-analysis/reranker
comparison, context projections, and benchmark contracts. Ordinary search and
context responses are unchanged until an explicitly scoped rollout is approved.

## Configuration

Copy `.env.local.example` to `.env.local` and replace placeholders locally. Do
not commit `.env.local` or provider credentials. Real-stack evaluation reads
only:

```text
STELE_TEST_RETRIEVAL_EVALUATION_DSN=<owned PostgreSQL 18 + pgvector DSN>
STELE_TEST_RETRIEVAL_EVALUATION_OWNED=true
STELE_RETRIEVAL_EVALUATION_PROVIDER_PROFILE=canonical-v1
STELE_RETRIEVAL_EVALUATION_REPORT_DIR=<optional local output directory>
```

The ownership marker is mandatory for direct Go harness runs. It is an explicit
assertion that the target is disposable and owned by the evaluation run; the
harness rejects an absent marker and normalized reuse of `STELE_POSTGRES_DSN`
before opening a connection or applying migrations. Each run also uses unique
fixture namespaces and IDs, and cleanup deletes only records returned by that
run's seed operation.

The evaluation DSN must not equal `STELE_POSTGRES_DSN`. If it is absent, the
workflow returns `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` and cannot authorize
an active rollout. Synthetic or skipped runs are evidence for development only,
not release passage.

## Evidence and gates

Run deterministic checks with repository-owned fixtures:

```powershell
$env:GOCACHE = "$PWD/.gocache-release"
go test ./internal/retrieval ./internal/config ./internal/memory ./internal/benchmark -count=1
```

Run the owned PostgreSQL + pgvector replay only when the DSN is configured:

```powershell
pwsh -File scripts/retrieval-evaluation.ps1
```

The release-evidence boundary is `retrieval.RunOwnedReleaseEvidence`. It accepts
one exact scope, an explicitly owned evaluation DSN, and a provider profile;
the runtime service DSN is never consulted as a fallback. Missing DSN or any
PostgreSQL/pgvector, fixture, freshness, or rollback prerequisite yields a
stable skipped/degraded verdict and must not be treated as release-ready.

For CI and operator review, serialize the bounded machine report with
`retrieval.MarshalReleaseEvidenceReport` and use
`retrieval.RenderReleaseEvidenceSummary` for a human-readable annotation. Both
forms contain only logical identities, scope hashes, aggregate counts, and
failure categories; they do not contain connection strings, scope values,
queries, content, raw scores, or provider payloads.

The report records logical fixture, representation, fusion, ranking, embedding,
reranker, analysis, and release-policy identities; protected recall, temporal
and multi-hop coverage, duplicate/diversity, candidate budgets, fallback
categories, safety outcomes, and bounded latency. It never records DSNs,
endpoints, keys, prompts, source text, raw provider payloads, hidden IDs, or raw
scores.

For query-adaptive planning, the same report must additionally record the
planner schema/planner/policy identities and compatible analysis, fusion,
ranking, and renderer identities. Family-level results must cover all seven
bounded query families and protected memory classes. First-pass and optional
second-pass evidence is reported separately, including aggregate disposition,
candidate counts, latency, context-item counts, reranker-headroom use, and
follow-up/fallback categories. A second pass is valid only when it is the one
policy-authorized follow-up and consumes the same request-local ledger.

The owned planner gate must prove a real pgvector-backed semantic hit. The
runner seeds and activates a fixture-owned vector revision, verifies the
production semantic repository returns the expected memory with a positive
semantic score, and verifies the replay attributes that candidate to the
semantic channel. Merely finding the `vector` extension, executing an empty
semantic query, or recovering the expected evidence through lexical recall is
not sufficient release evidence.

Planner activation gates require deterministic plan replay, compatible
dependencies, exact scope, zero lifecycle/isolation leakage, stable citations,
and green candidate, latency, context, and reranker hard envelopes. Planner
eligibility cannot activate a reranker by itself; the separately approved
reranker policy and reserved headroom must be present. Any malformed or
incompatible plan, unavailable optional channel, follow-up failure, or budget
exhaustion must demonstrate a baseline-equivalent fallback and a tested
rollback. Diagnostics and shadow runs must prove ordinary response equivalence.

Graph-distance rollout adds a separate hard gate: the report may include only
redacted hop aggregates, truncation/fallback rates, citation coverage, eligible
relation-hit rate, and latency buckets. Cross-scope traversal, lifecycle or
source-version leakage, untruncated cycles, limit overflow, citation mismatch,
ordinary API topology leakage, and nondeterministic replay are non-pass safety
failures even when graph quality delta is positive. A disabled or rolled-back
exact-scope graph policy must be followed by a baseline-equivalent request.

An absent owned evaluation DSN is a controlled non-pass. It produces
`SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED`, never authorizes `active_for_scope`,
and must not be replaced by the runtime `STELE_POSTGRES_DSN`.

### Latest local gate observation

On 2026-09-23 the repository-owned runner was invoked with an explicitly
owned disposable PostgreSQL 18 + pgvector container through:

```powershell
pwsh -File scripts/retrieval-evaluation.ps1
```

The runner returned exit code `0`. It used only
`STELE_TEST_RETRIEVAL_EVALUATION_DSN` (never `STELE_POSTGRES_DSN`) and retained
redacted baseline, candidate, gate, and RQ4 shadow artifacts. The exact-scope
shadow evidence recorded protected recall and citation coverage of `1`, no
safety failures, bounded context work, deterministic replay, and a tested
rollback. The calibration summary remained unavailable with zero evidence, so
the evaluation explicitly recorded `unavailable_baseline_fallback`; it does
not authorize active calibration.

When the owned DSN is absent, the runner still returns the controlled
non-pass `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` (exit code `2`). No runtime
PostgreSQL DSN is reused, no database is mutated, and no report is eligible.

The following repository verification commands also completed successfully on
2026-09-22 against the bounded graph-distance retrieval worktree:

```text
go test ./internal/retrieval ./internal/storage/postgres -count=1
go test ./... -count=1 -timeout 15m
go test -race ./... -timeout 20m
go vet ./...
openspec validate --all --strict
git diff --check
```

These checks validate the implementation and its deterministic fixtures. The
owned shadow result supplies real-stack evidence, but active calibration still
requires a fresh compatible exact-scope summary and separately governed rollout
authorization.

Progressive context compares short retrieval projection, medium session/context
overview, and canonical/chunk evidence. Each level must have a source watermark,
freshness result, token/character budget, citation coverage, and deterministic
rebuild identity. Parent-first retrieval remains offline or shadow-only and
uses exact-scope bounded expansion.

Isolation, lifecycle visibility, stale projections, missing/altered evidence,
and rollback failures are hard gates; aggregate quality gains cannot offset
them. A release policy is versioned whenever thresholds, protected categories,
budgets, prerequisites, retention, or rollback conditions change.

## Rebuild and rollback

Derived chunks, embeddings, duplicate clusters, projections, and parent-first
artifacts are rebuilt from PostgreSQL source records using their recorded policy
and renderer versions. Never perform a destructive down migration or mutate
canonical memory to roll back. Disable the experimental policy, return to the
last approved flat-fusion strategy, retain append-only history, and rerun the
owned evidence gate after rebuilding.

## Retention and integrity

Trajectory, diagnostics, reports, and fixtures are bounded derived artifacts.
Retention cleanup deletes only expired derived records and preserves canonical
source records. The memory-organization integrity report separately measures
action success and fact/evidence recall, placement accuracy, duplicate, missing,
altered, and unexpected evidence. Hidden or foreign evidence appears only as
aggregate categories and counts.

Maintenance conformance is an operational prerequisite for using fresh context
projections in a release run. It checks exact-scope coverage, lease recovery,
projection freshness/rebuild evidence, retention safety, telemetry redaction,
and evidence completeness. The durable maintenance path remains disabled until
that evidence is green; disabling it is the rollback path and does not mutate
canonical memory.

## Context-efficiency calibration (RQ4)

RQ4 adds bounded, redacted evidence for relevant-token ratio, evidence density,
duplicate/stale token rates, quality per context budget, candidate/context work,
and latency buckets. Reports are comparable only when fixture, renderer,
retrieval-plan, temporal, graph, feedback-summary, calibration-policy, and
release-policy identities match. Exact counts, query text, content, IDs, scope,
raw scores, feedback text, and DSNs are never emitted.

Feedback calibration is disabled by default. A policy may run in diagnostics-only
or shadow mode while returning the approved baseline. Active behavior requires a
fresh exact-scope summary with minimum evidence, confidence threshold, decay,
and contribution caps. Calibration runs only after lifecycle, valid-time,
scope, deduplication, and diversity checks; it cannot add candidates, expand
graph traversal, change citations, create a context section, or exceed the
caller budget. Disablement and rollback return to the baseline on the next
matching request and preserve all source history.

The following are hard failures even when quality-per-budget improves:

- protected recall or evidence-group loss;
- scope, lifecycle, temporal, citation, or ordinary API leakage;
- stale/duplicate overflow, budget overflow, or unbounded work;
- missing or stale calibration summary;
- nondeterministic fixed-clock replay;
- failed rollback or unavailable release evidence.

The deployment limits are configured with `STELE_CONTEXT_CALIBRATION_*` values.
The owned PostgreSQL + pgvector shadow DSN must remain distinct from
`STELE_POSTGRES_DSN`; a missing owned DSN is a controlled non-pass, never an
implicit activation.
