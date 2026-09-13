## ADDED Requirements

### Requirement: Maintenance controls projection freshness eligibility

Projection maintenance SHALL persist bounded rebuild/checkpoint state and
source/projection watermark freshness evidence. A projection with missing,
stale, divergent, foreign, or lifecycle-hidden evidence MUST remain excluded
from ordinary retrieval until a successful exact-scope rebuild revalidates it.

#### Scenario: Exact-scope rebuild revalidates a projection

- **WHEN** maintenance rebuilds a projection from PostgreSQL source records with a matching policy and renderer identity
- **THEN** it records a deterministic completion and the projection becomes eligible only after freshness and lifecycle checks pass

#### Scenario: Rebuild encounters hidden evidence

- **WHEN** a rebuild discovers suppressed, forgotten, deleted, or foreign evidence
- **THEN** it records a fail-closed lifecycle or isolation category and leaves the affected projection ineligible
