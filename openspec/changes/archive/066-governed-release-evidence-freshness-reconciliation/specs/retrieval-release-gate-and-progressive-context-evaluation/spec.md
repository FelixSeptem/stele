## ADDED Requirements

### Requirement: Release activation consumes current reconciliation eligibility

The release gate MUST consult the current exact-scope reconciliation eligibility before allowing or continuing governed activation. A stale, mismatched, missing, or revoked verdict MUST fail closed even when an older evidence handoff passed at submission time.

#### Scenario: Activation has current eligible evidence

- **WHEN** the release gate evaluates a candidate with a current eligible reconciliation verdict and all other protected gates pass
- **THEN** activation may proceed under the existing policy and scope controls

#### Scenario: Activation eligibility is revoked

- **WHEN** reconciliation has revoked the candidate's current eligibility for freshness, watermark, policy, or rollback reasons
- **THEN** the release gate denies or disables activation and returns only a bounded reason category

### Requirement: Reconciliation cannot alter ordinary retrieval

Reconciliation and eligibility changes MUST affect governed activation decisions only. They MUST NOT change default retrieval, context assembly, lifecycle visibility, or canonical memory behavior for requests that are not using an authorized active release policy.

#### Scenario: Evidence becomes stale while policy is inactive

- **WHEN** reconciliation revokes an inactive or shadow-only policy's eligibility
- **THEN** ordinary retrieval and context assembly produce the same result as before the reconciliation

