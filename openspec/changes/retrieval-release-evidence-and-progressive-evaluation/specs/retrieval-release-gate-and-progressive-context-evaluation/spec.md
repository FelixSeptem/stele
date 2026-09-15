## MODIFIED Requirements

### Requirement: Owned real-provider release evidence

The evaluator SHALL run a real-provider retrieval release gate only through an
explicitly owned PostgreSQL and pgvector evaluation DSN, provider profile,
exact scope, and compatible fixture/policy identities. The report MUST identify
compatible fixture, representation, fusion, ranking, embedding, reranker,
analysis, and release-policy versions using logical identities, and MUST exclude
endpoints, credentials, DSNs, prompts, source text, and raw provider payloads.
A skipped, degraded, or failed prerequisite MUST be a stable non-pass result
and MUST NOT authorize an active rollout. The evaluator MUST NOT consult or
fall back to the service DSN.

#### Scenario: Owned evaluation runs

- **WHEN** an operator supplies a valid owned evaluation DSN and compatible
  provider profiles
- **THEN** the evaluator runs the scoped fixture against PostgreSQL + pgvector,
  emits machine-readable and human-readable redacted evidence, and records
  logical provider identities, dimensions or capability mode, candidate counts,
  fallback categories, protected metrics, and bounded latency

#### Scenario: Evaluation DSN is absent

- **WHEN** the release command is invoked without an explicitly owned evaluation
  DSN
- **THEN** it returns `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED`, does not fall
  back to any ambient service DSN, and cannot mark a candidate eligible

#### Scenario: Provider or fixture identity is incompatible

- **WHEN** a candidate report has incompatible fixture, representation, fusion,
  ranking, provider, analysis, or release-policy identity
- **THEN** comparison is rejected as incompatible and no quality gain can be
  used as release evidence
