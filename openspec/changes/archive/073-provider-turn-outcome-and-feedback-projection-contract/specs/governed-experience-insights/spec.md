## ADDED Requirements

### Requirement: Runtime projection evidence enters insight derivation only through governed asynchronous policy

The service SHALL allow an eligible runtime projection and its active scoped
feedback to be cited by asynchronous insight derivation only with versioned
policy, current lifecycle/grant checks, bounded revision/provenance identity and
audit attribution. A projection MUST NOT autonomously activate an insight or
rewrite prior projection, feedback or insight evidence.

#### Scenario: Projection evidence lacks eligibility
- **WHEN** a derivation observes pending, failed, unavailable, foreign,
  superseded-only or lifecycle-hidden projection evidence
- **THEN** it preserves or records the ineligible disposition and creates no
  active insight from that evidence

