## ADDED Requirements

### Requirement: Adaptive planning requires compatible release evidence
A planner policy MUST remain diagnostics-only or shadowed until compatible repository-owned evaluation and explicitly owned PostgreSQL and pgvector evidence demonstrate zero safety failures, preserved protected-family quality, bounded resources, deterministic replay, safe reranker behavior, and tested fallback/rollback.

#### Scenario: Real-stack evidence is unavailable
- **WHEN** the evaluation DSN is absent or the real-stack run is skipped
- **THEN** the stable non-pass result cannot authorize active planning and baseline remains unchanged
