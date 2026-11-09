## ADDED Requirements

### Requirement: Procedural verification and revalidation are admin-inspectable
The admin inspection surface SHALL provide exact-scope, bounded creation,
triggering, listing, and detail inspection for procedural-memory verification
and revalidation. It MUST return stable disposition, policy, source-watermark,
attempt, correction-handoff, and audit metadata while enforcing grants before
record resolution and excluding raw hidden evidence content, prompts,
credentials, and foreign identifiers.

#### Scenario: Administrator inspects revalidation history
- **WHEN** an authorized administrator requests revalidation history for an
  in-scope procedural memory
- **THEN** the response includes stable verification/revalidation status and
  bounded audit links without exposing hidden or foreign evidence

#### Scenario: Administrator requests foreign verification history
- **WHEN** an administrator lacks the exact scope grant for a verification or
  revalidation record
- **THEN** the service rejects the request without revealing its existence,
  target, evidence count, or disposition
