## ADDED Requirements

### Requirement: Usefulness feedback is inspectable through admin surfaces
The service SHALL expose admin-only inspection for feedback records, feedback summaries, and feedback-linked evidence.

#### Scenario: Administrator lists feedback records
- **WHEN** an authorized administrator lists usefulness feedback for a scope with optional subject, feedback type, session, turn, time, or source filters
- **THEN** the admin surface returns matching feedback records with attribution, bounded categories, subject references, and evidence links

#### Scenario: Administrator reads usefulness summary
- **WHEN** an authorized administrator inspects a memory, raw event, citation, derived insight, session, session turn, verification, or expected-recall target
- **THEN** the admin surface can return effective usefulness summary, dominant categories, latest feedback time, and linked quality or repair references

#### Scenario: Administrator inspects superseded feedback
- **WHEN** an authorized administrator includes superseded feedback in an inspection request
- **THEN** the admin surface returns original and superseding feedback records with attribution and active-summary participation state

#### Scenario: Administrator requests out-of-scope feedback
- **WHEN** an administrator requests feedback outside an authorized scope
- **THEN** the admin surface rejects the request without exposing feedback existence or target content

### Requirement: Session inspection includes feedback and verification history
The service SHALL include feedback summaries and verification history in authorized memory session inspection.

#### Scenario: Administrator reads a session report
- **WHEN** an authorized administrator reads a session report with recorded feedback or repeated verifications
- **THEN** the report includes bounded feedback summaries, verification history, outcome event references, quality evaluation links, repair recommendation links, and next actions
