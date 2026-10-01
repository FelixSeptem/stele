# derived-insight-replay Specification

## Purpose
Provide bounded, administrator-authorized dry-run planning for derived-insight replay.

## Requirements

### Requirement: Derived insight replay planning is bounded and admin-only
The service SHALL provide an admin-only dry-run capability that plans derived insight replay for an authorized scope, bounded evidence window, and explicit execution limits without mutating derived insights or canonical memory.

#### Scenario: Operator previews replay impact
- **WHEN** an authorized operator requests a dry-run replay for one tenant, project, namespace, insight type set, time window, and limit
- **THEN** the service returns a replay plan with selected evidence counts, candidate insight fingerprints, expected create/update/suppress/skip decisions, and any validation warnings without applying those decisions

#### Scenario: Replay request lacks bounds
- **WHEN** an operator requests replay without an authorized scope, bounded time window, or execution limit
- **THEN** the service rejects the request before scanning evidence or scheduling replay work

### Requirement: Derived insight replay apply is durable and auditable
The service MUST execute replay apply or backfill through durable background work with actor attribution, reason, idempotency, retry state, and a replay report.

#### Scenario: Operator applies a replay plan
- **WHEN** an authorized operator submits a bounded replay apply request with actor and reason attribution
- **THEN** the service records a replay run, returns its durable identity, and makes the run eligible for worker execution instead of applying broad mutations inline

#### Scenario: Replay apply is retried
- **WHEN** a replay apply run is retried after worker failure or restart
- **THEN** the service uses replay identity and insight fingerprints to avoid duplicate insight records or duplicate lifecycle transitions

### Requirement: Replay reports explain outcomes

The service SHALL persist replay reports that explain replay selection,
decisions, skipped records, failures, feedback-influenced lifecycle effects,
and reserved-insight activation-policy compatibility. Reports MUST distinguish
non-authoritative would-activate dispositions from applied lifecycle changes
and MUST retain the policy, provider contract, source watermark, and reason
versions used for the decision.

#### Scenario: Replay completes

- **WHEN** a replay run finishes
- **THEN** the service stores counters for evidence evaluated, insights created, insights updated, insights suppressed, insights preserved, records skipped, non-authoritative would-activate candidates, and failures, together with stable reason codes and compatibility versions

#### Scenario: Replay skips an insight

- **WHEN** replay excludes a candidate because of scope, lifecycle, unsupported type, insufficient evidence, feedback policy, stale activation policy, incompatibility, or idempotency
- **THEN** the replay report records the skip category without requiring direct PostgreSQL inspection

#### Scenario: Replay evaluates a reserved candidate

- **WHEN** replay evaluates a reserved insight candidate under an enabled policy
- **THEN** the report records whether the candidate would be rejected, quarantined, or admitted while the replay itself leaves active insight state unchanged

### Requirement: Replay preserves canonical memory and evidence history
The service MUST keep replay limited to derived insight evaluation and SHALL NOT rewrite raw events, canonical memories, memory versions, vector revisions, or existing provenance in place.

#### Scenario: Replay evaluates historical evidence
- **WHEN** replay derives an updated insight decision from historical events, memory, job, feedback, or recovery records
- **THEN** the service records derived insight changes and replay audit history separately from canonical memory history

#### Scenario: Replay would require canonical rewrite
- **WHEN** a replay request asks to rewrite canonical memory content, memory versions, vector revisions, or event provenance
- **THEN** the service rejects the request as unsupported
