## MODIFIED Requirements

### Requirement: Reasoning outputs remain candidates or governed intents

Reasoning output SHALL be validated as bounded candidate records or governed
memory/insight intents containing scope, evidence citations, provenance,
derivation policy/version, provider metadata, confidence or uncertainty, and
replay identifiers. Provider output MUST NOT directly write canonical memory,
change lifecycle state, grant access, alter server configuration, or activate a
reserved insight type. A validated reserved candidate MAY enter a separate,
explicitly enabled activation policy that performs admission, lifecycle, audit,
and rollback decisions outside the provider boundary.

#### Scenario: Evidence-backed candidate is returned

- **WHEN** a provider returns a structurally valid derivation with allowed evidence and bounded metadata
- **THEN** the service records it as a candidate or intent for ordinary governance and preserves the source evidence and provider provenance

#### Scenario: Reserved candidate enters governed activation

- **WHEN** a provider returns a valid reserved insight candidate and an exact-scope activation policy is enabled for its type
- **THEN** the service passes the candidate to the separate admission policy as non-authoritative input and records no activation until that policy accepts it

#### Scenario: Provider returns an unsupported insight type

- **WHEN** a provider proposes `hypothesis`, `goal`, `contradiction`, or `causal_link` while that type is not enabled by a separate policy
- **THEN** the service rejects or quarantines the proposal and does not create an active insight

#### Scenario: Provider requests direct mutation

- **WHEN** a provider response asks to overwrite canonical memory, delete evidence, or set an active lifecycle state directly
- **THEN** the service rejects the mutation and retains no canonical side effect
