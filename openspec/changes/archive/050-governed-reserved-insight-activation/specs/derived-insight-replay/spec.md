## MODIFIED Requirements

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
