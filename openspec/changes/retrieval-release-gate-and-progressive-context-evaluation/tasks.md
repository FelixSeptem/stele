## 1. Release-gate contracts and policy identity

- [ ] 1.1 Add failing tests for release-report identity compatibility across
  fixture, representation, fusion, ranking, provider, analysis, and policy
  versions; implement fail-closed comparison and stable incompatibility codes.
- [ ] 1.2 Define versioned release-policy, protected-threshold, resource-budget,
  prerequisite, rollback, and retention contracts; reject unknown or unbounded
  values and preserve the immutable original-query baseline.
- [ ] 1.3 Add explicit owned evaluation DSN/provider-profile configuration and
  startup/entrypoint validation; verify no fallback to the service DSN and stable
  `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` behavior.

## 2. Real-provider replay and redacted quality evidence

- [ ] 2.1 Extend replay orchestration to run canonical-v1/baseline-v1 and
  compatible candidate profiles through PostgreSQL + pgvector when explicitly
  enabled, retaining logical embedding/reranker identity and bounded latency.
- [ ] 2.2 Add release reports for protected recall, temporal and multi-hop
  coverage, duplicate/diversity, candidate budgets, fallback categories,
  isolation/lifecycle failures, and policy decisions without raw payloads.
- [ ] 2.3 Add tests proving skipped, stale, incompatible, or failing real-stack
  prerequisites cannot authorize active rollout and that safety failures override
  aggregate quality gains.

## 3. Progressive context evaluation

- [ ] 3.1 Add offline fixture cases and evaluators for short retrieval projection,
  medium session/context overview, and canonical/chunk evidence with explicit
  level identity and baseline comparison.
- [ ] 3.2 Record source watermark, freshness, token/character budget,
  citation/evidence coverage, deterministic rebuild identity, and bounded failure
  reasons for each level.
- [ ] 3.3 Add stale/hidden/foreign projection tests and verify derived summaries
  remain non-canonical and cannot affect default retrieval.

## 4. Parent-first shadow experiment

- [ ] 4.1 Implement an offline/shadow parent-first strategy over validated
  projections or parent chunks with exact-scope bounded child/adjacent expansion.
- [ ] 4.2 Compare parent-first with flat fusion using protected recall, multi-hop,
  duplicate, isolation, latency, candidate-budget, and rollback gates; keep
  production ranking unchanged.
- [ ] 4.3 Add disable/rollback and foreign-scope/lifecycle expansion tests plus
  bounded expansion diagnostics.

## 5. Trajectory and memory-integrity evidence

- [ ] 5.1 Define an allowlisted redacted trajectory schema containing only channel,
  count, expansion, disposition, fallback, and latency buckets; add forbidden-
  field and public-surface leakage tests.
- [ ] 5.2 Add retention/deletion execution for trajectory, diagnostic, report, and
  fixture artifacts with deterministic expiry and canonical-source preservation.
- [ ] 5.3 Add memory-organization integrity fixtures and reports separating action
  success from fact/evidence recall, placement, duplicate, missing, altered, and
  unexpected evidence outcomes.

## 6. Runbooks, CI, and documentation

- [ ] 6.1 Add release checklist and rebuild/re-index/rollback runbooks for chunks,
  embeddings, duplicate clusters, projections, and parent-first artifacts.
- [ ] 6.2 Add CI smoke coverage using repository-owned fixtures and an opt-in
  release job for the owned real-provider gate; ensure skipped evidence is never
  reported as a pass.
- [ ] 6.3 Document local `.env.local` provider/evaluation placeholders, report
  redaction, retention, threshold ownership, and operational failure handling.
- [ ] 6.4 Add low-cardinality release-gate, trajectory, integrity, cleanup, and
  rollback telemetry/admin diagnostics consistent with existing observability.

## 7. Verification and OpenSpec completion

- [ ] 7.1 Run focused retrieval, projection, benchmark, config, storage, admin,
  and retention tests uncached; include PostgreSQL 18 + pgvector when the owned
  DSN is configured.
- [ ] 7.2 Run `go test ./... -count=1 -timeout 15m`, available quality gates,
  `git diff --check`, and the template-marker scan; record unavailable environment
  prerequisites without claiming release passage.
- [ ] 7.3 Run race tests where the host toolchain supports them and capture any
  exact environmental limitation.
- [ ] 7.4 Run strict OpenSpec validation for this change and `openspec validate
  --all`; update roadmap evidence only to the state actually demonstrated.
