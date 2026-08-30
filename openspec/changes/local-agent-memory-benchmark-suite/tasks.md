## 1. Contracts and package foundation

- [x] 1.1 Add versioned Go types and JSON schema validation for `DatasetManifest`, split metadata, embedding profile, and machine-readable prerequisite/error statuses.
- [x] 1.2 Add versioned normalized record types for `ConversationRecord`, `MemoryEventRecord`, `BenchmarkQuery`, evidence groups, `QREL`, provenance, and run manifest.
- [x] 1.3 Add deterministic JSON/JSONL serialization, canonical ordering, checksum helpers, and golden fixtures for all normalized types.
- [x] 1.4 Add a dataset registry API with Layer 0-4 entries and explicit runnable/metadata-only/planned support states.
- [x] 1.5 Add unit tests for required manifest fields, unsupported versions, duplicate ids, malformed turns, and stable checksums.

## 2. Cache, fetch, and normalization pipeline

- [x] 2.1 Implement deterministic cache layout `<data-dir>/<dataset>/<version>/{raw,normalized,embeddings,reports}` and smoke/full split discovery.
- [x] 2.2 Implement fetch command/library with explicit network opt-in, SHA256 verification, upstream revision recording, atomic cache locks, and non-destructive mismatch handling.
- [x] 2.3 Implement normalization command/library interface that consumes local raw data only and writes normalized corpus plus normalization checksum.
- [x] 2.4 Add fixture-based tests proving fetch never overwrites a valid cache on checksum failure and normalize is byte-deterministic.
- [x] 2.5 Document license review, user-provided restricted data, cache cleanup, and the no-redistribution repository policy.

## 3. LoCoMo adapter and evidence mapping

- [x] 3.1 Implement the Layer 1 LoCoMo adapter for ordered sessions, timestamps, speakers, source offsets, and stable conversation/turn ids.
- [x] 3.2 Map LoCoMo facts and supporting context to explicit memory classes, lifecycle expectations, scope fields, and provenance without canonical in-place updates.
- [x] 3.3 Convert LoCoMo questions and supporting facts into `BenchmarkQuery`, grouped evidence, graded qrels, and must-not-return metadata.
- [x] 3.4 Add a checked-in, license-safe LoCoMo smoke fixture with its manifest, conversion version, expected checksums, and documented provenance.
- [x] 3.5 Add LoCoMo golden tests for single-hop, multi-hop, temporal/update, profile/preference, unmapped evidence, and duplicate-id cases.

## 4. Corpus import and isolation

- [x] 4.1 Implement an idempotent benchmark corpus loader that replays normalized events through the existing ingestion/consolidation or equivalent controlled fixture path.
- [x] 4.2 Add run-scoped project, tenant, namespace, and session derivation with collision-resistant run ids and cleanup support.
- [x] 4.3 Persist bidirectional source-turn/evidence mappings so every retrieved memory can be traced to dataset provenance.
- [x] 4.4 Enforce scope predicates and default lifecycle filtering for suppressed, forgotten, and deleted memories in benchmark retrieval.
- [x] 4.5 Add PostgreSQL + pgvector integration tests for repeated import, cross-run leakage, lifecycle exclusion, and tenant/namespace isolation.

## 5. Offline execution and embedding profiles

- [ ] 5.1 Implement `list`, `fetch`, `normalize`, `run`, and `report` CLI subcommands with explicit config flags and `STELE_BENCHMARK_*` environment variables.
- [x] 5.2 Implement offline-by-default prerequisite admission that checks local dataset, normalized corpus, vectors, model revision, dimensions, normalization, and qrels compatibility.
- [ ] 5.3 Add local-model and pre-cached-vector embedding profile loaders with deterministic metadata and no implicit network downloads.
- [ ] 5.4 Implement smoke, local-full, and reproducible-extended mode policies, including query budgets, split selection, and random seed handling.
- [x] 5.5 Implement explicit lexical-only smoke mode and reject silent semantic/hybrid downgrade when embedding prerequisites are absent.
- [x] 5.6 Add tests that block network access in offline mode and assert stable `prerequisite_missing`, `invalid_manifest`, and `checksum_mismatch` outcomes.

## 6. Evaluation integration and qrels/reporting

- [x] 6.1 Add an adapter from normalized queries/qrels to the retrieval evaluation baseline replay interface without changing `/v1/memories/search` or context assembly contracts.
- [x] 6.2 Implement graded relevance, multiple evidence groups, multi-hop completeness, query reuse, and must-not-return evaluation logic.
- [x] 6.3 Extend metric aggregation with Recall@k, MRR, nDCG, group/multi-hop hit rate, query coverage, p50/p95 latency, and safety failures.
- [ ] 6.4 Extend strategy comparison for lexical, semantic, hybrid, chunk, and hybrid-rank configurations with aligned dataset/run provenance.
- [ ] 6.5 Implement deterministic machine-readable report JSON containing inputs, checksums, model profile, scope, skipped prerequisites, per-query rows, aggregates, and quality/safety gate outcomes.
- [ ] 6.6 Add unit and integration tests for graded qrels, partial versus complete evidence groups, forbidden returns, deterministic ordering, and release-policy rejection.

## 7. Extended dataset registration and operations

- [x] 7.1 Add metadata-only registry entries and adapter interfaces for LongMemEval, Multi-Session Chat/PersonaChat, HotpotQA, TimeQA, and BEIR with license and readiness notes.
- [ ] 7.2 Add a LongMemEval adapter spike behind a feature/support flag, including update/conflict and cross-session normalization tests before marking it runnable.
- [ ] 7.3 Add profile/preference and generic retrieval adapter test fixtures that verify their query types are reported separately from Agent Memory metrics.
- [ ] 7.4 Add capacity checks, batch import controls, run namespace cleanup, and clear operator diagnostics for local PostgreSQL/disk constraints.

## 8. Documentation, CI, and release gate

- [x] 8.1 Document dataset acquisition, license obligations, cache preparation, local embedding options, PostgreSQL + pgvector prerequisites, and offline commands.
- [ ] 8.2 Add a CI smoke workflow using only repository-owned fixtures; ensure full external datasets and remote services are never required for default tests.
- [x] 8.3 Add benchmark quality-gate examples and a troubleshooting matrix for missing data, checksum mismatch, model mismatch, isolation failures, and safety failures.
- [ ] 8.4 Run `go test ./...`, relevant race/integration tests, benchmark smoke against PostgreSQL 18 + pgvector, and `openspec validate local-agent-memory-benchmark-suite --strict`.
- [ ] 8.5 Completion gate: run the checksum-locked LoCoMo benchmark end to end through local PostgreSQL + pgvector retrieval, retain an auditable machine-readable report with runtime/input/strategy/quality/safety identity, and review API, security, license, and operational readiness. Synthetic smoke alone does not satisfy this task or permit archive.
