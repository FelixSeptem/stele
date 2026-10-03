## ADDED Requirements

### Requirement: Goal review and visibility are inspectable through the admin surface

The admin inspection surface SHALL provide an authorized exact-scope view of goal review and experimental visibility decisions. It MUST support bounded review attribution, policy version and status, replay and freshness categories, evidence eligibility, inclusion or omission outcomes, and rollback history while preserving the redaction and isolation rules of the goal capability.

#### Scenario: Admin inspects a scope

- **WHEN** an authorized administrator requests goal review diagnostics for one exact scope
- **THEN** the response includes bounded decision categories and audit attribution without goal content, prompts, provider payloads, or foreign identifiers

#### Scenario: Admin requests an unauthorized scope

- **WHEN** the administrator lacks the required scope proof or principal grant
- **THEN** the surface returns a bounded authorization failure without revealing hidden-record existence
