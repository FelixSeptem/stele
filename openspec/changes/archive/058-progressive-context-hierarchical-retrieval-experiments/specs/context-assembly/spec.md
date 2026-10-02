## ADDED Requirements

### Requirement: Progressive context is confined to authorized shadow envelopes

Context assembly MAY consume progressive levels or parent-first plans only for
an explicitly authorized diagnostic or shadow request. It MUST preserve the
existing section names, caller budget, citation contract, lifecycle visibility,
diversity policy, and exact scope, and MUST omit shadow-only output from
ordinary public responses.

#### Scenario: Authorized shadow context is requested
- **WHEN** an authorized diagnostic request supplies compatible level and plan
  identities for an exact scope
- **THEN** assembly reports bounded inclusion, omission, freshness, and budget
  diagnostics using the existing response envelope

#### Scenario: Ordinary context request has shadow candidates
- **WHEN** an ordinary caller requests context while shadow candidates exist
- **THEN** assembly uses the stable baseline and returns no experimental content
  or strategy internals

### Requirement: Progressive omission is fail-closed and budget-safe

An ineligible, stale, hidden, foreign, over-budget, or over-limit progressive
item MUST be omitted without increasing the caller budget, widening scope, or
fetching unvalidated children. Authorized diagnostics MAY expose only a stable
reason category and bounded counts.

#### Scenario: Projection cannot fit the envelope
- **WHEN** a progressive item exceeds the remaining context budget or fails
  lineage validation
- **THEN** it is omitted with a bounded diagnostic and baseline packing remains
  unchanged
