## ADDED Requirements

### Requirement: Integration workflows are inspectable through admin surfaces
The service SHALL expose admin-only inspection and bounded controls for integration workflow templates, runs, steps, evidence links, gap diagnostics, next actions, and workflow retention records.

#### Scenario: Administrator manages workflow template
- **WHEN** an authorized administrator creates, updates, disables, reads, or lists integration workflow templates within scope
- **THEN** the admin surface validates bounded step and evidence requirements and returns template state through the admin boundary

#### Scenario: Administrator inspects workflow run
- **WHEN** an authorized administrator reads a workflow run for a scope
- **THEN** the admin surface returns run status, step summaries, evidence link summaries, gap diagnostics, next actions, transitions, and bounded timestamps without requiring direct PostgreSQL access

#### Scenario: Administrator supersedes bad evidence link
- **WHEN** an authorized administrator supersedes an invalid workflow evidence link with actor and reason attribution
- **THEN** the admin surface records an auditable transition and preserves the prior link history without mutating the source evidence record

#### Scenario: Administrator requests out-of-scope workflow record
- **WHEN** an administrator requests a workflow template, run, step, evidence link, diagnostic, or next action outside an authorized scope
- **THEN** the admin surface rejects the request without exposing record existence or evidence counts
