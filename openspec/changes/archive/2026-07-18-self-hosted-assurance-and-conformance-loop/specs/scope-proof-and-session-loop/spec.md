## ADDED Requirements

### Requirement: Proof and session evidence support conformance checks
The service SHALL allow assurance and conformance runs to reference proof and memory-session evidence without changing proof or session execution semantics.

#### Scenario: Conformance run checks session evidence chain
- **WHEN** a conformance profile requires session, context, outcome, and verification evidence
- **THEN** the conformance run can inspect scoped memory session records and report bounded missing-evidence diagnostics

#### Scenario: Conformance run references proof status
- **WHEN** a conformance profile requires recent scope proof evidence
- **THEN** the conformance run can include proof verdict, degraded components, and failure categories in its diagnostic summary

#### Scenario: Proof or session evidence is out of scope
- **WHEN** a conformance run encounters proof or session references outside the requested scope
- **THEN** the service excludes them from conformance evidence and does not expose their existence

### Requirement: Recovery verification can rerun proof and session checks
The service SHALL allow recovery verification to reference or dispatch existing proof and session verification checks while preserving history.

#### Scenario: Recovery verification reruns proof
- **WHEN** an incident recommends validating scope usability after remediation
- **THEN** recovery verification can reference a new proof run linked to the incident without overwriting prior proof history

#### Scenario: Recovery verification reruns session verification
- **WHEN** a conformance or health failure involved session recall evidence
- **THEN** recovery verification can request a new session verification attempt and link the result to the recovery report

#### Scenario: Recovery verification does not execute agent
- **WHEN** recovery verification checks memory session health
- **THEN** Stele verifies service-side memory evidence without invoking the external agent or generating a final answer
