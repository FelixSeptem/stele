## ADDED Requirements

### Requirement: Feedback can create bounded quality findings
The service SHALL convert repeated active negative usefulness feedback, missing expected recall, and hidden-memory safety feedback into scoped quality findings or finding summaries.

#### Scenario: Repeated negative feedback is observed
- **WHEN** active feedback repeatedly marks recalled memory, citations, derived insights, or session context as noisy, stale, irrelevant, or needs review
- **THEN** the service can create a quality finding with bounded category, severity, component, subject kind, and scoped evidence links

#### Scenario: Negative feedback is superseded
- **WHEN** negative feedback has been superseded by an authorized correction
- **THEN** the service excludes that superseded feedback from new quality finding generation unless an administrator explicitly inspects historical evidence

#### Scenario: Expected recall is missing
- **WHEN** session feedback or verification records expected evidence that retrieval or context assembly did not recall
- **THEN** the service can create a retrieval or context quality finding without exposing hidden or out-of-scope memory content

#### Scenario: Hidden memory safety feedback is recorded
- **WHEN** feedback indicates suppressed, forgotten, expired, deleted, or out-of-scope memory was visible through a non-admin path
- **THEN** the service records a high-severity quality finding using stable lifecycle safety codes

### Requirement: Feedback-derived repairs remain approval-gated
The service MUST keep feedback-derived repair recommendations under existing admin approval boundaries.

#### Scenario: Feedback finding is actionable
- **WHEN** a feedback-derived finding maps to embedding retry, governance inspection, derived insight replay, suppression review, or manual review
- **THEN** the service may recommend a repair plan while leaving approval and execution to authorized admin actions

#### Scenario: Public feedback requests repair
- **WHEN** a public scoped caller records feedback that implies a repair
- **THEN** the service records feedback and quality evidence without approving or executing repair inline
