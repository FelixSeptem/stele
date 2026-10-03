## MODIFIED Requirements

### Requirement: Reasoning candidates are bounded and exact-scope

The service SHALL accept reasoning derivation only for one resolved tenant,
project, and namespace scope with an explicit operation mode, time/evidence
budget, lifecycle visibility policy, redaction policy, provider contract, and
insight-type contract. For `goal`, the request MUST also carry bounded goal
state metadata and any declared validity interval.

#### Scenario: Candidate generation has an authorized scope

- **WHEN** an offline or shadow derivation request includes an authorized exact scope, bounded limits, and valid goal metadata when the type is `goal`
- **THEN** the service evaluates only eligible evidence in that scope and records the resolved scope proof in the candidate envelope

#### Scenario: Candidate generation lacks a required bound

- **WHEN** a request omits exact scope, lifecycle visibility, redaction policy, execution limits, or required goal state metadata
- **THEN** the service rejects the request before provider invocation or evidence scanning

### Requirement: Reasoning candidates are evidence-backed and source-watermarked

Every reasoning candidate SHALL cite one or more eligible source records with
source watermarks, lifecycle state, provenance, and a stable evidence digest;
the service MUST reject or quarantine candidates with missing, foreign, hidden,
stale, redacted, or unverifiable evidence. A `goal` candidate MUST include at
least one eligible citation and preserve any bounded validity interval used by
its policy.

#### Scenario: Candidate cites eligible evidence

- **WHEN** provider output cites evidence that remains visible, in scope, and covered by the request watermark
- **THEN** the service records the citations, evidence digest, provenance, freshness result, and goal validity metadata when applicable

#### Scenario: Candidate cites hidden or foreign evidence

- **WHEN** provider output cites evidence outside the scope or excluded by lifecycle or redaction policy
- **THEN** the service records a bounded rejection or quarantine reason without exposing the cited record

### Requirement: Offline and shadow results are deterministic and non-authoritative

The service SHALL derive a deterministic replay identity from normalized input,
scope proof, evidence watermark, provider/schema identity, policy versions,
and any type-specific metadata such as goal state and validity. Offline and
shadow runs MUST NOT mutate canonical memory, active insight state, default
retrieval, or ordinary context assembly.

#### Scenario: Identical inputs are replayed

- **WHEN** the same normalized request, source watermark, provider contract, policy versions, and goal metadata are replayed
- **THEN** the service produces the same candidate fingerprint and bounded disposition without remote invocation

#### Scenario: Shadow candidate would pass activation

- **WHEN** shadow evaluation finds a candidate, including a `goal`, that satisfies the current activation gates
- **THEN** the service records a non-authoritative would-activate or review-required result while active records and ordinary behavior remain unchanged
