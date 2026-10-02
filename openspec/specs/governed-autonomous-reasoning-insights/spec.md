# governed-autonomous-reasoning-insights Specification

## Purpose
Provide a bounded, evidence-backed contract for generating and evaluating
reasoning-derived insight candidates before any separately governed activation.

## Requirements

### Requirement: Reasoning candidates are bounded and exact-scope

The service SHALL accept reasoning derivation only for one resolved tenant,
project, and namespace scope with an explicit operation mode, time/evidence
budget, lifecycle visibility policy, redaction policy, and provider contract.

#### Scenario: Candidate generation has an authorized scope

- **WHEN** an offline or shadow derivation request includes an authorized exact scope and bounded limits
- **THEN** the service evaluates only eligible evidence in that scope and records the resolved scope proof in the candidate envelope

#### Scenario: Candidate generation lacks a required bound

- **WHEN** a request omits exact scope, lifecycle visibility, redaction policy, or an execution limit
- **THEN** the service rejects the request before provider invocation or evidence scanning

### Requirement: Reasoning candidates are evidence-backed and source-watermarked

Every reasoning candidate SHALL cite one or more eligible source records with
source watermarks, lifecycle state, provenance, and a stable evidence digest;
the service MUST reject or quarantine candidates with missing, foreign, hidden,
stale, redacted, or unverifiable evidence.

#### Scenario: Candidate cites eligible evidence

- **WHEN** provider output cites evidence that remains visible, in scope, and covered by the request watermark
- **THEN** the service records the citations, evidence digest, provenance, and freshness result with the candidate

#### Scenario: Candidate cites hidden or foreign evidence

- **WHEN** provider output cites evidence outside the scope or excluded by lifecycle or redaction policy
- **THEN** the service records a bounded rejection or quarantine reason without exposing the cited record

### Requirement: Offline and shadow results are deterministic and non-authoritative

The service SHALL derive a deterministic replay identity from normalized input,
scope proof, evidence watermark, provider/schema identity, and policy versions.
Offline and shadow runs MUST NOT mutate canonical memory, active insight state,
default retrieval, or ordinary context assembly.

#### Scenario: Identical inputs are replayed

- **WHEN** the same normalized request, source watermark, provider contract, and policy versions are replayed
- **THEN** the service produces the same candidate fingerprint and bounded disposition without remote invocation

#### Scenario: Shadow candidate would pass activation

- **WHEN** shadow evaluation finds a candidate that satisfies the current activation gates
- **THEN** the service records a non-authoritative would-activate result while active records and ordinary behavior remain unchanged

### Requirement: Provider output and dependencies fail closed

The service MUST quarantine or skip reasoning output when provider capability,
schema, freshness, timeout, budget, redaction, or evidence dependencies are
missing or incompatible. Provider output MUST NOT request direct activation,
canonical mutation, evidence deletion, or policy bypass.

#### Scenario: Provider dependency is stale

- **WHEN** the provider contract or source watermark is stale, missing, or incompatible
- **THEN** the run is marked incomplete or stale and no candidate is eligible for activation

#### Scenario: Provider requests an unsafe operation

- **WHEN** provider output requests canonical mutation, direct activation, or deletion of evidence
- **THEN** the service rejects or quarantines the output and leaves canonical and derived records unchanged
