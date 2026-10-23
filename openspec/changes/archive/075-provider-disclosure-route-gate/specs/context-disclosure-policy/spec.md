## ADDED Requirements

### Requirement: Provider retrieval and turn routes enforce the current disclosure boundary

Public Provider retrieve and turn routes SHALL be treated as protected data
surfaces, not as exceptions to the disclosure policy. Each request MUST resolve
current exact-scope authority after runtime binding validation and before
retrieval or turn assembly. The default projection MUST be reference-only;
bounded content MUST require both the current `memory-content-v1` entitlement
and explicit `bounded_content`. Route selection, path selectors, turn inputs,
and response modes MUST NOT accept role or scope/session/turn overrides or
retrieve beyond the existing exact binding. Unauthorized or unavailable
authority MUST return the existing bounded 403 or retryable 503 category without
protected lookups or existence details.

#### Scenario: Grant changes after binding issuance

- **WHEN** a previously bound public principal has its disclosure grant narrowed or revoked before retrieve or turn creation
- **THEN** the next operation uses the current grant and is denied or reduced to an explicitly authorized projection

#### Scenario: Selector targets another memory path

- **WHEN** a public retrieval request supplies an exact path or prefix selector
- **THEN** the selector narrows candidates inside the bound exact scope and cannot widen scope, classification, lifecycle, or source eligibility

#### Scenario: Disclosure authority is unavailable

- **WHEN** the current grant store cannot provide authoritative disclosure entitlements
- **THEN** retrieve and turn routes return bounded retryable 503 and do not access protected candidates or persist a turn
