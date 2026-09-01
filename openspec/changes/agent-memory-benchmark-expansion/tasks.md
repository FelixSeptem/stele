## 1. Registry, manifest, and cache foundations

- [x] 1.1 Extend the benchmark registry with family identity, support state, license status, and local prerequisite metadata for all datasets named in the proposal.
- [x] 1.2 Extend dataset manifests with upstream revision, qrels checksum, conversion version, embedding profile, split identity, and redistribution fields.
- [x] 1.3 Add family-aware cache layout and validation for `raw`, `normalized`, `embeddings`, and `reports`, including deterministic checksum failure statuses.
- [x] 1.4 Add registry and manifest contract tests for runnable, metadata-only, planned, restricted, checksum-drift, and incompatible-input cases.

## 2. LongMemEval adapter

- [x] 2.1 Implement LongMemEval source loading for locked local JSON/JSONL artifacts and explicit `s`, `m`, and `oracle` subset selection.
- [x] 2.2 Normalize LongMemEval sessions, turns, timestamps, question dates, question types, answer session IDs, and provenance into the benchmark intermediate schema.
- [x] 2.3 Map update/conflict, preference, temporal, and abstention questions into expected lifecycle state, graded qrels, evidence groups, and must-not-return metadata.
- [x] 2.4 Add deterministic golden normalization tests, including repeated conversion, missing answer session, unmapped evidence, duplicate IDs, and abstention cases.
- [ ] 2.5 Add a retrieval-only LongMemEval runner that does not require an LLM judge and records oracle/retrieval-log modes as distinct comparison modes.

## 3. LongMemEval PostgreSQL and product gate

- [ ] 3.1 Import normalized LongMemEval data through the controlled benchmark ingestion path with benchmark-only project, tenant, namespace, and run scopes.
- [ ] 3.2 Add PostgreSQL + pgvector integration tests for repeated import, cross-run leakage, lifecycle exclusion, session boundaries, and answer-session qrels alignment.
- [ ] 3.3 Add local capacity preflight, subset limits, batch controls, and cleanup verification for the standard `s` subset and explicit larger runs.
- [ ] 3.4 Execute one non-synthetic checksum-locked LongMemEval retrieval run on local PostgreSQL 18 + pgvector and retain its machine-readable report as the core completion artifact.

## 4. BFCL memory provider contract

- [x] 4.1 Define the normalized offline contract fixture schema for `memory_kv`, `memory_rec_sum`, and `memory_vector` operations.
- [x] 4.2 Implement operation replay and validation for operation names, argument shapes, expected result shapes, refusal cases, and malformed calls.
- [x] 4.3 Propagate project, tenant, namespace, session, and lifecycle expectations through contract replay and detect hidden/foreign memory access.
- [x] 4.4 Add contract metrics and machine-readable reports separate from retrieval ranking metrics.
- [x] 4.5 Add offline regression tests for valid calls, irrelevant calls, malformed arguments, scope violations, and forgotten/suppressed/deleted memory.

## 5. Profile, preference, temporal, and multi-hop suites

- [ ] 5.1 Add a license-safe PersonaChat or Multi-Session Chat fixture and adapter with explicit profile facts, preference updates, session IDs, and provenance.
- [ ] 5.2 Add profile recall, preference consistency, current-versus-historical preference, and session isolation qrels and metrics.
- [ ] 5.3 Add TimeQA-style temporal fixtures covering date-bounded answers, stale-fact suppression, and update precedence.
- [ ] 5.4 Add HotpotQA-style multi-hop fixtures with grouped supporting evidence, graded qrels, partial coverage, and complete evidence-group metrics.
- [ ] 5.5 Add normalization, retrieval, lifecycle, and isolation tests for all specialized suites and retain representative reports.

## 6. Generic retrieval strategy regression

- [ ] 6.1 Select and lock a small C-MTEB/MTEB and BEIR subset set with documented license, language, corpus size, and local storage budget.
- [x] 6.2 Implement generic retrieval normalization and qrels loading under a `generic_retrieval` family identity.
- [x] 6.3 Add deterministic strategy profiles for lexical, semantic, hybrid, chunk variants, hybrid-rank, and optional reranker paths.
- [x] 6.4 Implement same-corpus strategy comparison with Recall@k, MRR, nDCG, candidate pool size, latency, and comparability checks.
- [ ] 6.5 Add tests proving incompatible corpus/qrels/embedding identities cannot be aggregated and generic runs cannot access production scopes.

## 7. Long-context and multimodal stress suites

- [ ] 7.1 Add controlled Needle-in-a-Haystack and OpenAI MRCR subset generation/loading with context length, needle count/depth, timeout, and sample budgets.
- [ ] 7.2 Add LongBench-v2 long-context subset metadata and capacity-gated execution without treating answer accuracy as memory-provider quality.
- [x] 7.3 Add VTCBench text-mode subset support and explicit visual capability/artifact checks; refuse unavailable visual mode without silent fallback.
- [x] 7.4 Add per-bucket degradation reports for context length, position, needle count, mode, latency, and capacity failures under a non-gating `stress` family.
- [x] 7.5 Add stress-run tests for budget refusal, missing visual artifacts, text-mode execution, and non-gating report classification.

## 8. Unified reporting, governance, and CLI

- [x] 8.1 Extend benchmark CLI commands to list, fetch, normalize, run, report, and clean by dataset family while preserving offline defaults.
- [ ] 8.2 Extend report schema with dataset/family identity, all input checksums, qrels/version, runtime identity, embedding/strategy profile, run scope, metrics, errors, safety outcomes, and artifact paths.
- [x] 8.3 Implement stable statuses for success, quality-gate failure, prerequisite missing, invalid manifest, checksum mismatch, capacity refusal, and internal error.
- [ ] 8.4 Add family-level report rendering and explicit non-comparability rules for memory, contract, specialized, generic IR, and stress results.
- [x] 8.5 Add cleanup commands and tests proving benchmark cleanup preserves retained manifests/reports and leaves production scopes unchanged.

## 9. Documentation, CI, and verification

- [ ] 9.1 Document local dataset acquisition, license review, ModelScope cache preparation, offline execution, embedding profiles, resource budgets, and cleanup.
- [x] 9.2 Add CI smoke coverage using repository-owned fixtures only, including manifest validation, adapter golden tests, report schema, and offline network blocking.
- [ ] 9.3 Add local integration instructions and a reproducibility checklist for PostgreSQL 18 + pgvector, dataset checksums, qrels, strategy identity, and report retention.
- [ ] 9.4 Run `go test ./...`, benchmark integration tests, OpenSpec strict validation, and the real LongMemEval gate; record failures and residual prerequisites in reports.
- [ ] 9.5 Review API, security, license, capacity, isolation, lifecycle, and operational readiness before marking the change complete or archivable.
