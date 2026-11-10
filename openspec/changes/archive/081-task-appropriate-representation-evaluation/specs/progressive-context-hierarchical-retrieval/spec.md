## ADDED Requirements

### Requirement: Task-family representation comparisons retain source retrieval

The capability SHALL compare the stable `flat-fusion-v1` baseline with declared
derived representations for exact-fact, cross-document, profile-preference,
and procedural-lesson task families.  A candidate representation SHALL be one
of exact excerpts plus search, topic projection plus search, or bounded
hierarchical/parent-first evidence plus search.  Every candidate SHALL retain
the authorized source-search channel, exact tenant/project/namespace and
lifecycle predicates, source watermark, renderer/policy identity, fixed
candidate and byte/character/token limits, and a deterministic replay
identity.  Derived output SHALL remain non-canonical and offline, diagnostic,
or shadow-only.

#### Scenario: A task-family candidate is compared
- **WHEN** an authorized evaluation supplies a compatible exact-scope fixture
  and a declared representation candidate for one task family
- **THEN** the report compares that candidate with `flat-fusion-v1` under the
  same visible source records and budgets, records bounded aggregate evidence,
  and leaves ordinary retrieval and context responses unchanged

#### Scenario: A candidate attempts to replace search or widen visibility
- **WHEN** a representation candidate omits its authorized source-search
  companion, has an incompatible source watermark, or reaches foreign, hidden,
  stale, or lifecycle-ineligible evidence
- **THEN** the candidate is a stable non-pass, the run records only a bounded
  category, and the experiment retains the flat baseline without broader lookup

### Requirement: Representation comparison reports preserve task-family safety

Representation comparison artifacts SHALL report only task-family and
representation identities, aggregate critical-fact retention, source/citation
coverage, broken-reference and stale-result categories, bounded representation
size and maintenance-cost observations, task-utility observation coverage,
replay identity, fallback, and rollback disposition.  The artifacts MUST NOT
contain task text, source text, queries, answer text, raw scores, scope values,
memory identifiers, source identifiers, provider payloads, credentials, or
per-stage costs.  Quality or observational task-utility differences MUST NOT
override an isolation, lifecycle, freshness, deterministic-replay,
broken-reference, or resource-bound failure.

#### Scenario: A comparable candidate produces an aggregate report
- **WHEN** all declared source, representation, and task observations are
  compatible and remain within the configured bounds
- **THEN** the report emits deterministic, redacted, family-level aggregates
  and labels condition differences as observational rather than causal or
  activation-authorizing

#### Scenario: A representation loses a critical fact or exceeds a bound
- **WHEN** a candidate loses required exact evidence, produces a broken
  reference, exceeds a representation bound, or cannot reproduce its declared
  replay identity
- **THEN** the run is non-pass, records a bounded failure/fallback category,
  requires rollback to the baseline for that run, and cannot influence ordinary
  retrieval
