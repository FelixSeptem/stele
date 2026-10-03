# governed-release-evidence-reconciliation Specification

## Purpose
Maintains a durable, exact-scope record of whether previously submitted release evidence remains eligible for governed activation as freshness, source, policy, and rollback conditions change over time.

## Requirements

### Requirement: Reconciliation evaluates exact-scope evidence compatibility

The service SHALL evaluate each activation evidence handoff against its exact project, tenant, namespace, release policy version, fixture identity, representation identity, source watermark, freshness deadline, and rollback proof requirements. A check MUST fail closed when a required identity or proof is missing, foreign, malformed, or no longer compatible.

#### Scenario: All evidence identities remain compatible

- **WHEN** reconciliation evaluates an active-scope handoff whose source watermark, policy, fixture, representation, freshness, attestation, and rollback proof all remain compatible
- **THEN** it records an eligible verdict for that reconciliation key without changing the handoff or ordinary retrieval behavior

#### Scenario: A required identity is missing or foreign

- **WHEN** reconciliation cannot resolve a required identity within the exact authorized scope
- **THEN** it records a fail-closed ineligible verdict with a bounded reason category and does not expose the missing evidence details

### Requirement: Reconciliation revokes current activation eligibility without deleting evidence

The service MUST maintain current activation eligibility separately from immutable evidence and prior verdicts. When freshness, watermark, policy, fixture, representation, attestation, or rollback requirements fail, the current eligibility MUST transition to a revoked state with a bounded reason category and actor or job attribution.

#### Scenario: Freshness expires

- **WHEN** an otherwise valid handoff passes its freshness deadline before the next reconciliation
- **THEN** reconciliation appends an expiry verdict and revokes current activation eligibility while retaining the original handoff and report

#### Scenario: Source watermark advances

- **WHEN** the source watermark recorded in a handoff is behind the governed current watermark for the exact scope
- **THEN** reconciliation records a mismatch and revokes current activation eligibility without changing canonical memory or historical evidence

### Requirement: Eligibility restoration requires a new compatible handoff

Reconciliation SHALL never promote an ineligible activation based solely on a repeated check. Restoration MUST consume a newly submitted compatible evidence handoff or a newly completed governed evaluation that satisfies the current release gate and rollback requirements.

#### Scenario: New evidence restores eligibility

- **WHEN** a new exact-scope handoff matches the current policy, watermark, fixture, representation, freshness, attestation, and rollback proof
- **THEN** the service records a new eligible transition linked to the new handoff and leaves all prior revoked history intact

#### Scenario: Replaying stale evidence cannot restore eligibility

- **WHEN** an operator replays reconciliation against the same stale handoff without supplying new compatible evidence
- **THEN** the service records an idempotent ineligible result and leaves current activation disabled

### Requirement: Reconciliation history is append-only and replay-safe

Each reconciliation run SHALL have a stable scope-bound identity, deterministic replay key, input watermark, policy identity, bounded verdict, reason category, and timestamps. Duplicate scheduler fires, retries, and manual replays MUST not create conflicting current state or duplicate effective transitions.

#### Scenario: Duplicate trigger arrives

- **WHEN** scheduler and worker receive the same scope and reconciliation window more than once
- **THEN** they converge on one durable run identity and one effective verdict while preserving attempt history

#### Scenario: A run resumes after failure

- **WHEN** a worker restarts after recording a checkpoint for a bounded reconciliation batch
- **THEN** it resumes from the monotonic checkpoint and produces the same verdicts for already processed evidence

### Requirement: Reconciliation is isolated and redacted

All reads, writes, diagnostics, and transitions MUST enforce project, tenant, and namespace boundaries. Reconciliation responses and records MUST use bounded reason categories and opaque references and MUST exclude raw evidence payloads, query text, provider output, credentials, and foreign scope values.

#### Scenario: Operator requests a foreign scope

- **WHEN** an administrator triggers or inspects reconciliation outside an authorized exact scope
- **THEN** the service rejects the request without revealing run existence, evidence counts, or eligibility state
