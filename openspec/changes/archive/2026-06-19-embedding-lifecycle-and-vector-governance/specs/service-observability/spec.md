## MODIFIED Requirements

### Requirement: Operational metrics and tracing hooks
The service MUST expose baseline operational signals for API, governance, retrieval, and lifecycle workloads.

#### Scenario: Operator monitors service behavior over time
- **WHEN** the service handles ingest, governance, retrieval, or forgetting work
- **THEN** the runtime exposes metrics or tracing hook points for latency, throughput, error rate, and backlog-oriented inspection

#### Scenario: Operator monitors embedding lifecycle backlogs
- **WHEN** semantic projection work accumulates because embeddings are missing, stale, failed, or provider-mismatched
- **THEN** the runtime exposes backlog and execution telemetry for embedding rebuild eligibility, attempted generation, promotion outcomes, and provider or model drift processing
