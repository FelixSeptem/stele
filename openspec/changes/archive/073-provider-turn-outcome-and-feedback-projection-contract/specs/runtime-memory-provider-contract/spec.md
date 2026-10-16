## ADDED Requirements

### Requirement: Strict consumers negotiate and validate projection evidence

Strict runtime-memory Provider consumers SHALL negotiate
`turn-outcome-feedback-projection-v1`, validate closed serialized request and
response DTOs, distinguish accepted/pending/completed/failed/unavailable
reference attribution and correction dispositions, and reject incompatible or
missing required fields without interpreting evidence as runtime business state.

#### Scenario: Consumer observes a pending durable reference
- **WHEN** a valid projection response reports pending reference attribution
- **THEN** the consumer records it as pending evidence and does not treat it as
  a completed memory reference, task completion or ranking input

#### Scenario: Consumer receives malformed projection output
- **WHEN** required identity, revision, attribution state or business-state
  disclaimer fields are missing, null, aliased or incompatible
- **THEN** the consumer reports a failed contract outcome without local fallback

