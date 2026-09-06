# Benchmark Expansion Readiness Review

Review date: 2026-09-03

## Evidence

- `go test ./... -count=1 -timeout 15m`: passed.
- PostgreSQL integration tests for repeated import, cross-run isolation,
  lifecycle exclusion, and auditable runtime identity: passed against
  PostgreSQL 18.6 with pgvector 0.8.6.
- `openspec validate agent-memory-benchmark-expansion --strict`: passed.
- `openspec validate --specs --strict`: 46 passed, 0 failed.
- Real LongMemEval `s` report retained at
  `.tmp/benchmark-expansion/longmemeval/modelscope-cleaned-f180315e-s/reports/longmemeval-s/retrieval.json`.

The LongMemEval report is checksum locked, non-synthetic, and records the
normalized corpus checksum, qrels checksum, runtime identity, strategy, scope,
metrics, safety outcomes, and artifact path. Its PostgreSQL benchmark scope was
cleaned after replay.

## Safety and isolation review

- PostgreSQL remains the only system of record; benchmark code does not alter
  production memory APIs or lifecycle semantics.
- Every benchmark import and query uses benchmark-owned tenant/project/namespace
  scopes. Generic strategy comparison rejects production scopes and incompatible
  corpus/qrels/embedding/normalization identities.
- Retrieval excludes suppressed, forgotten, and deleted memories by default.
- Batch import uses deterministic IDs and bounded transactions; cleanup is
  dependency ordered and limited to seeder-owned IDs.
- The final database check found zero records under benchmark run scopes after
  cleanup; production scopes were not included in the cleanup predicates.

## License and operational review

- Restricted external corpora remain outside Git under the local cache.
- Manifests require upstream revision, source checksum, conversion version,
  split identity, qrels checksum, license, and redistribution status.
- C-MTEB/MTEB/BEIR selections are metadata-locked with explicit local storage
  budgets; users must perform the upstream license review before fetching.
- Needle, MRCR, LongBench-v2, and VTCBench visual artifacts remain planned or
  prerequisite-gated unless a local artifact and capability are explicitly
  supplied. They are non-gating stress tracks and do not affect the memory
  product gate.
- Offline mode is the default; no implicit network, model, remote judge, or
  public leaderboard dependency is introduced.

## Residual prerequisites

The expansion is ready for local product use of the implemented tracks. A
future change is still required to promote any external metadata-only/planned
dataset to runnable status: acquire a license-approved local artifact, lock its
checksum/qrels, add normalization goldens, and run its family-specific gate.
This is an explicit prerequisite, not a hidden fallback.
