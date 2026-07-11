# insight-quality-feedback Specification

## Purpose
TBD - created by archiving change governed-insight-quality-feedback. Update Purpose after archive.
## Requirements
### Requirement: Insight quality feedback is durable and scoped
The service SHALL persist quality feedback for derived insights as durable records scoped by tenant, project, and namespace.

#### Scenario: Operator records feedback for an insight
- **WHEN** an authorized operator records feedback for a derived insight within an authorized scope
- **THEN** the service stores the insight identity, scope, feedback type, actor attribution, reason, observed time, and audit metadata as a durable feedback record

#### Scenario: Feedback targets an out-of-scope insight
- **WHEN** an operator attempts to record feedback for an insight outside the authorized tenant, project, or namespace
- **THEN** the service rejects the request without creating a feedback record

### Requirement: Feedback uses bounded quality signals
The service MUST classify insight feedback with bounded quality signal values that can be consumed by policy, derivation, diagnostics, and context assembly.

#### Scenario: Feedback type is supported
- **WHEN** feedback is recorded with a supported type such as `useful`, `noisy`, `incorrect`, `stale`, `redundant`, or `needs_review`
- **THEN** the service accepts the feedback when the rest of the request is valid

#### Scenario: Feedback type is unsupported
- **WHEN** feedback is recorded with an unsupported type
- **THEN** the service rejects the feedback instead of storing an unrecognized quality signal

### Requirement: Feedback preserves insight evidence history
The service MUST NOT rewrite or delete derived insight evidence, provenance, or lifecycle history when feedback is recorded.

#### Scenario: Negative feedback is recorded
- **WHEN** an operator records `noisy`, `incorrect`, or `stale` feedback for an insight
- **THEN** the service preserves the insight's linked evidence and lifecycle history while recording the feedback separately

#### Scenario: Feedback is superseded
- **WHEN** an authorized operator supersedes a prior feedback record
- **THEN** the service records the supersession with actor and reason attribution without deleting the prior feedback row

### Requirement: Effective feedback state is queryable
The service SHALL provide scoped reads that summarize effective feedback state for derived insights.

#### Scenario: Feedback summary is requested
- **WHEN** an internal caller or admin inspection path requests quality state for an authorized derived insight
- **THEN** the service returns active feedback counts or signals by type, current review indicators, and supersession-aware effective state

#### Scenario: Superseded feedback exists
- **WHEN** a feedback summary is computed for an insight with superseded feedback records
- **THEN** the service excludes superseded records from active quality signals while preserving them in audit history reads

