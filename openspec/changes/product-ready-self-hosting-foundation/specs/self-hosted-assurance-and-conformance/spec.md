## ADDED Requirements

### Requirement: Executable restore verification can satisfy backup/restore proof
The assurance loop SHALL accept bounded evidence from the repository's
documented restore-verification procedure as backup/restore proof only after
the result identifies a successful manifest-validated restore, current schema
validation, and scope-safe service proof. It SHALL remain a diagnostic record
and SHALL NOT execute backup or restore itself.

#### Scenario: Operator records successful restore verification
- **WHEN** an authorized administrator records a successful restore-verification
  result with its bounded manifest/checksum reference, verification time, schema
  result, and scoped proof result
- **THEN** the next health evaluation can treat backup/restore proof as fresh
  within the configured window and includes only lifecycle-safe aggregate
  evidence in readiness reports

#### Scenario: Restore verification is stale or incomplete
- **WHEN** restore-verification evidence is missing, failed, older than the
  configured freshness window, missing schema validation, or missing scoped
  proof outcome
- **THEN** assurance reports backup/restore as unknown, stale, degraded, or
  unhealthy and does not promote the scope to production-ready

