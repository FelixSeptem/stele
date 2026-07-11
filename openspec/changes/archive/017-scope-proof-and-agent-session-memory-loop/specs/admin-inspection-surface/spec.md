## ADDED Requirements

### Requirement: Scope proof reports are inspectable through the admin surface
The service SHALL expose admin-only inspection of scope proof runs and reports.

#### Scenario: Administrator lists proof runs
- **WHEN** an authorized administrator lists proof runs for a scope
- **THEN** the admin surface returns matching proof runs with status, verdict, timestamps, actor attribution, and bounded summary counts

#### Scenario: Administrator reads proof report
- **WHEN** an authorized administrator reads a proof report within scope
- **THEN** the admin surface returns step evidence, failure categories, linked evaluations, linked repair plans, linked replay runs, and recommended next actions

#### Scenario: Administrator reads out-of-scope proof report
- **WHEN** an administrator requests a proof report outside the authorized scope
- **THEN** the admin surface rejects the request without exposing report existence or content

### Requirement: Memory session reports are inspectable through scoped boundaries
The service SHALL expose scoped inspection of memory session runs and reports.

#### Scenario: Caller reads session report
- **WHEN** an authorized caller reads a session report within scope
- **THEN** the service returns session turns, context evidence summaries, outcome event ids, verification status, and bounded failure reasons

#### Scenario: Caller reads out-of-scope session report
- **WHEN** a caller requests a memory session report outside the authorized scope
- **THEN** the service rejects the request without exposing session content
