## ADDED Requirements

### Requirement: Temporal validity is a release gate
The release policy SHALL require compatible current/historical evidence proving
zero stale-fact wins, temporal provenance mismatches, validity ambiguity,
scope/lifecycle leakage, and unbounded temporal resource use before an
active-for-scope temporal retrieval rollout.

#### Scenario: Temporal candidate passes all gates
- **WHEN** owned PostgreSQL + pgvector evidence is compatible and all temporal,
  quality, safety, latency, fallback, and rollback gates pass
- **THEN** the policy may authorize the explicitly requested scoped rollout stage

#### Scenario: Temporal gate is skipped or fails
- **WHEN** temporal evidence is absent, skipped, incompatible, or contains a
  stale-fact or provenance failure
- **THEN** the policy keeps temporal behavior at diagnostics/shadow or approved
  current baseline and records a stable non-pass category
