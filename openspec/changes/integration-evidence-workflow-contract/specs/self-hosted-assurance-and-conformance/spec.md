## ADDED Requirements

### Requirement: Workflow health participates in conformance and readiness
The service SHALL include recent integration workflow completion and gap diagnostics in conformance runs, health evaluations, readiness reports, incidents, alert candidates, and recovery verification.

#### Scenario: Conformance run checks workflow evidence
- **WHEN** a conformance profile requires workflow evidence for an external integration
- **THEN** the conformance run inspects scoped workflow runs, step completion, evidence links, and gap diagnostics without executing the external agent

#### Scenario: Readiness is degraded by incomplete workflow
- **WHEN** recent required workflow runs are missing, stale, incomplete, blocked, or contain required gap diagnostics
- **THEN** the readiness report records degraded or unknown integration readiness with bounded workflow diagnostic categories

#### Scenario: Workflow gap creates incident candidate
- **WHEN** workflow gap diagnostics exceed configured severity or freshness thresholds
- **THEN** the assurance loop can create or update a scoped incident and alert candidate with bounded component, severity, reason category, and recommended admin surfaces

#### Scenario: Recovery verification references workflow run
- **WHEN** recovery verification is requested after a workflow-related incident or conformance failure
- **THEN** the recovery report can link workflow run, step, evidence, diagnostic, conformance, proof, or session evidence without overwriting prior history
