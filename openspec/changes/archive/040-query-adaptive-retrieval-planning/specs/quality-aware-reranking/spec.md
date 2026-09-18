## ADDED Requirements

### Requirement: Planner eligibility cannot independently activate reranking
An adaptive retrieval plan MAY mark reranking eligible, but the service MUST invoke or apply a reranker only when the separate reranker runtime mode, provider identity, exact-scope rollout, evidence gate, limits, timeout, and remaining request budget also authorize it.

#### Scenario: Plan permits but rollout denies reranking
- **WHEN** a valid plan marks reranking eligible but no compatible exact-scope reranker rollout is active
- **THEN** the service retains the deterministic quality and fusion baseline without invoking an active reranker
