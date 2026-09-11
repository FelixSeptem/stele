## Purpose

Define bounded, deterministic query understanding and multi-signal retrieval that improves temporal, entity-centric, mixed-language, ambiguous, and multi-hop recall without replacing the caller query or weakening retrieval safety.

## ADDED Requirements

### Requirement: Versioned deterministic query analysis
The service SHALL analyze an accepted retrieval query through a selected,
versioned, provider-independent policy that produces deterministic bounded
normalization, aliases or terms, and optional entity, time, memory-class, and
intent hints. Equivalent inputs under the same policy version and limits MUST
produce the same ordered analysis result, including explicit unknown or
unavailable categories instead of invented values.

#### Scenario: Equivalent query is replayed
- **WHEN** the same accepted query and analysis inputs are evaluated repeatedly under the same policy version and bounds
- **THEN** the service produces the same ordered normalized terms, hints, subqueries, and analysis disposition

#### Scenario: A hint cannot be derived safely
- **WHEN** the analysis policy cannot deterministically resolve an entity, time window, memory class, or intent
- **THEN** the service records a bounded unknown or absent category and does not fetch broader data or invent a hint

#### Scenario: Analysis policy changes
- **WHEN** normalization, alias, hint, decomposition, or limit behavior changes
- **THEN** the analysis policy version changes and authorized evaluation identifies the effective version

### Requirement: Original query is immutable and retained
The service MUST preserve the accepted caller query as an immutable, mandatory
retrieval signal. Normalized terms, hints, and subqueries SHALL be additive and
MUST NOT replace, mutate, or persist over the original query.

#### Scenario: Derived signals are accepted
- **WHEN** query analysis produces one or more valid derived signals
- **THEN** the original query remains the first mandatory retrieval signal and every derived signal is additional

#### Scenario: Normalization changes query text
- **WHEN** normalization produces text different from the accepted caller query
- **THEN** the service retains the original query unchanged and does not treat normalized text as the caller's canonical query

### Requirement: Bounded subquery decomposition
The service SHALL enforce versioned limits on derived signal count, subquery
count, term length, subquery length, analysis work, and retrieval fan-out.
Accepted derived signals MUST be deterministically ordered and deduplicated;
over-limit, empty, duplicate, or malformed derived signals MUST NOT consume
retrieval fan-out.

#### Scenario: Analysis produces repeated subqueries
- **WHEN** normalization, aliases, or decomposition produce equivalent derived signals
- **THEN** the service retains one deterministically ordered signal within the configured bounds

#### Scenario: Analysis exceeds a configured bound
- **WHEN** derived terms, hints, subqueries, or analysis work exceed an effective versioned limit
- **THEN** the service rejects or truncates the excess deterministically, records a bounded over-limit category, and preserves the original query path

#### Scenario: Multi-hop query is decomposed
- **WHEN** an accepted query contains independently retrievable facts that the deterministic policy recognizes
- **THEN** the service may add a bounded ordered set of subqueries without requiring an online model or replacing the original query

### Requirement: Exact-scope lifecycle-safe signal orchestration
Every accepted original or derived query signal MUST execute only within the
resolved tenant, project, namespace, and authorized optional session or user
scope, and MUST use the same lifecycle-visible bounded recall, stable fusion,
identity and validated-lineage deduplication, diversity selection, citation,
and result-budget controls. Query analysis MUST NOT widen a scope, lifecycle,
memory-class, or time constraint supplied or resolved for the request.

#### Scenario: Derived signal would widen scope
- **WHEN** a derived hint or subquery implies evidence outside the resolved request scope or lifecycle visibility
- **THEN** the service rejects that derived effect and neither retrieves nor discloses the foreign or hidden evidence

#### Scenario: Multiple signals recall the same evidence
- **WHEN** original and derived signals retrieve one canonical memory or validated source lineage through multiple paths
- **THEN** the service preserves one canonical result identity through the existing deterministic fusion, deduplication, and diversity path

#### Scenario: A time hint conflicts with an explicit request window
- **WHEN** a derived time hint falls partly or wholly outside an explicit authorized request time window
- **THEN** the service does not widen the explicit window and excludes the conflicting derived effect

### Requirement: Original-query fallback is fail closed
Failure, unavailability, rejection, timeout, malformed output, adversarial
output, or exhausted budget in query analysis MUST NOT fail an otherwise valid
retrieval request. The service SHALL use the immutable original query through
the existing stable retrieval pipeline and MUST NOT substitute partially
trusted derived scope or query data.

