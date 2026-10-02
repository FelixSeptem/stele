## ADDED Requirements

### Requirement: Maintenance run history has bounded retention semantics

The service SHALL retain terminal run summaries, attempt counts, duplicate and
recovery dispositions, checkpoint/watermark identities, and cleanup outcomes
for a configured bounded history window. Retention MUST remove only derived
attempt/detail records that are outside policy while preserving the latest
terminal summary and required audit transitions.

#### Scenario: Old attempt detail expires
- **WHEN** an attempt detail record exceeds its configured retention window
- **THEN** cleanup removes only the eligible derived detail, records a bounded
  deletion outcome, and retains the terminal run summary

#### Scenario: Cleanup repeats after restart
- **WHEN** the same run-history retention window is processed more than once
- **THEN** subsequent cleanup is a no-op or duplicate disposition and leaves
  surviving summaries unchanged

### Requirement: Maintenance run history is exact-scope and paginated

Authorized inspection SHALL resolve run history within one exact
tenant/project/namespace scope and SHALL provide bounded pagination and stable
filters for job class, terminal state, recovery disposition, cadence window,
and observed time. Missing or unauthorized scope proof MUST fail before any
run existence or count is disclosed.

#### Scenario: Operator pages run history
- **WHEN** an authorized operator requests a bounded page for one exact scope
- **THEN** the service returns stable ordering, an opaque continuation cursor,
  bounded run summaries, and no records from another scope

#### Scenario: Operator requests a foreign scope
- **WHEN** the caller lacks a grant for the requested tenant, project, or
  namespace
- **THEN** the service rejects the request without revealing run existence,
  counts, identifiers, or failure details

### Requirement: Recovery evidence remains freshness-aware

Each retained terminal summary SHALL expose bounded checkpoint/source-watermark
identity, freshness category, retry/recovery result, and SLO bucket. Missing,
stale, divergent, foreign, or lifecycle-hidden evidence MUST make the summary
non-eligible for conformance claims until a new run validates it.

#### Scenario: Recovery summary is fresh
- **WHEN** a terminal run has a matching source watermark and valid checkpoint
  recovery within the configured freshness window
- **THEN** the summary records a fresh recovery/SLO outcome suitable for
  authorized conformance inspection

#### Scenario: Recovery summary is stale
- **WHEN** a retained summary is outside its freshness window or its watermark
  diverges from the source
- **THEN** the service marks it stale/divergent and excludes it from readiness
  or conformance success
