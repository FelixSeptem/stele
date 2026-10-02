## ADDED Requirements

### Requirement: Scheduler run history is admin-inspectable

The admin surface SHALL expose exact-scope, paginated inspection for scheduler
run summaries, attempt dispositions, lease/recovery state, retry exhaustion,
duplicate fires, terminal status, checkpoint/freshness categories, and cleanup
outcomes. Responses MUST be bounded and MUST not expose raw error payloads,
credentials, query text, source content, or unrelated scope data.

#### Scenario: Administrator lists scheduler runs
- **WHEN** an authorized administrator lists runs for an exact scope with
  bounded state, job-class, or time filters
- **THEN** the response contains stable run identity, status, attempt count,
  lease/recovery category, freshness/SLO bucket, and continuation metadata

#### Scenario: Administrator reads run attempts
- **WHEN** an authorized administrator reads one run within an authorized scope
- **THEN** the response includes bounded attempt and terminal disposition history,
  checkpoint/watermark identity, retry outcome, and cleanup state

#### Scenario: Administrator requests out-of-scope run history
- **WHEN** an administrator requests a run or page outside an authorized exact
  scope
- **THEN** the service rejects the request without exposing run existence,
  counts, identifiers, or failure details
