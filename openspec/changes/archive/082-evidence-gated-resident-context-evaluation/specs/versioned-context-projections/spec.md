## ADDED Requirements

### Requirement: Resident placement experiments require current evidence

The service SHALL evaluate proposed resident promotion, retention and demotion
offline or in shadow against the existing profile confidence/size policy.
Promotion SHALL require exact-scope active current-valid source references,
matching source versions and claim fingerprints, independent attributable
support, explicit stability/applicability and fresh held-out utility evidence.
Reading frequency MUST NOT confer correctness or resident authority. Existing
always-visible class restrictions and source disclosure lineage SHALL remain.
Support, utility and comparison events SHALL require disclosable `attested_data`
source attribution under existing exact-scope attestation authorization; a body
actor MUST NOT establish trust, truth or instruction authority. Observation
fields SHALL use exact canonical names and reject duplicate keys, case aliases
and nulls. Only validated compatible evidence for eligible evaluated sources
SHALL advance the stage required of subsequent comparison outcomes.

#### Scenario: Popular unverified evidence is considered
- **WHEN** a frequently read profile lacks compatible independent support or
  held-out utility evidence
- **THEN** it remains on-demand and the experiment records insufficient evidence
  without changing canonical memory or the live resident projection

#### Scenario: Resident evidence becomes stale or corrected
- **WHEN** a resident source changes version or its support, applicability or
  utility becomes invalid
- **THEN** the experiment proposes demotion with a bounded reason while retaining
  source history, citations and on-demand access where otherwise eligible

#### Scenario: Evidence crosses a safety boundary
- **WHEN** a candidate or observation is foreign, hidden, outside valid time,
  disclosure-ineligible, unresolved or version-incompatible
- **THEN** it cannot enter the candidate projection and no aggregate benefit
  overrides the failure

#### Scenario: Untrusted or ambiguously encoded evidence is supplied
- **WHEN** an event is disclosable but untrusted, or uses a case alias to override
  a support, utility or comparison field
- **THEN** the evidence fails closed without promotion or accepted comparison

#### Scenario: Discarded observations carry later stages
- **WHEN** unrelated, foreign, expired, untrusted or invalid-stage observations
  accompany valid evidence
- **THEN** they do not raise the stage required of a compatible later comparison
