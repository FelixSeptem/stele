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

### Requirement: Replay uses the unified derived work queue

Derived insight replay and backfill SHALL enqueue bounded, exact-scope work items
with stable replay identity, source watermark, actor/reason attribution, and
idempotent retry behavior.

#### Scenario: Operator applies a replay plan

- **WHEN** an authorized operator submits a bounded replay apply
- **THEN** the service records a durable work identity and returns before broad replay execution

#### Scenario: Replay worker restarts

- **WHEN** replay execution stops after partial progress
- **THEN** the next worker resumes from the durable checkpoint and does not duplicate insight lifecycle transitions

### Requirement: Replay loss is not reported as applied

A replay task dropped in `memory_buffer` mode, rejected during flush, or exhausted
without completion MUST remain non-applied and visible as a bounded disposition.

#### Scenario: Replay work is dropped

- **WHEN** an unflushed replay task is lost
- **THEN** the replay report records a dropped/non-applied category and active insights remain unchanged

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

### Requirement: Replay reports contradiction temporal dispositions

Bounded replay SHALL preserve contradiction keys, both source-version
identities, valid-time intervals, source watermarks, and policy versions, and
SHALL distinguish `contradiction`, `temporal_coexistence`,
`unresolved_temporal`, `stale_evidence`, `review_required`, and
`would_activate` outcomes.

#### Scenario: Replay finds a temporal coexistence

- **WHEN** replay evaluates incompatible fact values whose valid-time intervals do not overlap
- **THEN** the report records temporal coexistence and does not schedule activation

#### Scenario: Replay finds an overlapping contradiction

- **WHEN** replay evaluates two visible, mutually exclusive fact versions with overlapping intervals
- **THEN** the report records a deterministic contradiction candidate and either review-required or would-activate according to policy

#### Scenario: Replay evidence is stale

- **WHEN** either source version or its watermark no longer satisfies the replay contract
- **THEN** the report records stale evidence and performs no activation or source refresh
