## ADDED Requirements

### Requirement: Activation consumes an exact evidence handoff

The release gate SHALL accept activation eligibility only when the submitted
evidence handoff matches the exact authorized scope identity, stable run
identity, source watermark/freshness window, compatible fixture and policy
versions, integrity summary, and rollback verdict. A quality-positive report
without a complete matching handoff MUST remain non-pass.

#### Scenario: Complete handoff is reviewed
- **WHEN** an authorized operator submits a completed owned evidence bundle
  whose identities and freshness all match the candidate
- **THEN** the gate exposes a bounded activation-eligibility result while
  leaving the separate activation authorization step unchanged

#### Scenario: Handoff is incomplete
- **WHEN** the run, watermark, integrity summary, rollback proof, or compatible
  policy identity is missing, stale, or mismatched
- **THEN** the gate returns a stable non-pass category and does not authorize
  activation regardless of aggregate quality gain

### Requirement: Disablement and rollback close the release evidence loop

The release gate SHALL record activation disablement and rollback outcomes as
redacted, append-only operational evidence linked to the affected run and
policy identities. A disablement or rollback MUST restore the previously
approved strategy and MUST NOT rewrite canonical memory, source records, or
the public default retrieval contract.

#### Scenario: Activation is disabled
- **WHEN** an operator disables a candidate because evidence expires, becomes
  incompatible, or fails a safety check
- **THEN** the gate records a bounded disablement outcome and subsequent
  retrieval uses the previously approved strategy

#### Scenario: Rollback completes
- **WHEN** an authorized rollback is requested for an activated candidate
- **THEN** the gate records a redacted rollback verdict tied to the evidence
  identity, confirms the approved baseline is restored, and leaves canonical
  memory and source records unchanged

#### Scenario: Rollback proof is unavailable
- **WHEN** rollback cannot be verified within the configured operational bound
- **THEN** the gate records a non-pass rollback category and keeps the
  candidate disabled or otherwise ineligible for activation
