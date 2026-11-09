## ADDED Requirements

### Requirement: Procedural verification lineage is retained in authorized history
Authorized provenance/history inspection SHALL link a procedural canonical
version to its append-only verification records, evidence/source-version
references, revalidation dispositions, and any verification-attributed intent,
candidate, or successor version. The lineage MUST remain exact-scope and
lifecycle-safe and MUST preserve prior verification and canonical history after
correction or source drift.

#### Scenario: Operator follows a verification-driven successor
- **WHEN** an authorized operator inspects a procedural successor created from
  a refuted verification
- **THEN** the service returns bounded links to the target version, verification,
  intent, candidate, predecessor, and source watermark without exposing foreign
  or hidden content

#### Scenario: Source drift invalidates a verification
- **WHEN** a cited source later changes or becomes ineligible
- **THEN** history retains the original verification and exposes the later
  needs-revalidation lineage without rewriting either record
