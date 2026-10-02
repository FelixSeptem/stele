## ADDED Requirements

### Requirement: Reasoning insights require explicit context authorization

The context assembly service MUST exclude reasoning-derived candidates and
reserved insights from default retrieval and ordinary context. A reasoning
section may be included only when an authorized policy explicitly enables it
for the exact scope and supplies lifecycle, evidence, freshness, citation, and
budget checks.

#### Scenario: Default context is assembled

- **WHEN** a normal retrieval or context request has no reasoning-section authorization
- **THEN** reasoning candidates and reserved insights are absent from the result

#### Scenario: Authorized reasoning section is requested

- **WHEN** an authorized request enables a reasoning section with compatible policy and fresh evidence
- **THEN** only eligible, cited, in-scope, non-hidden records within the section budget are included

### Requirement: Non-authoritative reasoning output cannot affect context

The context assembler SHALL ignore offline, shadow, stale, quarantined, or
would-activate reasoning results even when they are available in diagnostics
or replay reports.

#### Scenario: Shadow result is present

- **WHEN** a shadow run contains a candidate that would activate under policy
- **THEN** ordinary context assembly excludes it until a separate governed activation creates an eligible insight
