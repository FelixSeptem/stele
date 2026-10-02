## ADDED Requirements

### Requirement: Replay accepts a normalized reasoning candidate envelope

The replay service SHALL support a normalized reasoning envelope containing
exact scope proof, eligible evidence citations and watermark, provider/schema
identity, policy versions, operation mode, and deterministic replay identity.

#### Scenario: Reasoning envelope is replayed

- **WHEN** an authorized bounded replay receives a complete normalized envelope
- **THEN** it evaluates the candidate against the same evidence and activation governance as scheduled derivation

#### Scenario: Envelope is incomplete or stale

- **WHEN** replay cannot verify the envelope's scope, evidence watermark, provider contract, or policy freshness
- **THEN** it records a stable stale or incomplete disposition and performs no activation

### Requirement: Replay distinguishes non-authoritative reasoning outcomes

Replay reports SHALL distinguish rejected, quarantined, skipped, would-activate,
and applied derived-insight outcomes for reasoning candidates, while offline or
shadow reasoning replay leaves active insight state unchanged.

#### Scenario: Replay predicts activation

- **WHEN** a reserved reasoning candidate would satisfy a current policy during a non-authoritative replay
- **THEN** the report records a would-activate result with compatibility versions and does not create or update an active insight

#### Scenario: Replay is applied under governance

- **WHEN** an operator submits a bounded apply run for an eligible candidate
- **THEN** durable work invokes the ordinary admission lifecycle with idempotency and audit history rather than bypassing governance
