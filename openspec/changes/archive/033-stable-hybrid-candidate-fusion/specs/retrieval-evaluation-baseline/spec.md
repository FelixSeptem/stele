## MODIFIED Requirements

### Requirement: Deterministic retrieval replay and report
The service SHALL execute a selected fixture through the real scoped retrieval
path and produce a deterministic, bounded report identifying fixture,
representation, selected fusion strategy name and version, ranking, and
compatible embedding-revision metadata.

#### Scenario: Replay evaluates the current retrieval implementation
- **WHEN** an operator or CI runs a valid fixture against an owned PostgreSQL
  harness
- **THEN** the evaluator seeds only the fixture scope, executes its queries
  through the lexical, semantic, enabled relation, and authorized chunk retrieval
  paths, and emits machine-readable and human-readable reports with the
  effective fusion strategy identity

#### Scenario: Replay compares a candidate with baseline
- **WHEN** an evaluator is given a baseline and candidate report with compatible
  fixture and representation versions
- **THEN** it reports per-metric deltas, protected-category regressions, the
  strategy versions, and the ranking versions that produced both reports

#### Scenario: Real database prerequisite is absent locally
- **WHEN** the local replay command is invoked without an explicitly owned
  PostgreSQL test DSN
- **THEN** it exits with a stable non-pass skip category and does not connect to
  a default, ambient, or operator database

### Requirement: Redacted and bounded evaluation diagnostics
The evaluator SHALL expose diagnostics only through local, CI, or authorized
administrative paths and SHALL redact sensitive or hidden material while retaining
bounded strategy identity, candidate channel, channel rank, fusion disposition,
and final-rank evidence for visible evaluated results.

#### Scenario: Diagnostic records a visible candidate disposition
- **WHEN** a lifecycle-visible expected or returned memory is evaluated
- **THEN** diagnostics can record fixture alias, selected strategy name/version,
  candidate channel, channel rank, bounded fusion disposition, final rank, and
  bounded inclusion or omission reason

#### Scenario: Diagnostic encounters hidden or foreign evidence
- **WHEN** an evaluator detects a hidden or foreign candidate
- **THEN** the report records only a stable aggregate failure or exclusion
  category and does not include content, memory ID, source ID, fusion score, or
  foreign scope values

#### Scenario: Report is rendered
- **WHEN** a machine-readable or human-readable report is generated
- **THEN** it excludes credentials, DSNs, raw source event payloads, full
  database errors, unbounded query plans, and raw per-provider score values
