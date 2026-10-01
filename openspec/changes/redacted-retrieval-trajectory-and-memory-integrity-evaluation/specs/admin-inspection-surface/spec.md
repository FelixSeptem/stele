## ADDED Requirements

### Requirement: Redacted trajectory and integrity reports are admin-inspectable

The admin surface SHALL expose exact-scope inspection of completed trajectory,
memory-integrity, replay, and retention reports. Responses MUST be bounded and
redacted, MUST enforce the caller's grants before resolving report existence,
and MUST not expose raw queries, content, hidden identifiers, foreign scope
values, provider payloads, or credentials.

#### Scenario: Administrator reads a report within scope

- **WHEN** an authorized administrator requests a completed report for an exact scope
- **THEN** the service returns bounded aggregate categories, logical policy and source-watermark identities, verdicts, and cleanup status

#### Scenario: Administrator requests an out-of-scope report

- **WHEN** an administrator requests a report outside the authorized exact scope
- **THEN** the service rejects the request without exposing report existence or evidence details
