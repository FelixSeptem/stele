## ADDED Requirements

### Requirement: Scope health evaluations are durable and scoped
The service SHALL persist operational health evaluations for one tenant, project, and namespace using bounded status, severity, component, and reason categories.

#### Scenario: Administrator runs health evaluation
- **WHEN** an authorized administrator requests a health evaluation for a scope
- **THEN** the service records runtime, backlog, dependency, proof, session, feedback, task, repair, ranking rollout, conformance, capacity/load, and backup/restore health summaries for that scope

#### Scenario: Health evaluation targets unauthorized scope
- **WHEN** an administrator requests a health evaluation outside an authorized scope
- **THEN** the service rejects the request without exposing health state, incidents, or evidence from that scope

#### Scenario: Health source is stale
- **WHEN** a required health source has no recent evidence inside the configured freshness window
- **THEN** the evaluation records `unknown` or `stale` status with bounded reason codes rather than treating missing evidence as healthy

### Requirement: Incidents preserve operational degradation history
The service SHALL create durable incident records for degraded or unhealthy assurance findings without mutating source evidence.

#### Scenario: Health evaluation detects degraded condition
- **WHEN** a health evaluation detects backlog pressure, failed repair execution, unsafe feedback, insufficient ranking rollout evidence, task-success degradation, conformance failure, capacity/load proof failure, or backup/restore proof failure
- **THEN** the service can create or update a scoped incident with severity, component, status, reason category, observed window, runbook hints, and linked evaluation references

#### Scenario: Incident is acknowledged or resolved
- **WHEN** an authorized administrator acknowledges, suppresses, or resolves an incident with actor and reason attribution
- **THEN** the service records an incident transition while preserving the original incident and evidence history

#### Scenario: Incident references hidden evidence
- **WHEN** hidden, suppressed, forgotten, deleted, or out-of-scope evidence contributes to an incident
- **THEN** the incident exposes only aggregate lifecycle-safe diagnostics and stable reason codes outside authorized admin detail

### Requirement: Alert candidates and delivery attempts are bounded
The service SHALL derive alert candidates from assurance and conformance findings and optionally attempt self-host-friendly delivery.

#### Scenario: Alert candidate is generated
- **WHEN** an incident or health evaluation meets configured alert criteria
- **THEN** the service records a bounded alert candidate with severity, component, reason category, deduplication key, delivery policy, and recommended admin surfaces

#### Scenario: Alert delivery is disabled
- **WHEN** the configured alert adapter is `disabled`
- **THEN** the service records the alert candidate and skips delivery without marking the underlying incident resolved

#### Scenario: Webhook delivery is attempted
- **WHEN** the configured alert adapter is `webhook` and an alert candidate is eligible
- **THEN** the service sends a bounded payload only after validating URL scheme, network target safety, timeout, payload size, redirect behavior, and header safety, and records delivery result, attempt count, failure category, and next retry time without exposing webhook URL or high-cardinality evidence in metrics

#### Scenario: Webhook target is unsafe
- **WHEN** webhook configuration points to an unsupported scheme, unsafe local or metadata target, unapproved insecure endpoint, oversized payload, or rejected header
- **THEN** the service fails configuration validation or records a delivery failure category without attempting the unsafe outbound request

### Requirement: Operational proof checks cover capacity and backup restoration
The service SHALL include bounded service-owned capacity/load and backup/restore proof evidence in assurance evaluations.

#### Scenario: Capacity proof is within thresholds
- **WHEN** recent capacity/load proof evidence shows backlog depth, scheduler latency, worker latency, and bounded query or processing checks within configured thresholds
- **THEN** the health evaluation records the capacity/load component as healthy with bounded evidence counters

#### Scenario: Capacity proof exceeds thresholds
- **WHEN** recent capacity/load proof evidence exceeds configured thresholds or reaches configured processing bounds
- **THEN** the health evaluation records degraded or unhealthy capacity/load status and can create an incident with recommended admin surfaces

#### Scenario: Backup restore proof is fresh
- **WHEN** a scope has recent backup/restore proof evidence inside the configured freshness window
- **THEN** the health evaluation records the backup/restore component as healthy using bounded status, timestamp, checksum or marker reference, and operator-supplied proof metadata

#### Scenario: Backup restore proof is missing or stale
- **WHEN** backup/restore proof evidence is absent, failed, or older than the configured freshness window
- **THEN** the health evaluation records unknown, stale, degraded, or unhealthy backup/restore status rather than reporting the scope as production-ready

### Requirement: Integration conformance profiles are durable and scoped
The service SHALL allow authorized administrators to define integration conformance profiles for expected external-agent evidence chains.

#### Scenario: Administrator creates conformance profile
- **WHEN** an administrator creates a profile for a scope with expected session, context, outcome, verification, usefulness feedback, task evaluation, and optional ranking rollout evidence
- **THEN** the service persists the profile with bounded required evidence kinds, freshness windows, minimum counts, actor, reason, and creation time

