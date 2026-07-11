## ADDED Requirements

### Requirement: Proof and session failures can reference quality evaluation
The service SHALL allow proof and memory-session reports to create or reference scoped quality evaluations when memory quality affects proof or session verdicts.

#### Scenario: Proof detects retrieval quality failure
- **WHEN** a proof run misses expected fixture recall or observes lifecycle-hidden output
- **THEN** the proof report links to a scoped quality evaluation with bounded finding codes

#### Scenario: Session verification detects degraded recall
- **WHEN** session verification cannot recall expected turn outcome evidence
- **THEN** the session report can link to a quality evaluation or finding summary for the same scope

### Requirement: Proof and session repair recommendations remain approval-gated
The service MUST NOT automatically approve repair actions from proof or memory-session failures.

#### Scenario: Proof produces actionable findings
- **WHEN** proof-linked quality findings can be mapped to embedding retry, governance requeue, derived insight replay, or manual review
- **THEN** the report may recommend a repair plan but leaves approval to an authorized admin action

#### Scenario: Session produces actionable findings
- **WHEN** session verification produces actionable quality findings
- **THEN** the service preserves the existing repair planning approval boundary and does not run repairs inline with the session request
