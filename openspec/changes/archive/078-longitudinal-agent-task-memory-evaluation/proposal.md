## Why

Stele stores externally judged task outcomes and evaluates retrieval quality, temporal precedence, provenance, and safety, but those reports do not evaluate a repeated task series as one longitudinal memory experiment. This leaves a measurable gap: a memory strategy can improve isolated retrieval scores while making later tasks worse through stale or noisy context, and current reports do not quantify that trajectory.

## What Changes

- Add a deterministic, checksum-locked task-series fixture and evaluator that compares externally supplied task outcomes and retrieval evidence across ordered task stages and declared memory conditions.
- Report bounded sequence-level task utility, stale/noisy-memory harm, provenance/evidence coverage, and exact-scope/lifecycle safety separately; unsafe evidence always fails the safety gate.
- Add a PostgreSQL + pgvector runner that evaluates an owned, non-synthetic task-series fixture using existing retrieval and reporting contracts.
- Keep evaluation observational: Stele does not execute an agent, invoke a model, construct prompts, judge answers, infer missing task outcomes, or claim causal attribution.
- Correct the roadmap's stale item 9 archive-status paragraph and promote item 11 only after the longitudinal gap and acceptance measures are represented by this change.

## Capabilities

### New Capabilities
- `longitudinal-agent-task-memory-evaluation`: Reproducible, bounded evaluation of repeated externally judged tasks and their memory evidence over time.

### Modified Capabilities

## Impact

- Affected code: `internal/benchmark` task-series fixture, evaluator, report and PostgreSQL runner; benchmark tests and owned fixture data.
- Affected documentation: benchmark guide, roadmap status, and archived verification evidence.
- APIs and production behavior: no public API or runtime retrieval behavior changes.
- Dependencies: no new dependencies; reuse existing benchmark, task-evaluation, retrieval, and PostgreSQL + pgvector infrastructure.

## Non-goals

- Running or evaluating an external agent, model, prompt, or generated answer.
- Treating observational outcome differences as causal proof that memory caused success or failure.
- Adding task-success ingestion APIs, a second benchmark store, cross-scope evaluation, or automatic rollout/activation.
- Replacing LoCoMo, LongMemEval, qrels, retrieval release gates, or memory-integrity evaluation.

## References

- Roadmap candidate: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`, item 11.
- Existing contracts: `openspec/specs/task-success-evaluation/spec.md`, `openspec/specs/benchmark-qrels-and-reporting/spec.md`, and `openspec/specs/redacted-retrieval-trajectory-and-memory-integrity-evaluation/spec.md`.
- Validation: `openspec validate --all --strict`; benchmark tests and owned PostgreSQL + pgvector verifier.
