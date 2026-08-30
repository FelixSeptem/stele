## Context

Stele currently combines lifecycle-visible lexical, semantic, and optional relation
retrieval into a scoped ranked result. The service also has provenance, canonical-memory
versioning, embedding revisions, quality feedback, ranking rollout controls, and
context assembly. Those capabilities make quality changes possible, but the product has
no stable retrieval corpus, no deterministic replay surface, and no contract for
deciding whether a change is an improvement rather than an anecdotal result.

The next roadmap step is therefore measurement-only. The design must preserve the
following non-negotiable boundaries: PostgreSQL remains the system of record; raw events
remain append-only; `tenant`, `project`, and `namespace` isolation is enforced on every
fixture operation; and ordinary retrieval continues to exclude suppressed, forgotten,
expired, and deleted memory.

## Goals / Non-Goals

**Goals:**

- Define a versioned, repository-owned fixture format that creates or references
  scoped source events and declares expected retrieval evidence.
- Execute deterministic retrieval replay against the real service/repository behavior,
  with a selected representation, ranking, and fixture version recorded in each report.
- Evaluate recall, rank quality, multi-hop evidence coverage, duplicate rate, candidate
  pool size, and latency while treating scope/lifecycle violations as hard failures.
- Expose bounded diagnostics only to local test, CI, or authorized administrative paths.
- Establish comparison and regression thresholds that later chunk, fusion, diversity,
  query-understanding, and reranking changes must satisfy.

**Non-Goals:**

- Change default candidate generation, fusion, feedback policy, context packing, or
  public response shapes.
- Add chunks, new memory lifecycle states, external indexes, benchmark adapters, or
  an online/model reranker.
- Use production tenant data, external benchmark data, or answer-generation output as
  fixture material.

## Decisions

### Decision: Use a repository-owned, versioned fixture contract

Fixtures SHALL be structured data checked into the repository and named by a stable
fixture version. A fixture declares test-only scope identifiers, source events, query
cases, expected evidence IDs or acceptable evidence groups, and expected exclusion
conditions. Generated memory identifiers are resolved through stable test aliases,
rather than recording database UUIDs directly.

This makes the corpus auditable and portable across fresh PostgreSQL instances. It also
allows a query to express multi-hop success as a set of required evidence groups rather
than incorrectly requiring one pre-composed answer record.

Alternatives considered:

- Query production data: rejected because it leaks tenant material, cannot be replayed
  deterministically, and makes CI unsafe.
- Use only unit-test mocks: rejected because score merge, lifecycle SQL, vector
  revision behavior, and repository filtering must be evaluated together.
- Adopt an external benchmark fixture: deferred because its licensing, retention, and
  task contract would create a separate product commitment.

### Decision: Separate hard safety assertions from quality metrics

The evaluator SHALL report quality metrics only after safety assertions pass. Cross-scope
retrieval, hidden lifecycle visibility, malformed fixture scope, or unauthorized
diagnostic disclosure fails the run regardless of Recall, MRR, or nDCG.

This prevents an apparently better ranker from being accepted when it improves recall by
retrieving disallowed evidence.

### Decision: Run replay through real retrieval interfaces and isolated PostgreSQL

The fixture runner will use a uniquely named test scope and real repository/retrieval
interfaces. Unit tests remain valuable for individual calculations, but the baseline
must include PostgreSQL full-text, lifecycle, scope, and active-vector-revision behavior.
The runner can skip explicitly when an operator has not supplied an owned PostgreSQL
test DSN, while CI and release gates require the real-stack execution.

Alternatives considered:

- Make a database mandatory for every local unit test: rejected because ordinary
  contributor feedback must remain fast.
- Add a second system of record/search index: rejected by repository architecture and
  unnecessary for a measurement baseline.

### Decision: Keep diagnostics redacted, bounded, and non-public by default

Each case can record candidate channel presence, channel rank, final rank, disposition,
metric contribution, and bounded timing. It SHALL not serialize raw event content,
hidden memory identifiers or content, credentials, DSNs, complete SQL errors, or
cross-scope candidate details. Ordinary `POST /v1/memories/search` and context assembly
responses remain unchanged.

Alternatives considered:

- Return full diagnostics on ordinary search responses: rejected because it expands
  the public contract and risks exposing hidden evidence.
- Emit no diagnostics: rejected because later ranking regressions could not be
  explained or corrected efficiently.

### Decision: Version representation and ranking explicitly without implementing new ranking

Reports SHALL include `fixture_version`, `representation_version`, `ranking_version`,
and active embedding revision metadata in bounded form. The initial values describe the
existing canonical representation and current score-merge ranking. Future strategies
such as RRF or chunked representation can then produce comparable reports without
changing the evaluator's contract.

### Decision: Define release thresholds as policy, not embedded constants

The baseline report contains measured values; a small versioned policy defines allowed
latency and quality regression thresholds. Initial policy requires zero safety failures,
no decrease in required-evidence coverage or Recall@k for protected fixture categories,
and an explicitly approved tolerance for noisy aggregate measures. Threshold changes
must be reviewed as policy changes, not hidden inside code.

## Risks / Trade-offs

- [A small fixture overfits the current implementation] -> Cover multiple memory
  classes, negative examples, temporal and multi-hop cases, and require fixture review
  whenever a ranking strategy is introduced.
- [Embedding providers make semantic results nondeterministic] -> Record provider/model
  revision, use deterministic local test providers where possible, and compare reports
  only within compatible representation/embedding revisions.
- [Real PostgreSQL evaluation is slower than unit testing] -> Keep unit tests focused,
  place the real replay behind an owned DSN and bounded CI/release command, and report
  latency separately from local fixture setup.
- [Diagnostics reveal sensitive information] -> Use aliases and aggregate counters;
  enforce tests that reject raw content, DSNs, credentials, hidden IDs, and foreign
  scope identifiers from reports.
- [Thresholds become a blocking source of false regressions] -> Version policy,
  distinguish protected hard gates from advisory metrics, and require explicit review
  to relax a gate.

## Migration Plan

1. Add the fixture schema, parser, validation, and deterministic aliases without
   changing retrieval behavior.
2. Seed isolated fixture scopes into a harness-owned database and execute current
   retrieval as `canonical-v1` / `baseline-v1`.
3. Publish the first redacted baseline artifact and configure advisory CI execution.
4. Promote zero-leakage and lifecycle-visibility checks to hard CI gates; promote
   quality/latency thresholds after an approved baseline is recorded.
5. Require later retrieval changes to compare their candidate report with the approved
   baseline before enabling a scoped rollout.

Rollback is configuration and policy based: disable the evaluation CI job or return it
to advisory mode without changing production retrieval. The evaluator owns no canonical
production data and all fixture records can be deleted by their unique harness scope.

## Open Questions

- Which deterministic embedding provider and dimensions should be the canonical local
  semantic fixture configuration? Resolve before implementation so reports are
  comparable across machines.
- What initial p95 latency budget is realistic for the real PostgreSQL harness on CI?
  Establish it from the first baseline evidence instead of guessing.
- Should report artifacts be committed as golden JSON or uploaded only from CI? The
  initial implementation should keep the fixture and policy in Git while avoiding
  credential-bearing or environment-specific reports.
