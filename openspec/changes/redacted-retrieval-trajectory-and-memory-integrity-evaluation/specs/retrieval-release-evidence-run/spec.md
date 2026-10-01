## ADDED Requirements

### Requirement: Owned evidence runs include redacted integrity summaries

An explicitly owned retrieval evidence run SHALL include bounded trajectory and
memory-integrity summaries when the configured evaluation profile requests
them. The run MUST fail closed when required summaries are unavailable,
incompatible, stale, or unsafe, and MUST exclude raw source and provider data.

#### Scenario: Owned run records integrity summaries

- **WHEN** an authorized owned evaluation completes with compatible trajectory and organization checks
- **THEN** the report contains logical identities, aggregate categories, safety verdicts, and bounded latency/resource outcomes

#### Scenario: Required summary is unavailable

- **WHEN** the run cannot produce a required redacted trajectory or integrity summary
- **THEN** the run is skipped or degraded with a stable non-pass category and cannot authorize release
