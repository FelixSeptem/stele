## ADDED Requirements

### Requirement: Insight derivation uses a bounded provider-neutral envelope

The reasoning provider boundary SHALL define a provider-neutral request and
response for insight derivation with exact scope, redaction and lifecycle
constraints, evidence references, output schema/version, uncertainty bounds,
and a non-authoritative operation mode.

#### Scenario: Provider receives bounded evidence

- **WHEN** the service invokes a provider for offline or shadow derivation
- **THEN** the request contains only authorized evidence references/content, bounded by scope, watermark, redaction, and execution limits

#### Scenario: Provider response is normalized

- **WHEN** a provider returns one or more insight candidates
- **THEN** the adapter normalizes type, claims, citations, uncertainty, provenance, and refusal/errors into the shared envelope without preserving raw provider payloads

### Requirement: Provider fallback is safe for reasoning derivation

The boundary MUST fail closed on unsupported capability, timeout, malformed
output, budget exhaustion, or incompatible schema and MUST report a bounded
fallback category without converting provider failure into an active insight.

#### Scenario: Provider times out

- **WHEN** a provider exceeds its invocation or evidence budget
- **THEN** the service returns a bounded incomplete/fallback result and leaves canonical and active derived state unchanged

#### Scenario: Provider emits unrecognized output

- **WHEN** a provider emits an unsupported type, unbound citation, or unsafe mutation request
- **THEN** the adapter quarantines the result and returns a stable validation category to the caller
