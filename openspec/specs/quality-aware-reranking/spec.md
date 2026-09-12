# quality-aware-reranking Specification

## Purpose
Provide a measurable and reversible quality-aware ranking layer that can improve visible retrieval ordering while preserving Stele's deterministic baseline, scope isolation, lifecycle rules, and self-hosted operation.

## Requirements

### Requirement: Quality features are versioned, bounded, and visible-only
The service SHALL derive a named quality-feature version for each lifecycle-visible candidate from bounded evidence such as evidence coverage, freshness, source reliability, conflict state, usefulness feedback, task-success summaries, and verification outcomes.

#### Scenario: Candidate has complete quality evidence
- **WHEN** a visible candidate has active, scope-matching quality evidence
- **THEN** the service emits a bounded feature vector identified by a feature version and uses only the permitted aggregate categories for ranking

#### Scenario: Candidate evidence is missing or hidden
- **WHEN** a candidate has missing, superseded, hidden, forgotten, or foreign evidence
- **THEN** the service uses neutral or explicitly conservative defaults and excludes that evidence from score adjustments

### Requirement: Quality adjustments are deterministic and bounded
The service MUST apply quality-aware score adjustments only after validated stable fusion and before diversity or context packing, with explicit per-feature and total adjustment bounds and deterministic tie-breaking.

#### Scenario: Default quality policy is selected
- **WHEN** no exact-scope quality rollout is active
- **THEN** the service preserves the original fused scores and ordering

#### Scenario: Active quality policy is selected
- **WHEN** an exact-scope rollout is active and its evidence gates pass
- **THEN** the service applies the named feature version's bounded adjustment and records the effective policy identity without changing visibility

#### Scenario: Adjustment exceeds a bound
- **WHEN** feature contributions or the combined adjustment exceed configured bounds
- **THEN** the service clamps the adjustment deterministically and records a bounded diagnostic category

### Requirement: Optional model reranking fails closed
The service SHALL expose a provider-independent reranker contract and MAY call a configured external provider only in diagnostics, shadow, or explicitly approved exact-scope active mode.

#### Scenario: Reranker is disabled or unavailable
- **WHEN** reranking is disabled, unconfigured, times out, returns malformed output, or exceeds candidate/token limits
- **THEN** the service retains the deterministic quality/RRF baseline, records a redacted fallback status for authorized diagnostics, and does not fail ordinary retrieval solely because the optional provider is unavailable

#### Scenario: Provider returns an invalid candidate
- **WHEN** a provider returns an unknown, duplicate, out-of-scope, hidden, or unbounded candidate score
- **THEN** the service rejects the invalid contribution and never allows it to affect ranking or visibility

### Requirement: Rerank diagnostics are bounded and redacted
The service MUST expose rerank strategy/version, feature version, provider availability category, candidate-count bucket, fallback reason, and aggregate impact only through authorized evaluation or admin diagnostics.

#### Scenario: Ordinary retrieval response
- **WHEN** a caller performs ordinary search or context assembly
- **THEN** the response omits raw features, model scores, provider payloads, endpoint information, credentials, query text, and hidden candidate identifiers

#### Scenario: Authorized evaluation response
- **WHEN** an authorized evaluator requests rerank diagnostics
- **THEN** the report includes bounded strategy identities, changed-rank counts, protected-metric deltas, and redacted fallback categories without sensitive payloads

### Requirement: Rerank configuration is externalized
The service SHALL load reranker endpoint, model, timeout, candidate/token bounds, and credentials from environment variables, Docker secrets, or explicitly ignored local configuration and SHALL provide safe disabled defaults.

#### Scenario: Repository is checked for committed secrets
- **WHEN** configuration examples or tests are committed
- **THEN** they contain placeholders only and no real endpoint, API key, DSN, or evaluation dataset content

#### Scenario: Runtime has no reranker configuration
- **WHEN** no reranker provider is configured
- **THEN** all runtime modes start with reranking disabled and the degraded state is diagnosable
