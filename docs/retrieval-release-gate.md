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
STELE_RETRIEVAL_EVALUATION_PROVIDER_PROFILE=canonical-v1
STELE_RETRIEVAL_EVALUATION_REPORT_DIR=<optional local output directory>
```

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

The report records logical fixture, representation, fusion, ranking, embedding,
reranker, analysis, and release-policy identities; protected recall, temporal
and multi-hop coverage, duplicate/diversity, candidate budgets, fallback
categories, safety outcomes, and bounded latency. It never records DSNs,
endpoints, keys, prompts, source text, raw provider payloads, hidden IDs, or raw
scores.

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
