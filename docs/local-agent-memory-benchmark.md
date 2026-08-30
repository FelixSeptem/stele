# Local Agent Memory Benchmark

Stele's benchmark suite evaluates retrieval evidence, not generated answers. It is designed to run on a developer machine with local data and without a required public service, remote embedding API, or LLM judge.

## Dataset policy

The repository contains only a synthetic, repository-owned LoCoMo-shaped smoke fixture. It does not redistribute the LoCoMo, LongMemEval, Multi-Session Chat, PersonaChat, HotpotQA, TimeQA, or BEIR full corpora.

Before fetching a full external dataset, review its current license and upstream terms. Create a manifest that locks the dataset name, version, upstream URL, upstream revision, SHA256, conversion version, split, and embedding profile. Keep the downloaded files under a local cache; do not add them to Git unless redistribution is explicitly permitted.

## Local cache

Set these variables for a local full run:

```powershell
$env:STELE_BENCHMARK_DATA_DIR = "D:\stele-benchmark-data"
$env:STELE_BENCHMARK_DATASET = "locomo"
$env:STELE_BENCHMARK_DATA_VERSION = "<locked-version>"
$env:STELE_BENCHMARK_OFFLINE = "true"
```

The deterministic cache layout is:

```text
<data-dir>/<dataset>/<version>/
  raw/
  normalized/
  embeddings/
  reports/
```

`fetch` is the only operation allowed to use an explicitly enabled network connection. It must verify the manifest SHA256 before replacing raw data. `run` is offline by default and never downloads data, models, vectors, or judges implicitly.

## Smoke verification

The repository-owned smoke check requires no PostgreSQL, model download, or network request:

```powershell
$env:STELE_BENCHMARK_DATA_DIR = "$PWD\.tmp\benchmark"
go run ./cmd/stele benchmark list
go run ./cmd/stele benchmark run-smoke
```

`run-smoke` writes a JSON report to the local cache. It records the dataset version and checksum, mode, offline status, query metrics, latency, and safety failures. The fixture's candidate generator is intentionally deterministic; it verifies the adapter, cache, qrels, report and safety contracts, not real retrieval quality.

## Embedding profiles

Semantic and hybrid modes require an explicit local model or pre-cached vector profile. Lock model name, revision, dimensions, normalization and vector source in the manifest. A dimension or profile mismatch fails admission with `prerequisite_missing`.

Lexical-only smoke is allowed only when explicitly selected. The runner does not silently turn a semantic or hybrid run into lexical mode.

## Run modes

- `smoke`: repository-owned bounded fixture; suitable for CI and installation verification.
- `local-full`: full locally cached dataset and vectors; no network fallback.
- `reproducible-extended`: full data plus locked model/profile, strategy/chunk settings, qrels and random seed metadata.

## Troubleshooting

| Status | Meaning | Action |
| --- | --- | --- |
| `prerequisite_missing` | Local dataset, normalized split, vectors or model profile is absent. | Prepare the artifact locally; do not expect automatic download. |
| `checksum_mismatch` | Raw data or normalized metadata does not match the lock. | Remove only the affected cache version and fetch/normalize again from the verified source. |
| `invalid_manifest` | Version, license, provenance, split or embedding profile is incomplete. | Fix and re-lock the manifest before running. |
| safety failure | A forbidden, cross-scope or non-active memory was returned. | Treat the run as non-releasable and inspect scope/lifecycle filters. |
| quality gate failure | Metrics regressed without a safety violation. | Compare the report using the same corpus, qrels and embedding profile before changing ranking or chunking policy. |

## PostgreSQL and pgvector

The end-to-end full benchmark uses Stele's PostgreSQL + pgvector retrieval path and must run against the supported local PostgreSQL version. Keep benchmark runs in an isolated benchmark project/namespace. Do not import public benchmark corpus data into a production tenant or namespace. The baseline replay integration and full PostgreSQL proof are tracked in the active `local-agent-memory-benchmark-suite` OpenSpec change.
