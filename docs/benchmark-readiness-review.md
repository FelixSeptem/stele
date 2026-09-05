# Benchmark Expansion Readiness Review

Review date: 2026-09-05

Scope: `agent-memory-benchmark-expansion`, including the retained
LongMemEval evidence report and the generic retrieval dataset locks.

## Decision

The change is implementation-complete and suitable for OpenSpec archival. The
benchmark surface is operationally ready for its declared support states. It is
not a claim that every external dataset is runnable: generic MTEB/C-MTEB locks
remain `metadata-only` until their upstream license terms and conversion are
reviewed, and the full 246,738-event LongMemEval `s` PostgreSQL batch runner is
an explicitly recorded follow-up prerequisite.

## Review matrix

| Area | Evidence and control | Outcome |
| --- | --- | --- |
| API | Benchmark functionality is CLI/cache scoped; no public memory API or production schema changed. Family reports carry dataset, split, checksums, qrels, runtime, strategy, scope, metrics, errors, safety outcomes, and artifact paths. | Pass |
| Security | Every import/report derives a fresh benchmark tenant/project/namespace run scope. Generic runs reject production scopes. Cleanup validates run IDs and never targets raw or normalized inputs. | Pass |
| License | LongMemEval is retained in the user-authorized local ModelScope cache. SciFact and BQ locks record ModelScope metadata, per-file SHA-256, and `needs_review`/`restricted` status; no external corpus is committed. | Conditional by design |
| Capacity | LongMemEval local PG18 + pgvector evidence completed for one real qrels case. `s`/`m` capacity planning, batch size, subset limits, and cleanup tests exist. Full `s` ingestion requires a batched runner before a standard full-storage run. | Pass with residual prerequisite |
| Isolation | Scope predicates, run-derived namespaces, cross-run tests, session checks, and generic production-scope rejection are covered. Cleanup tests verify production scopes are untouched. | Pass |
| Lifecycle | Normalization preserves expected active/suppressed/forgotten/deleted state. Retrieval filters inactive state and reports must-not-return violations; update/conflict and abstention mappings are retained. | Pass |
| Operations | Offline is the default, missing artifacts return stable `prerequisite_missing`, checksum drift returns `checksum_mismatch`, and retained reports/manifests survive cleanup. CI uses repository-owned fixtures and blocks network access. | Pass |

## Retained evidence

- Real report: `.tmp/benchmark-data/agent_memory/longmemeval/f180315e/reports/real-s-pg18.json`
- LongMemEval normalized checksum: `35b7d539546dbefa6abd54fd6d743a5e951443064d9f3549041aed6bd2e117b1`
- PostgreSQL `18.6`, pgvector `0.8.6`, selected real queries: `1`, selected events: `396`, must-not-return violations: `0`.
- Generic lock metadata: `internal/benchmark/testdata/generic-retrieval-dataset-locks.json`
- Generic lock documentation: `docs/benchmark-dataset-locks.md`

The real LongMemEval report records lexical Recall@k of zero for its selected
case. This is retained as an observed quality result; execution success is not
represented as a quality pass.

## Follow-up controls

1. Complete an explicit upstream license review before promoting SciFact or BQ
   from `metadata-only` or allowing redistribution.
2. Implement and run the batched full-`s` PostgreSQL path under an explicit
   local capacity budget; do not infer full-subset quality from the retained
   single-case evidence.
3. Keep generic IR and stress reports family-scoped and non-comparable with
   agent-memory quality gates.
