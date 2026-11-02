## 1. Deterministic longitudinal contract

- [x] 1.1 Add versioned task-series manifest, ordered stage, condition, outcome, and report types under `internal/benchmark`; validate exact scope, unique stage/condition identities, compatible condition coverage, supported external verdicts, bounded labels, checksums, and explicit stale/noisy annotations. Add table-driven tests for valid input and each rejection case.
- [x] 1.2 Implement a pure longitudinal evaluator that calculates versioned utility, observed condition differences, stage coverage, stale/noisy exposure, and verifiable provenance coverage; keep safety independent and hard-failing. Add hand-calculated repeated-sequence tests, including missing verdicts, correction/supersession exclusion, and quality-up/safety-down cases.
- [x] 1.3 Add deterministic JSON report serialization and redaction tests proving same compatible inputs yield stable output and that task text, query, scope values, internal IDs, raw scores, hidden evidence, prompts, and provider payloads are absent.

## 2. Retrieval and real-stack integration

- [x] 2.1 Add a longitudinal adapter from normalized benchmark query/qrel/source lineage and externally supplied stage outcomes; validate evidence mappings and reject identity drift without inferring outcomes. Test repeated task stages that refer to current, stale, noisy, missing, and foreign evidence.
- [x] 2.2 Add a PostgreSQL + pgvector runner that isolates each declared condition/stage in benchmark-owned scopes, uses the existing fixture seeder and retrieval replay path, joins actual ranked evidence to supplied outcomes, reports runtime/input identity, and cleans only seeder-owned records. Add opt-in integration tests for isolation, lifecycle exclusion, deterministic replay, and cleanup.
- [x] 2.3 Add a bounded non-synthetic fixture loader that requires a local checksum-locked source and compatible externally judged stage outcomes; missing data, mismatched checksums, or incomplete verdicts must produce a prerequisite/incomplete result rather than a synthetic fallback.

## 3. Documentation, evidence, and roadmap closure

- [x] 3.1 Document the task-series manifest, metric formulas/version, observational interpretation, redaction, exact-scope rules, offline behavior, local source prerequisites, report fields, and cleanup in `docs/local-agent-memory-benchmark.md` and benchmark CLI help; no public runtime API changes.
- [x] 3.2 Add and retain a bounded report from a checksum-locked non-synthetic task-series source on fresh PostgreSQL + pgvector, including source/normalized/qrel checksums, strategy, runtime identity, stage/condition coverage, separate quality/safety gates, and cleanup evidence. Synthetic unit/smoke evidence cannot satisfy this task.
- [x] 3.3 Correct item 9's stale archive wording and update item 11's status in the maintained roadmap only after the non-synthetic real-stack acceptance gate passes; preserve the tracked compatibility roadmap link and update the archive index through the sequencing script.
- [x] 3.4 Run focused and full Go tests, race tests, vet, documentation and smoke-doc checks, strict OpenSpec validation, and `git diff --check`; record exact outcomes and all unmet prerequisites before archive.
