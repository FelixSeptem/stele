## Context

See `proposal.md` for the uncovered evaluation gap. Existing benchmark inputs normalize conversations into events, queries and qrels, while task-success records remain externally reported evidence. The new evaluator must compose those contracts without becoming an agent runtime or a task judge.

## Goals / Non-Goals

**Goals:**

- Make an ordered task series a validated, checksummed benchmark input with explicit condition and stage identities.
- Compute descriptive condition-level outcomes and evidence-integrity metrics from supplied verdicts and actual bounded retrieval observations.
- Reuse current corpus normalization, retrieval evaluation, exact-scope fixture seeding and runtime identity collection.
- Keep redacted aggregate output deterministic and make safety failures override all quality outcomes.

**Non-Goals:**

- Persisting a new production task-series table or adding a public API.
- Deriving task outcomes from answers, retrieval scores, or memory state.
- Running live agents, adapting task prompts, tuning memory automatically, or approving rollout.

## Decisions

### Ordered conditions are explicit input, not inferred experiments

Represent the input as a versioned manifest containing an exact scope, ordered stages, declared condition labels, external verdicts, source/qrel checksums and evidence mappings. Require every comparable condition to declare the same task stages; preserve missing outcomes as incomplete rather than imputing them. The evaluator computes condition rates and absolute observed differences only. This avoids an implicit causal model and keeps externally judged outcome ownership with the caller.

Alternatives considered: reusing a single retrieval corpus and deriving condition labels from rank changes would not represent task outcomes; adding a service API/table would duplicate durable task-evaluation evidence and expand production ownership unnecessarily.

### Separate task utility, memory harm, provenance, and safety

Use a versioned mapping (`succeeded=1`, `partial=0.5`, `failed=0`, and `inconclusive` excluded) for descriptive utility, with denominators and missing counts reported. Compute stale/noisy harm only from explicit stage-level labels and the retrieved evidence actually observed for that stage. Compute provenance coverage only where source lineage resolves to the exact scoped fixture mapping. Keep scope/lifecycle violations as a hard independent gate; never average them into utility.

Alternatives considered: a single composite score obscures a memory strategy that improves success while increasing harmful context; inferring stale/noisy categories from text or age would turn the evaluator into an unreviewed classifier.

### Reuse the existing real PostgreSQL retrieval fixture path

Add a longitudinal benchmark runner alongside `RunLongMemEvalPostgres` and `RunPostgresCorpus`. It partitions each declared condition/stage into isolated fixture scopes, seeds only its own corpus, invokes the existing retrieval evaluator, and feeds actual ranked evidence plus the external outcome manifest to the longitudinal metric evaluator. Collect PostgreSQL and pgvector identities using existing helpers. Unit/smoke fixtures validate determinism but cannot satisfy non-synthetic completion evidence.

Alternatives considered: evaluate only caller-provided rankings would be useful for unit tests but not prove the runtime retrieval path; implementing a second SQL search/seeder path would duplicate tested fixture and safety behavior.

### Persist only aggregate report artifacts

The machine-readable report contains schema/metric versions, input checksums, runtime/strategy identity, counts, aggregate metric values, and separate quality/safety status. It omits per-task content, queries, scope values, internal IDs, raw scores, and candidate details. Stable stage order is represented by counts and ordered aggregate arrays without caller identifiers.

Alternatives considered: per-stage raw report rows would aid debugging but expand sensitive-data retention and are unnecessary for reproducible aggregate checks; debugging stays in local test output, not retained artifacts.

## Risks / Trade-offs

- [Externally supplied outcome quality varies] → Record source/checksum and verdict counts, describe comparisons as observational, and never infer causal effect.
- [A condition can have incomplete stages] → Report explicit coverage and missing counts; mark quality incomplete and exclude missing outcomes from denominators.
- [Fixture labels can misclassify stale/noisy evidence] → Require explicit versioned annotations and test the metric against hand-computed adversarial sequences.
- [Synthetic tests can be mistaken for release proof] → Mark synthetic runs and require a checksum-locked non-synthetic owned PostgreSQL + pgvector run for completion evidence.

## Migration Plan

No database migration or production rollout is required. Add the benchmark capability behind its local CLI/config entry point, document the manifest format and redaction contract, run deterministic unit tests, then run the fixture against a fresh isolated PostgreSQL + pgvector database. If evaluation fails, discard the derived report and fixture scope; canonical production memory is not modified.
