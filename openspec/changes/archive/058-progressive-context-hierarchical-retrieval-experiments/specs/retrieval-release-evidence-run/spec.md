## ADDED Requirements

### Requirement: Experiment evidence is bound to exact scope and freshness

An owned evidence run that includes progressive or parent-first results MUST
record the exact-scope identity, source and projection watermarks, freshness
window, strategy identity, stable baseline identity, and fallback disposition.
Missing, stale, mismatched, or unowned values MUST make the evidence non-pass
and unusable for activation.

#### Scenario: Evidence handoff matches the run
- **WHEN** an owned run completes with compatible source, projection, strategy,
  and baseline identities inside the freshness window
- **THEN** the redacted evidence can participate in release review without
  changing default behavior

#### Scenario: Evidence is stale or mismatched
- **WHEN** a report references an older watermark, another scope, or an
  incompatible strategy or baseline
- **THEN** the run records a stable mismatch category and cannot authorize
  activation
