# Agent memory benchmark expansion

The expanded benchmark surface remains local and offline by default. It is an
evaluation facility for this service; it does not modify the public memory API
or permit benchmark records to enter production project, tenant, or namespace
scopes.

## Dataset preparation

1. Review the upstream license and redistribution status before acquiring any
   non-fixture corpus. Restricted source artifacts must stay in the local
   benchmark cache and must not be committed.
2. Lock the upstream revision and SHA-256 digest in the dataset manifest.
   Record qrels checksum, conversion version, split, and embedding profile.
3. Place checked source files in the cache layout
   `<data-dir>/<dataset>/<version>/raw`. The runner uses sibling `normalized`,
   `embeddings`, and `reports` directories and never fetches remotely when
   `STELE_BENCHMARK_OFFLINE` is unset (the default is true).
4. Prepare ModelScope or other local embedding assets in advance. A semantic or
   hybrid run rejects missing or incompatible embedding identities.

## Commands and resource budgets

`stele benchmark list` lists family, support, license, and prerequisite
metadata. `fetch`, `normalize`, `run`, `report`, and `clean` are family-aware
offline command entrypoints; they require a registered dataset and a
checksum-locked local cache. `run-smoke` runs only repository-owned LoCoMo
fixture data.

Use LongMemEval `s` first. Larger `m` and oracle runs require explicit capacity
approval, batching, available local PostgreSQL + pgvector, and enough disk for
raw, normalized, embedding, and retained report artifacts. Stress fixtures
must declare context/sample/time budgets. Visual VTCBench mode is refused when
local visual artifacts/capability are absent; it never silently falls back to
text mode.

## Reproducibility and cleanup checklist

- Record PostgreSQL 18 and pgvector identity for database-backed runs.
- Retain manifest, raw and qrels checksums, conversion version, split, strategy
  profile, embedding identity, run scope, metrics, safety outcomes, and report
  paths.
- Use a benchmark-only tenant/project/namespace and a fresh scope per run.
  Verify cross-run isolation, lifecycle exclusions, and must-not-return rules.
- Preserve requested manifests and reports during cleanup. Delete only the
  benchmark run corpus, embeddings, and database scope; verify production
  scopes are unchanged.
- A product-ready result requires a non-synthetic checksum-locked LongMemEval
  retrieval run on local PostgreSQL + pgvector plus retained provider-contract,
  profile, temporal, and multi-hop reports. Repository fixtures do not replace
  that gate.
