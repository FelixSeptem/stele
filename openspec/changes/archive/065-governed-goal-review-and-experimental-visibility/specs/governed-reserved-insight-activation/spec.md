## ADDED Requirements

### Requirement: Reserved goal activation precedes experimental visibility

The reserved-insight activation boundary MUST evaluate goal visibility only after scope, lifecycle, principal grant, policy, replay, evidence, freshness, and review precedence checks succeed. A provider, replay operation, or activation result MUST NOT bypass the visibility policy or directly emit `goal_context`.

#### Scenario: Precedence checks pass

- **WHEN** a governed goal has a compatible activation policy and all visibility gates pass
- **THEN** the service records an eligible handoff for the separately authorized experimental section

#### Scenario: Provider attempts direct visibility

- **WHEN** provider output requests goal activation or context inclusion without a valid visibility-policy evaluation
- **THEN** the service rejects or quarantines the request and records a bounded precedence failure
