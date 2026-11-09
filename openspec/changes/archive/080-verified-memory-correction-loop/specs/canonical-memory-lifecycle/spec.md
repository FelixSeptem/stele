## ADDED Requirements

### Requirement: Verification-driven procedural correction preserves canonical history
A procedural-memory correction that originates from a refuted verification SHALL
enter canonical consolidation only through an attributed governed update. The
service MUST retain the verified target version and verification reference in
successor provenance, append a new candidate/version when accepted, and leave
the prior canonical version unchanged.

#### Scenario: Refuted procedure is promoted as a successor
- **WHEN** governed processing accepts an attributed correction for a refuted
  current procedural-memory version
- **THEN** the service appends a successor canonical version and retains the
  former version, verification attribution, and ordinary lifecycle history

#### Scenario: Verification correction loses eligibility
- **WHEN** target version, verification evidence, lifecycle, scope, or policy
  is no longer eligible before consolidation
- **THEN** the service records a nonactive governed outcome and preserves the
  existing canonical version without an in-place rewrite
