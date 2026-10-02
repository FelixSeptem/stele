# retrieval-release-evidence-run Specification

## Purpose
This capability runs an explicitly owned retrieval evaluation against real
PostgreSQL and pgvector, compares progressive context and parent-first shadow
strategies, and produces redacted evidence that can never authorize release
when safety or prerequisite gates are incomplete.

## Requirements

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

### Requirement: Progressive and parent-first results remain shadow-only

The evaluator SHALL compare short retrieval projection, medium session/context
overview, canonical/chunk evidence, and bounded parent-first expansion against
the flat baseline on the same exact scope. These strategies MUST remain offline
or shadow-only and MUST NOT alter default retrieval, canonical memory, or public
search/context response behavior.

#### Scenario: Progressive levels are comparable

- **WHEN** a run evaluates all configured context levels
- **THEN** each level has a separate identity, source watermark, freshness
  category, budget, citation coverage, rebuild identity, and bounded metrics

#### Scenario: Parent-first expansion is evaluated

- **WHEN** parent-first shadow evaluation expands validated parents to children
  or adjacent chunks
- **THEN** expansion is exact-scope, bounded by candidate/latency limits, and
  compared with flat fusion without changing production ranking

#### Scenario: Experimental strategy would cross a safety boundary

- **WHEN** progressive or parent-first output includes foreign, hidden, stale,
  or lifecycle-ineligible evidence
- **THEN** the level records a hard isolation/lifecycle/freshness failure and
  remains ineligible regardless of quality metrics

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

### Requirement: Evaluation runs expose bounded operational closure

The service SHALL classify evaluation preflight and execution outcomes with
stable redacted categories, including missing or unowned DSN, ownership-marker
failure, evaluation-target reuse, PostgreSQL/pgvector prerequisite failure,
fixture incompatibility, timeout, and incomplete-run cleanup. A classified
failure MUST remain non-pass and MUST NOT consult or fall back to the runtime
service DSN.

#### Scenario: Preflight fails before database access
- **WHEN** the evaluation DSN, ownership marker, provider profile, exact scope,
  or PostgreSQL/pgvector prerequisite is missing or invalid
- **THEN** the run returns a stable skipped/degraded category, emits a redacted
  operator summary, and does not attempt the service database

#### Scenario: Evaluation target is reused
- **WHEN** the supplied evaluation target cannot be proven distinct from the
  runtime service target or its ownership marker is inconsistent
- **THEN** the run records an ownership/reuse non-pass category and cannot
  produce activation-eligible evidence

#### Scenario: Evaluation exceeds its bound
- **WHEN** an owned run reaches its configured timeout
- **THEN** the run terminates or is marked incomplete with a stable timeout
  category, removes incomplete artifacts, and remains non-pass

### Requirement: Evaluation artifacts have isolated cleanup and retention

Each evaluation run SHALL use an isolated report location and bounded cleanup
policy. Failed, timed-out, skipped, or incomplete runs MUST NOT leave
activation-consumable artifacts. Completed runs MAY retain only redacted
machine-readable and human-readable evidence with a stable run identity,
configured retention metadata, and append-only history semantics.

#### Scenario: Incomplete run is cleaned
- **WHEN** a run fails, is cancelled, or times out before producing a complete
  evidence bundle
- **THEN** its temporary fixture and report artifacts are removed or marked
  non-consumable, and cleanup outcome is visible as a bounded category

#### Scenario: Completed evidence is retained
- **WHEN** an owned run completes with a redacted evidence bundle
- **THEN** only the redacted bundle and bounded summary remain addressable by
  its stable run identity, while raw credentials, DSNs, source content, and
  provider payloads are absent

### Requirement: Evidence handoff is bound to source and run identity

An evaluation evidence bundle SHALL include a verifiable logical attestation
linking the stable run identity, exact-scope identity, source watermark and
freshness verdict, compatible fixture/policy identities, and rollback verdict.
Missing, stale, incompatible, or mismatched attestation data MUST make the
bundle non-pass and unusable for activation.

#### Scenario: Attestation matches the completed run
- **WHEN** an authorized operator reviews a completed bundle whose watermark,
  policy identities, scope identity, and run identity match
- **THEN** the bundle is reviewable as release evidence but remains separate
  from activation authorization

#### Scenario: Attestation is stale or mismatched
- **WHEN** a bundle references another run, an older source watermark, or an
  incompatible fixture or policy
- **THEN** the bundle records a bounded mismatch category and cannot authorize
  release or activation
