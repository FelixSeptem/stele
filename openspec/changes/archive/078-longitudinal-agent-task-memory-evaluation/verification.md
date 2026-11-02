# Longitudinal Task-Series Verification

Date: 2026-11-02

## Acceptance evidence

The retained aggregate report is
[`docs/evidence/longitudinal-dolphinbench-v81cb6f-live.json`](../../../docs/evidence/longitudinal-dolphinbench-v81cb6f-live.json).
It was produced from a local checksum-locked, non-synthetic DolphinBench-derived
task-series source built from upstream revision
`81cb6f8405b40a9e76089cef650806a80af06ea2`. The raw task text, queries,
retrieval snippets, exact evaluation scope values, database DSN, and credentials
remain local and are not retained in this repository.

The source lock and bounded report record these reproducibility identities:

- `source_checksum`: `883fb1beac28fd508986ab7ba1dacdd78f06dfa7d8423bca12ba170e99dce34e`
- `normalized_checksum`: `4a95a014d1a7d0db10fda9052d34bad200230d96eaaa642ab764d559215071e8`
- `qrels_checksum`: `ab7aece7b86d552a947ad818f5c7b9ef042abc254221067b0b550e1624ed6022`
- `manifest_checksum`: `7d9b877f66db82b698e7a3ad01de92cb69ec1442bff1c75a416e3fda97d98b1c`
- runtime: PostgreSQL 18.6 and pgvector 0.8.6
- strategy: `lexical`; comparison semantics: `observational`

The fresh PostgreSQL + pgvector run used a dedicated owned container and
checksum-locked local bundle. It produced `synthetic_fixture: false`,
`status: success`, `safety_status: passed`, and `cleanup_verified: true`.
Both declared conditions had 20 stages, 20 evaluated stages, 20 evidence
stages, 20 of 20 verified provenance records, provenance coverage `1`, and zero
safety failures. The baseline utility was `0.75`; the comparison utility was
`0.70`, with observed difference `-0.05`. This is a descriptive observation,
not a causal claim about memory.

After the run, direct counts for the benchmark-owned tenant were zero for
`raw_events`, `candidate_memories`, `canonical_memories`, and `memory_versions`.
The retained evidence JSON was semantically compared with the report emitted by
the final rerun and matched exactly.

## Delivered behavior

- A checksum-locked local task-series source declares ordered stages, exact
  scope, external verdicts, condition coverage, qrels and expected evidence.
- The pure evaluator reports descriptive utility, stage coverage, stale/noisy
  retrieved-evidence exposure, verified provenance coverage, and a separate
  hard safety gate. It labels comparisons as `observational`.
- Aggregate reports redact task text, queries, scope values, aliases, memory or
  event identifiers, raw scores, prompts, provider payloads and candidate
  details.
- `benchmark lock-task-series` computes QREL/source locks. `benchmark
  run-task-series` requires a local locked bundle, a PostgreSQL DSN, and an
  explicit report path; it has no download or synthetic fallback.
- The PostgreSQL runner applies the complete migration set before fixture
  seeding, injects citation lookup into retrieval replay for provenance
  verification, and fails a non-synthetic report if cleanup was not verified.

## Fresh verification

The following commands passed from the change worktree on 2026-11-02:

```text
STELE_TEST_TASK_SERIES_POSTGRES_DSN=<owned temporary database>
go test ./internal/benchmark -run "^Test(TaskSeries|EvaluateTaskSeries|MarshalTaskSeriesReport|RunTaskSeriesPostgresWhenConfigured)" -count=1

STELE_POSTGRES_DSN=<owned temporary database>
go run ./cmd/stele benchmark run-task-series --source <local locked bundle> --report <local bounded report> --tenant <benchmark tenant> --project <benchmark project> --namespace <benchmark namespace>

go test ./... -count=1 -timeout 15m
go test -race ./... -count=1 -timeout 20m
go vet ./...
pwsh -NoProfile -File scripts/check-documentation.ps1
pwsh -NoProfile -File scripts/check-self-hosting-smoke-docs.ps1
openspec validate --all --strict
git diff --check
```

The focused PostgreSQL suite passed with the owned PostgreSQL 18 + pgvector
container. The full and race suites passed. `go vet`, documentation checks,
self-hosting smoke-documentation check, and `git diff --check` passed. Strict
OpenSpec validation reported 94 passed and 0 failed items; existing long
requirement notices were informational only.
