## ADDED Requirements

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
