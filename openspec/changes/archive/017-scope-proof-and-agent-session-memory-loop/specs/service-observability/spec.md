## ADDED Requirements

### Requirement: Proof and session metrics are exported
The service MUST expose low-cardinality metrics for scope proof and memory session execution.

#### Scenario: Proof step completes
- **WHEN** a proof step completes, fails, retries, or is skipped
- **THEN** metrics record step, status, verdict, component, and failure category without tenant, project, namespace, proof id, session id, event id, memory id, actor, or reason labels

#### Scenario: Session verification completes
- **WHEN** session verification completes, degrades, fails, or requires manual review
- **THEN** metrics record verification status and failure category without high-cardinality identifiers

### Requirement: Proof and session diagnostics are operator-visible
The service SHALL expose diagnostics that explain proof and memory-session loop health.

#### Scenario: Operator inspects loop health
- **WHEN** an operator requests diagnostics for an authorized scope
- **THEN** the service can summarize recent proof verdicts, session verdicts, dominant failure categories, pending verification work, and recommended next admin surfaces

#### Scenario: Diagnostics reference hidden evidence
- **WHEN** hidden lifecycle state contributes to a proof or session failure
- **THEN** diagnostics expose aggregate counts and stable reason codes without exposing hidden memory content through public metrics
