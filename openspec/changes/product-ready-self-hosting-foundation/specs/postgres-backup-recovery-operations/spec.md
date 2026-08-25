## ADDED Requirements

### Requirement: Operators receive executable PostgreSQL backup guidance
The repository SHALL provide a documented, executable operator command for
creating a PostgreSQL backup of a Stele deployment. The command MUST require an
explicit source target, avoid printing credentials, produce a verifiable backup
artifact and manifest, and state its PostgreSQL client prerequisites.

#### Scenario: Operator creates a backup
- **WHEN** an operator supplies a valid explicit PostgreSQL source target and
  backup destination to the documented backup command
- **THEN** the command creates a portable backup artifact and a manifest with
  checksum, creation time, service compatibility metadata, and schema version
  without echoing secret material

#### Scenario: Backup command is misconfigured
- **WHEN** source connection information, destination, client tooling, or
  required explicit confirmation is missing or invalid
- **THEN** the command fails before writing an ambiguous or partial backup and
  reports an actionable non-secret diagnostic

### Requirement: Restore commands require a safe explicit target
The repository SHALL provide a documented, executable restore command that
requires an explicit target and refuses an implicit, broad, or source-equal
restore target. It MUST NOT delete, overwrite, or recreate a database unless
the operator supplies the command's documented explicit destructive
confirmation.

#### Scenario: Operator restores into a disposable verification database
- **WHEN** an operator provides a backup artifact, manifest, and distinct
  explicit disposable target with required confirmation
- **THEN** the restore command validates the artifact and manifest before
  restoring and reports the target it restored without exposing credentials

#### Scenario: Restore target is unsafe
- **WHEN** the requested target is empty, ambiguous, equal to the source,
  missing destructive confirmation, or cannot be verified as the intended
  target
- **THEN** the command refuses to execute destructive PostgreSQL operations

### Requirement: Restore verification proves usable scoped data
The repository SHALL provide a bounded restore-verification procedure that
checks migration currency and proves that restored data remains usable through
authorized scoped service behavior. Successful verification SHALL be recordable
as backup/restore proof for the existing assurance and readiness loop.

#### Scenario: Restored database verifies successfully
- **WHEN** an operator starts Stele against a restored verification target with
  valid scoped credentials and runs the restore-verification command
- **THEN** it validates schema currency, runs a bounded scope-safe read proof,
  reports the manifest/checksum reference and outcome, and identifies how to
  record the successful result as assurance proof

#### Scenario: Restore verification fails
- **WHEN** migration validation, service startup, authorization, or bounded
  scoped proof fails against the restored target
- **THEN** the verification reports a stable failed category, does not mark
  backup/restore proof healthy, and directs the operator to the relevant
  migration, bootstrap, or recovery diagnostic

