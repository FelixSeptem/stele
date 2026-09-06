## 1. Evaluation Contract And Fixture Safety

- [x] 1.1 Define versioned retrieval-evaluation domain types for fixture cases, source
  events, aliases, evidence groups, exclusions, ranking metadata, metric reports, and
  release-policy decisions; add unit validation tests for malformed and unsafe fixtures.
- [x] 1.2 Add repository-owned fixture data covering single-fact, multi-hop, temporal,
  profile, episodic, procedural, summary, relation, contradiction, noisy-neighbor,
  duplicate, hidden-lifecycle, and cross-scope cases.
- [x] 1.3 Implement deterministic fixture seeding into explicitly owned scopes with
  alias-to-record resolution, append-only provenance checks, and cleanup-safe ownership
  markers; add real PostgreSQL integration coverage gated by an explicit test DSN.
- [x] 1.4 Add redaction tests proving fixture parsing, validation errors, and generated
  reports cannot contain raw event payloads, credentials, DSNs, hidden IDs/content, or
  foreign scope values.

## 2. Retrieval Replay And Diagnostics

- [x] 2.1 Implement a deterministic replay service that executes fixture queries through
  the real lexical, semantic, and enabled relation retrieval interfaces without changing
  ordinary search or context behavior.
- [x] 2.2 Capture bounded authorized diagnostics for visible candidate channels, channel
  rank, final rank, disposition, candidate-pool size, and timing; add tests that ordinary
  public search responses do not expose evaluation diagnostics.
- [x] 2.3 Implement metric calculation for Recall@1/5/10, MRR, nDCG@k, multi-hop
  evidence coverage, duplicate rate, candidate-pool size, and bounded latency; add
  deterministic unit tests for each metric and edge case.
- [x] 2.4 Treat cross-scope results, hidden lifecycle visibility, malformed fixture scope,
  and unsafe diagnostic disclosure as stable hard-failure categories; add regression
  tests that quality metrics cannot override a safety failure.

## 3. Reporting And Comparison

- [x] 3.1 Add machine-readable and concise human-readable report renderers containing
  fixture, representation, ranking, compatible embedding revision, and policy versions
  while preserving redaction and bounded output.
- [x] 3.2 Add baseline-versus-candidate comparison for compatible reports, including
  per-metric deltas, protected-category regressions, advisory observations, and stable
  incompatibility errors for mismatched fixture or representation versions.
- [x] 3.3 Define a versioned quality release policy with hard safety gates, protected
  recall/multi-hop coverage thresholds, advisory metrics, and a measured p95 latency
  budget; add validation and decision tests.
- [x] 3.4 Add a local command or focused test entrypoint that exits with a stable
  non-pass skip when its explicit owned PostgreSQL DSN prerequisite is absent and never
  falls back to an ambient database.

## 4. Observability And Release Integration

- [x] 4.1 Add low-cardinality, redacted telemetry for replay completion, fixture
  validation failures, isolation/lifecycle failures, and baseline comparison decisions;
  test metric labels and ordinary logs for prohibited data.
- [x] 4.2 Add CI/release-gate wiring that runs deterministic unit coverage always and
  runs real PostgreSQL replay only against a harness-owned database with bounded timeout
  and cleanup behavior.
- [x] 4.3 Document local execution, explicit prerequisite skip semantics, fixture
  ownership, report interpretation, threshold changes, rollout evidence, rebuildability,
  and cleanup/retention expectations in the self-hosting and retrieval-quality docs.
- [x] 4.4 Run focused retrieval, PostgreSQL, telemetry, docs, and full-suite tests;
  execute the owned real-PostgreSQL replay; record a redacted `canonical-v1` /
  `baseline-v1` baseline and validate the change with `openspec validate
  retrieval-evaluation-baseline --strict`.
