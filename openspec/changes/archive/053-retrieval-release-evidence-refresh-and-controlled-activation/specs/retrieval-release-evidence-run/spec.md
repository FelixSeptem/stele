## MODIFIED Requirements

### Requirement: Evaluation run is explicitly owned and fail-closed
The service SHALL accept a release-evidence run only when the operator supplies
an explicitly owned evaluation DSN, ownership marker, provider profile, exact
scope, compatible fixture/policy identities, and bounded timeout. Each run MUST
use isolated fixtures, deterministic cleanup, a stable redacted run identity,
and a freshness verdict. Missing, stale, incompatible, or unavailable
prerequisites MUST produce a bounded skipped or degraded result and MUST NOT be
reported as release-ready. The evaluator MUST never fall back to the service
database DSN.

#### Scenario: Owned real-stack run executes
- **WHEN** an operator supplies a valid owned evaluation DSN, ownership marker,
  PostgreSQL/pgvector prerequisites, compatible profiles, and an exact
  authorized scope
- **THEN** the service runs the release evidence with bounded timeout and
  cleanup, records redacted logical identities, freshness, quality metrics,
  safety outcomes, latency, replay, and rollback evidence, and preserves the
  stable baseline until activation is separately authorized

#### Scenario: Evaluation DSN is absent
- **WHEN** the run is requested without an explicitly owned evaluation DSN or
  ownership marker
- **THEN** the result is `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` or a stable
  ownership non-pass category, no service DSN is consulted, and readiness is
  not eligible

#### Scenario: Evaluation DSN is unowned
- **WHEN** the run is requested without the explicit ownership marker
- **THEN** the result is a stable ownership non-pass category, no service DSN
  is consulted, and readiness is not eligible

#### Scenario: Required prerequisite is unavailable
- **WHEN** PostgreSQL/pgvector, fixture compatibility, source freshness,
  semantic-hit proof, integrity summary, projection freshness, or rollback
  evidence is unavailable, stale, or incompatible
- **THEN** the run records skipped/degraded categories and cannot pass the
  release gate or authorize activation

#### Scenario: Required prerequisite is stale
- **WHEN** source freshness, projection freshness, or rollback evidence is
  outside its configured window
- **THEN** the run records a stable stale/degraded category and cannot pass the
  release gate or authorize activation

### Requirement: Release evidence is redacted, reproducible, and reviewable
The service SHALL emit machine-readable and human-readable reports containing
only a stable run identity, bounded logical identities, aggregate
quality/resource metrics, safety categories, latency buckets,
watermark/freshness state, citation coverage, rebuild identity, replay result,
and rollback verdict. Reports MUST exclude DSNs, credentials, queries,
prompts, source content, raw scores, memory/event identifiers, hidden records,
foreign scope values, and provider payloads. Repeated runs over identical
source records and compatible policies MUST be deterministic while preserving
prior derived reports as append-only history.

#### Scenario: Repeated rebuild is deterministic
- **WHEN** identical PostgreSQL source records are rebuilt with the same policy,
  renderer, strategy versions, and fixed evaluation clock
- **THEN** the run identity inputs, derived level identity, item ordering, and
  aggregate verdict categories are stable while prior reports remain history

#### Scenario: Safety failure overrides quality gain
- **WHEN** quality improves but the run has a scope, lifecycle, freshness,
  integrity, replay, or rollback failure
- **THEN** the final verdict is non-pass, no activation eligibility is emitted,
  and the approved baseline remains active

#### Scenario: Authorized report is read
- **WHEN** an authorized evaluation or admin caller reads a completed report
- **THEN** the caller receives bounded redacted evidence and no raw trajectory,
  provider internals, DSN, scope value, source content, or identifier

### Requirement: Owned evidence runs include redacted integrity summaries
An explicitly owned retrieval evidence run SHALL include bounded trajectory and
memory-integrity summaries when the configured evaluation profile requests
them. The run MUST fail closed when required summaries are unavailable,
incompatible, stale, or unsafe, and MUST exclude raw source and provider data.
The summary MUST be linked to the stable run identity and source watermark so
that activation cannot consume evidence from a different or older run.

#### Scenario: Owned run records integrity summaries
- **WHEN** an authorized owned evaluation completes with compatible trajectory
  and organization checks
- **THEN** the report contains the stable run identity, logical policy and
  watermark identities, aggregate categories, safety verdicts, and bounded
  latency/resource outcomes

#### Scenario: Required summary is unavailable
- **WHEN** the run cannot produce a required redacted trajectory or integrity
  summary, or the summary is stale or linked to another run/watermark
- **THEN** the run is skipped or degraded with a stable non-pass category and
  cannot authorize release or activation

#### Scenario: Required summary is mismatched
- **WHEN** the summary is stale or linked to another run or source watermark
- **THEN** the run is skipped or degraded with a stable non-pass category and
  cannot authorize release or activation
