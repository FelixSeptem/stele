## Purpose

This capability runs an explicitly owned retrieval evaluation against real
PostgreSQL and pgvector, compares progressive context and parent-first shadow
strategies, and produces redacted evidence that can never authorize release
when safety or prerequisite gates are incomplete.

## ADDED Requirements

### Requirement: Evaluation run is explicitly owned and fail-closed

The service SHALL accept a release-evidence run only when the operator supplies
an explicitly owned evaluation DSN, provider profile, exact scope, and compatible
fixture/policy identities. Missing, stale, incompatible, or unavailable
prerequisites MUST produce a bounded skipped or degraded result and MUST NOT be
reported as release-ready. The evaluator MUST never fall back to the service
database DSN.

#### Scenario: Owned real-stack run executes

- **WHEN** an operator supplies a valid evaluation DSN, PostgreSQL/pgvector
  prerequisites, compatible profiles, and an exact authorized scope
- **THEN** the service runs the release evidence and records bounded logical
  identities, quality metrics, safety outcomes, latency, and rollback evidence

#### Scenario: Evaluation DSN is absent

- **WHEN** the run is requested without an explicitly owned evaluation DSN
- **THEN** the result is `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED`, no service DSN
  is consulted, and readiness is not eligible

#### Scenario: Required prerequisite is unavailable

- **WHEN** PostgreSQL/pgvector, fixture compatibility, projection freshness, or
  rollback evidence is unavailable
- **THEN** the run records skipped/degraded categories and cannot pass the
  release gate

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
only bounded logical identities, aggregate quality/resource metrics, safety
categories, latency buckets, watermark/freshness state, citation coverage,
rebuild identity, and rollback verdict. Reports MUST exclude DSNs, credentials,
queries, prompts, source content, raw scores, memory/event identifiers, hidden
records, foreign scope values, and provider payloads.

#### Scenario: Repeated rebuild is deterministic

- **WHEN** identical PostgreSQL source records are rebuilt with the same policy,
  renderer, and strategy versions
- **THEN** the derived level identity and item ordering are stable while prior
  derived artifacts remain append-only history

#### Scenario: Safety failure overrides quality gain

- **WHEN** quality improves but the run has a scope, lifecycle, freshness, or
  rollback failure
- **THEN** the final verdict is non-pass and no rollout eligibility is emitted

#### Scenario: Authorized report is read

- **WHEN** an authorized evaluation or admin caller reads a completed report
- **THEN** the caller receives bounded redacted evidence and no raw trajectory or
  provider internals