#### Scenario: Analyzer is unavailable
- **WHEN** query analysis is unavailable or fails before producing a valid bounded result
- **THEN** retrieval continues with the original query and records only an authorized bounded fallback category

#### Scenario: Every derived signal is rejected
- **WHEN** all derived signals are malformed, unsafe, duplicate, empty, or over budget
- **THEN** retrieval executes the original-query baseline without widening scope or changing the ordinary response contract

#### Scenario: Original retrieval path fails
- **WHEN** the underlying original-query retrieval path fails independently of query analysis
- **THEN** the service preserves the existing retrieval error semantics rather than masking that failure as an analysis fallback

### Requirement: Exact-scope reversible rollout
Query analysis and multi-signal effects SHALL use the existing exact-scope
rollout lifecycle with diagnostics-only, shadow, active, disabled, and rollback
dispositions. A missing, malformed, foreign-scope, expired, or unapproved
rollout MUST resolve to the original-query baseline, and shadow output MUST NOT
change ordinary ranked results.

#### Scenario: No approved rollout applies
- **WHEN** no approved query-analysis rollout applies to the exact resolved scope
- **THEN** the service uses the original-query baseline and does not activate derived signals

#### Scenario: Shadow rollout applies
- **WHEN** an approved query-analysis rollout is in shadow for the exact resolved scope
- **THEN** the service may evaluate bounded derived signals for authorized comparison while returning the original-query baseline result

#### Scenario: Active rollout is disabled or rolled back
- **WHEN** an operator disables or rolls back an active exact-scope query-analysis rollout
- **THEN** subsequent retrieval returns to the original-query baseline without rewriting memory or requiring data migration

### Requirement: Query-analysis diagnostics are bounded and authorized
The service SHALL expose query-analysis diagnostics only through authorized
evaluation or administrative paths. Such diagnostics MAY include bounded
policy identity, original-query-retained status, normalization status, hint and
subquery counts, time-window status, fallback category, rollout disposition,
and candidate-pool counts, but MUST NOT expose raw query plans, subquery text,
provider reasoning, hidden or foreign identifiers, raw internal candidates, or
unbounded scores. Ordinary retrieval responses MUST preserve their existing
shape.

#### Scenario: Authorized evaluator observes analysis
- **WHEN** an authorized evaluation executes a scoped query-analysis fixture
- **THEN** it receives only bounded category and count diagnostics sufficient to reproduce the policy disposition and compare it with the original-query baseline

#### Scenario: Ordinary retrieval executes
- **WHEN** a caller uses an ordinary retrieval API
- **THEN** the response does not expose analysis policy internals, normalized terms, hints, subquery text, query plans, or shadow-only results

#### Scenario: Unsafe derived evidence is encountered
- **WHEN** a derived signal encounters hidden or foreign evidence
- **THEN** diagnostics record only a stable aggregate exclusion or failure category and disclose no content, identifier, scope value, or score for that evidence

### Requirement: Active decomposition requires real-stack gate evidence
An exact-scope rollout MUST NOT activate query decomposition until an explicitly
owned PostgreSQL and pgvector evaluation proves the preceding approved
identity/lineage duplicate-rate, protected evidence coverage, candidate-budget,
and latency gates and proves the query-analysis candidate eligible against the
original-query baseline. Synthetic-only evidence or a skipped real-stack run
MUST NOT authorize activation.

#### Scenario: Owned real-stack evaluation passes all gates
- **WHEN** compatible real PostgreSQL and pgvector baseline and candidate reports have zero safety failures and satisfy the approved prerequisite and query-analysis release policies
- **THEN** the query-analysis candidate may become eligible for an exact-scope active rollout

#### Scenario: Evaluation DSN is absent
- **WHEN** the real-stack gate runs without an explicitly owned evaluation DSN
- **THEN** it records `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` as a non-pass state, does not use an ambient runtime DSN, and does not authorize active decomposition

#### Scenario: Preceding diversity gate fails
- **WHEN** duplicate-rate, protected evidence coverage, candidate-budget, latency, isolation, or lifecycle gates from the preceding retrieval stage are not green
- **THEN** query decomposition remains ineligible for active rollout regardless of synthetic or query-analysis quality gains