#### Scenario: Profile uses unsupported evidence kind
- **WHEN** a conformance profile references an unsupported free-form evidence kind
- **THEN** the service rejects the profile rather than creating unbounded conformance behavior

#### Scenario: Profile remains service-side
- **WHEN** a conformance profile is created or run
- **THEN** Stele does not execute the external agent, invoke models, build prompts, or generate final answers

### Requirement: Conformance runs detect missing integration evidence
The service SHALL run scoped conformance checks against durable Stele records and record missing, stale, contradictory, or out-of-scope evidence diagnostics.

#### Scenario: Conformance run passes
- **WHEN** a scope has the expected session, context, outcome, verification, feedback, task evaluation, and rollout evidence within the profile windows
- **THEN** the service records a passing conformance run with bounded evidence counters and linked record references for admin inspection

#### Scenario: Conformance run finds missing outcome
- **WHEN** a session turn exists but no bounded outcome evidence was recorded within the expected window
- **THEN** the service records a missing-evidence diagnostic such as `session_without_outcome` and recommends the session outcome admin or public route

#### Scenario: Conformance run finds incomplete task evidence
- **WHEN** a task evaluation exists without required scoped evidence links or with only opaque evidence where internal evidence is required
- **THEN** the service records a bounded diagnostic such as `task_evaluation_missing_evidence` without exposing out-of-scope target existence

### Requirement: Scope readiness reports combine assurance and conformance
The service SHALL expose a durable scope readiness report that summarizes operational health, integration conformance, memory-loop health, active incidents, alert candidates, and recommended next actions.

#### Scenario: Administrator reads readiness report
- **WHEN** an authorized administrator requests scope readiness
- **THEN** the service returns readiness status, component summaries, conformance status, recent proof/session verdicts, repair health, ranking rollout health, capacity/load status, backup/restore status, active incident counters, alert counters, and recommended admin surfaces

#### Scenario: Readiness is degraded by conformance failure
- **WHEN** runtime health is acceptable but the external-agent integration is missing required evidence
- **THEN** the readiness report returns degraded status with conformance diagnostics rather than reporting the scope as ready

#### Scenario: Readiness is degraded by operational proof failure
- **WHEN** runtime health and integration conformance are acceptable but capacity/load proof or backup/restore proof is failed, stale, or missing
- **THEN** the readiness report returns degraded or unknown status with operational proof diagnostics rather than reporting the scope as production-ready

#### Scenario: Readiness is unknown
- **WHEN** no recent health evaluation or conformance run exists for the scope
- **THEN** the readiness report returns unknown status with recommended actions to run health evaluation and conformance checks

### Requirement: Recovery verification preserves incident audit history
The service SHALL support recovery verification records that prove whether a prior incident or conformance failure has been remediated.

#### Scenario: Administrator requests recovery verification
- **WHEN** an authorized administrator requests recovery verification for an incident, alert candidate, conformance run, repair result, ranking rollback, proof run, session verification, capacity/load proof, or backup/restore proof
- **THEN** the service records verification status, checked surfaces, bounded result categories, linked evidence references, and recommended next actions

#### Scenario: Recovery verification succeeds
- **WHEN** the configured recovery checks pass for the affected scope
- **THEN** the service records successful verification and can mark linked incidents as resolved through an explicit auditable transition

#### Scenario: Recovery verification fails
- **WHEN** recovery checks still detect degraded or unhealthy conditions
- **THEN** the service records failed verification and preserves the linked incident or conformance failure for further remediation

### Requirement: Assurance and conformance do not mutate source records
The service MUST treat assurance evaluations, incidents, alert candidates, conformance runs, readiness reports, and recovery verification as diagnostic records only.

#### Scenario: Evaluation references canonical memory
- **WHEN** an assurance or conformance record references memory, feedback, task, repair, ranking, session, or proof evidence
- **THEN** the service preserves source records unchanged and stores diagnostic references or aggregate counters in the assurance records

#### Scenario: Incident recommends remediation
- **WHEN** an incident recommends repair, replay, ranking rollback, proof rerun, or session verification
- **THEN** the service points to existing governed admin surfaces instead of executing those actions inline

### Requirement: Assurance and conformance history retention is bounded
The service SHALL apply configurable retention and cleanup to high-volume assurance and conformance records while preserving incident audit history.

#### Scenario: Diagnostic history exceeds retention
- **WHEN** health evaluations, component summaries, alert delivery attempts, conformance runs, missing-evidence diagnostics, readiness reports, or recovery verification reports exceed configured retention windows
- **THEN** the service can clean up eligible high-volume records without deleting incident records or incident transition audit history

#### Scenario: Cleanup is rerun
- **WHEN** the assurance/conformance cleanup job is retried after a partial run or restart
- **THEN** cleanup remains idempotent and preserves tenant, project, and namespace isolation
