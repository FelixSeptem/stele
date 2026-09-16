## 1. Run contracts and ownership

- [x] 1.1 Define bounded evaluation-run, provider-profile, exact-scope, prerequisite, strategy, and verdict models; verify unknown identities, oversized fields, missing scope, and invalid DSN ownership fail closed.
- [x] 1.2 Add explicit evaluation DSN/provider-profile configuration and entrypoint validation; verify absent configuration returns `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` without reading the service DSN.
- [x] 1.3 Implement compatibility checks across fixture, representation, fusion, ranking, embedding, reranker, analysis, policy, and renderer identities; verify incompatible candidates cannot be release-eligible.

## 2. Owned PostgreSQL and pgvector evidence

- [x] 2.1 Add an opt-in PostgreSQL 18 + pgvector evaluation runner over one exact scope; verify it refuses missing/incompatible prerequisites and never mutates canonical records outside governed fixture ingestion.
- [x] 2.2 Execute `canonical-v1` / `baseline-v1` and compatible candidate profiles with bounded quality, latency, candidate-budget, fallback, isolation, lifecycle, and rollback evidence; verify reports contain logical identities but no DSNs, credentials, content, or raw provider payloads.
- [x] 2.3 Add deterministic offline CI smoke fixtures and a separate real-stack release job; verify offline/skipped runs remain non-pass and cannot authorize rollout.

## 3. Progressive context evaluation

- [x] 3.1 Run short retrieval projection, medium session/context overview, and canonical/chunk evidence levels against the same exact scope; verify each level has separate identity and baseline comparison.
- [x] 3.2 Record source watermark, freshness, token/character budget, citation coverage, deterministic rebuild identity, and bounded failure category per level; verify stale, hidden, and foreign evidence fail closed.
- [x] 3.3 Verify repeated rebuilds from identical PostgreSQL source records produce stable derived ordering/identity while preserving append-only history and leaving default retrieval unchanged.

## 4. Parent-first shadow comparison

- [x] 4.1 Execute bounded parent-first projection/chunk expansion in offline or shadow mode; verify exact-scope child/adjacent expansion respects candidate, latency, lifecycle, and budget limits.
- [x] 4.2 Compare parent-first with flat fusion using protected quality, duplicate, multi-hop, isolation, latency, rollback, and candidate-budget gates; verify safety failures override quality gains.
- [x] 4.3 Add disable/rollback and unchanged-default-retrieval tests; verify foreign or lifecycle-hidden expansion is recorded as non-pass without canonical mutation.

## 5. Redacted reports and operational evidence

- [x] 5.1 Compose one machine-readable and human-readable bounded report from prerequisite, progressive, parent-first, quality, safety, freshness, citation, and rollback outcomes; verify forbidden-field redaction and stable non-pass categories.
- [x] 5.2 Reuse trajectory, integrity, retention, and deletion contracts for derived evaluation artifacts; verify cleanup never deletes canonical source records.
- [x] 5.3 Publish operator runbook for evaluation DSN ownership, PostgreSQL/pgvector prerequisites, rebuild/re-index, rollback, retention, and threshold review; verify docs consistency checks pass.

## 6. Verification and roadmap evidence

- [x] 6.1 Add focused tests for missing DSN, service-DSN fallback denial, incompatible identities, stale/hidden/foreign evidence, deterministic rebuild, budget overflow, rollback, and release verdict precedence.
- [x] 6.2 Run focused and full Go tests, available quality gates, and real-stack tests when an owned evaluation DSN is present; record unavailable environment prerequisites without claiming release passage.
- [x] 6.3 Run `openspec validate retrieval-release-evidence-and-progressive-evaluation --strict`, `openspec validate --all`, and `git diff --check`; update roadmap evidence only to the state actually demonstrated.
